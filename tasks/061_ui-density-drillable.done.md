# 061 -- Compact drillable list layout; screen state survives drill-in

## Context

Phase 3 of the admin UX overhaul. List screens lose their query, results, scroll, and
selection every time a cataloger drills into a record and returns (state is component-
local); rows are two lines and airy; every screen is capped at the 62rem column even when
triaging wide rows. The tag-review app keeps selection in stores and renders one-line rows,
which is what makes it fast.

## Scope

- Tokens in app.css: `--row-h`, `--fs-row`, `--fs-meta`, `--control-h-sm`, staged/ok/danger
  `color-mix` tints; `main.wide` (96rem) opt-in; `.split` two-pane helper; compact one-line
  screen headers; `.rowlist` rows get the density defaults.
- `lib/screenState.svelte.ts`: keyed module `$state` objects (`screenState(key, init)`),
  `resetScreenStates()` wired into sign-out.
- WorkSearch + Authorities: query/results/selection live in screenState; fresh (<60s)
  renders instantly with selection intact, stale refetches in the background.
- Queue: filters/items/cursor/selection in screenState; decision store gains `persistKey`
  (sessionStorage) so staged decisions survive reload and drill-in; `Enter`/`o` opens the
  selected work; `u` unstages; rows compact to one line with the action cluster only on
  the selected row; staged rows tint.
- `main.wide` on works/authorities/queue/batch/exports/copycat/duplicates.

## Acceptance

- Return from a work editor to /works lands on the same query, rows, and selected row.
- Reloading mid-triage keeps staged queue decisions.
- Rows are single-line (~30px); axe suite green; dark-mode tints verified by screenshot.
- `npm run check`, `npm run test`, `npm run build` green.
