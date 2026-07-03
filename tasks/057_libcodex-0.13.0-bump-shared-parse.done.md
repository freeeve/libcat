# 057 -- bump libcodex to v0.13.0; adopt ParseNQuadsShared

## Context (filed from libcodex, task 083 there)

libcodex v0.13.0 lands the two allocation fixes profiled out of
BenchmarkProject:

- `Graph.index` rebuilt (exact-size int32 buckets over one shared arena):
  at corpus scale the index build went from 156MB/131k allocs to
  16.2MB/388 allocs per run. This arrives with the version bump alone --
  no code change; BenchmarkProject on the 430K playground corpus already
  drops 4.87MB -> 3.06MB per projection (-37% bytes, -77% allocs).
- `ParseNQuadsShared` (and `ParseNTriplesShared`): opt-in zero-copy parse
  that backs terms with the caller's buffer instead of a private copy,
  saving one input-sized allocation (-44% of parse bytes at 51MB scale).
  Contract: the caller must not modify data while the Dataset, or anything
  derived from it, is in use.

## Work

- Bump `github.com/freeeve/libcodex` to v0.13.0.
- Switch `project.Project` and `project.Redirects` from `rdf.ParseNQuads`
  to `rdf.ParseNQuadsShared` -- both parse a buffer that is read from disk
  and never mutated, so the contract holds. Check other ParseNQuads call
  sites (ingest, cmd) the same way before switching them.
- Re-run BenchmarkProject with LCAT_BENCH_CATALOG for before/after.

Note: BenchmarkProject hardcodes provider "overdrive"; the playground
corpus (~/libcatalog-playground/site/catalog.nq) is provider "marc", so it
projects zero works and the bench fails its own guard. Worth making the
provider an env override while in there.

## Progress (2026-07-02)

- **Bench provider override delivered**: `LCAT_BENCH_PROVIDER` (default
  "overdrive") on both BenchmarkProject and BenchmarkFacets. Baseline on the
  playground corpus (430KB, provider marc) at libcodex v0.12.0:
  `8.69ms/op, 4.87MB/op, 3407 allocs/op` -- matches the task's "before".
- **Bump blocked on publish**: libcodex v0.13.0 existed only as a local
  unpushed commit until the maintainer pushed and tagged it; this repo
  consumes libcodex from published tags (tasks/053 convention, no replace).

## Delivered (2026-07-02, after the v0.13.0 tag landed)

- **go.mod bumped to v0.13.0** (root + backend). Zero test fallout -- the
  exact-size index rebuild is internal, and the loss-table gates stayed
  green.
- **`ParseNQuadsShared` adopted in `project.Project` and
  `project.Redirects`** -- the corpus-scale parses of read-only catalog.nq
  buffers. Both doc comments state the contract: the buffer must not be
  modified while the Catalog / RedirectMap is in use (projected strings
  alias it). Callers (lcat project, export emitCSV) hold the buffer for the
  projection's lifetime already.
- **Call-site audit, deliberately NOT switched**: `identity.ScanGrain`,
  `ingest.SummarizeGrain`, and `vocab.Load` build long-lived structures
  (resolver seeds, summaries, the term index) whose strings would alias --
  and therefore pin -- every per-grain buffer, trading one transient
  allocation for permanent retention. The bibframe grain mutators
  (editorial/merge/items/...) parse small per-grain buffers and immediately
  re-serialize; negligible win, left on the copying parse.
- **Benchmark, playground corpus (430KB, provider marc, M3 Max)**:
  - v0.12.0 baseline: `8.69ms/op, 4.87MB/op, 3407 allocs/op, 50.8 MB/s`
  - v0.13.0 + Shared: `1.25ms/op, 2.62MB/op,  777 allocs/op, 351.6 MB/s`
  - -86% time, -46% bytes, -77% allocs -- the index rebuild and the
    zero-copy parse combined, in line with upstream's per-change profiling.
