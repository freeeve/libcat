# 106 -- httpapi: interactive requests run full-corpus scans (dup check, barcodes, dup/withdrawn lists)

> Filed from the 2026-07-05 full-code review. Highest-impact performance finding:
> the editor hot path is O(corpus) per request.

## Symptom

At catalog scale (millions of works), a single record edit -- including every
dry-run preview the SPA fires while a cataloger works -- reads and parses the
entire `data/works/` tree.

## Cause

- `POST /v1/works/{id}/ops` calls `findDuplicate` on both the dry-run branch
  (httpapi/records_handlers.go:192) and again after a successful write (:225).
  `findDuplicate` (httpapi/duplicate.go:50-80) iterates `bs.List("data/works/")`
  and does `bs.Get` + `identity.ScanGrain` for every grain -- no cache, no index,
  unlike `worksList.summaries` (30s cache).
- `POST /v1/works/{id}/items/bulk` calls `allBarcodes` (httpapi/items_bulk.go:80,
  121-145): full list + `rdf.ParseNQuads` of every grain to enforce global
  barcode uniqueness -- and the scan runs before the `dryRun` check at :93.
- `GET /v1/duplicates` does two independent full walks per request
  (`bibframe.LoadPriorStore` at maintenance_handlers.go:234 plus
  `ingest.ScanSummaries` at :240); `GET /v1/withdrawn` (:161) also scans
  uncached instead of using the shared `worksList` cache.

## Fix sketch

Introduce a maintained in-memory (or blob-persisted) identity index -- provider
keys, cluster keys, and barcodes per work -- built once and updated on write,
or at minimum route these paths through the existing `worksList`-style cache.
The duplicates/withdrawn handlers should reuse one scan and cache it.

## Acceptance

- A record-op dry run and execute perform O(1)-ish blob reads (the target grain
  plus index lookups), verified against a seeded store with a work count high
  enough to show the difference.
- Bulk item dry runs do no corpus scan.
- Repeated `GET /v1/duplicates` / `/v1/withdrawn` within the cache window do not
  re-walk the store.

## Status (2026-07-05 session)

Done. Implemented `backend/workindex.Index` -- a shared in-memory index over
`data/works/` holding provider keys, cluster keys, barcodes, and WorkSummaries
per grain. Freshness is two-layered: reads refresh by ETag diff on a 30s TTL
(one List per window; only changed grains are re-fetched/re-scanned), and every
httpapi grain write (mutateWorkGrain, PUT works, ops execute, MARC save) pushes
its bytes in synchronously via `Apply`, so sessions read their own writes.
`appdeps` builds the index and warms it in a background goroutine at boot;
`Deps.WorkIndex` lets a deployment inject one to share with copycat/workers
(tasks/107/109).

Routed through it: `findDuplicate` (dry-run + execute), bulk-item barcode
collision checks (`allBarcodes` deleted), `GET /v1/duplicates` (LoadPriorStore
+ second scan both gone), `GET /v1/withdrawn`, and the works list/tags cache
(`worksList` now delegates; its private 30s cache removed). `identity` gained
`ScanDataset` and `ingest` gained `SummarizeDataset` so the index scans
identity, summaries, and barcodes off one parse per grain.

Acceptance verified against a 5000-work seeded store on the throwaway 8491
instance: ops dry-run/execute and bulk-item dry runs are sub-millisecond after
warm-up (were full corpus read+parse per request); repeated
duplicates/withdrawn reads do no I/O within the TTL (unit-asserted with a
counting store in workindex_test.go); bulk-add continuation reads its own
writes (SEED-0003/0004 written -> next preview 0005). Derived lookup maps are
rebuilt lazily O(corpus-in-memory) after a change; per-key incremental
maintenance is the next step if profiling ever shows it on the save path.
Non-httpapi writers (copycat commit, publish, enrich) stay visible within one
TTL until tasks/107/109 route them through the shared index. The
refresh-window List still pays DirStore.List's read-every-file ETag cost
(~150ms at 5k works) -- that fix belongs to tasks/109.
