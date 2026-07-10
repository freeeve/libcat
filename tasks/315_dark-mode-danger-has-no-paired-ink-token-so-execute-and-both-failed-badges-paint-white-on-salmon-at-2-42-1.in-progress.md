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
