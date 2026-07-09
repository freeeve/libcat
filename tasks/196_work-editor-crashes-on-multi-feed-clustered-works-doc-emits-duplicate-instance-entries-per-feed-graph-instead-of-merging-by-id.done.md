# 196 -- work editor crashes on multi-feed clustered works: /doc emits duplicate instance entries per feed graph instead of merging by id

Opened 2026-07-09.

## Outcome

Fixed in ad0de8b (backend) + 800a4ca (UI), released v0.46.0. Found
while verifying tasks/193 on River of teeth (feed:marc + feed:copycat
cluster): ScanGrain reports an instance once per feed graph, so ToDoc
grew duplicate instance entries -- the first claimed every quad, the
rest were empty husks whose duplicate ids crashed the editor's keyed
tab list (each_key_duplicate, editor stuck on "Loading…").

- ToDoc dedupes instance ids (backend/editor/doc.go); new
  TestMultiFeedClusterInstances proves one entry per id with fields
  intact AND that a two-graph grain still round-trips byte-identical
  -- per-graph FieldValues are load-bearing (ToGrain renders each back
  into its Prov graph), so they were NOT merged server-side.
- Display-side, ProfileForm collapses values identical across graphs
  into one row wearing every provenance badge (title/summary/isbns
  show "copycat marc" instead of doubling); ops key on the value, so
  Remove suppresses every assertion, and override semantics are
  untouched.
