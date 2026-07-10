# 301 -- browse mode has no pagination -- past the first 60 results the reader is told the total and given no way to reach the rest

Opened 2026-07-10. Split out of tasks/281, which fixed the base set but left this.

## State after v0.122.0

`lcat-browse.js` now filters the whole ranked match set, so a query + facet
returns its true result count. It renders the first `PAGE = 60` of them and says
so: *"showing the first 60 of 8,307 results"*.

There is no way to see result 61. `grep -n offset assets/lcat-browse.js` still
returns nothing, though `RrsCatalog.search(query, offset, len, ...)` takes one and
`RrsCursor` (`headCount`, `page(offset, limit)`, `next`, `loadTail`) exists for
exactly this.

The static Hugo paginator is hidden while browse owns the list (tasks/281): it
pages the server-rendered, unfiltered corpus, so *Next* silently discarded the
reader's query and facets. Hiding it stopped the trap; it did not give the reader
the rest of their results.

## Ask

Drive a pager from the browse state. Sketch, not a prescription:

- `records.getMany(ids.slice(off, off + PAGE))` already renders any window; the
  ids are in hand. A pager over `base.ids`/`fi.ids` needs no new reader call.
- The filtered id set is already fully materialized, so `Math.ceil(total / PAGE)`
  is known and the control can be a real numbered pager rather than "more".
- It needs a URL. A reader who pages, follows a work, and comes back should not
  land on page 1 -- and a shared link to page 4 of a facet selection should work.
  Facet state is not in the URL today either, so this is the larger half.
- a11y: the static paginator's markup and `aria-current` are the reference; the
  browse pager should be the same shape so the page does not change structure
  between modes.
- Restore or replace `setPagerHidden()` accordingly.

## Verification it will need

`hugo/e2e/browse-scope.spec.mjs` already builds a 600-work catalog where a query
matches 400 works and query+facet matches 300 -- both larger than a page. It
asserts the totals; extend it to walk to the last page and check the final page
holds `total % PAGE` cards, that the ids do not repeat across pages, and that
clearing still restores the static list *and* the static pager.
