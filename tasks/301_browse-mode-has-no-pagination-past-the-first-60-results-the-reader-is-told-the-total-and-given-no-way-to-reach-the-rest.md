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

## Measured evidence (libcat-e2e, 2026-07-10)

Driven read-only against the published queerbooks OPAC on `:8502` by
`harness/probe_opac_browse_pagination.mjs` (4/9); retested by `t301`. tasks/312 was filed
against this and is a duplicate -- its measurements are folded in here.

```
corpus                                62602 works, 2609 static pages of 24
search "lesbian"                       9923 matches
browse renders                           60 cards      <- 99.4% of the result set discarded
visible pager links, query active         0
next / more / load-more controls          0
scrolling to the bottom              60 -> 60 cards
```

**The engine paginates today**, so the cap is the UI's choice and not a reader limitation:

```
search("lesbian", 0,  60, 0, [])   -> 60 ids
search("lesbian", 60, 60, 0, [])   -> 60 ids, 0 shared with the first call,
                                       and exactly ids[60..120] of the 9923-id full set
```

Two things the original note does not cover:

**The facet-only path caps too, and that is the path a reader actually takes.** `refresh()`
restores the static list only when the query **and** the filters are empty, so clicking one
subject with an empty search box hands `allIds` to `filterIds` and slices at `PAGE`:

```
no query, facet "LGBTQ+ people" only   21792 matches, 60 cards, 0 pager links
"lesbian" + "LGBTQ+ people"             8307 matches, 60 cards
```

On a public catalog the reader never types -- they click a subject. **21,792 works about LGBTQ+
people; 60 reachable.**

**Un-hiding the static pager would not help.** Measured: searching from `/works/page/2/` renders
the same first 60 cards as searching from `/works/`. The static pager pages the server-rendered
corpus; browse's result set has no relationship to `/page/N/`. So `setPagerHidden()` is not the
thing to restore -- the pager has to be driven from browse state, exactly as the Ask says.

Only the unfiltered a-to-z list paginates, 2609 pages deep. **The catalog paginates right up
until the reader expresses an interest in something.**

## Verification it will need

`hugo/e2e/browse-scope.spec.mjs` already builds a 600-work catalog where a query
matches 400 works and query+facet matches 300 -- both larger than a page. It
asserts the totals; extend it to walk to the last page and check the final page
holds `total % PAGE` cards, that the ids do not repeat across pages, and that
clearing still restores the static list *and* the static pager.
