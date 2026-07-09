# 154 -- Persist the work index so cold starts don't rescan the corpus

Filed from the queerbooks-demo session (2026-07-07). Leave uncommitted; the
queerbooks side owns/adopts it. Makes the **writable serverless (Lambda)**
shape viable at real corpus size.

## Problem

`appdeps.Build` wires `workindex.New(blob, "data/works/")` and warms it in a
background goroutine (appdeps.go:102). The index is an in-memory
`grains map[path]*grainEntry`; `refreshLocked` (workindex.go:272) only skips a
grain already in that map with a matching ETag. So a **fresh process** (empty
map) GETs every grain.

On a long-lived server that is a one-time warm. On **Lambda** it is fatal at
scale: the queerbooks admin deploy (48,515 works, S3 grain store) times out
`/v1/works` every cold start --

- 48k S3 GETs can't finish in the 30s Lambda / API-Gateway window;
- Lambda **freezes** the process between invocations, so the background
  warm-up goroutine never advances;
- the map is in-memory only, so every cold container rescans from zero.

Index-free endpoints (/v1/stats) are fine; anything needing `Summaries`
(the works list/search, duplicates) 503s. Memory is not the issue (~99MB);
it's purely the corpus scan.

## Proposed fix: a snapshot blob the index loads instead of rescanning

The `grains` map IS the serializable source of truth (the derived
byProvider/byCluster/barcodes/summaries views already rebuild from it with no
I/O -- rebuildLocked). So:

1. **Serialize** `grains` (path -> {etag, identity, merges, barcodes,
   summaries}) to one blob, e.g. `data/workindex.snapshot` (gob is easiest;
   version the header). Add `Index.Save(ctx) error`.
2. **Load** on startup: `New` (or a `LoadSnapshot(ctx)`) reads the snapshot
   into `grains` before the first refresh. Then the normal ETag-diff
   `refreshLocked` runs -- now every unchanged grain is a cache hit (List
   only), so a cold start with a current snapshot costs one blob GET + the
   List pages + GETs for *only* grains changed since the snapshot. Correctness
   is preserved even against a stale snapshot: the ETag diff re-reads the
   delta and the unlisted-path sweep drops deletions.
3. **Keep it current**: write the snapshot after publishes. The index already
   stays exact in-process via `Apply`/`Update` (records/copycat/publish
   paths), so add a `Save` after those commit -- inline, since it's an active
   invocation (no reliance on background goroutines). A whole-map reserialize
   (~a few MB for 48k works) on each publish is fine; incremental is a later
   optimization.
4. **Build offline for first boot / re-seed**: a one-shot to produce the
   snapshot without the running server -- either a `lcat` subcommand
   (`lcat workindex-snapshot --blob-dir <dir> --out data/workindex.snapshot`)
   or documented "run lcatd once against the store off-Lambda to Save it."
   The consumer seeds the snapshot next to the grains.

Guard rails: if the snapshot is missing/corrupt/older-format, log and fall
back to the full scan (current behavior) -- never fail boot. A configurable
path (default `data/workindex.snapshot`) keeps it out of the `data/works/`
prefix the index lists.

## Consumer impact / rollout

queerbooks-demo admin backend (admin.queerbooks.evefreeman.com) is fully
deployed and gating; login works; only `/v1/works` 503s on the corpus scan.
After this ships: queerbooks rebuilds `cmd/lcatd-lambda` from this tree,
builds the initial snapshot from its `data/backend` grains, uploads it to the
S3 grain bucket, and redeploys -- no module re-pin, static site unaffected.
Consider bumping the Lambda + API-GW timeout only as a fallback; with the
snapshot the cold path is fast and no timeout bump is needed.

## Verify

- Cold start against a 48k-grain S3 store with a current snapshot serves
  `/v1/works` in well under the timeout (index-load, not corpus-scan).
- Publish -> the snapshot updates -> a subsequent cold start reflects the edit
  without a full rescan.
- Missing/corrupt snapshot falls back to a full scan and still boots.

## Direction chosen (libcatalog session, 2026-07-07)

Design discussion converged on a two-plane (CQRS-ish) architecture; this task's
"snapshot only" is the first half of it. Decisions of record, decomposed into
tasks 155-160:

**Plane 1 -- admin read index (hot, Lambda-native).**
- The snapshot is the in-memory **projection** (`grainEntry`: etag, identity,
  merges, barcodes, summaries per grain), **not** `catalog.nq`. The backend
  holds only the projection -- it cannot re-serialize the full-RDF `catalog.nq`
  without rescanning every grain, so `catalog.nq` belongs to the public plane,
  not the backend's write path. Saving the projection is cheap (straight from
  memory). -> **task 155**.
- Cross-container read-your-writes is the real requirement behind "read those
  updates in admin". A bare snapshot does not give it: on Lambda the writer's
  in-memory `Apply` only helps its own container, and reconciling another
  container via `List`+ETag-diff is O(corpus) per read. Solved with an
  append-only **change feed** of projected entries (+ tombstones): publish
  appends (durable before it returns), readers replay the small feed over the
  snapshot, feed folds back into the snapshot in memory when it grows. S3
  ETag-CAS now; DynamoDB swap for high write-concurrency later. -> **task 156**.
- Encoding: **JSON records** (shared by snapshot and feed), **v1 container = a
  gzipped-JSON blob** (versioned, add-tolerant). Chosen over gob, which is a
  Go-only opaque stream for ephemeral RPC, not durable upgrade-surviving
  storage: JSON is portable, inspectable, stdlib, and readable by the Rust/WASM
  side. Optional forward container is RoaringRange RRSR wrapping the same JSON
  records (zstd shared dict) for **compression/range access -- not sharded
  writes**: `splitset` shards RRS/RRTI *search* bodies, not RRSR records, so the
  admin plane's incremental writes are snapshot+feed+fold (156) and `splitset`
  base+delta is the public search index's job (158/159). A first-class
  sharded-write record store to subsume snapshot+feed is requested in roaringrange
  task 075 (base+delta over `RRSR`). The snapshot is a rebuildable cache, so the
  swap is no migration. Rejected: gob
  (durability/portability); `catalog.nq` as load source -- measured **~448 MB**
  of full RDF for 48,515 works (~15-40x the projection, full-parse+re-derive
  every cold start, un-producible from memory, no per-grain delta for the feed),
  it is the public plane's artifact; per-read `List`-diff freshness (O(corpus)).

**Plane 2 -- public catalog (cold, async, eventually consistent).** The change
feed is the linchpin here too: it drives an *incremental* rebuild instead of a
full one.
- **Minimal static by default:** per-work detail pages + one browse shell +
  sitemap only. Stop pre-rendering facet/search/browse combinations -- they
  have no finite pre-renderable set and are what make the build slow and
  non-incremental. -> **task 157**.
- **Client-side browse/search/facets** over the RoaringRange WASM reader
  (term/trigram search, facet counts, paged listing, RRSR record details;
  `splitset`-sharded, range-fetched). -> **task 158**.
- **Feed-driven incremental propagation:** regenerate only the changed works'
  detail pages + touched index shards; full ECS rebuild only for seed or
  schema/template change. Trigger seam evolves command -> async job dispatch
  (SQS->ECS). -> **task 159**.
- **Opt-in curated static views:** a deployment may pin specific views (e.g.
  curated lists) to hard HTML for SEO, beyond the default single-combination
  views -- opt-in only, never every combination by default. -> **task 160**.

Net: the full-corpus scan is eliminated from steady state on both planes;
it survives only at initial seed and full rebuilds. Tasks 155-160 are
libcatalog-owned; this file (154) remains the queerbooks-filed problem
statement.
