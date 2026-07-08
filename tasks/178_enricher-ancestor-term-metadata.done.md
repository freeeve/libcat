# 178: Enrichers emit label/broader metadata for ancestor terms

## Done (2026-07-08)

All three legs landed; a minted-but-labeled ancestor now renders as a real
tree node (176's display rule), and rollups cross ancestors no work carries.

1. **Metadata into the graph.** vocab.Index.Ancestors walks a term's full
   transitive skos:broader closure (BFS, every parent -- unlike Path's one
   shortest chain -- cycle-safe, depth-capped). Producers with vocab access
   use it: the crosswalk enricher and the suggest enricher (new optional
   Index, wired to deps.Vocab in appdeps; a match whose scheme is installed
   also upgrades to the full local term description) ride ancestors along as
   the new Enrichment.Terms -- standalone descriptions RunEnrich writes as
   prefLabel/broader quads with no bf:subject link (enrichmentQuads/
   termQuads). The publish path mirrors it: bibframe.AppendAuthorityTerms
   writes ancestor descriptions into the authority:<vocab> graph next to
   AppendAuthoritySubject (Publisher.ancestorTerms). ingest/locsh and
   hardcover stay as-is (no local vocabulary to walk).
2. **Sideband out of the projection (schema v10).** Catalog.Terms: every
   referenced subject URI plus its transitive broader closure, with labels/
   broader/scheme from the projector's corpus-wide indexes; URIs the graph
   says nothing about are skipped. Rebuild's incremental path already falls
   back to full on a schema mismatch.
3. **BuildBrowse consults the sideband when minting**: labels + scheme fill
   in, and the sideband's broader edges extend the ancestry walk, so
   postings roll up through ancestors known only to the sideband.

Also fixed en route: polyhierarchy twin rows -- a concept under two parents
renders once per parent, and toggling one instance now syncs its twins
(lcat-browse.js syncTwins); before, clearing a filter could leave the twin
checked and the result set silently filtered.

Verified: Go tests for Ancestors, the sideband (transitive-only ancestors,
bare-URI skip), sideband-labeled minting, enrichmentQuads Terms (label quads
without a subject link), crosswalk/suggest ancestor rides, publisher
authority append; e2e fixture (v10) grew a sideband-labeled second parent
("Trans community") -- 33 Playwright checks green across both profiles,
including "sideband-labeled minted ancestor renders as a root" alongside the
original "unlabeled minted ancestor never renders".

queerbooks: their own pipeline must emit ancestor labels (or they build with
a graph that carries them) for the 52 URI-root case to fully label -- noted
in their tasks/ as an uncommitted follow-up.

---

Original note follows.

Follow-up to tasks/176. expandSubjectAncestry (search.BuildBrowse) mints
skos:broader ancestors that no feed ever described: enrichmentQuads
(ingest/enrich.go) emits prefLabel + broader only for a Work's DIRECT
subject terms, so an ancestor that is never itself a direct subject reaches
browse-subjects.json label-less. On queerbooks that was 52 of 63 homosaurus
tree roots; 176 keeps such nodes out of the rendered tree (pass-through
parents), which means real vocabulary structure is invisible until some
work happens to carry the ancestor directly.

The fix at the source: when an enricher resolves a subject, walk its
broader chain in the vocabulary (vocabsrc has the full dump in the vocab
index; the suggest-API enricher path may need a lookup per ancestor) and
emit each ancestor's prefLabel + broader quads into the enrichment graph
too -- depth-capped like ancestryDepthCap. The terms are pure authority
metadata, so co-grained preservation semantics are unchanged; grains grow
by a few quads per chain.

Plumbing note: emitting the quads is not enough on its own. Work.Subjects
carries only direct terms, so BuildBrowse mints ancestors from per-work
metadata and never sees the ancestor labels even when the graph has them.
The projection needs a vocabulary sideband (e.g. Catalog.Terms: every term
URI seen in the graph with labels + broader, which the projector's
labels/broader indexes already contain) that BuildBrowse consults when
minting -- likely a schema bump.

Then: minted-but-labeled entries render as normal tree nodes (176's
display rule already handles this -- Minted entries WITH labels display),
and the homosaurus tree gets its real top levels back.

Consumers with their own pipelines (queerbooks-demo ingests directly) need
the equivalent change on their side; noted in their tasks/026.
