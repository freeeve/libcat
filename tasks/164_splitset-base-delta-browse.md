# 164 -- splitset base+delta browse artifacts (deferred from 159)

The remaining piece of tasks/159: replace the interim monolith re-emit in
`lcat rebuild` with true incremental search artifacts.

- Build side: `WriteSplitSet` + per-split RRS emit (Go builders exist in
  roaringrange); an incremental run folds changed works into a delta split
  with tombstones; compaction rolls deltas into the base.
- Client side: move `lcat-browse.js` from `RrsCatalog.openAll` (monolith) to
  `RrssIndex` (+ records/facets as today). Both sides move as one
  coordinated change, Playwright-verified like tasks/158.

Not needed until corpus size makes the monolith re-emit noticeable: at 48k
works it is seconds (the tasks/159 interim measured well under that for the
playground corpus). Revisit at ~500k works or when rebuild frequency makes
the emit the dominant cost.
