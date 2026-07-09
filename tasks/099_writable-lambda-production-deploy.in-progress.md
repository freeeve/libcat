# 099 -- Writable-production Lambda deploy (Dynamo/S3 + worker model + CI)

Split out of `097_lambda-deploy-persistence.done.md`. 097 delivered the Lambda
*entrypoint* (shared `appdeps`, `cmd/lcatd-lambda` wired) and the read-only demo
shape (Function URL, in-memory store + bundled grains, `$0`). This task is the
rest: running a **writable** production Lambda backed by the persistent stores.

## Context

The read-only demo skips persistence and skips the background workers (they have
nothing to drain). A writable deployment can't:

- **Worker model.** `appdeps.Build` only spawns the vocab-download and
  export-job ticker drains when `!ReadOnly`; a writable Lambda would either spawn
  tickers that freeze between invocations or never drain its queues. Needs a
  Lambda-appropriate model: a scheduled drain (EventBridge rule -> the same
  function in a "drain" mode, or a separate worker Lambda), or the existing
  EventBridge/SQS `rebuild_events` plumbing extended to cover vocab/export jobs.
- **Persistence wiring.** Point the writable Lambda at DynamoDB + S3 via
  `LCATD_DYNAMO_TABLE` / `LCATD_S3_BUCKET` (already supported by `appdeps` +
  `awsstore`); the terraform stack (`deploy/terraform/`: `dynamodb.tf`, `s3.tf`,
  `lambda.tf`, `apigw.tf`, `ssm.tf`, `events.tf`) already provisions the table,
  bucket, IAM, and API. Verify the env map + IAM line up with the current config
  knobs and that a real apply serves a writable instance.
- **Seed idempotency under a shared table.** `SeedDefaultTargets` runs every cold
  start; confirm `CondIfAbsent` makes concurrent cold starts a no-op / race-free
  against one Dynamo table.
- **TTL parity.** DynamoDB deletes expired items lazily (documented at
  `store/dynamo/dynamo.go`) with no read-time filter, whereas `store.NewMem` is
  strict. Confirm the `expireAt` callers (abuse rate-limiter via `Increment`,
  suggestion supporter TTLs) tolerate reading an expired record, or add a
  read-side filter; record the decision.
- **CI.** Run the `store/dynamo` conformance suite against DynamoDB Local (it
  `t.Skip`s unless `DYNAMO_ENDPOINT` is set). Locally confirmed green with a
  `pk`/`sk` + `expireAt`-TTL table.

## Non-goals

- The read-only demo (done: `097`, `backend/deploy/README.md`).
- No new store/blob backend code (dynamo + s3 exist and pass conformance).

## Acceptance

- A writable `lcatd-lambda` backed by DynamoDB + S3 through the terraform stack,
  with queued vocab/export work drained on a schedule (no frozen tickers).
- Seed idempotency and TTL-parity decisions recorded.
- CI exercises the dynamo conformance suite against DynamoDB Local.

## Status (2026-07-09): code-side complete in bc699ad / v0.57.0; apply is Eve's call

Everything buildable without touching the AWS account is done:

- **Worker model**: cmd/lcatd-lambda accepts the EventBridge drain
  event {"lcatd":"drain"} beside HTTP payloads and runs both queued
  drains once per firing; tickers are disabled on Lambda
  (config.DisableTickers, set programmatically). Terraform grows
  `drain_schedule` (rule + target + permission, empty = disabled;
  `terraform validate` green).
- **Seed idempotency**: the task said "confirm CondIfAbsent" -- it was
  actually CondNone. Fixed to create-only with a losers-no-op path and
  an 8-way concurrent race test.
- **TTL parity**: decision recorded (deploy README + store contract):
  lazy expiry is a floor on invisibility; windowed rate keys carry
  exact semantics, the supporter dedup marker fails conservative
  (re-support inside the reap lag reads duplicate, never
  double-counts). No read-side filter needed.
- **CI**: first workflow (.github/workflows/dynamo-conformance.yml)
  runs the store/dynamo conformance suite against DynamoDB Local on
  store changes; the suite self-provisions its table. (Local docker
  daemon wasn't running this session; the suite was previously
  confirmed green locally per this task's own note, and the workflow
  will prove it in CI on push.)
- README's new "Writable production on Lambda" section ties it
  together with the tasks/154 snapshot seeding.

**Remaining -- needs Eve**: a real `terraform apply` (account, billing,
domain) and the acceptance smoke test against the deployed writable
instance. The queerbooks adoption path is their tasks/041 (snapshot)
plus this drain_schedule once applied.
