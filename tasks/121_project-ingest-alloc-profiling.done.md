# 121 -- Profile and reduce allocations in the project / ingest hot paths

User ask (2026-07-05): take an optimization pass over the projection and ingest
code -- profile first, then reduce allocations where the profiles say it
matters.

## Scope

- `project.Project` and its helpers (graph merge/shadow, label/broader/alias
  index builds, per-work projection) -- now also on the export hot path per
  grain (tasks/108).
- Ingest: `ingest.Run` / `RunStore` (crosswalk, canonicalization, grain
  writes), `ScanSummaries` / `SummarizeGrain`, and `identity.ScanGrain` --
  ScanGrain + SummarizeGrain also run per changed grain in the workindex
  refresh (tasks/106), so their allocation profile shows up at boot and on
  every TTL refresh of a large corpus.
- The rdf parse/encode layer usage patterns (ParseNQuads vs ParseNQuadsShared,
  Encoder buffer reuse) -- the parsers themselves live in libcodex; if a
  profile points into libcodex internals, file a task there rather than
  patching around it (repo convention).

## Approach

1. Benchmarks first: `go test -bench -benchmem` micro-benchmarks over a
   seeded corpus (the tasks/085 repro harness in the scratchpad measured
   ~57KB/work RSS; reuse its corpus generator) plus `pprof` heap/CPU profiles
   of a full ingest run and a full-corpus projection.
2. Rank by profile, not by eye. Likely suspects to check, not assume:
   per-quad string concatenation, repeated map growth without size hints,
   `append` churn in per-work slices, duplicate parses of the same grain
   (the tasks/116 single-parse work removed several -- verify none remain),
   `SummarizeGrain`'s merged-graph copy.
3. Land wins with before/after benchmark numbers in the commit message;
   no semantic changes -- byte-identical grains and projection output
   (the catalog.nq byte-equality test is the guard).

## Acceptance

- Benchmarks committed (`project`, `ingest`, `identity` packages) so
  regressions are measurable.
- Documented before/after allocs/op and ns/op for the top findings.
- Full-corpus projection and ingest wall time and peak RSS measured against
  a seeded store before and after (tasks/085 harness scale or larger).

## Outcome

Profile-first pass done (7435338, released v0.56.0), ranked by
alloc_space over real corpora per the task's rule -- benchmarks
committed for all three packages (acceptance): project's existing
LCAT_BENCH_CATALOG corpus benchmark, new BenchmarkSummarizeGrain
(ingest) and BenchmarkScanGrain (identity) over LCAT_BENCH_GRAINS
real-grain trees.

Measured (62,602-work coll corpus; 2,000 real grains for per-grain):

- Project (full catalog, 632MB): 2.7s, 8.6GB allocated. Top heads:
  rdf.parseNQuads 40% (Dataset representation; the input copy is
  already avoided via ParseNQuadsShared), libcat's splitGraphs 30%
  (the materialized []Triple IS the data -- exact-sized already),
  Graph.index 12%. All three need a zero-copy graph view at the
  libcodex layer -> filed as libcodex tasks/098 per the repo
  convention, with the numbers.
- SummarizeGrain (per grain at workindex boot/refresh + batch scans):
  Dataset.Graph materialization + unsized Graph.Add re-append = 55%
  of allocations. FIXED: one exactly-sized append pass in identical
  graph-grouped order. 132,551 -> 64,961 B/op (-51%), 44 -> 30
  allocs (-32%), 31.7 -> 25.9 us (-18%).
- ScanGrain: same Dataset.Graph head (33.7%) but its per-graph QUERY
  semantics are load-bearing (feed/editorial separation; cf.
  tasks/196) -- not mergeable libcat-side; covered by the libcodex
  098 ask.

Equivalence guard: full-corpus workindex snapshots byte-identical
(sha256 equal, 54,722,809 bytes uncompressed) built with the old and
new code over all 62,602 grains; project/ingest/identity suites green
(catalog.nq byte-equality guard included).
