# 280 -- admin work search shows tombstoned works undifferentiated: hide them by default behind a 'show tombstoned' checkbox

Opened 2026-07-09, from Eve. The design call is hers: **do not show tombstoned
works by default; put them behind a "show tombstoned" checkbox.**

## What is true today

`GET /v1/works` returns every work, tombstoned or not, and every summary row
already carries the flag:

```
$ curl -s -H "Authorization: Bearer $TOK" 'localhost:8481/v1/works?limit=3' | jq '.works[0] | keys'
["Contributors", "ISBNs", "Subjects", "Tags", "Title", "Tombstoned", "WorkID"]
```

Nothing in the admin SPA reads it. `Works.svelte` never mentions `Tombstoned`;
the only UI that knows the concept is `VisibilityPanel.svelte`, which sets it on
a record you are already looking at.

The result on the demo playground right now:

```
50 works listed, 49 tombstoned
  - w00jsjpd0e6s3q 'rt232b-obv0r'
  - w011bkh1obpijk 'rt232b-69qmk'
  - w01btprv93jo00 'rt239b-8eq4d'
  ...
```

A cataloger opening work search sees fifty rows, one of which is a real book.
Forty-nine are retired e2e sentinels, and nothing on the row says so.

The OPAC is not affected -- `lcat project` drops tombstoned works, which is why
the public site shows ~34 works while admin shows 50+. The divergence is the
tell: the projection has an opinion about tombstones and the admin list has none.

## Why it matters

A tombstone is the catalog's way of saying *this record is retired; here is where
it went*. Retired records still need to be findable -- to un-retire, to check a
redirect, to audit a merge -- so removing them from search is wrong. But they are
not what a cataloger is looking for, and showing them by default:

- **buries live records.** Any listing capped at N spends its N on the dead.
- **invites accidental editing.** Nothing stops a cataloger opening a tombstoned
  work and adding holdings to it.
- **makes the merge/withdraw workflows lie.** `tasks/078`'s withdrawal queue and
  the merge redirect both assume the retired record is out of the way.

## Expected

- **A `show tombstoned` checkbox on the work search screen, off by default.**
  Off: tombstoned rows are omitted. On: they appear, visibly marked.
- **Mark them when shown.** A badge, and the muted/strikethrough treatment the
  design language already uses for suppressed content. A row that is retired must
  never look like a row that is live.
- **Filter server-side, not in the client.** A client-side filter over a paged
  response shows "10 results" and renders 1. The listing route should take the
  flag (`?tombstoned=include|only|exclude`, defaulting to `exclude`) so paging and
  counts stay honest. `workindex` already holds `Tombstoned` on the summary, so
  the filter costs nothing.
- **Decide what the default does to a direct link.** Navigating to a tombstoned
  work by id must still work, and should say plainly that the record is retired
  (and where it redirects). That is `VisibilityPanel`'s job and it already does
  half of it.
- **`only` is worth having**, not just `include`: "show me what I retired last
  week" is the audit question, and it is the one the withdrawal queue answers
  badly today.

## Open questions for Eve

- Should **suppressed** works get the same treatment? They are hidden from the
  OPAC but not retired, and the same argument applies more weakly. Suggest: keep
  them visible, badge them, and revisit.
- Should the checkbox persist across sessions (screen state) or reset each visit?
  `screenState` already persists the work search's query and facets, so persisting
  would be consistent -- but a sticky "show tombstoned" is a foot-gun.

## Notes

Cheap to verify once built: the playground has 49 tombstoned works and 1 live one,
which is a better fixture than anything worth constructing.

## Outcome

Shipped in **v0.113.0** (`79fd9a4`). `GET /v1/works` takes `?tombstoned=exclude|include|only`, defaulting to
`exclude`, and the work search screen has a "Show tombstoned" checkbox, off by
default.

Two things in the report above are wrong and worth correcting for the next
reader. The screen is `WorkSearch.svelte`, not `Works.svelte`. And the rows
*were* already badged -- `WorkSearch.svelte` renders a `tombstoned` flag chip in
`--danger` alongside `suppressed` and `withdrawn`. The badge was never the
missing piece; the filter was. Nothing hid the rows, so on a catalog that had
retired anything the badge just meant a wall of red.

### The fixture was worse than measured

The "49 tombstoned, 1 live" figure was already stale when it was written. After
one more e2e sweep, the live playground:

```
[default]           total=38    matched=38    {public: 37, suppressed: 1}
[tombstoned=include] total=2595 matched=2595  {tombstoned: 2557, public: 37, suppressed: 1}
[tombstoned=only]   total=2557  matched=2557  {tombstoned: 2557}
[tombstoned=yes]    400
```

2557 retired sentinels above 38 real records. The default view is 1.5% of what it
used to be.

### Server

`tombstoneMode(raw)` returns the `keep func(ingest.WorkSummary) bool` the request
searches under, and `nil, false` for anything unrecognized -- a `400`, not a
silent fall back to the default. A client that asked for `only` and got `exclude`
would be shown an empty list and conclude the records were gone.

The filter runs **before the query match and before the facet counter**, and
`total` now counts inside that loop instead of being `len(all)`. That is the whole
point: `total`, `matched`, the paging window and the facet rail all describe the
same set. The facet rail stops offering a `tombstoned` bucket that would match
nothing.

Mutation-proven, `go test -count=1`:

| mutation | fails |
|---|---|
| default `""` maps to `include` | 3 tests |
| filter moved after the query/facet counting | 4 tests |
| unknown mode falls back to the default instead of 400 | 1 test |

`reservedWorkParams` gains `tombstoned`, so a deployment's extras facet cannot be
named `tombstoned` and swallow the parameter (`TestTombstonedIsAReservedWorkParam`).

### Client

`fetchWorks` takes the mode as its own argument rather than folding it into
`WorkFilters`. It cannot go there: `WorkFilters` means "unselected = match all",
and the default here is "match all **except** tombstoned", which that shape has no
way to say.

`setShowTombstoned(false)` also drops a selected `visibility=tombstoned` facet.
Those two settings can only ever match nothing together, and an empty list reads
as "the records are gone", not as "your two filters disagree". Only that value is
dropped -- an unrelated `visibility=suppressed` selection is the cataloger's.

Mutation-proven, `vitest`:

| mutation | fails |
|---|---|
| mode pinned to `include` | 3 of 6 |
| `fetchWorks` never sends the parameter | 4 of 6 |
| the facet-cleanup branch removed | 2 of 6 |

### The open questions, answered as asked

- **Suppressed works stay visible and badged.** A suppressed record is hidden from
  the public, not retired; it is exactly the sort of record a cataloger opens work
  search to find. Its badge already exists. The playground has one, and it stays
  in the default view.
- **The checkbox does not persist across sessions.** It lives in `screenState`, so
  it survives a trip into a work and back -- otherwise the checkbox would read
  "off" above rows that are still showing -- and it resets on reload and on
  sign-out. It is deliberately not in the hash and not in `localStorage`: a
  "show tombstoned" that remembered itself would quietly re-bury the catalog, and
  a deep link carrying it would do that to someone else.

### Direct links are unaffected

Checked rather than assumed, against a retired playground record:
`GET /v1/works/{id}/doc` -> `200`, `GET /v1/works/{id}/visibility` ->
`{"tombstoned":true,...}`. `VisibilityPanel` already says the record is retired.
Nothing about hiding it from a *list* hides it from a *link*.

### Adoption

Server plus SPA. Rebuild the playground.

- `GET /v1/works` **excludes tombstoned records by default.** A client that
  counted on seeing them must send `?tombstoned=include`.
- `total` under the default is the number of live records, not everything in the
  catalog.
- Unrecognized values are `400`, not ignored.
