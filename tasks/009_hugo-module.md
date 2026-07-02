# 009 -- Hugo module: content adapter, layouts, facet UI

## Context

`lcat project` now emits `catalog.json` (one record per Work: title, contributors,
subjects, languages, BISAC classifications, instances with ISBNs + provider ids)
and `facets.json` (precomputed per-dimension value counts), both carrying a
top-level `version` (`project.SchemaVersion`). That is the contract this task
consumes (ARCHITECTURE §7). Parallelizable with `tasks/010` (search) and
`tasks/004` (availability) once the JSON shape is stable.

## Scope

The `hugo/` module (`hugo mod get github.com/freeeve/libcatalog/hugo`), its own
`go.mod` so Hugo sites don't pull the Go build deps:

1. **Content adapter** (`_content.gotmpl`, Hugo >= 0.126): mint a Page per Work
   from `catalog.json` -- no content files, no per-record markdown.
2. **Layouts**: a faceted, paginated Work list and a Work detail page (format
   facets from the Instances, live-availability placeholder, subjects/contributors
   as links).
3. **Partials/assets**: facet sidebar (language / format / subject / contributor),
   search box wired to the roaringrange WASM reader (`tasks/010`), and the
   availability JS hook (`tasks/004`).
4. **Accessible by default** (§2): semantic HTML, ARIA on facet/search UI, full
   keyboard nav, adequate contrast -- a build-time constraint.

## Contract (decided)

- **JSON is the contract, consumed as a resource.** `_content.gotmpl` should
  `resources.Get "catalog.json" | transform.Unmarshal` and iterate -> `AddPage`,
  **not** load it as `.Site.Data` (which pins the whole corpus in global site
  data). JSON is a *derived* artifact (§7); the graph stays source of truth.
- **Three separate artifacts, don't conflate.** `catalog.json` (page/content),
  `facets.json` (facet counts -- already emitted), and the search index
  (roaringrange `RRTI`/`RRS` **binary**, `tasks/010`) each have their own contract.
- **Schema version.** Both JSON files carry `version`; the module should check it
  against the version it targets and fail loudly on mismatch.
- **Shard at scale.** One `catalog.json` (~4.4M / 5,659 Works today) is fine; past
  a few hundred k Works, shard by language or id-prefix so Hugo build memory stays
  bounded (the §3 out-of-core threshold, not a today concern).

## Facets

Use the projector's `facets.json` (value + Work count per dimension: languages,
subjects, contributors, classifications; format facet pending `tasks/011`) rather
than aggregating `catalog.json` in templates.

## Acceptance

- [ ] `hugo mod get` + content adapter renders one page per Work from catalog.json.
- [ ] Facets filter the list; Work detail shows its Instances/formats.
- [ ] No per-record content files; theme overrides layer cleanly on top.
- [ ] Axe/Lighthouse a11y pass on list + detail.
