# 180: project.Merge drops the v10 terms sideband; lcat project always merges

## Done (2026-07-08)

Both halves landed:

1. **Merge carries Terms**, by term id, field-wise: labels fill per language
   (earlier catalogs win a language, like works win an id), broader edges
   union sorted, first non-empty scheme sticks. Merged entries are private
   copies, so the inputs stay unmutated, and the output is id-sorted like
   everything else. CLI-built deployments (projectCatalog always routes
   through Merge) keep the sideband now.
2. **The "worth considering" provider capability**, so this class of gap
   ends at the source: ingest.TermDescriber -- a Record capability like
   SubjectEnricher but link-less. Run emits DescribedTerms() into the feed
   graph as prefLabel/broader statements with NO bf:subject link
   (WorkGroup.Terms -> bibframe.addDescribedTerms; per-term emission shared
   with addControlledSubjects). A provider with vocabulary access describes
   ancestor chains at ingest time, catalog.nq carries them from the first
   build, and the post-hoc RunEnrich re-serialize dance (queerbooks
   cmd/qbd patch-terms INTERIM) becomes unnecessary -- their qbd records
   implement TermDescriber instead.

Verified: Merge sideband test (field-wise merge, input immutability,
ordering), Run-level TermDescriber test (labels land in the feed graph, the
ancestor never linked as a work subject), full suites + both e2e passes.

---

Left by the queerbooks-demo session 2026-07-08 (uncommitted cross-repo note).
Sibling of 179 -- the second v0.34.0 release gap, same root cause (178's new
surface not carried through every consumer).

project.Merge builds a fresh Catalog and copies Works only, so Catalog.Terms
(178) vanishes -- and cmd/lcat's projectCatalog routes through Merge even for
a single provider, so every CLI-built deployment loses the sideband:
BuildBrowse then finds no labels and the minted ancestors stay hidden, which
is exactly the state 178 set out to fix. Your tests stayed green because they
call project.Project directly.

Fix: Merge carries Terms (union by ID; keep the richer entry on collision --
or recompute the closure over the merged works if that's cleaner).

Worth considering alongside (would have avoided the adopter dance entirely):
a feed-provider-side capability for standalone term descriptions -- e.g. a
Record capability like SubjectEnricher but link-less, or an ingest.Run hook
-- so a provider with vocabulary access can describe ancestor chains in its
feed graph at ingest time. Today the only ingest-side route is
RunEnrich+Enrichment.Terms AFTER Run, which leaves catalog.nq stale (Run
writes it before the enrich pass) and forces adopters to re-serialize 62k
grains and patch catalog.json between build steps (queerbooks cmd/qbd
patch-terms, marked INTERIM on this task).
