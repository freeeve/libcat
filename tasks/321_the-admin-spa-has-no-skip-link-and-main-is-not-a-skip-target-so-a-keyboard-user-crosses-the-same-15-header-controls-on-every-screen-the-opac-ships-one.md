# 321 -- the admin SPA has no skip link and `<main>` is not a skip target, so a keyboard user crosses the same 15 header controls on every screen -- the OPAC ships one

Filed from libcat-e2e on 2026-07-10 (cross-repo ask).

Sibling of `tasks/319`: same probe, same header, different root cause and fix.

## Symptom

Every screen puts **15 focusable elements** in front of its content, and offers no
way past them.

Measured on `:8481` (`harness/probe_admin_keyboard.mjs`), counting focusable
elements in DOM order before the first focusable element inside `<main>`:

```
/dashboard      15      /copycat        15
/works          15      /duplicates     15
/authorities    15      /withdrawals     -   (no focusable control inside <main>)
/vocabularies   15      /queue          15
/batch          15      /promotions     15
/macros         15      /profiles       15
/exports        15
```

The same fifteen, in this order, on all of them:

```
libcat · Works · Authorities · Vocabularies · Batch · Macros · Exports ·
Import · Duplicates · Withdrawals · Queue · Promotions · Profiles ·
Dark mode · Sign out
```

There is no skip link -- the first focusable element on every screen is the brand
link `<a class="brand" href="#/">libcat</a>`, whose `#/` is the dashboard route, not
a skip target. And there is nothing to skip *to*: `<main>` carries **no `id` and no
`tabindex`** on all thirteen screens.

## Root cause

`backend/ui/src/App.svelte` renders `<header class="top">` -- brand, the nav derived
from `sidebarScreens()`, and the two `.side` buttons -- then `<main>`, with no
bypass link and no `id`:

```
$ grep -rni "skip" backend/ui/src/App.svelte backend/ui/index.html
(nothing)
```

The published OPAC, **in this same repo**, does it textbook-correctly:

```html
<!-- hugo/layouts/baseof.html:15 -->
<a class="lcat-skip" href="#lcat-main">Skip to main content</a>
<!-- hugo/layouts/baseof.html:28 -->
<main id="lcat-main" class="lcat-main" tabindex="-1">
```

```css
/* hugo/assets/lcat.css:108 -- off-screen until focused */
.lcat-skip { position: absolute; left: -999px; }
.lcat-skip:focus { left: 0.5rem; top: 0.5rem; z-index: 10; … }
```

The asymmetry has a cause: `tasks/014` ("Formal accessibility audit") was scoped to
the **Hugo module**. The SPA has never had one.

## Why it matters

WCAG 2.4.1 *Bypass Blocks*, Level A. Fifteen tab presses before the first control of
the actual screen, on every screen.

Two caveats, so this is not oversold:

- The SPA *does* expose a `<main>` landmark, and `ARIA11` (ARIA landmarks) is listed
  among WCAG's sufficient techniques for 2.4.1, so an auditor could accept it. But a
  landmark helps only a screen-reader user who knows to jump by landmark; it does
  nothing for the sighted keyboard-only user who is the one tabbing fifteen times.
  And with no `id` on `<main>`, even that jump has no anchor to offer.
- Focus is **not** reset on route change, so a user who tabs to a nav link and
  follows it does not re-cross the whole header. But focus *is* at the document
  start after a page load, after every `g <letter>` chord, and after every
  command-palette navigation -- measured, `document.activeElement` is `BODY`. Those
  are the paths a keyboard user actually takes.

This compounds `tasks/318`: the same fifteen-element header is what pins the app's
minimum width at 1342px. One header, two defects.

## Expected

Match what the OPAC already does:

1. A first-in-DOM `<a class="skip" href="#main">Skip to main content</a>`, visually
   hidden until it takes focus.
2. `<main id="main" tabindex="-1">`. The `tabindex` matters: without it the browser
   scrolls to the anchor but leaves focus in the header, so the next Tab returns to
   nav link two.
3. `<main>` is rendered per screen (`screens/*.svelte` each own theirs), so the
   `id`/`tabindex` want a shared home -- hoisted into `App.svelte` around the screen
   slot, or a small `<Main>` component the screens use.

Worth considering separately: move focus to `<main>` (or its `<h1>`) on route
change, so a screen-reader user is told the page changed. Today focus stays on
`BODY` after a chord and on the nav link after a click.

## Repro

```
cd ~/libcat-e2e && node harness/probe_admin_keyboard.mjs   # B1, B2, B3 fail
node harness/retest.mjs                                     # t321
```

By hand: open `:8481` and press Tab. The ring lands on "libcat". Keep pressing. The
fifteenth press is "Sign out"; the sixteenth is the first control of the screen.

### Controls

`C2` confirms focus is **visible**: the first Tab lands on `a.brand` and takes the
`3px solid rgb(30,107,78)` ring `app.css:79` declares, matching `:focus-visible`.
This is not a focus-indicator bug -- and a "bogus focus-ring probe" is already on
this harness's own list of past errors.

`B4` confirms all thirteen `g <letter>` chords navigate and `Cmd+K` opens the
palette (`t244`). Keyboard navigation is deliberate here, which is why the missing
skip link reads as an omission rather than a posture. A chord is not a WCAG 2.4.1
technique, and a first-time keyboard user cannot discover one.
