# 315 -- dark mode: `--danger` has no paired ink token, so Execute and both FAILED badges paint white on salmon at 2.42:1

Filed from libcat-e2e on 2026-07-10 (cross-repo ask).

## Symptom

In dark mode, the three surfaces that signal danger are the three that become
hard to read. Measured on `:8481` against the shipped stylesheet
(`harness/probe_admin_dark_mode.mjs`, 6/8):

```
                                              light    dark    AA needs
.button--danger  "Execute"      /batch        6.25:1   2.42:1    4.5:1
.badge[data-status=FAILED]      /exports      6.25:1   2.42:1    4.5:1
.badge[data-status=FAILED]      /vocabularies 6.25:1   2.42:1    4.5:1
```

All three paint `rgb(255,255,255)` on `rgb(224,148,121)`. 2.42:1 fails the
normal-text threshold (4.5:1) and also the large/bold threshold (3:1), so the
finding does not depend on which one applies.

Everything else is clean, and that is what makes this specific: **0 AA failures
across all 13 screens in light mode, and 0 in dark mode** once these three are
set aside. The dark theme is careful work. It has one hole.

## Root cause

`--accent` ships with a paired ink token, redefined per theme. `--danger` does not.

```css
/* app.css:18-19  (light) */        /* app.css:290-291  (dark) */
--accent-ink: #ffffff;              --accent-ink: #10261c;
--danger:     #a34224;              --danger:     #e09479;
/*  no --danger-ink  */             /*  no --danger-ink  */
```

So every surface that needs ink on `--danger` hard-codes `#fff`. There are
exactly three `color: #fff` declarations in the whole SPA, and they are exactly
these three sites:

```
app.css:163                     VocabSources.svelte:389      Exports.svelte:476
```

```css
/* app.css:160-164 */
.button--danger { background: var(--danger); border-color: var(--danger); color: #fff; }
```

Twice more the broken rule sits *directly beneath* a sibling that gets it right.
`Exports.svelte:468-477` and `VocabSources.svelte:381-390` are character-for-character
identical here:

```css
.badge[data-status="DONE"]   { background: var(--accent); …  color: var(--accent-ink); }  /* inverts */
.badge[data-status="FAILED"] { background: var(--danger); …  color: #fff; }               /* does not */
```

`--danger` **has** to brighten for dark, because it is also an *ink* token:
`.error` is `color: var(--danger)` (`app.css:126`, and again at `App.svelte:315`,
`BatchOps.svelte:600`, `Queue.svelte:475`). At `#a34224` that ink would sit at
1.8:1 on the `#171b19` surface; `#e09479` holds 7.19:1. The brightening is
correct for the ink role and fatal for the background role.

**One token, two roles, and a single dark value cannot satisfy both.** The
comment at `app.css:279-281` shows the reasoning was applied deliberately to the
provenance inks -- *"provenance inks brighten to hold AA on dark"* -- and
`--danger` was brightened the same way, without noticing it was also a background.

## Why it matters

Look at *which* three surfaces these are.

- **Execute** on `/batch` is the button that CAS-writes a bulk edit across the
  entire selection. It is styled `--danger` precisely *because* it is the
  irreversible one, and `BatchOps.svelte:402` gates it behind a fresh dry run for
  the same reason. It is unreadable exactly when a cataloger is checking what
  they are about to rewrite across thousands of works.
- **`FAILED`** on `/exports` and `/vocabularies` is how a librarian learns a job
  did not finish. The status a user most needs to read is the one rendered at
  2.42:1.

Dark mode is not an edge case: `lib/theme.ts:initTheme` follows
`prefers-color-scheme` on first visit. A cataloger whose OS is in dark mode has
never seen these three surfaces any other way, and has no reason to suspect a
light mode would show them clearly.

And AA is this stylesheet's own stated bar (`app.css:279`), so this is a miss
against its own standard, not an external one being imposed on it.

## Expected

Give `--danger` the ink token `--accent` already has, and use it at the three sites:

```css
:root                    { --danger: #a34224;  --danger-ink: #ffffff; }
html[data-theme="dark"]  { --danger: #e09479;  --danger-ink: #10261c; }
```

`#10261c` on `#e09479` measures **6.60:1**. Then `color: var(--danger-ink)` at
`app.css:163`, `Exports.svelte:476` and `VocabSources.svelte:389`.

Two notes for whoever takes it:

- Do **not** fix this by darkening `--danger` for dark mode. It is load-bearing
  as ink in four places and would fall to ~1.8:1 against the dark surface.
- `grep -rn "color: #fff"` is the entire audit surface. After this change no
  hard-coded ink should remain anywhere a token supplies the background, and the
  probe's `D5` enforces that over the shipped stylesheet rather than over a list.

## Repro

```
cd ~/libcat-e2e && node harness/probe_admin_dark_mode.mjs   # 6/8: T1-T4, D1, D2 pass; D4 + D5 fail
node harness/retest.mjs                                      # t315
```

`D4` drives the real UI: it dry-runs a batch on `/batch` -- which writes nothing
(`batch.go:283`, *"DryRun reads and diffs without writing"*, and the audit entry
rides on an execution) -- waits for **Execute** to become enabled, and measures the
live button. `D5` audits the shipped stylesheet through the CSSOM with the dark
tokens substituted, and reports only rules that hold AA in light mode, so it
cannot be inflated by a pre-existing low-contrast rule.

Screenshot: `shots/admin-dark-batch-execute.png`.

## Two harness notes, since they shaped what the checks assert

**A disabled control is WCAG-exempt.** `D1`'s first version reported this very
button while it was *disabled*. WCAG 1.4.3 exempts inactive components, and
`.button:disabled` is `opacity: .45`, so that report was not valid even though the
colour pair was. `D1` now skips disabled controls, and `D4` exists to measure
Execute in the state a cataloger actually clicks.

**`D5` was green until it was debugged, because it never read a single rule.**
The CSSOM walk began `if (r.cssRules) { walk(r.cssRules); continue; }`. Since
nested CSS shipped, a plain `CSSStyleRule` *also* exposes an empty, truthy
`cssRules` -- so the guard skipped every style rule in the sheet and pronounced
the stylesheet clean. A check that cannot see the rule it was written for is not
passing; it is broken.

## Outcome

Shipped in `e2c799d`, released as **v0.136.0**. `probe_admin_dark_mode.mjs` goes
**6/8 -> 8/8**, and `D4` measures the live enabled Execute button at **6.6:1**.

Taken exactly as specified, including both warnings:

```css
:root                   { --danger: #a34224;  --danger-ink: #ffffff; }
html[data-theme="dark"] { --danger: #e09479;  --danger-ink: #10261c; }
```

and `color: var(--danger-ink)` at `app.css:163`, `Exports.svelte:476`,
`VocabSources.svelte:389`. Every ratio in the report reproduced: 6.25 / 2.42 /
6.60, and `--danger` as ink holds 7.19:1 on `--bg` and 6.29:1 on `--surface`.

### The palette is now checked, not asserted

`a11y.test.ts` skips axe's `color-contrast` rule -- jsdom has no rendering engine
-- and its header said instead that *"the palette in app.css is chosen for WCAG AA
contrast."* That sentence was the only evidence there was, and Execute shipped at
2.42:1 behind it.

A ratio between two hex colours needs no engine. `contrast.test.ts` (23 assertions)
now checks, in both themes:

- every background token against its paired ink token (`--accent`/`--accent-ink`,
  `--danger`/`--danger-ink`);
- every ink token (`--ink`, `--ink-muted`, `--danger`, `--info`) against both
  `--bg` and `--surface` -- this is what encodes *why* `--danger` cannot simply be
  darkened;
- and the structural rule: **no rule anywhere in the SPA may paint a literal colour
  on a `var()` background.** A theme that redefines a background must be able to
  redefine its ink. That is the property `grep -rn "color: #fff"` was standing in
  for, asserted over the whole tree rather than over three known files.

Mutation-checked. Restoring the dark `--danger-ink` to `#ffffff` fails
`dark: --danger-ink on --danger holds AA`. Restoring one badge to `color: #fff`
fails `holds across the SPA`, naming the file. And **darkening `--danger` for dark
mode** -- the fix the report warns against -- fails three checks at once: the ink
no longer holds on either surface. The test now argues the report's case.

Both suites carry a control that would have caught the `D5` failure mode on our
side: the palette block asserts both themes parsed, and the structural check
asserts it found style rules to look at before concluding they are all clean.

### One incidental

Vitest stubs CSS imports to `""`, and the stub beats an explicit `?raw` query, so
`contrast.test.ts` read an empty string and threw rather than passing vacuously --
the control did its job before the check did. `vite.config.ts` now processes
`app.css` for real, scoped to that one file: no component test wants its `<style>`
evaluated. The pattern cannot be anchored (`/app\.css$/`) because it is matched
against a module id that carries the `?raw` query.

No new dependency: the sources come in through Vite's raw glob rather than
`node:fs`, so `svelte-check` needs no Node types.

## Independently verified by libcat-e2e, 2026-07-10

`probe_admin_dark_mode.mjs` 6/8 -> **8/8** after `e2c799d`.

```
D4  Execute, ENABLED, paints rgb(16,38,28) on rgb(224,148,121) = 6.60:1   (was 2.42:1)
D5  0 rules in the shipped stylesheet fall below AA under the dark tokens (was 3)
D1  13 screens, 0 AA contrast failures in dark mode
```

**6.60:1 is the exact figure this task predicted** for `#10261c` on `#e09479`.
`grep -rn "color: #fff" backend/ui/src` now returns nothing, so the audit surface
the task named is empty. `D5` walks all 718 rules of the shipped stylesheet with
the dark tokens substituted and reports only rules that hold AA in light mode, so
it cannot be satisfied by a rule disappearing.

One correction to the harness, not the fix: `D4`'s **passing** message still recited
*"there is no `--danger-ink`"*, because the string was written against the symptom
and used on both branches. It now states the outcome. That is the fourth time a
check in this harness has described the bug while reporting a pass.
