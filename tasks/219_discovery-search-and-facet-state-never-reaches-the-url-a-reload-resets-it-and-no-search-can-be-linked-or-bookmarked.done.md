# 219 -- discovery search and facet state never reaches the URL: a reload resets it and no search can be linked or bookmarked

Filed from libcat-e2e on 2026-07-09 (cross-repo ask).

## Symptom

The works screen keeps its query and its facet selections entirely in memory.
The hash never changes, so a reload drops everything and a search cannot be sent
to anyone.

Measured through the real SPA on 8501 (`node ui/probe_opac_state.mjs`,
2026-07-09). Read-only: only navigation, the search box, and facet checkboxes.

| # | Check | Result |
|---|---|---|
| S1 | search narrows | `q=lesbian` → `matched=1547` of 62602 |
| S2 | **search reflected in the URL** | hash is still `#/works` — no `q=` |
| S3 | facet narrows further | `+ no holdings` → `matched=1526` |
| S4 | **facet reflected in the URL** | hash still `#/works`, unchanged by the facet |
| S5 | **state survives a reload** | `1526` → **62602**; search box back to `""` |
| S6 | **a search URL is shareable** | `#/works?q=lesbian&holdings=none` → `matched=62602`, box `""` |
| S8 | Back restores the search (control) | `1547` → `1547`, box still `lesbian` ✅ |
| S9 | Back restores the loaded page (control) | 100 rows → 100 rows ✅ |

S8 and S9 are the control, and they matter: in-app navigation **does** preserve
the search. This is not "the works screen forgets its state." It forgets it only
across the URL — reload, bookmark, paste-to-a-colleague.

The deep link in S6 is not rejected, either. `#/works?q=lesbian&holdings=none`
renders the unfiltered catalog, so a shared link silently shows the wrong result
set rather than failing.

## Root cause

State lives in a module-level map, not the location:

- `backend/ui/src/lib/screenState.svelte.ts:9` — `screenState(key, init)` keeps
  each screen's state in a `Map` held for the life of the page. That is what
  makes Back work (`resetScreenStates()` at `:20` clears it only on sign-out).
- `backend/ui/src/screens/WorkSearch.svelte:23` — `const st = screenState("works", …)`
  holds `q`, `filters`, `facets`, `loadedAt`. Nothing in the file reads or writes
  `location`; the only navigation is `navigate("/works/" + id)` at `:228` when a
  result is clicked.
- `backend/ui/src/lib/router.ts:50` — `navigate()` sets `location.hash`, and the
  shell's `hashchange` listener matches the route by path alone. A `?query`
  string on the hash is neither parsed nor rejected, which is why S6 renders the
  unfiltered list.

So the persistence layer the screen already relies on is deliberate and works
(tasks/191 leans on it to keep resolved subject labels across a mount). The URL
is the half that was never wired.

## Why it matters

The facet rail on 8501 carries **82 values across 7 groups**. Building a
selection — "no holdings" ∧ "missing subjects" → the 27,628 records that need
work — takes real clicks, and a cataloger loses the whole stack on any reload.
Reloads are not rare on this surface: they follow a publish, a batch run, a
session timeout, or an accidental ⌘R.

It also means a queue of work cannot be handed off. There is no way to send
someone "here are the 27,628 records missing subjects" as a link, or to bookmark
a saved view, or to open two facet stacks in two tabs and compare. Saved queries
(`/v1/queries`) cover the batch-selection case but do not drive this screen.

And a shared link is actively misleading rather than merely inert: S6 shows
`#/works?q=lesbian&holdings=none` rendering all 62,602 works with an empty
search box. A recipient has no signal that the filter was dropped.

## Expected

The works screen syncs `q` and `filters` to the hash, and reads them back on
mount:

- On search or facet change, replace the hash (`history.replaceState`, so the
  Back stack is not flooded with one entry per keystroke) with
  `#/works?q=…&holdings=none&needs=subjects`.
- On mount, parse the hash's query string into `st.q` / `st.filters` before the
  first fetch, so a deep link and a reload both restore the view.
- Keep `screenState` as the fast path for in-app Back — the URL is the durable
  copy, not a replacement.

Repeated filter values within a group already work as repeated query params on
the API (`sources=a&sources=b`), so the hash and the API can share a shape.

If deep links are deliberately out of scope, the router should at least not
silently drop a query string it does not understand.

## Repro

```
cd ~/libcat-e2e && node ui/probe_opac_state.mjs   # S2, S4, S5, S6 FAIL; S8, S9 PASS
```

Read-only against 8501. Toggles facets and types in the search box; clicks no
mutating control.

## Not bugs (verified clean this cycle)

The rest of the discovery surface reproduces every number recorded for it
(`node harness/probe_opac_regress.mjs`, 18/18): 62,602 works; `holdings=none`
advertising 57,112 and matching exactly; cross-group AND landing on 27,628;
`sources=` filtering to 22,743 while `source=` is ignored; `q=lesbian` at 1,547;
search composing with a facet down to 1,526; pagination advancing with zero
overlap; an unknown facet value narrowing to 0 rather than being dropped; and
`sort=` still not a server capability. The UI sweep (`ui/explore_opac.mjs`) is
green too, including Load more and the record tabs, with no console errors.

## Outcome

Fixed by `de23874` (feat(ui): works search and facet state round-trips through
the URL). Verified 2026-07-09 against **8481**, which carries the new binary.

`WorkSearch.svelte:157` adds `syncURL()`, mirroring `st.q` and `st.filters` into
the hash with `history.replaceState` -- so the Back stack does not gain an entry
per keystroke and the shell never re-routes. `onMount` (`:173`) parses the hash
first and lets a deep link win over remembered state, falling back to the
`screenState` map for a plain `#/works`. `lib/worksurl.ts:23` (`parseWorksQuery`)
reads repeated params per group with `query.getAll(key)`, matching the API's
`sources=a&sources=b` shape.

Round-trip, measured through the SPA:

```
search "frog"        -> hash #/works?q=frog
+ facet holdings=none -> hash #/works?q=frog&holdings=none
reload                -> matched 2 -> 2, search box "frog"   (was 2 -> 71, box "")
```

Deep links open a fresh mount correctly -- each URL loaded in its own page load,
so `onMount` really runs:

| URL | matched | search box | facets checked |
|---|---|---|---|
| `#/works` | 71 | `""` | 0 |
| `#/works?q=frog` | 2 | `frog` | 0 |
| `#/works?q=zzxqqnope123` | **0** | `zzxqqnope123` | 0 |
| `#/works?q=frog&holdings=none` | 2 | `frog` | **1** |

Distinct queries yield distinct counts, so the parameters are genuinely applied
rather than coincidentally matching. The controls still hold: in-app Back
restores the search (`2 -> 2`, box `frog`) and the loaded page.

`harness/retest.mjs` reports `219 FIXED`; `node ui/probe_opac_state.mjs` passes
S2/S4/S5/S6 on 8481.

Note, not a defect: **8501 still fails this probe** (hash stays `#/works`, reload
drops 1526 -> 62602). That instance is running an older `lcatd` with the previous
SPA embedded in the binary; it picks the fix up on its next rebuild. Called out
here so the next cycle does not read the lag as a regression.
