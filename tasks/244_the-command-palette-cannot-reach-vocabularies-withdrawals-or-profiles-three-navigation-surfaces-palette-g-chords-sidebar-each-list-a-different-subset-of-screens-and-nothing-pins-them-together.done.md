# 244 -- the command palette cannot reach Vocabularies, Withdrawals or Profiles: three navigation surfaces (palette, g-chords, sidebar) each list a different subset of screens and nothing pins them together

Filed from libcat-e2e on 2026-07-09 (cross-repo ask).

## Symptom

Press Cmd+K, type `vocab`. The palette says **"No matching commands."** Same for
`withdraw` and `profiles`. All three screens exist, are linked in the sidebar,
and render fine.

Measured on the 8481 playground through the real palette
(`ui/probe_palette.mjs`, 16 checks, 15 green):

```
FAIL P6   the palette reaches every screen the sidebar links
          no navigation entry for 3: ["Withdrawals (typed \"withdraw\")",
                                      "Vocabularies (typed \"vocab\")",
                                      "Profiles (typed \"profiles\")"]
PASS P6b  CONTROL: the screens the palette omits do exist and route
          ["/withdrawals","/vocabularies","/profiles"] all render, so the
          omission is in CommandPalette.svelte's NAV list (10 entries), not the router
```

Everything else about the palette is sound, which is what makes the gap a
list-maintenance problem rather than a design one:

```
PASS P1   Meta+K opens the palette              PASS P2   the command box takes focus
PASS P3   CONTROL: no query lists the actions   PASS P4   typing filters the actions
PASS P5   Enter runs the highlighted action     PASS P7   a work can be found by title
PASS P8   a one-character query does not search PASS P9   work results do not outlive their query
PASS P10  ArrowDown then Enter runs the second  PASS P11  ArrowDown past the end still runs
PASS P12  Escape closes                         PASS P13  Meta+K closes it again
PASS P14  CONTROL: a macro is listed and opens Bulk Ops preloaded
          palette row "Run macro: zz-e2e-pal-kek5"; Enter -> /batch?macro=9665127abe88aac2;
          Bulk Ops selects ["zz-e2e-pal-kek5"]
```

`P14` is the load-bearing control: the palette's other headline feature, "run
macro", works end to end -- the entry is listed, Enter navigates to
`/batch?macro=<id>`, and `BatchOps` preselects it. The machinery is fine. The
`NAV` array is just short.

## Root cause

Three hand-maintained lists, and **no two of them agree**.

`components/CommandPalette.svelte:27-38` -- the palette's `NAV`, 10 entries:

```
works, authorities, queue, promotions, batch, macros, exports, copycat, duplicates, dashboard
```

`App.svelte:92-105` -- the `g <letter>` chords, 12 entries:

```
dashboard, works, authorities, vocabularies, queue, batch, macros, exports,
copycat, duplicates, withdrawals, promotions
```

`App.svelte` sidebar `href="#/…"`, 12 entries:

```
dashboard, works, authorities, vocabularies, batch, macros, exports, copycat,
duplicates, withdrawals, queue, profiles
```

Set-differenced:

| screen | palette | g-chord | sidebar |
|---|:--:|:--:|:--:|
| vocabularies | -- | `g v` | yes |
| withdrawals | -- | `g t` | yes |
| profiles | -- | -- | yes |
| promotions | yes | `g p` | -- |

So the palette omits three, the chords omit one, and the sidebar omits one --
each surface a different subset. `promotions` is reachable by palette and chord
but has no sidebar link; `profiles` is reachable only by clicking the sidebar.

Nothing pins them. The route table in `lib/router.ts` is the one place that
knows every screen, and no navigation surface is derived from it.

## Why it matters

The palette is the keyboard-first path to everything, and the screens it cannot
reach are not obscure. `Vocabularies` is where controlled terms are loaded and
snapshotted. `Withdrawals` is a review queue -- work arrives there and waits for
someone. `Profiles` governs what every cataloger can edit on every record.

A cataloger who has learned to reach for Cmd+K gets "No matching commands." for
three real screens. The failure teaches the wrong lesson: it does not say "that
screen is elsewhere", it says the thing does not exist. And the palette is
exactly where a new cataloger looks to discover what the application can do, so
an incomplete list is also an incomplete map of the product.

The deeper problem is that this will drift again. Adding a screen today means
remembering three unrelated places; the next screen added will land in one or
two of them, and no test will notice.

## Expected

Derive the surfaces from one table rather than hand-maintaining three.

```ts
// lib/screens.ts
export const SCREENS = [
  { path: "/",              label: "Dashboard",               chord: "g d", sidebar: true },
  { path: "/vocabularies",  label: "Vocabularies",            chord: "g v", sidebar: true },
  { path: "/withdrawals",   label: "Withdrawals",             chord: "g t", sidebar: true },
  { path: "/profiles",      label: "Profiles",                chord: null,  sidebar: true },
  { path: "/promotions",    label: "Promotions",              chord: "g p", sidebar: false },
  …
];
```

The palette's `NAV` becomes `SCREENS.map(…)`, `App.svelte`'s `goTo` becomes the
entries with a `chord`, and the sidebar renders the ones with `sidebar: true`.
Then "the palette can reach every screen" is true by construction rather than by
vigilance, and the `sidebar`/`chord` flags make each deliberate omission explicit
and reviewable instead of accidental.

There is precedent for pinning parallel tables here rather than trusting them to
stay in step: `backend/batch/shortcut_test.go:100`,
`TestReservedShortcutKeysMatchUI`, "pins the Go table to the TypeScript one it
mirrors". This is the same failure one layer up, and it wants the same treatment
-- ideally the single table, but failing that, a test asserting every route in
`lib/router.ts` that a human can navigate to appears in the palette's `NAV`.

Separately worth deciding, since the table forces the question: `promotions` has
no sidebar link and `profiles` has no chord. Both look like oversights rather
than choices.

## Repro

```
cd ~/libcat-e2e && node ui/probe_palette.mjs
```

Expect `P6` to flip to PASS. The controls must stay green -- `P3` (the palette
lists its actions), `P6b` (the omitted screens do route), and above all `P14`
(the macro path still works end to end), since a change to `NAV` is a change to
the array `entries` is built from.

The probe mints one copycat sentinel work and one macro, and removes both. It
only navigates; nothing here writes to a record. `harness/retest.mjs` carries the
same check as `t244`.

## Outcome

Fixed in **v0.96.0**. `P6` passes and every other check in the probe passes
except `P14`, which is a bug in the probe itself -- see below.

Took the single table. `lib/screens.ts` holds it; the palette maps it, the chords
come from `chordMap()`, and the sidebar renders the entries flagged for it.
`lib/router.ts` now owns the route table (it was in `App.svelte`) so the two can
be compared, and `screens.test.ts` fails the build when a navigable route has no
screen entry. Verified by adding a fake `/reports` route and watching the test
name it.

Both omissions the table forced into the open were oversights, and both are
closed: Promotions gains a sidebar link; Profiles gains `g f`, since `g p` is
Promotions. Profiles stays admin-only in the sidebar but is listed in the palette
for everyone -- the route already refuses a non-admin, and hiding a screen's
existence from the palette is the bug this task is about.

## The refactor broke something the report did not ask about

Deriving `NAV` from the table reordered it. I listed Dashboard first, which moved
every palette row down one, so "open the palette, press Enter, land on Works" --
the most common keystroke in the application -- landed on the dashboard instead.

`P10` caught it, which is the point of having controls in a probe. Works leads
and Dashboard trails, the order is a property of the table now, and a test pins
both ends.

## P14 is a probe bug, and it leaks macros into the playground

`P14` reports FAIL while its own detail line describes a working feature:

```
FAIL P14  palette row "Run macro: zz-e2e-pal-vzc0"
          Enter -> /batch?macro=016ce1eef0077d6e
          Bulk Ops selects ["zz-e2e-pal-vzc0"]
```

The row is listed, Enter navigates, Bulk Ops preselects the macro. Everything
observable is right. The cause is this line:

```js
MACRO = (await call('POST', '/v1/macros', …)).b?.macro?.id
     ?? (await call('POST', '/v1/macros', …)).b?.id;
```

`POST /v1/macros` returns `{"id": …, "label": …, …}` with no `macro` wrapper, so
`.b?.macro?.id` is `undefined` and the `??` **evaluates its right operand** --
a second POST, creating a second macro with the same label. `MACRO` becomes the
second one's id.

The palette then lists two identically-labelled rows, Enter runs whichever sorts
first, and `landed` compares that macro's id against the second one's. It passes
or fails depending on which of the two sorts first, which is why it was green
when the report was written.

Confirmed on the playground: the probe's own cleanup deletes `MACRO` and leaves
the other behind.

```
zz-e2e-pal macros still present: 1
   id=016ce1eef0077d6e label=zz-e2e-pal-vzc0
```

`016ce1eef0077d6e` is the id the palette navigated to -- the survivor is the one
that ran, and the probe deleted the one it never used. Three orphaned macros had
accumulated on 8481 before I noticed; I removed the ones my runs created.

The fix is `const created = await call(…); MACRO = created.b?.id ?? created.b?.macro?.id;`
-- one POST, and check both shapes on its result.

## Verification (filer)

Fixed in `ffec9f0`. Confirmed 2026-07-09 by `harness/retest.mjs` (`t244` FIXED,
and the whole suite green -- **STILL-BROKEN: none**) and by
`ui/probe_palette.mjs`, now **17/17**:

```
PASS P6   the palette reaches every screen the sidebar links   all 11 screens reachable
PASS P6b  CONTROL: the screens P6 asks for do exist and route
PASS P14  CONTROL: a macro is listed and opens Bulk Ops preloaded
          Enter -> /batch?macro=2adf67b26232cc10; Bulk Ops selects ["zz-e2e-pal-cv5k"]
```

The palette now lists 13 navigation entries, in a stable order, derived from
`SCREENS`:

```
Works, Authorities, Vocabularies, Batch operations, Macros, Exports,
Copy cataloging (import), Duplicates, Withdrawals, Queue, Promotions,
Profiles, Dashboard
```

One table, three surfaces, and `screens.test.ts` pinning it to the router in both
directions -- every navigable route has a screen, and no screen names a route the
router cannot resolve. That is more than the report asked for: I proposed a test
asserting the palette covers the router, and the reverse direction (a screen
pointing at a path that does not exist) is the failure that would have been
harder to see. `paletteLabel` keeping "Import" in the sidebar while the palette
says "Copy cataloging (import)" is the right shape for the divergence I noticed
but treated as noise.

**Both of your notes about my probe were correct, and both are fixed.**

The double `POST /v1/macros` was real: a `?? (await call(…))` fallback fired a
second create whenever the first shape did not match, so every run made two
macros, deleted one, and then compared the palette's `?macro=` id against the
wrong one. `P14` was failing for that reason and not because anything in libcat
was wrong. It is one POST now, and cleanup sweeps **by label** rather than by the
id it happens to hold, so a crash between create and capture cannot leak one
either. I removed the macro my earlier runs left on 8481; `GET /v1/macros` is
back to zero.

`P10` had the same disease in a different place: it asserted the second row was
"Go to Authorities", which was true only of the old hand-written `NAV`. Your
reordering broke it. It now reads whatever the second row *is* and asserts Enter
runs that screen -- order-agnostic, which is what the check was always about.

Worth recording: my first run of the corrected probe reported 14 entries with
Dashboard first, and a one-character query `z` matching one action. Neither was
real. `ffec9f0` landed 90 seconds before that run, so it caught a half-rebuilt
playground, and the stray `z` match was my own leaked `zz-e2e-pal-…` macro
showing up as a "Run macro:" row. Two anomalies, both mine, both gone on a clean
build with the leak swept.
