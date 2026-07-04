# 095 -- Optional persistent stores for lcatd (DynamoDB / S3 selection)

## Context

Everything the backend kept in the document store (`store.Store`) was in-memory
and died on restart: the audit trail (and the editing-stats rollup that reads
it, `093`), the review queue/suggestions, folk terms, promotions, copycat
batches/targets, drafts, the seed marker, rate-limit counters. `cmd/lcatd`
hardcoded `store.NewMem()`; the comment said the DynamoDB selection would
"arrive with the deployment task." What survived a restart did so through the
blob store (grains, vocab snapshots), not the KV.

The persistent backends already existed and were conformance-tested
(`store/dynamo`, `blobs3`) and the terraform table was provisioned
(`deploy/terraform/dynamodb.tf`: pk/sk, TTL on `expireAt`, PITR). This task wired
them into `lcatd` as an **opt-in** choice, keeping the laptop/demo path on the
in-memory + local-directory stores.

## What shipped

- Config knobs `LCATD_DYNAMO_TABLE`, `LCATD_S3_BUCKET`, `LCATD_AWS_ENDPOINT`
  (`config` stays SDK-free). New `backend/awsstore` builds the AWS clients
  (added the `aws-sdk-go-v2/config` credential loader). `buildDeps` selects
  DynamoDB / S3 when configured, else `store.NewMem()` / `blob.NewDir` -- so a
  laptop and the demo run with no AWS at all, logging the backend choice.
- **Bug found and fixed via the e2e run:** `dynamo.Put` marshalled `Data` as a
  Binary attribute, which DynamoDB rejects when empty, so a data-less
  index/existence marker (the `auth/local` user index) failed and boot died at
  "bootstrap admin". `dynamo.Put` now REMOVEs the data attribute for empty data
  (round-trips as nil), and the conformance suite gained an `EmptyData` case the
  mem and dynamo stores both pass. Invisible before because the dynamo
  conformance test `t.Skip`s without `DYNAMO_ENDPOINT`. (Committed separately as
  `fix(store)`.)

## Verified

- Demo/local fallback unchanged: the playground boots on memory + dir and logs
  the backend choice; existing tests pass unchanged.
- Against DynamoDB Local: the full server boots, bootstrap admin succeeds, and a
  written KV record (a copycat target) **survived a restart** -- the reset
  problem this task exists to fix. The `store/dynamo` conformance suite passes
  against dynamo-local.

## Remainder -> 097

The Lambda entrypoint (`cmd/lcatd-lambda` still serves empty deps), the shared
`buildDeps` factoring around its container-only worker goroutines, the deploy
glue (terraform env + IAM), CI wiring for the dynamo conformance test, seed
idempotency under a shared table, and the TTL-parity check moved to
`097_lambda-deploy-persistence.md`.
