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

Done.

- **Shared summaries source.** `ingest.SummarySource` (implemented by
  `workindex.Index.SummariesWithPaths`) plus the `ingest.SummariesOf` helper;
  batch.scan, publish.PromoteTag, authoritiesvc.Merge, and enrich.runQueued
  all take an optional `Summaries` field (appdeps wires `deps.WorkIndex` into
  each) and fall back to their old `ScanSummaries` walk when nil (tests,
  index-less callers). With the index wired, a `KindAll` batch request
  resolves from memory -- no per-request corpus load to reject; the
  `MaxWorks` check in `Run` stays where it was but is now O(matched) over
  in-memory summaries (it also guards explicit id-list selections, which
  never scanned).
- **DirStore.** ETags stay content-sha256, but computed tags are cached
  against each file's (mtime, size): List stats and reuses the cached tag,
  reading only files whose signature changed (Get/Put/List all maintain the
  cache), so scan+load flows read each file at most once. The walk is scoped
  to the deepest directory the prefix names, and a file deleted mid-walk is
  skipped instead of silently truncating the listing (other errors now
  surface instead of being swallowed with the old blanket ErrNotExist check).
  Pinned by dir_test.go: cache use (same-signature rewrite serves the cached
  tag), invalidation (external rewrite lists the new tag), scoping (an
  unreadable sibling subtree no longer breaks a scoped List), missing-subtree
  lists empty.

Not done here: these workers' grain writes reach the index via its 30s
ETag-diff TTL rather than a synchronous `Apply` (their rewrites don't feed
the editor's barcode/duplicate hot checks); `ingest.RunEnrich` (direct-mode
enrichment, root module) still does its own ScanSummaries walk; the
no-index fallback path still materializes summaries before the MaxWorks
rejection (a streaming ScanSummaries iterator wasn't worth it once the
deployed shape resolves from memory).
