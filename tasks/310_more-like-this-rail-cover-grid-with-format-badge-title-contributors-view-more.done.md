# 310 -- more like this rail -- cover grid with format badge, title, contributors, view more

Opened 2026-07-10, from a design reference the maintainer supplied: an 8-across
cover grid, an AUDIOBOOK badge on the one audiobook, bold titles over dimmed
contributors-with-roles, and a centred "View more" pill.

This supersedes the visible `Shares: x · y · z` line tasks/302 shipped one day
earlier.

## Outcome

Shipped in `627ea2f`.

The rail is a `grid-template-columns: repeat(auto-fit, minmax(7rem, 1fr))` shelf:
eight columns at 1280px, two on a 390px phone, no breakpoints to keep in sync with
the neighbour limit and no horizontal overflow. Each tile is one `<a>` wrapping the
cover, the title and the contributors -- a cover and a title that are separate links
are two tab stops for one destination.

Three decisions worth recording, none of them asked for:

**The data comes from `catalog.json`, not from a widened sidecar.** `similar.json`
carries `id`, `title`, `shared` -- the scorer's output. A cover path and a
contributor list are the *neighbour's own* data, so the adapter indexes the catalog
by id once and reads them out. Widening the sidecar would have duplicated every
cover path once per rail that names the Work, and staled it independently of the
catalog it was copied from.

**The badge names the carrier only when it is not the unmarked case.** A print book
is what a catalog is assumed to hold, so a `BOOK` badge on seven of eight tiles
spends the reader's attention on nothing. A Work with two carriers gets no badge
rather than an arbitrary first one.

**`View more` needed a decision, and it is `[params] similarShown` (default 8).**
`--similar` decides how deep the pool is; `similarShown` decides how much of it the
page opens with. Without the split, a deployment that wants a reveal button has to
render 24 tiles. The button and `lcat-similar.js` are emitted only when the sidecar
is deeper than the cap -- a button that reveals nothing is worse than none.

The extras are **not** `hidden` in the markup. Without JavaScript there is no way to
reveal them, so a `hidden` attribute would bury half the rail for a reader whose
script did not run. The script hides them and unhides the button in one pass, so
failing to run leaves the reader with more than they asked for rather than less.
Verified with `javaScriptEnabled: false`: 12 of 12 tiles painted, 0 buttons visible.

The `Shares:` line is not deleted -- it is `lcat-visually-hidden` on each tile, as is
the "suggested automatically" note. tasks/284's claim was that a rail nobody can
explain is worse than none, and tasks/296/302 spent two releases making that line
correct in every language. The covers took its room, not its job. Everything those
tasks proved about it is still pinned by the same assertions.

### Verified in a browser, not in the markup

Twelve neighbours, cap 8, real cover art, over HTTP (`file://` resolves the site's
root-relative CSS against the filesystem root and renders an unstyled page -- every
grid measurement taken that way is a measurement of nothing, which is how the first
run reported "1 column, 0 covers painted"):

```
light: 12 tiles, 8 columns, covers painted: 8
dark:  12 tiles, 8 columns, covers painted: 8
phone:  2 columns, horizontal overflow: false
reveal: 8 of 12 tiles painted on load, button visible: true
reveal: 12 tiles painted after click, button removed: true, focus on: ...--extra
no-js:  12 of 12 tiles painted, 0 buttons visible
```

Visibility is read as painted height, never as `!el.hidden` -- that assertion went
green over a visible, clickable paginator for two releases (tasks/303).

Gates: 26 seam checks, `npm run test:js` (28 + 7 + 7), a11y audit clean over 124
pages, link check clean, e2e 17/10/16/18.

### Mutation-tested

- badge from the first format, always: 2 fail (the audiobook *and* the two-carrier tile)
- neighbour `cover` never looked up: 1 fail
- role parenthetical rendered unconditionally: 1 fail
- extras `hidden` in the markup: 1 fail
- button rendered whether or not there is anything to reveal: 1 fail

Two of my own test helpers were wrong, and both were found by a mutation rather than
by reading:

- `tiles()` split on `<li class="lcat-similar-item">`, closing quote included. A tile
  past the cap carries a second class, so the split returned **zero** extras while
  every "the extras are not hidden" assertion went green against an empty list.
- `assert(page.includes("lcat-similar.js"))` can never hold: the asset is
  fingerprinted to `lcat-similar.<hash>.js`. The check passed in both worlds. It now
  matches the fingerprinted name.

### Not done

The admin SPA's `SimilarPanel` still renders the compact `shares` list. It is a
sidebar in a cataloging tool, not a shelf, and a cover grid there would push the
editor's fields below the fold. Left alone deliberately.
