# libcat

## Local servers -- standard testing config

Use these fixed ports and credentials; do not invent new ones.

| Instance | Port | URL | Login |
|---|---|---|---|
| Demo playground (persistent store) | 8481 | http://localhost:8481/ | eve@example.org / changeme123 |
| Throwaway verify instance (fresh store) | 8491 | http://localhost:8491/ | eve@example.org / changeme123 |

Auth flow for both: `POST /v1/auth/login` with
`{"email":"eve@example.org","password":"changeme123"}` returns
`accessToken`; send it as `Authorization: Bearer <token>`. The bootstrap
admin holds the admin role (librarian-gated routes included).

### Demo playground (port 8481)

`~/libcat-playground/run.sh` execs `~/lcatd-play` with the blob dir
`~/libcat-playground/site` -- the store is persistent, so copycat
targets, the seed marker, and edits survive restarts. Restart recipe
after each completed task (post-commit):

1. `cd backend/ui && npm run build`
2. `cd backend && go build -o ~/lcatd-play ./cmd/lcatd`
3. `cd .. && go build -o ~/libcat-playground/lcat ./cmd/lcat`
   (`cmd/lcat` is at the repo root, NOT under backend/)
4. `git checkout backend/ui/dist` (the dist in git is a placeholder;
   the real build is embedded into the binary in step 2)
5. `pkill -f lcatd-play`, then run `~/libcat-playground/run.sh` in
   the background
6. The server loads vocabularies for a few seconds before listening --
   poll `curl -s localhost:8481/` for 200 rather than assuming 2s is
   enough
7. `~/libcat-playground/opac/refresh.sh` -- the playground OPAC
   (:8482) is a COPIED projection, not a live read, so it drifts
   until this reruns (serialize -> project -> hugo; the :8482 serve
   process reads files per request and needs no restart)

### Throwaway verify instance (port 8491)

For end-to-end verification against a fresh store (e.g. seeding logic),
build and run from a scratch dir:

```sh
go build -o <scratch>/lcatd-verify ./cmd/lcatd   # from backend/
LCATD_LISTEN_ADDR=:8491 LCATD_BLOB_DIR=<scratch>/blob \
  LCATD_LOCAL_AUTH=1 \
  LCATD_BOOTSTRAP_ADMIN=eve@example.org:changeme123 \
  LCATD_ABUSE_SECRET=verify-0123456789abcdef01234567 \
  <scratch>/lcatd-verify
```

A fresh blob dir seeds the default copycat SRU targets at boot; kill the
process with `pkill -f lcatd-verify` when done.

## Releasing -- pick the version slot deliberately

`./scripts/release.sh vX.Y.Z` tags root, `hugo/` and `backend/` in lockstep.
Full policy in [docs/versioning.md](docs/versioning.md); the short form:

- **Patch** (`v0.114.0` -> `v0.114.1`) -- the release only makes wrong
  behavior right. The adoption note says "rebuild and restart", nothing more.
- **Minor** (`v0.114.0` -> `v0.115.0`) -- the consumer has something to do:
  something additive to adopt, or something breaking to fix. Highest wins.

The test is *what does the adoption note say?* If it says "and also...", it is
a minor. **Do not reach for a minor by reflex** -- most bug fixes are patches,
and an inflated minor stops carrying information. Patch releases are ordinary
here (`v0.4.1`, `v0.7.2`, `v0.100.1`, `v0.103.1`).

### The backend tag is built by CI (since v0.229.4)

`release.sh` tags root and `hugo/` itself but does **not** tag `backend/`
locally. It pushes the `go.mod`-bump commit, then `gh workflow run
release-backend.yml`, then **polls origin up to 5m** for the CI-created
`backend/v<V>` tag before reporting success. The workflow `npm ci && npm run
build`s `backend/ui`, commits the real `ui/dist` on top of the release commit,
tags THAT, and pushes only the tag -- so `main` keeps its committed placeholder
but `go install .../backend/cmd/lcatd@<tag>` ships the real SPA. Rationale (Go
tags are immutable once proxied; `go install` can't run npm) is in
[docs/versioning.md](docs/versioning.md).

Operational notes when cutting a release:
- `release.sh` now **requires `gh`** (workflow scope) and blocks ~1-2m on the CI
  poll -- that is normal, not a hang.
- root/hugo are tagged before the CI dispatch, so a CI failure leaves them tagged
  with no backend tag; check `gh run list --workflow=release-backend.yml`.
- **Never hand-tag `backend/`** to "unstick" a release -- a local tag embeds the
  placeholder SPA.
- Wrap `release.sh` in the shared-machine resource lock (it runs the full Go
  suites + the hugo build before dispatching).
