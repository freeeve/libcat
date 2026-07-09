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
