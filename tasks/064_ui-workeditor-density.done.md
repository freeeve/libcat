# 064 -- Compact sticky work-editor header, tab keys, two-column fields

## Context

Phase 6 of the admin UX overhaul. The work editor stacked everything (title block,
visibility, banners, tabs, fields, instances, items, passthrough, macro bar, save bar) in
one 62rem column -- heavy vertical scrolling, and the record identity scrolled away.
Staged-ops/dry-run/If-Match flow stays untouched (`lib/editor.ts` unmodified).

## Scope

- Sticky one-line header (title ellipsized · id chip · etag prefix · staged counter ·
  busy dot) under the double rule; VisibilityPanel folds into a details beneath it.
- Tab keys `1/2/3` = Native/MARC/History in the `editor` scope.
- DiffPreview pushes an `editor-preview` sub-scope while open so Escape closes it first.
- `main.wide` + ProfileForm fields flow into `repeat(auto-fit, minmax(24rem, 1fr))`
  columns (DOM order preserved); uniform field hairlines.

## Acceptance

- axe suite green; check/test/build green; editor legend shows 1/2/3 tabs · p preview ·
  mod+s save; Escape dismisses the diff preview without leaving the screen.
