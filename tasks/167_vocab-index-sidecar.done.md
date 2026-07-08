# 167: Vocab index on gochickpeas RCPG + roaringrange RRTI, range-read from the blob store

## Shipped (2026-07-08)

The serve path shipped on roaringrange RRSR + RRIL + a compact LCVS search
blob -- reading the vocab package's real contract showed Search is
whole-normalized-label prefix (not tokens, so RRTI does not match its
semantics) and Path/neighborhood walk Broader lists carried in the Term
payloads (so no resident graph is needed). gochickpeas RCPG therefore
stays where the measurement left it: the 085-convergence artifact, not a
serve-time dependency; no new backend deps beyond roaringrange v0.29.0
(published tag, no replace).

- storage/blob: RangeReader optional capability (Signer precedent) +
  blob.ReaderAt adapter with whole-Get fallback; DirStore/MemStore/blobs3.
- vocab: per-scheme sidecar backend behind the existing Index API. Armed
  only when the manifest's source ETag matches and no loose quads for the
  scheme exist anywhere else (manifest records every scheme its source
  file carries, so shared files never drop a scheme); any doubt falls back
  to the map path. Retired terms resolvable but unsearchable, identifier
  tiers preserved cross-backend, sidecar read errors log and report a miss.
- vocabsrc: installs build artifacts automatically; `lcatd vocab-index
  (--name|--all)` retrofits existing deployments.
- Tests: full-surface parity suite (map vs sidecar over the same fixture),
  stale-source/loose-quads/partial-arming fallbacks, blob conformance
  additions.

Verified on the playground (real LCSH, 513,125 terms): RSS 1,250MB ->
483MB (macOS RSS; live heap lower -- lazy page return), autocomplete and
resolve serving from the sidecar through /v1/terms. Found two
pre-sidecar-era leftovers in the playground store that correctly bypassed
arming (a fixture-copy seed file and a redundant cache/lcsh entry; the
lcsh-redundant pieces removed, homosaurus intentionally stays map-backed
because 7 works reference its legacy v4 terms). Note for deployments: the
per-term subset-fetch cache legitimately keeps a scheme map-backed --
subset schemes are small, so that is the right outcome.

Measured (tasks/165 notes, 2026-07-08): lcatd RSS is ~1.25GB with a 31-work
catalog, dominated by the vocab package's map-based term index over the full
LCSH+LCSHAC+LCGFT snapshots (254MB on disk). `lcat vocab-subset` is the
shipped fix for small deployments (README "Deployment styles"); this task is
the other shape -- full-LCSH cataloging without the multi-GB residency -- and
doubles as the low-risk pilot of gochickpeas before the tasks/085 catalog
seam.

## Why gochickpeas fits

- SKOS access patterns map cleanly: URI->concept validation (equality
  index), broader/narrower/related neighborhood (CSR traversal),
  ReadNQuads ingest from the authority grains.
- Install-time build matches the engine's write model: Builder ->
  Finalize() -> immutable Snapshot, swapped atomically -- vocabulary
  installs already replace wholesale (vocab-install, tasks/163).
- The RCPG codec does lazy section-planned loading (ParseLazy /
  SectionFetch) designed for range-fetched transports -- rustychickpeas
  range-reads straight from S3. The vocab graph can live as an RCPG blob
  and be range-fetched on demand instead of resident.

## Design sketch

- vocab-install builds the RCPG file from the SKOS grains and Puts it next
  to the snapshots under data/authorities/.
- Typeahead: gochickpeas full-text is whole-token BM25 (checked: no prefix
  search), but roaringrange RRTI v2 covers it range-fetched: a blocked
  front-coded term dictionary with only a small FST router resident
  (O(#blocks), not O(vocabulary)) and explicit prefix/autocomplete support
  (TERMS.md). vocab-install writes an RRTI over the concept labels next to
  the RCPG; RRTI's doc-ID space is assigned to equal the RCPG node IDs, so
  an autocomplete hit resolves to the graph node with no join table.
  roaringrange is already a root-module dependency (search/ consumes its
  BM25 sidecars) -- no new dep for this half.
- Everything else (topology, properties, neighborhoods, validation) goes
  through ParseLazy over ranged blob reads with a small section cache
  (data/authorities/cache precedent; invalidate by blob ETag, as the
  workindex snapshot does).
- Net residency: FST router + section cache -- a few MB regardless of
  vocabulary size; full LCSH stops being a deployment-sizing input at all.
- Seam: add an optional capability interface to storage/blob (the Signer
  precedent): GetRange(ctx, path, offset, length). blobs3 maps to S3
  GetObject Range; fs/mem are trivial; stores without it fall back to one
  whole-object Get (fine for subset-scale vocabs).

## Measured (2026-07-08, harness in session scratchpad -- 085 precedent, not committed)

Source: the playground's full LCSH snapshot, `lcsh.nq` 233MB, 513,497
concepts / 626,488 rels. Apple M-series, artifacts on local SSD; fetch
counts are the S3 round-trip proxy.

Build (install-time, offline):
- ReadNQuadsFile + Finalize: **1.6s**; peak RSS ~1.1GB (batch step only).
- Artifacts: RCPG **66MB** (atoms 47 + rels-CSR 13.7 + node-cols 5.3),
  RRTI **10.9MB** (178,788 terms), RRIL **7.8MB**. 233MB nq -> 85MB total.

Query (server shape):
- ParseLazy topology-only: 0.07s, 6 fetches, 60.7MB -- **the 47MB atoms
  section loads even topology-only**; resident after load 175MB.
- 3-hop traversals: p50 125ns (resident CSR).
- OpenTermIndex: 2 fetches / 13KB -- **router resident = 0.9MB**. Exact
  Posting: p50 48us, 3.0 fetches / 9.5KB per query.
- RRIL Lookup: p50 8us but **21 fetches/query** (binary search) -- over S3
  that is 21 RTTs; at 7.8MB just hold it resident (or use the gochickpeas
  equality index) and validation is microseconds with zero fetches.
- **Resident total today: ~180MB** (vs ~1.2GB status quo, ~7x) -- dominated
  by decoded atoms. With atoms lazy/working-set (gochickpeas's deliberately
  deferred CSR-skeleton layer), resident falls to CSR + routers ~= **20MB**.

Decision: proceed against gochickpeas v0.8.0 as-is -- 180MB resident is
already a 7x win, and the atoms improvement lands transparently later.
Sibling asks filed as uncommitted tasks: gochickpeas (lazy atoms),
roaringrange (Go TermIndex prefix/autocomplete reader -- the format
supports it per TERMS.md; the Go reader has only exact find today; RRTI
is small enough to hold resident meanwhile).

## Open questions

- Latency budget: per-keystroke stays on the resident typeahead structure;
  concept detail / neighborhood / save-time validation can afford a ranged
  round trip -- confirm against the editor's UX.
- Builder finalize wall time and peak RSS at LCSH scale (~450k concepts) --
  it runs at install time, but on what hardware?
- Local headings: incremental writes vs the immutable snapshot --
  generation swap via Manager, or a small mutable overlay for the local
  scheme merged at query time.
- Verify named-graph/quad fidelity for SKOS routing (the tasks/085
  caveat) -- scheme identity comes from the authority:<vocab> graph name.
- Dependency policy: consume a published gochickpeas tag (libcodex
  precedent -- deliberate, announced bumps; no local replace).
- Any gochickpeas-side gaps (e.g. a SectionFetch transport helper) get an
  uncommitted task filed in that repo, per the multi-repo rule.
