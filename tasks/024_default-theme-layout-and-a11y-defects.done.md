# 024 -- Default theme: fix narrow sidebar-less pages + a11y gaps

## Context

Filed from the **libcatalog-demo** adopter (uncommitted; a request, not in-progress).
These are defects in the module's default `layouts` + `assets/lcat.css` that every adopter
inherits; the demo patched them in an override stylesheet, but they should be fixed at the
source so the reference theme is correct and WCAG-AA out of the box.

## Defects

1. **Sidebar-less pages collapse to a ~16rem column.** `baseof.html` wraps content in
   `.lcat-layout { display: grid; grid-template-columns: 16rem 1fr }`, with the sidebar as
   the first grid child. But `page.html` (the Work detail layout) defines only `main`, no
   `sidebar` -- so on every Work detail page `<main>` is the *only* grid child and CSS grid
   places it in the **16rem** track. All the detail copy renders in a ~256px column (reads
   as a broken "mobile" layout on desktop). Fix options: have a lone `.lcat-main` span all
   tracks (`.lcat-layout > .lcat-main:only-child { grid-column: 1 / -1 }`), or switch the
   grid to `:has()`-based columns, or give `page.html` an (empty) sidebar block. This is
   the module's single most visible layout bug.
2. **Links in text rely on color alone (WCAG 1.4.1).** `.lcat-card-title a` (and other
   in-text links) are distinguished only by color; contrast against surrounding text is
   < 3:1, so Lighthouse flags `link-in-text-block`. Add an underline (or another
   non-color affordance) to in-text links in `lcat.css`.
3. **Pagination touch targets too small (WCAG 2.5.8).** The Hugo `_internal/pagination.html`
   `.page-link` controls are below the 24-44px target-size guidance and lack spacing;
   Lighthouse flags `target-size`. Add sizing/spacing for `.pagination .page-link` in
   `lcat.css`.

## Acceptance

- Work detail / any sidebar-less page uses the full content width.
- Lighthouse accessibility on a built site is 100 (no `link-in-text-block` /
  `target-size` on the default theme); axe stays clean.
- `exampleSite` visibly improves on desktop with no adopter overrides.

## Related

- `tasks/025` (promote more of the theme into the module so adopters stop reimplementing
  chrome) -- these fixes are the minimum; 025 is the broader direction.

## Resolution

All three fixed at the source in `hugo/assets/lcat.css` (no `baseof.html` change needed):

1. **Full-width sidebar-less pages.** Added `.lcat-layout > .lcat-main:only-child
   { grid-column: 1 / -1 }`. Verified against the built site: on a Work detail page the
   `.lcat-layout` div's only element child is `<main>` (the undefined `sidebar` block emits
   only whitespace), so the rule spans it across both tracks; list/term/taxonomy pages keep
   the two-column grid because they define `sidebar`.
2. **In-text links carry a non-color affordance.** `a` now defaults to
   `text-decoration: underline`; chrome links with their own affordance opt out -- the brand
   and facet rows already set `text-decoration: none`, and tag pills (`.lcat-tags li a`,
   pill background) plus the pagination buttons now do too (both underline on `:hover`).
3. **Pagination touch targets.** `.pagination .page-link` is sized as a `>=2.75rem`
   (44px) inline-flex button with border, padding, and active/disabled states, matching the
   real `_internal/pagination.html` markup (`ul.pagination > li.page-item > a.page-link`,
   `.active`/`.disabled` on the `<li>`).

Verified: `npm run test:a11y` (95 pages, no WCAG 2.1 A/AA violations) and `npm run
test:links` (95 pages, all resolve) both green. `link-in-text-block`/`target-size` are
Lighthouse-only checks (jsdom has no layout); addressed structurally above.
