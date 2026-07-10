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
