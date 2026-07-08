# 162 -- workindex-snapshot: dir-built seeds can't prime an S3 store (ETag scheme mismatch)

Filed from the queerbooks-demo session (2026-07-07) while adopting 155/156;
renumbered from 161 (collision with the committed release task).

## Problem

`lcatd workindex-snapshot --blob-dir <dir>` (backend/cmd/lcatd/snapshot.go)
builds the snapshot through `blob.NewDir`, whose ETags are **sha256 of
content** (storage/blob/dir.go). An S3 store's ETags are **MD5-based**. So the
documented Lambda seed flow -- build from a local grain mirror, `aws s3 cp` the
snapshot next to the grains -- produces a snapshot in which *every* entry
fails the ETag-diff on the first refresh against S3. The refresh re-GETs all
48k grains; on Lambda that read blocks past the 30s window and never completes
across frozen invocations, so `/v1/works` still 503s permanently. The "stale
snapshot degrades to a small catch-up" guarantee silently doesn't hold across
store backends: identical bytes, different fingerprint scheme.

queerbooks hit this for real (their tasks/014). Workaround used there: run the
full `lcatd` off-Lambda with `LCATD_S3_BUCKET=<bucket>` and let the boot
goroutine warm-scan + `Save` back into the bucket -- correct ETags, but ~1h of
sequential GETs over a home uplink and no progress signal (reads block behind
the refresh lock, so only network counters show it's alive).

## Suggested fixes (either suffices; first is more useful)

1. Teach the seed tool to talk to the target store directly:
   `lcatd workindex-snapshot --s3-bucket <bucket>` (reuse awsstore.S3 wiring
   from appdeps). Snapshot then carries native S3 ETags and writes straight to
   the bucket. Concurrent GETs would also fix the ~1h sequential scan.
2. And/or make the mismatch loud: log at WARN when >N% of snapshot entries
   fail the ETag diff on first refresh ("snapshot ETag scheme may not match
   this store") so the misuse is diagnosable instead of a silent full rescan.

Docs: the deploy README seed recipe should say the snapshot must be built
against the same store backend it will serve (or use fix 1).

## Outcome (2026-07-07)

Both fixes shipped:

1. `lcatd workindex-snapshot (--blob-dir <dir> | --s3-bucket <bucket>)
   [--aws-endpoint <url>] [--concurrency 16]` -- the S3 path reuses
   `awsstore.S3`, so the snapshot carries the bucket's native ETags and writes
   straight back into it. The scan runs through the new
   `workindex.Index.WarmScan` (concurrent GETs + progress reporting, default
   16 workers), fixing the ~1h sequential-scan pain in the same stroke. The
   tool also loads any existing snapshot first, so a re-seed only fetches the
   ETag delta.
2. Drift detection: `loadSnapshotLocked` arms a counter; the first reconcile
   (refreshLocked or WarmScan) records how many primed entries its ETag diff
   re-fetched anyway, exposed as `Index.SnapshotDrift()`. appdeps' boot warmer
   logs WARN when >=50% of primed entries re-fetched ("snapshot likely built
   against a different store backend"); the seed tool prints the same note.

Covered by `TestSnapshotDriftForeignETagScheme` (foreign vs native ETag
scheme) and `TestWarmScan` (skip/changed/new/deleted semantics + zero-Get
primed boot). The deploy-README recipe note belongs to the queerbooks repo;
their next seed run should just switch to `--s3-bucket`.
