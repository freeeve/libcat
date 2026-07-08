# 159 -- Feed-driven incremental public rebuild + async trigger seam

## Outcome (2026-07-07)

Shipped items (1) and (2) of the updated sequencing; (3) splitset base+delta
is spun out to tasks/164 (deferred until corpus size demands it).

1. **`lcat rebuild`** (cmd/lcat/rebuild.go): feed-cursor consumer.
   `lcat rebuild --store <blob-root> --out <dir> [--index-out] [--cursor]
   [--provider] [--full]`. Reads `data/workindex.feed` (decoding the backend's
   JSON shape directly -- lcat cannot import the backend module), diffs
   against a persisted cursor {epoch, applied, artifact schema versions},
   re-projects only the changed grains (per-grain projection is exact because
   grains are self-contained), patches catalog.json/facets.json/redirects.json
   in place, and re-emits the search + browse artifacts from the patched
   catalog (interim monolith rebuild -- in-memory, seconds at current scale).
   Any doubt -- no feed, no cursor, a fold, a schema bump, unreadable
   artifacts -- falls back to the full serialize -> project -> index chain.
   A missing feed on a server-managed store (snapshot present) counts as
   "no changes since the fold" via the snapshot's epoch. catalog.nq is only
   refreshed on full runs (nothing downstream reads it once catalog.json is
   patched directly).
2. **Trigger seam**: `trigger.Coalesce` debounces a publish burst into one
   downstream event (union of paths, quiet-window + max-delay, async
   delivery); appdeps wires it via `LCATD_REBUILD_DEBOUNCE=5s`. The existing
   awstrigger SQS/EventBridge transports are now configurable
   (`LCATD_TRIGGER_SQS_URL`, `LCATD_TRIGGER_EVENT_BUS`) -- the async job
   dispatch. Seam contract for a queue worker: the message body is the
   trigger.Event JSON ({kind, paths[], at}); the worker syncs the store
   (e.g. `aws s3 sync`) and runs `lcat rebuild --store ... --out ...`; the
   cursor makes coalesced/duplicate deliveries idempotent.

Verified: unit tests (edit/delete/add/merge-redirect patch semantics, dedup,
fold fallback, schema-bump fallback, coalescer batching + max-delay) and an
end-to-end run against a copy of the playground store -- full rebuild (31
works, all 16 artifacts), a real editorial publish via /v1/works/{id}/ops,
rsync, incremental run re-projecting exactly 1 grain with the edit visible in
catalog.json, and an "up to date" no-op re-run.

Original framing below.

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
