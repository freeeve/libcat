# 150 -- facet sidebar as a shared fragment (87% of every term page)

(Renumbered from 149, which collided with the term-page head-title task.)

Filed from queerbooks-demo (2026-07-06, Eve pressing on build size). Do not
let a queerbooks session edit this repo -- implement here.

## Measured (queerbooks, 48.5k works bilingual, hugo/v0.21.0)

A term page is 38.5KB of which the facet sidebar is **33.5KB (87%)** -- 137
rows / 8 groups, ~245B/row post-tasks/148. The sidebar is page-invariant per
language (that is WHY partialCached works on it), yet its output is inlined
into every sidebar-bearing page: ~300k term/list pages x 33.5KB ~= **8 of
the site's 9.1GB**. Deploy-size history there: 3.3GB -> 5.3GB -> 9.1GB as
facet dimensions grew; every new group/row multiplies by page count.

gzip hides it on the wire (5.4KB/page) but storage, build I/O (the
post-cache profile in tasks/133 was ~40% file-write syscalls), deploy diffs,
and CDN cache all scale with raw bytes -- and every future facet feature
pays the x300k tax again.

## Suggested shape

- Render the sidebar ONCE per language as a fragment asset
  (/lcat/facets.<lang>.<hash>.html or a JS-consumable JSON+template); term/
  list pages ship an empty <aside> + a small loader that fetches and
  inserts it (fingerprinted -> immutable cache, one fetch per visit).
- No-JS fallback: a "Browse all facets" link to the taxonomy landing pages
  inside the empty aside (arguably better than 10k-row sidebars for screen
  readers anyway).
- Opt-in ([params.facets] shared = true?) if inlining should stay the
  default for small catalogs; large-catalog README gains the note.
- Interaction with tasks/144 negatives + 141-addendum filter: both already
  hydrate client-side, so they compose naturally with a fetched fragment.
- Expected at queerbooks scale: term pages ~5KB, site ~2GB, and facet
  features stop multiplying by page count.

## Done

Shipped as suggested, opt-in via `[params.facets] shared = true` (inline
stays the default; small catalogs keep zero-fetch crawlable sidebars).

- facets.html split: facets-body.html is the old partial verbatim (nav +
  hydration script tags, still page-invariant per language); facets.html is
  now a dispatcher -- default inlines the body, shared mode publishes it via
  resources.FromString as /lcat/facets.<lang>.<hash>.html (fingerprinted ->
  immutable-cacheable) and emits a host div + lcat-sidebar.js loader.
- The script tags stay in the BODY: innerHTML-inserted scripts are inert, so
  the loader re-creates each executable script element after insertion (JSON
  config scripts pass through as data). That is what makes tasks/141 filter
  and tasks/144 negatives compose unchanged -- both already hydrate over the
  rendered rows -- and future facet scripts ride the fragment for free.
- No-JS / fetch-failure fallback: taxonomy landing links (module dims +
  tasks/143 extra facets), localized like the group headings.
- a11y_audit.js skips /lcat/ fragment assets (not documents; audited in
  context via the inline default build).
- Tests: sidebar_test.cjs (jsdom, wired into npm run test:js) covers fetch/
  insert/re-activation/config passthrough/404-fallback/no-host; verified
  end-to-end over HTTP -- fetched sidebar hydrates, exclude click hides
  cards, on-load x-params apply after late sidebar arrival, es pages pull
  the es fragment, missing fragment leaves the fallback usable. Default-mode
  exampleSite output byte-identical to pre-change (modulo the lcat.css
  fingerprint, which gained the fallback styles).
- exampleSite: 14.4KB term page -> 8.5KB + one shared 6.6KB fragment per
  language, at 137 sidebar rows the queerbooks ratio (~87%) applies.
