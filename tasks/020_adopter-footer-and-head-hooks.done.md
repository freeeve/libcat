# 020 -- Add footer/head template hooks so adopters need not shadow baseof

## Context

This task was filed from the **libcatalog-demo** adopter site (Eve's Library) while
implementing its branding pass (that repo's `tasks/002`). It is a request for a small
module change; it is intentionally left uncommitted so a concurrent session working in
this repo isn't disrupted. Do not treat it as in-progress.

The module's `layouts/baseof.html` exposes `head`, `sidebar`, and `main` blocks but has
**no hook** for (a) site-wide `<head>` additions beyond the `head` block (which per-page
layouts must each `define`) or (b) a persistent site footer. To add a footer + site-wide
SEO/meta, an adopter must **shadow the entire `baseof.html`**, which is a vendored copy of
a module template -- exactly what the module's "Overriding" guidance tells adopters to
avoid, because it reintroduces merge pain on every module bump.

## Proposed change

Add two no-op partial hooks to the module `baseof.html`, so adopters can inject without
copying the base:

1. **Footer hook.** Before `</body>`, render an overridable partial, e.g.
   `{{ partial "footer.html" . }}` where the module ships an empty (or minimal)
   `layouts/_partials/footer.html`. Adopters override just that partial.
2. **Head-extra hook.** In `<head>` after the stylesheet, render
   `{{ partial "head-extra.html" . }}` (module ships an empty default) so adopters can add
   canonical/OG/Twitter/JSON-LD/icons without redefining `<title>` or forking the base.
   Optionally also a `{{ block "hero" . }}{{ end }}` slot between the header and
   `.lcat-layout` for full-width intro content on the home layout.

Keep all three additive and empty-by-default so existing sites are byte-for-byte
unchanged. Document them under README "Overriding".

## Why

The demo currently shadows `baseof.html`, `page.html`, and `work-card.html`. Only the
`baseof` shadow is forced by the missing hooks; the page/work-card shadows are for cover
art and are legitimately adopter-specific. These hooks would let the demo drop its
`baseof` shadow entirely and cut its module-bump reconciliation surface.

## Acceptance

- Adopters can add a footer and head metadata via partial overrides alone -- no
  `baseof.html` copy.
- `exampleSite` builds unchanged; existing adopters see no output diff unless they add the
  partials.
- README "Overriding" documents the hooks.

## Done (2026-07-02)

All three hooks added to `layouts/baseof.html`, appended to existing lines so an empty
hook emits **zero** output:

- `{{ partial "head-extra.html" . }}` after the stylesheet; empty default partial
  `layouts/_partials/head-extra.html`.
- `{{ block "hero" . }}{{ end }}` between the header and `.lcat-layout` (no module-side
  define, so empty by default).
- `{{ partial "footer.html" . }}` after the layout `</div>`, before the deferred scripts;
  empty default partial `layouts/_partials/footer.html`.

**Verified:** exampleSite rebuilt and `diff -r` against the pre-change build is **empty**
(byte-for-byte identical, all 91 pages); temporary override partials confirmed the footer
and head-extra render in place and the a11y audit still passes clean. README "Overriding"
documents the three hooks. The demo (libcatalog-demo `tasks/002`) can now drop its
`baseof.html` shadow.
