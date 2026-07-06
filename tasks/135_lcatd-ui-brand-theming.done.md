# 135 -- backend/ui: runtime brand theming (palette override without a fork)

Filed from queerbooks-demo (2026-07-06). Do not let a queerbooks session edit
this repo -- implement here.

## Why

queerbooks-demo is theming its two surfaces with the QLL "Q" logo palette.
The static side is easy: the hugo module documents re-theming via the
--lcat-* custom properties and head-extra.html loads site CSS after the
module stylesheet. The lcatd SPA has the same token discipline internally
(--accent, --ink, ... in backend/ui/src/app.css) but no override hook -- a
deployment cannot re-brand without rebuilding the SPA.

## Suggested shape (pick one)

- LCATD_BRAND_CSS=<path or blob key>: the server serves it at a stable route
  and index.html links it after app.css -- smallest change, full freedom; or
- LCATD_THEME_JSON / config endpoint carrying just token values, injected as
  an inline :root style block (bounded, no arbitrary CSS -- nicer for the
  hosted/lambda story where shipping a file is annoying).

Either way both light and dark values need overriding (app.css presumably
has a dark block like the hugo module's).

## The concrete palette that motivated this (QLL logo)

magenta #df41a5 (slash; deepen to ~#c42a8c for 4.5:1 light-theme links),
steel blue #678cb8, sky #95c2e5, mint #a0d3c7, lavender #d2bed6, mauve
#b599be. queerbooks' static-site tokens (site/assets/qb-theme.css there) are
the reference mapping, including dark variants.
