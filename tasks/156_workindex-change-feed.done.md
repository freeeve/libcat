# 156 -- Workindex change feed: cross-container read-your-writes without a scan

Plane 1 freshness layer of [154]. Depends on [155] (snapshot baseline).

## Problem

"Update several records, then read those updates in admin" must hold on Lambda,
where there are N containers. The writer's in-memory `Apply`
(`backend/workindex/workindex.go`) only updates its own container; a read on
another container never sees the write. Reconciling another container against
the store via `List`+ETag-diff is O(corpus/1000) `List` calls per refresh --
fine at 48k, a wall at millions, and still a per-read cost.

## Fix: an append-only change feed of projected entries

The snapshot ([155]) is the periodic checkpoint; the feed is the tail of changes
since it. Both carry the **projection**, so neither needs a grain rescan. This is
the admin plane's base+delta -- distinct from the public search index's
`splitset` base+delta (tasks 158/159), which shards RRS/RRTI search bodies, not
this projection. roaringrange task 075 requests a first-class sharded-write
record store (base+delta over `RRSR`) that would subsume this snapshot+feed+fold;
adopt it when it lands.

1. **Append on write.** A publish/commit PUTs its grain(s) then appends their
   projected entries (`{path, etag, entry}`, plus tombstones for deletes) to the
   feed, durably, before returning. A multi-grain op (copycat commit/revert)
   appends its **whole batch in one CAS write**, not one per grain -- so the
   write cost is O(1) feed writes, never O(N). Entries use the **same JSON record
   encoding as the snapshot** ([155]) so replay and snapshot load share code.
2. **Replay on read.** A container serves reads from `snapshot + feed replay`.
   The feed is small and bounded; fetch it with a conditional GET (304 when
   unchanged) on a short TTL, or per-read for strict read-your-writes.
3. **Fold in memory.** When the feed crosses a size/count threshold, regenerate
   the snapshot from memory (no grain reads), stamp it with `epoch+1`, and reset
   the feed to empty at the new epoch, under ETag-CAS. Keeps the feed bounded and
   cold-start cheap. Snapshot and feed share the epoch counter; a reader that
   sees the epoch advance **reloads the snapshot** instead of replaying, and the
   epoch also disambiguates "same feed, more records" from a reset-then-append
   (so applied-count tracking stays correct across a fold).

## Large deltas / bulk updates

A bulk op (re-ingest, mass authority rewrite, a copycat commit over the whole
catalog) must not grow an unbounded feed or force a giant replay. The three
mechanisms above compose to handle it: the batch appends once; if it exceeds the
fold threshold the writer **folds instead of appending** -- one full snapshot
rewrite + an empty feed at the next epoch; and readers **reload the snapshot
once** on the epoch bump rather than replaying. So "the whole catalog changed"
costs O(catalog) *once* on the writer and one snapshot reload per reader --
never O(catalog^2) writes or a corpus-sized replay. At true scale the
per-shard rewrite/reload of the sharded record store (roaringrange 075) reduces
even that to only the touched shards.

Read-your-writes argument: C1 writes grain A and appends A to the feed durably
before its 200; a later read on C2 fetches the latest feed, replays A over its
snapshot, and sees A -- independent of which container wrote it. Freshness cost
per read is one conditional GET of a bounded object -- O(1), not O(corpus).

## Substrate (swap behind one interface)

- **Now / low write-concurrency:** the feed is a small S3 object appended under
  ETag-CAS (read-modify-write with `If-Match`, retry on conflict). Pure
  `blob.Store`; no new service. Fine for queerbooks.
- **High write-concurrency later:** DynamoDB -- `PutItem` per change (no CAS
  contention), `Query` since-seq, strongly consistent. This is where the
  DynamoDB option belongs: the change feed, not the whole index.

Keep the periodic full `List`+ETag reconcile as a rare backstop for out-of-band
changes (writes not routed through the feed, e.g. the public plane); it is no
longer on the hot path.

## Consumer note

The same feed is the event source for the public plane's incremental rebuild
(task 159) -- one change feed, two consumers.

## Verify

- Two-container simulation: write on A, read on B sees it without a corpus
  `List`.
- Feed fold keeps cold-start load O(snapshot) and steady-state reads O(feed).
- Concurrent appends resolve via ETag-CAS with no lost entries.
