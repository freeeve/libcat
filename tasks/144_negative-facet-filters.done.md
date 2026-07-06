# 144 -- opt-in negative facet filters (exclude a term: xhomosaurus=Gay men)

Filed from queerbooks-demo (2026-07-06, Eve's ask). Do not let a queerbooks
session edit this repo -- implement here.

## Ask

Filter works OUT by a facet term, not just in: reader-side "I do not want
X" is a real discovery need in a queer catalog (e.g. browse Lesbians
excluding Gay men, or hide a triggering topic). Prior art in ~/qllpoc:
query-param convention `x<facet>=<term>` (e.g. `xhomosaurus=Gay Men`)
applies the exclusion client-side.

Eve's framing: an **upstream module feature, enable/disable-able per site**
-- not every catalog wants exclusion UI (and a public library may not want
"hide X" links crawlable). Suggested shape:

- `[params.facets] negatives = true` opt-in.
- Facet sidebar: each term row gets a small "exclude" affordance alongside
  the include link; active exclusions render as dismissible "NOT <term>"
  chips above the results.
- Mechanics: the works list is already client-filtered (lcat-search.js
  substring path) / Pagefind-filtered; exclusion = hide result cards
  carrying the excluded term. For the plain taxonomy term pages (static,
  no JS) exclusion cannot be precomputed -- scope the feature to the
  list/search views and document that.
- URL state: `x<taxonomy>=<term>` (repeatable), matching the qllpoc
  convention so links are shareable/bookmarkable.
- Roaringrange note (queerbooks 004): the wasm index supports andnot() --
  a deployment whose search shadow uses roaringrange can apply the same
  x-params server... client-side cheaply; keep the param convention shared.

## Done

Opt-in via [params.facets] negatives = true (module default off, documented
in hugo/hugo.toml + README):

- facets.html: every term row (subject groups, plain dims, tasks/143 extra
  dims) gets an exclude button via new partial lcat-facet-exclude.html --
  a <button> with data-lcat-taxonomy/-term/-label, aria-pressed, localized
  aria-label. Buttons, not links: exclusion URLs stay uncrawlable. The
  sidebar stays page-invariant (partialCached-safe).
- work-card.html: when enabled, cards carry pipe-separated term keys per
  taxonomy as data-lcat-<taxonomy> attributes (same keys the sidebar
  links/excludes by: adapter slugs, capped contributor names, raw values).
  Zero HTML weight when disabled.
- New asset lcat-negatives.js (ES5, dependency-free like its neighbors):
  x<taxonomy>=<term> URL state (repeatable, qllpoc convention) via
  history.replaceState; hides matching cards (li class lcat-neg-hidden);
  renders dismissible "Not X" chips (role=status) above #lcat-results;
  rewrites sidebar + pagination hrefs to carry active exclusions across
  navigation; unknown x-params ignored. Chip strings localize through a
  JSON config emitted by facets.html (excludeTerm/excludedTerm/
  removeExclusion i18n keys, en + exampleSite es).
- Scope documented: client-side, current page's cards only; term pages and
  result counts stay precomputed. Roaringrange andnot() param convention
  noted in README for server-shaped deployments.
- Tests: negatives_test.cjs (jsdom, 5 cases: load-state, link rewrite,
  toggle, chip dismiss, unknown params) wired into npm test:js; a11y +
  link check pass over the exampleSite with the feature enabled; manual
  jsdom run over the real built page verified cross-feature exclusion by
  the tasks/143 sources dimension.
