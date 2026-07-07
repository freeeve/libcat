# 159 -- Feed-driven incremental public rebuild + async trigger seam

## Status (2026-07-07): NOT started -- plan updated against the shipped 155-158

What changed under this task since it was written:

- **The page-count wall is already gone.** With the minimal static profile
  (157) the public build is O(works) detail pages + shell, not O(works x
  facets); a "full" Hugo run is now radically cheaper, so incrementality's
  remaining wins are (a) skipping the corpus scan (grains -> catalog.nq /
  catalog.json) and (b) the search-index rebuild.
- **The feed exists** (156: `data/workindex.feed`, JSON records + tombstones,
  epoch-folded). The rebuild job can read it directly from the blob store to
  learn the changed work set since its last run (persist a cursor: epoch +
  applied count, same semantics as the admin reader's).
- **The client currently opens a trigram monolith** (`browse-index.rrs` via
  `RrsCatalog.openAll`, 158). Splitset delta therefore needs BOTH sides moved:
  build side to `WriteSplitSet` + per-split RRS emit (Go builders exist in
  roaringrange), client side from `RrsCatalog` to `RrssIndex` (+ records/facets
  as today). Do that as one coordinated change, Playwright-verified like 158.
  Until corpus size demands it, an interim incremental job may simply rebuild
  the monolith browse artifacts -- at 48k works that is seconds, and the feed
  still scopes the detail-page/nq regeneration.
- **Sequencing within this task**: (1) feed-cursor consumer in `lcat` (e.g.
  `lcat rebuild --blob-dir ... --since-cursor ...`) that regenerates catalog.nq/
  catalog.json entries + detail-page inputs for only the changed works and
  re-emits search artifacts; (2) trigger seam: `trigger.Command` stays, add a
  queue-dispatch option (SQS -> ECS RunTask) with coalescing; (3) splitset
  base+delta client+build switch when scale demands.

Original framing below.

Plane 2 propagation of [154]. Consumes the change feed ([156]) so a publish
regenerates only what changed, not the whole site. Depends on [156], [157],
[158].

## Scope

1. **Incremental propagation via `splitset` base+delta.** For each entry in the
   change feed since the last public build: regenerate that work's detail page
   (task 157) and fold its change into the search index as a **`splitset` delta
   split** rather than rebuilding the monolith -- the reader merges the delta
   (task 158), and a later **compaction** rolls deltas into the base. Deletes are
   tombstoned in the delta and dropped at compaction. `catalog.nq` /
   `catalog.json` regenerate for the touched works. No full-site render, and no
   full search-index rebuild, on a publish. (This is where sharded/incremental
   writes actually live -- `splitset` over RRS/RRTI search bodies; the admin
   snapshot's analog is the feed, task 156.)
2. **Full rebuild only for seed / schema change.** A from-scratch build (initial
   seed, template or index-schema change that invalidates everything) is the
   only path that scans the whole corpus. That is the one place heavy compute is
   expected -- an ECS/Fargate batch, rare, not per publish.
3. **Trigger seam.** Evolve the existing `trigger.Fanout` /
   `trigger.Command` (`backend/appdeps/appdeps.go`, `cfg.RebuildCmd`) from a
   synchronous command into an async job dispatch (SQS -> ECS `RunTask`, or Step
   Functions) at scale. Small change to the trigger seam; no backend request-path
   logic change. Publishes coalesce/queue so a burst of edits batches into one
   incremental run.

## Out of scope

- The ECS task definition / infra deployment itself -- a consumer (queerbooks)
  concern; note the seam contract here.
- Admin-plane freshness -- that is [155]/[156]; the public plane is allowed to
  lag.

## Verify

- A single-work publish regenerates one detail page + its shard(s), not the
  corpus.
- A burst of publishes coalesces into one incremental run.
- A schema-change full rebuild reproduces the whole site + indexes.
