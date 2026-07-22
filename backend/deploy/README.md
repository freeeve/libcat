# Deploying the Tier 2 backend

The backend is one `net/http` handler with two wrappers, so every deployment
shape serves identical routes. **The static tier needs none of this** -- the
graph is the contract, and a catalog built with `lcat` alone keeps working.

## Prebuilt release artifacts (skip the build)

Every backend release publishes ready-to-deploy artifacts on its GitHub release
(`backend/v<X.Y.Z>`), each with the admin SPA already embedded -- so a consumer
neither `go build`s nor stage-zips, they download and deploy:

| Artifact | Shape it feeds |
|---|---|
| `lcatd-lambda-arm64.zip` | the terraform `lambda_zip` (provided.al2023, arm64) |
| `lcatd-linux-amd64.tar.gz` / `lcatd-linux-arm64.tar.gz` | self-host / container (`lcatd` + `lcat` binaries) |
| `ghcr.io/freeeve/libcat:v<X.Y.Z>` | container hosts (Cloud Run / Fargate / k8s) |

`SHA256SUMS` accompanies the downloads. The from-source commands below stay
valid for local builds and unreleased commits; each notes its published
equivalent. (`go install .../backend/cmd/lcatd@backend/v<V>` also ships the real
SPA, but a `go install` still compiles -- the artifacts above do not.)

## Shapes

| Shape | Entry point | Datastore | Grain store |
|---|---|---|---|
| Laptop / dev | `go run ./cmd/lcatd` | in-memory | local dir (`LCATD_BLOB_DIR`) |
| Container (Cloud Run / Fargate / k8s / self-host) | `deploy/docker/Dockerfile` | DynamoDB (or DynamoDB-local) | S3 / R2 / MinIO |
| AWS serverless | `deploy/terraform/` (API GW v2 + Lambda) | DynamoDB | S3 |

## Local, no cloud at all

```sh
cd backend
LCATD_LISTEN_ADDR=:8471 \
LCATD_LOCAL_AUTH=1 \
LCATD_BOOTSTRAP_ADMIN=admin@example.org:changeme123 \
LCATD_ABUSE_SECRET=$(openssl rand -hex 16) \
LCATD_BLOB_DIR=/path/to/catalog-repo \
go run ./cmd/lcatd
```

Point `LCATD_BLOB_DIR` at a tree containing `data/works/` grains (any
`lcat ingest` output). Login, suggestion, moderation, publish, record
editing, and exports all work against the local directory; published edits
land as `editorial:` quads in the grain files, ready for
`lcat serialize && lcat project`.

## Read-only demo on Lambda (Function URL, ~$0)

A public **read-only** instance (patrons/catalogers explore but nothing
persists, `LCATD_READ_ONLY=1`) needs no DynamoDB or S3: the in-memory document
store plus a **bundled read-only grain dir** is enough, and each cold start
re-creates the bootstrap admin. `cmd/lcatd-lambda` builds the same handler as
`cmd/lcatd` (shared `backend/appdeps`) and serves the embedded SPA + API through
the Lambda adapter, so a **Function URL** (no API Gateway -> no per-request
charge) keeps it inside Lambda's always-free tier.

```sh
cd backend
# Build the SPA into the binary, then the arm64 Lambda bootstrap.
(cd ui && npm ci && npm run build)
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o bootstrap ./cmd/lcatd-lambda
# Bundle the read-only grains next to the binary (LCATD_BLOB_DIR points at them).
zip -r lcatd-demo.zip bootstrap grains/
```

> Or skip the build: unzip the release's `lcatd-lambda-arm64.zip` (SPA already
> embedded) and add your `grains/` to it -- `zip -gr lcatd-lambda-arm64.zip grains/`.

Deploy the zip as a `provided.al2023` (arm64) function with a Function URL and:
`LCATD_READ_ONLY=1`, `LCATD_BLOB_DIR=/var/task/grains`, `LCATD_LOCAL_AUTH=1`,
`LCATD_BOOTSTRAP_ADMIN=demo@example.org:<pw>`, `LCATD_LOCAL_SIGNING_KEY=<key>`
(a stable key so a session survives a warm instance), `LCATD_ABUSE_SECRET=<...>`.
Background workers are skipped in read-only mode, so the freeze-between-
invocations model is fine.

**A richer demo:**

- `LCATD_SANDBOX=1` (implies read-only) lets a visitor *edit* -- the record
  editor shows Save and renders each change as if committed, wiped on refresh --
  without anything persisting.
- **Subject search works out of the box:** the built-in `lcsh` source proxies to
  `id.loc.gov` live (`/v1/vocabsuggest`), so the picker autocompletes all of LCSH
  with no local load (the Lambda just needs outbound internet).
- **Existing subjects display** their real headings if you bundle a
  corpus-sized authority snapshot: `lcat vocab-subset --catalog catalog.json
  --out lcsh.nq`, drop it under `grains/data/authorities/vocab/lcsh.nq`, and set
  `LCATD_VOCAB_SCHEMES=lcsh` (a small file -> fast cold start). Reproject with it
  loaded to fill the public catalog's labels too.

**Turnkey terraform module** (`deploy/terraform/modules/readonly-demo/`): a
consumer supplies the built zip (via the module's `build-zip.sh`) and gets the
Lambda + Function URL + a CloudFront distribution wired with the right cache
split:

- `/assets/*` (hashed, immutable) -> cached hard at the edge, so a page renders
  without waking Lambda;
- `/config` and `/v1/*` -> never cached, forwarded to the function
  (all-viewer-except-Host, so auth/cookies pass but the Function URL's Host is
  preserved);
- HTML / SPA routes -> served fresh so a redeploy shows immediately.

So the cold start is only felt on the first API call. See the module's README to
wire it up (and a custom domain via an ACM cert in us-east-1). Caveat: concurrent
cold instances have separate in-memory stores, so token *refresh* can miss across
instances -- use a generous access-token TTL. The **writable** production stack
(below) is a separate deployment (tasks/099).

## Writable production on Lambda (DynamoDB + S3)

The read-only demo above skips persistence and workers. The writable shape
(tasks/099) adds three things on top of the same `cmd/lcatd-lambda` binary:

1. **Persistence**: set `LCATD_DYNAMO_TABLE` and `LCATD_S3_BUCKET`; the
   terraform stack provisions the table (pk/sk, `expireAt` TTL), bucket, and
   IAM. TTL expiry is lazy on DynamoDB (documented in `store/dynamo`); every
   `expireAt` consumer treats it as a floor on invisibility -- the windowed
   rate-limit keys carry exact semantics and the supporter dedup marker
   fails conservative (a re-support inside the reap lag reads as duplicate,
   never double-counts).
2. **Queued-work drain**: container tickers freeze between invocations, so
   set the terraform `drain_schedule` variable (e.g. `rate(1 minute)`); the
   EventBridge rule invokes the function with `{"lcatd":"drain"}` and the
   entrypoint runs the vocab-download and export-job drains once per firing.
   Read-only demos leave it empty.
3. **Work index snapshot**: seed `data/workindex.snapshot` next to the
   grains (`lcatd workindex-snapshot --s3-bucket <bucket>`, tasks/154) so a
   cold start loads the index instead of scanning the corpus; publishes keep
   it current and the change feed gives cross-container read-your-writes.

Cold-start seeding (`SeedDefaultTargets`) is race-free against a shared
table: the seed marker is create-only, so concurrent cold starts collapse to
one seeding pass.

## Terraform (AWS reference)

```sh
cd backend
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o bootstrap ./cmd/lcatd-lambda
zip lcatd-lambda.zip bootstrap
cd deploy/terraform
terraform init
terraform apply -var grain_bucket_name=my-catalog-grains -var lambda_zip=../../lcatd-lambda.zip
```

Or point `lambda_zip` straight at the release's `lcatd-lambda-arm64.zip` (no
build): download it and `terraform apply -var lambda_zip=/path/to/lcatd-lambda-arm64.zip`.

Creates: the pk/sk DynamoDB table (TTL + PITR), the versioned grain bucket
(exports auto-expire), the API lambda + HTTP API (CORS from
`allowed_origins`, throttle backstop, no gateway authorizer -- JWTs verify
in-function), SSM secret parameters to fill in after apply, and optionally
the EventBridge bus + SQS queue that carry grains-changed rebuild events.
Non-secret configuration (issuers, role maps, provider) goes in the
`environment` variable map.

## Docker / compose

`deploy/docker/compose.yaml` boots lcatd beside MinIO and DynamoDB-local --
the self-contained integration stack. The image is distroless static
(`deploy/docker/Dockerfile`, built from the repo root). The Dockerfile builds
the Svelte SPA in a `node` stage and embeds it before the Go build, so the image
serves a working browser UI.

**Building lcatd by hand:** the SPA is embedded via `go:embed backend/ui/dist`,
and the committed `dist/` is only a placeholder. A bare `go build ./cmd/lcatd`
therefore serves an API with a "UI not built" notice at `/` (and logs a warning
at startup). To embed the real app, build it first:

```sh
cd backend/ui && npm ci && npm run build
cd .. && go build ./cmd/lcatd
```

Kubernetes notes: the API is stateless (all state lives in the document
store and grain store), `GET /v1/healthz` serves liveness/readiness, SIGTERM
drains gracefully, and the advisory ingest lease already coordinates
single-flight work across replicas -- horizontal scaling is safe. The deeper
k8s ergonomics review (probe wiring, manifests vs Helm, the no-AWS worker
path, and the hugo-watcher local preview loop) is tracked as tasks/054.

## Branding (runtime theme override)

The SPA's palette, type stacks, and corner radius are CSS custom properties
(`backend/ui/src/app.css`). `LCATD_BRAND_CSS=<path>` re-brands them at boot
without forking or rebuilding the SPA: the server reads the file once,
serves it at `/brand.css`, and links it from `index.html` after `app.css`,
so its rules win the cascade. The link is render-blocking like any head
stylesheet -- the first paint already carries the brand. An unreadable file
fails the boot loudly.

Author plain CSS, overriding the same selectors `app.css` defines --
`:root` for light and `html[data-theme="dark"]` for dark (the attribute the
in-app toggle sets):

```css
:root {
  --accent: #c42a8c;
  --info: #678cb8;
}
html[data-theme="dark"] {
  --accent: #df41a5;
  --info: #95c2e5;
}
```

Tokens worth overriding: the color palette (`bg`, `surface`, `surface-alt`,
`ink`, `ink-muted`, `rule`, `accent`, `accent-ink`, `danger`, `ok`, `info`,
the `prov-*` provenance inks, the `pend-*` pending-edit trio), the type
stacks (`font`, `display`, `mono`), and `radius`. Keep each pair AA-contrast
on its background; the defaults are. Any other CSS works too -- the file is
served verbatim.

On Lambda, bundle the file into the deployment package and point
`LCATD_BRAND_CSS` at it (e.g. `/var/task/brand.css`). A deployment hosting
`dist/` on its own CDN (API-only lcatd) appends its own stylesheet instead
-- the env var only affects the embedded index.html.

## Rebuild pipeline

Publishes and enrichments change grains; something must re-run
`lcat serialize && lcat project` (and `lcat index` / Pagefind) and redeploy
the static site. Pick one:

- **Webhook** (`LCATD_WEBHOOK_URL`/`LCATD_WEBHOOK_SECRET`): lcatd POSTs a
  signed grains-changed event to any CI endpoint (verify with
  `trigger.Verify`).
- **Local command** (`LCATD_REBUILD_CMD`, optional `LCATD_REBUILD_DIR`): run
  a shell command after each publish (changed paths in
  `$LCAT_CHANGED_PATHS`). This is the hugo-watcher dev loop: point it at
  `lcat serialize && lcat project` targeting a running `hugo server`'s data
  directory and published edits live-reload in the discovery site within
  seconds -- no cloud, no CI. Composes with the webhook (both fire).
- **EventBridge/SQS** (Terraform `rebuild_events`): a worker consumes the
  queue and rebuilds.
- **Schedule**: rebuild on cron; skip event plumbing entirely.
