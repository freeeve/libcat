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

Not started -- the scans are unchanged. Related context from the same review
round: tasks/101 (done) made `WorkKey` return `""` for title-less records, so
`findDuplicate` and the duplicates handler no longer see the shared
`"\x1f\x1f"` pseudo-key as a match; the O(corpus) cost itself remains. The
shared index proposed here is also the intended fix vehicle for tasks/107 and
part of tasks/109 -- design them together.
