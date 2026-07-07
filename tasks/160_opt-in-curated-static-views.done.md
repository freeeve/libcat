# 160 -- Opt-in static generation of curated views for SEO

## Outcome (2026-07-07): shipped

`hugo/layouts/curated.html`: a curated view is a plain content page (`layout:
curated`, front-matter `works: [id, ...]`, optional intro prose) -- each id
resolves to its adapter-minted work page and renders the shared work-card
partial, so curated rows match browse rows; unresolvable ids skip with a build
warning, never a failed build. Reference page at
`exampleSite/content/lists/staff-picks.md`; documented in `hugo/README.md`
next to the minimal static profile. Also fixed `list.html` to browse Works
only (`where .RegularPagesRecursive "Section" "works"`) so curated/prose pages
never leak into the works list as pseudo-cards.

Verified: builds under both the default and the minimal profile; front-matter
order preserved; page in the per-language sitemap; bogus-id skip path warns and
builds; browse E2E still 5/5; a11y clean; pre-160 default output functionally
identical (whitespace-only diff). Facet-query-pinned views (a static page for a
named facet combination) stay a future extension -- explicit work-id lists cover
the curated-lists ask.

Original framing below.

Plane 2 opt-in of [154]. A deployment may pin specific views -- e.g. curated
lists -- to hard HTML for SEO, beyond the default single-combination views
(details + browse shell, task 157). **Opt-in only; never every combination by
default.**

## Rationale

Editorially important collections (curated lists, a themed subject page) benefit
from a crawlable, hard-HTML URL. But pre-rendering *all* combinations is the
explosion task 157 removed. So: keep the default minimal, and let the operator
name the handful of views worth freezing.

## Scope

- A deployment-config list of views to render statically -- e.g. curated-list
  slugs, or specific named facet/subject queries -- each producing a hard HTML
  page included in the sitemap.
- The client-side app (task 158) still serves those same views interactively;
  the static page is an SEO/first-paint mirror, not a replacement.
- These pinned views regenerate on the incremental path (task 159) when their
  inputs change (e.g. a list's membership, or a matching work).

## Design notes

- Reuse the per-facet rendering capability task 157 preserves-behind-the-flag;
  this task is the opt-in surface that selects which ones to emit.
- Curated lists are editorial data (see the site-data overlays in the reference
  deployments); the config points at those, it does not re-derive them.

## Out of scope

- Auto-selecting "popular" views to freeze -- explicit opt-in only for now.

## Verify

- With no opt-in config, the build emits only the task-157 default set.
- With a curated list pinned, that list renders to static HTML, appears in the
  sitemap, and still works client-side.
- Editing the list's membership regenerates its static page on the incremental
  path.
