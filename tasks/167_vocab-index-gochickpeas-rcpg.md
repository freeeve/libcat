# 167: Vocab index on gochickpeas RCPG + roaringrange RRTI, range-read from the blob store

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
