# 109 -- batch/publish/enrich/authoritiesvc: whole-corpus summary loads, and MaxWorks checked after the scan

> Filed from the 2026-07-05 full-code review. Sibling of
> [[106_httpapi-per-request-corpus-scans]] (worker/admin paths rather than the
> editor hot path).

## Symptom

Admin and worker operations materialize every work summary in memory per
invocation, and batch's own size guard only rejects after the full scan has
already been paid.

## Cause

- `batch.scan` (backend/batch/batch.go:156-169) calls `ingest.ScanSummaries`
  over `data/works/`; `Run` (batch.go:189-195) checks `len(targets) > maxWorks`
  only after Resolve has materialized all summaries, so a `KindAll` request
  scans and holds the entire corpus just to be rejected.
- The same whole-corpus `ScanSummaries` runs in `publish.PromoteTag`
  (publish/promote.go:38), `authoritiesvc.Merge` (authoritiesvc/service.go:182),
  and `enrich.runQueued` (backend/enrich/enrich.go:85).
- Compounding at the storage layer: `DirStore.List`
  (storage/blob/dir.go:99-135) reads the full content of every matching file
  just to compute ETag/Size, and `LoadPriorStore` then Gets each grain again --
  every scan reads every grain from disk twice. (Also: List walks the entire
  root tree regardless of prefix, and a file deleted mid-walk silently truncates
  the listing because the `fs.ErrNotExist` from `os.ReadFile` is swallowed at
  dir.go:124.)

## Fix sketch

- Enforce `MaxWorks` during/before the scan (stream summaries with an early
  abort), not after.
- Share one summaries source (the `worksList` cache or the [[106]] index)
  across these paths instead of five independent scanners.
- DirStore.List: stat instead of read (mtime+size ETag, or lazy ETag), scope the
  walk to the prefix subtree, and surface (or safely skip-and-continue) races
  with concurrent deletes instead of silently truncating.

## Acceptance

- An over-limit batch request is rejected without loading the corpus.
- A grain scan reads each file at most once (verified by I/O counting against
  DirStore).
- Listing under a deep prefix does not walk unrelated subtrees.

## Status (2026-07-05 session)

Not started. Note `batch.Run` changed nearby in tasks/111 (audit now rides on
`result.Applied > 0`) -- the MaxWorks-after-scan ordering this task fixes is
unchanged. The shared summaries source now exists: `backend/workindex.Index`
(tasks/106, done) exposes `Summaries(ctx)` (same shape and order as
`ingest.ScanSummaries`), refreshes by ETag diff, and is injectable via
`httpapi.Deps.WorkIndex` / constructed in appdeps -- batch, publish,
authoritiesvc, and enrich can take it instead of five independent scanners
(and should `Apply` their writes). The DirStore.List fixes here also directly
cut the index's refresh cost: it Lists once per 30s window, and each List
currently reads every file's content just to compute the ETag.
