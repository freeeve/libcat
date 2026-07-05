# 102 -- ingest: RunEnrich drops enrichment for all-but-one Work in a shared grain

> Filed from the 2026-07-05 full-code review.

## Symptom

After an editorial merge leaves two Works in one grain, running an enricher
persists controlled subjects for only one of them; the other Work's enrichment
subjects vanish on every run.

## Cause

`ScanSummaries` maps every Work to its grain path (ingest/enrich.go:207), so
co-grained Works share a `grainPath`. `RunEnrich` loops over per-Work results and
calls `replaceGrainGraph(grainPath, graph, enrichmentQuads(res))` once per Work
(enrich.go:111-121). `bibframe.ReplaceGraph` (bibframe/editorial.go:138-146)
drops every quad in the named graph and inserts only the passed quads -- so
processing Work B wipes the `enrichment:<name>` graph that Work A's pass just
wrote, including A's `bf:subject` links. The existing test
(`TestRunEnrichDirect`) only covers single-Work grains and cannot catch this.

## Fix sketch

Group enrichment results by grain path and apply a single `ReplaceGraph` per
grain with the union of its Works' quads.

## Acceptance

- A grain containing two Works, both returned by the enricher, retains both
  Works' subject links and terms after `RunEnrich`, idempotently across runs.
- A multi-Work-grain test covers it.
