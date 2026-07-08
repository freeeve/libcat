# 171: Admin works view -- facet by provenance (extras/sources)

Left by the queerbooks-demo session 2026-07-08 (uncommitted cross-repo note).

## Shipped (2026-07-08) -- extras generally, sources by default

- ingest.WorkSummary carries `Extras map[string]string`: every
  `lcat:extra/*` literal on the Work node, raw and keyed without the
  prefix. Extraction is unconditional, so facet-config changes never
  require a rescan. workindex snapshotVersion bumped to 2 -- a v1 snapshot
  would keep serving extras-less summaries for unchanged grains, so it is
  discarded and rebuilt at boot (the change feed's per-write records are
  self-correcting; pre-upgrade feed records lack extras only until the
  next write of that grain).
- works_facets: the fixed [5]-array machinery became a []facetGroup slice
  (name = response key = query param, valuesOf, fold, top-N cap); extras
  groups bucket on the comma-split, trimmed value (the hugo extraFacets
  split convention). Keys that would shadow built-in params
  (visibility/holdings/needs/subject/tag/q/limit/offset) are dropped with
  a warning.
- Config: `LCATD_EXTRA_FACETS` (comma list of extras keys), default
  `sources`; set empty to disable. Flows to httpapi.Deps.ExtraFacets and,
  via /config `extraFacets`, to the SPA rail, which appends one humanized
  group per key after the built-ins.
- QLL's two triage shapes verified live on a seeded instance:
  `?sources=overdrive queer scan` -> the two scan works;
  `?sources=mombian` -> the community-only work. Counts self-exclude and
  compose with the other groups; e2e Playwright pass over the real SPA.

The public projection is untouched: extras faceting is admin-side only,
which is exactly where the privacy-dimension curation happens. (visibility, holdings, completeness, controlled
subjects, raw tags) covers cataloger triage well, but the first real triage
QLL wanted was provenance: "show me everything from `overdrive queer scan`"
(bulk-imported scan works needing review), or its inverse ("community-only
works" -- the ones whose provenance QLL keeps private; cf. the 019/021
rediscovery motivation).

workindex summaries (ingest.SummarizeDataset) don't carry the feed graph's
`lcat:extra/sources` today, so there's nothing to filter on. Ask: index the
sources extra (or extras generally, adopter-configurable like the hugo
module's extraFacets) and add it to the rail.

Context from our deployment: provenance is now a PRIVACY dimension --
queerbooks strips all but LOC/OverDrive/QLL from the public projection and
the public catalog.nq.gz download, while the backend keeps the full set for
curators. An admin-side sources facet is where that curation actually
happens (e.g. "which community-only works still lack a public-citable
source").
