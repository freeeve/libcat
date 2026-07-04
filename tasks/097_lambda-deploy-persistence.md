# 097 -- Lambda entrypoint + deploy glue for the persistent stores

Split out of `095_wire-persistent-stores.done.md`, which delivered the `lcatd`
store selection (config-gated DynamoDB/S3 with mem/local fallback) and fixed the
`dynamo.Put` empty-data bug. This task carries the rest: making the Lambda
entrypoint actually use those stores, plus the deployment plumbing.

## Context

`cmd/lcatd-lambda/main.go` still constructs an empty `httpapi.Deps{Logger:...}`,
so the Lambda serves no persistent state. The obvious move -- share
`buildDeps` between `cmd/lcatd` and `cmd/lcatd-lambda` -- is complicated by
`buildDeps`'s container-worker goroutines: the vocab-download drain
(`main.go` ~140) and export-job drain (~314) run tickers on `ctx` that suit a
long-lived container but not Lambda's freeze-between-invocations model. So this
needs a worker-model decision, not a straight extraction.

## Scope

- **Factor a shared dependency builder** both entrypoints can call, with the
  container-only workers gated off (or moved to a separate container/worker
  target) so the Lambda build doesn't spawn tickers it can't run. The store/blob
  selection already lives in `buildDeps` via `awsstore` -- reuse it verbatim.
- **Wire `cmd/lcatd-lambda`** to the shared builder so the Lambda serves the
  full surface backed by DynamoDB + S3.
- **Deploy glue.** Flow the terraform `sidecar` table name and the S3 bucket
  into the Lambda environment (`LCATD_DYNAMO_TABLE`, `LCATD_S3_BUCKET`); confirm
  the execution-role IAM grants table read/write and bucket access (TTL/PITR are
  already set in `deploy/terraform/dynamodb.tf`).
- **CI.** Run the `store/dynamo` conformance test against DynamoDB Local in CI
  (it `t.Skip`s unless `DYNAMO_ENDPOINT` is set). Locally confirmed green:
  `DYNAMO_ENDPOINT=http://localhost:8000 ... go test ./store/dynamo/...` with a
  `pk`/`sk` + `expireAt`-TTL table.
- **Seed idempotency under a shared table.** `SeedDefaultTargets` runs every
  boot; confirm it is `CondIfAbsent` and safe when several Lambda cold starts
  seed the same table concurrently (harmless against mem, must be a no-op / race
  free against Dynamo).
- **TTL parity.** DynamoDB deletes expired items lazily (documented at
  `store/dynamo/dynamo.go` header) with no read-time expired filter, whereas
  `store.NewMem` is strict. Confirm the `expireAt` callers (abuse rate-limiter
  via `Increment`, suggestion supporter TTLs) tolerate reading an expired
  record, or add a read-side filter. Note the decision here.

## Non-goals

- No new store/blob backend code (dynamo + s3 exist and pass conformance).
- No change to the local/demo path -- `lcatd` on mem + local dir must keep
  working with no AWS (already true after 095).

## Acceptance

- `lcatd-lambda` serves the full API backed by DynamoDB + S3, with no
  container-only workers spawned in the Lambda process.
- Terraform passes the table + bucket into the Lambda env; IAM verified.
- CI runs the dynamo conformance suite against DynamoDB Local.
- Seed idempotency and TTL-parity decisions recorded.
