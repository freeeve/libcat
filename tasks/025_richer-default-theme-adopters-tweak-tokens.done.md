# 025 -- Ship a fuller default theme; adopters restyle via tokens, not reimplementation

## Context

Filed from the **libcatalog-demo** adopter (uncommitted; a request, not in-progress).
Direct adopter feedback: *"the initial theming should be the base module theme, with
updates on top in the adopter site."* Today `assets/lcat.css` is an intentionally minimal
reference. Building a credible-looking public catalog (the demo) required the adopter to
write ~250 lines of CSS re-doing chrome the module could own -- and to discover layout/a11y
defects (`tasks/024`) the hard way. The base theme should look good on desktop by default;
adopters should only override brand tokens + copy.

## Proposed direction

1. **Promote reusable chrome into `lcat.css`** with sensible desktop defaults: card layout
   with a cover slot, result spacing/typography scale, facet-panel treatment, a
   wider-but-readable content measure (the minimal theme reads as "narrow/mobile" on wide
   screens), and refined focus/hover states. Keep it token-driven.
2. **Expose a documented token surface.** The theme already uses `--lcat-*` custom
   properties; formalize them (palette, surfaces, radius, shadow, max-width, spacing) as
   the supported override API so an adopter re-themes by re-setting variables, not by
   re-implementing components.
3. **Add opt-in structural hooks** so adopters layer brand chrome without forking layouts:
   a `hero` block above the layout and `footer` / `head-extra` partials (see `tasks/020`),
   and forward extra Work fields (cover/rating) to page params (see `tasks/022`) so cover
   art and ratings are first-class rather than adopter-bolted.
4. **Cover art as a first-class, optional feature.** Define a cover slot + graceful
   lettered fallback in the module (the demo built this; note the fix that cover `<img>`
   needs explicit CSS width AND height, or the HTML height hint distorts the box).

## Acceptance

- A brand-new adopter gets a desktop-credible catalog by importing the module and setting
  a handful of tokens + copy -- no component-level CSS, no layout forks.
- The demo's override stylesheet shrinks to brand identity only (palette, logo, hero/about
  copy), validating the split.

## Related

- `tasks/020` (baseof hooks), `tasks/022` (adapter extra params), `tasks/024` (the
  concrete defects this direction subsumes).

## Resolution

Promoted the **generic catalog chrome** into the module and formalized the token surface;
the library-site *shell* (nav/hero/events/docs/footer) stays adopter-owned by design, since
it composes a whole website around the catalog rather than styling the catalog itself.

- **Token override API** (`hugo/assets/lcat.css` `:root`): added `--lcat-accent-ink`,
  `--lcat-surface`, `--lcat-surface-alt`, `--lcat-radius`, `--lcat-shadow`, `--lcat-gap`,
  and widened `--lcat-maxw` (68->72rem). Documented as a table under README "Theming" -- an
  adopter re-brands by re-setting these, not by reimplementing components.
- **Richer defaults, all token-driven:** header on a surface with responsive
  `clamp()` padding, facet sidebar as a raised panel (`.lcat-sidebar`), result cards as a
  flex row with a body wrapper, larger card titles, tag chips on `--lcat-surface-alt` with a
  border, accent-ink headings/brand, and `text-underline-offset` on links. (The 024 defects
  -- lone-`main` width, link underline, pagination target size -- are already in and subsumed.)
- **Cover art as a first-class optional feature** (proposed direction §4): new
  `layouts/_partials/lcat-cover.html` renders an `<img>` when a Work carries a `cover`
  param (via the 022 `extra` passthrough) or a graceful lettered placeholder otherwise
  (rune-aware first character; `aria-hidden`). Wired into `work-card.html` (card slot) and
  `page.html` (floated detail cover). Gated on `[params] covers = true` (off by default =
  unchanged). CSS sets BOTH width and height per variant so the HTML size hints don't
  distort the 2:3 box (the demo's noted fix). `alt=""` keeps the image decorative since the
  adjacent title names the Work.

Verified: `exampleSite` opts covers on (placeholders, since its works ship no cover URL) and
builds credibly; a real `https://` cover URL emits a clean `src` (Go's `html/template`
passes it; only `data:` URLs are filtered, so covers use normal image URLs -- no `safeURL`
needed/added). `npm run test:a11y` (95 pages) and `npm run test:links` (95 pages) stay
green; covers=false emits zero cover markup.

The demo's upstream-flagged catalog chrome (`assets/lcat-theme.css` sidebar panel, card
covers, pagination/link a11y, desktop widths) is now module-owned, so that adopter override
can shrink toward brand identity. Its library-site shell remains adopter chrome as intended.
