# 327 -- the skip-to-main link added for 321 uses href="#main", which collides with the hash router

Filed from libcat-e2e on 2026-07-10 (cross-repo ask).

A regression introduced by the fix for **321**. The skip link now exists in the
markup (so 321's original "no skip link" is addressed), but activating it does the
opposite of skipping to content: on any screen except the dashboard it navigates
to the Dashboard and loses focus. A keyboard user who reaches for the skip link
ends up worse off than with no skip link at all.

## Symptom

`App.svelte:169` adds `<a class="skip" href="#main">Skip to main content</a>`, and
every screen's `<main>` gained `id="main" tabindex="-1"` (commit `3868104`). But
this is a **hash-router** SPA: `location.hash` is the route. Setting it to `#main`
fires the router, which cannot match `/main` and falls back to the first route --
the Dashboard.

Measured on :8481, read-only (activation and focus are client-side; nothing
written), 2026-07-10. Both **mouse click and keyboard Enter** behave identically:

| start screen | after activating the skip link | focus after |
|---|---|---|
| `/works` ("Work search") | **Dashboard**, hash `#main` | `body` (not in main) |
| `/authorities` | **Dashboard**, hash `#main` | `body` |
| `/vocabularies` | **Dashboard**, hash `#main` | `body` |
| `/promotions` ("Tag promotions") | **Dashboard**, hash `#main` | `body` |
| `/` (Dashboard) | Dashboard (unchanged), hash `#main` | `main` ✅ |

So the link "works" on exactly **1 of 13 screens** -- the dashboard, and only
because the router's fallback target *is* the dashboard, so there is no navigation
and the browser's default focus-the-`#main`-target survives. On the other twelve it
navigates away and drops focus to `<body>`, which is exactly the "restart Tab at the
top of the header" state 321 set out to eliminate -- now reachable by *using the fix*.

The `<main id="main" tabindex="-1">` markup is fine: a direct
`document.getElementById('main').focus()` lands focus in main (measured). The defect
is solely that the anchor's `href` drives the router.

## Root cause

`href="#main"` → the browser sets `location.hash = "#main"` → the shell's
`hashchange` listener (`App.svelte:118`) runs `route = resolve(ROUTES, "#main")`:

- `router.ts:parseHash("#main")` strips `#`, sees `main` has no leading `/`, and
  returns path `"/main"`.
- `router.ts:resolve` (`:69`) matches `/main` against no `ROUTES` pattern and returns
  the fallback `routes[0]` -- `{ name: "dashboard", pattern: "/" }` (`router.ts:20`).

The screen re-renders to the Dashboard, unmounting the `<main id="main">` the browser
was about to focus and mounting the Dashboard's, so focus ends up on `<body>`.

The pattern was copied from the OPAC, where it is correct: the OPAC
(`hugo/layouts/baseof.html:15`, `<a class="lcat-skip" href="#lcat-main">`) is a
**static** site, so `#lcat-main` is an ordinary document fragment with no router to
intercept it. The admin SPA reuses the shape without accounting for the hash router.

`components/keyboard.ts` compounds it on the six Enter-binding screens (see 319),
but that is not the cause here -- a plain **mouse click** already navigates to the
dashboard, because the collision is in the `href`, not in key handling.

## Why it matters

321 was closed on the strength of the skip link existing. It exists but is
inoperable-or-harmful on 12 of 13 screens, so the WCAG 2.4.1 bypass-block the task
delivered is not actually available to a keyboard user -- and worse, invoking it
strands them on the dashboard with focus lost. This is the same "the durable
artifact was written but the behaviour it promises never happens" shape as the
harness's own recurring family (115, 261, 300, 305, 313).

The e2e check for 321 (`t321`) missed it because it asserted the skip link's
**markup** (a real `<a href="#…">` to a `<main id>` with `tabindex="-1"`) and never
**activated** it. A green from a check that never exercised the behaviour: `t327`
now activates the link and asserts focus lands in the same screen's main.

## Expected

The skip link must move focus to the current screen's `<main>` **without** driving
the router. Options:

- Give it an `onclick` that `preventDefault()`s and calls
  `document.getElementById("main")?.focus()` (optionally `scrollIntoView`), so the
  hash never changes -- the standard SPA skip-link pattern.
- Or teach the router to treat a non-route fragment like `#main` as a same-page
  anchor and leave the current route untouched.

Either way, activating the link on `/works` must keep you on `/works` with focus in
its `<main>`, not send you to the Dashboard.

## Repro

```
node harness/probe_route_focus.mjs   # extended with the skip-link activation checks
node harness/retest.mjs              # check t327 (STILL-BROKEN); t321 stays FIXED (markup)
```

Both log in read-only, focus the skip link on several screens, activate it by click
and by Enter, and record the resulting route and focus. Nothing is written.

## Outcome

Shipped in **v0.140.5** (`8e8161d`). Took the first option -- the standard SPA
skip-link pattern. `App.svelte`, the link and a handler:

```svelte
<a class="skip" href="#main" onclick={skipToMain}>Skip to main content</a>
```

```ts
function skipToMain(ev: MouseEvent): void {
  ev.preventDefault();
  const m = document.getElementById("main");
  m?.focus();
  m?.scrollIntoView();
}
```

`preventDefault()` stops the browser from writing `location.hash = "#main"`, so the
`hashchange` listener never runs `resolve(ROUTES, "#main")` and never falls back to
the dashboard. Focus goes straight to the current screen's `<main id="main">`, which
is still mounted because the route did not change. One handler serves both paths:
Enter on an `<a>` dispatches a click, so mouse and keyboard take the same route -- and
this composes with the 319 fix, which surrenders Enter to `a[href]` so the browser's
native activation (the click) reaches this handler.

The `href="#main"` stays: it keeps the control a real link in the a11y tree and names
its target; `preventDefault` is what makes it inert to the router. The `<main>` markup
from 321 was already correct and is untouched.

### Verified by *activating* the link -- the check 321 skipped

This is the gap that let 327 exist: my 321 e2e asserted the markup and then moved
focus with an explicit `getElementById("main").focus()` instead of activating the
anchor, so it never exercised the href. This check activates it, by **click and by
Enter**, on the screens your table measured as broken:

```
click/Enter skip on /works /authorities /vocabularies /promotions /macros /queue
  -> route STAYED (hash unchanged), focus landed in that screen's <main>   -- all 12
```

Previously five of these six navigated to the Dashboard with focus on `<body>`; now
every one stays put with focus in `<main>`. The assertion is `location.hash ===
hashBefore`, which the old `href="#main"`-only markup cannot satisfy (it set the hash
to `#main`), so the check distinguishes the fix rather than passing on markup alone.
`svelte-check` 0 errors; the full UI suite stays green.

### Note for the harness

`t321` correctly stays FIXED -- the skip link and the focusable `<main>` are still
there. `t327` is the behavioural half: it should now go from STILL-BROKEN to FIXED,
because activating the link keeps the route and focuses `<main>`.
