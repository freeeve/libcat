# 319 -- the global key dispatcher preventDefaults Enter over focused links and buttons, so on 6 of 13 screens no link or button can be activated by keyboard -- and on /works Enter opens a work record instead

Filed from libcat-e2e on 2026-07-10 (cross-repo ask).

## Symptom

Tab to the **Batch** link in the primary nav. Press **Enter**. On `/works` you land
on a work detail page. On `/queue` nothing happens at all.

Measured on `:8481` (`harness/probe_admin_keyboard.mjs`). The nav link is focused
directly, then one key is pressed:

```
screen          Enter on nav link   Enter on <button>   Space on <button>
/                   OK                  OK                  OK
/works              BLOCKED             BLOCKED             OK
/authorities        BLOCKED             BLOCKED             OK
/vocabularies       OK                  OK                  OK
/batch              OK                  OK                  OK
/macros             BLOCKED             BLOCKED             OK
/exports            BLOCKED             BLOCKED             OK
/copycat            OK                  OK                  OK
/duplicates         BLOCKED             BLOCKED             OK
/withdrawals        OK                  OK                  OK
/queue              BLOCKED             BLOCKED             OK
/promotions         OK                  OK                  OK
/profiles           OK                  OK                  OK
```

**Six of thirteen screens.** And the split is not random -- it is exactly the
screens on which something has bound `Enter`. The seven that work are the control:
they prove the router, the links and the key delivery are all fine.

A `<button>` still has `Space` as a fallback. **An `<a href>` has none:**

```
/works   Space        on the focused "Batch" link -> #/works        (page scroll)
/works   Enter        on the focused "Batch" link -> #/works/w0cfnsjg6micju
/works   NumpadEnter  on the focused "Batch" link -> #/works/w0cfnsjg6micju
/queue   Space | Enter | NumpadEnter                -> #/queue      (nothing)
```

So on `/works` the keystroke is not merely swallowed. It performs a **different,
unrelated action**: it opens whichever work row the list happens to have selected.
A cataloger who tabs to "Batch" and presses Enter is silently taken to a record.

Two of the six do this. The other four eat the key and do nothing:

```
/works        Enter on the "Batch" link -> #/works/w0cfnsjg6micju        (a work record)
/authorities  Enter on the "Batch" link -> #/authorities/a0d7go0nob80r8  (an authority record)
/macros /exports /duplicates /queue                                      (no navigation)
```

Which record you land on depends on where the list's `selected` index happens to
sit -- it is whatever row was last highlighted, not anything the user aimed at.

## Root cause

`backend/ui/src/lib/keyboard.ts`, `onKeydown`. The guard that stops a binding from
stealing a key the focused control needs covers **form controls only**:

```ts
if (target?.closest?.("input, textarea, select, [contenteditable]") && !chord.startsWith("mod+")) {
  …
  return;
}
…
const b = lookup(chord);
if (b) {
  ev.preventDefault();     // <-- fires even when target is <a href> or <button>
  b.handler(ev);
  return;
}
```

`a[href]`, `button`, `summary` and `[role=button]` are not in that selector, so a
plain-key binding fires over them and `preventDefault()` cancels the native
activation. `Enter` is the native activation key for a link; `Enter` and `Space`
are the native activation keys for a button.

The module's own doc comment states the intent this misses:

> Plain keys are ignored while focus sits in a form control **so typing never
> triggers actions**.

The rule was written against *typing*. Activation is the other thing a focused
control needs its keys for, and it was never considered.

Who binds `Enter`: `components/RowList.svelte:56` does it for every list that
passes an `onactivate`:

```ts
if (onactivate) {
  specs.Enter = { description: `open the selected ${itemName}`, legend: "open", handler: () => activate() };
}
```

plus `screens/Queue.svelte:64`, `screens/Macros.svelte:45`,
`screens/Exports.svelte:109`, `components/CopycatResults.svelte:40`. Those are
precisely the six blocked screens. The bindings are all from `tasks/065`
("Shortcuts on every screen") -- the feature is right; the dispatcher is not.

Also affected, from the same guard: `RowList` binds `j`, `k`, `ArrowDown`,
`ArrowUp` as plain keys, so `ArrowDown` on a focused link moves the row selection
instead of scrolling the page. Measured: `activeElement` stays the link.

## Why it matters

This is **WCAG 2.1.1 Keyboard, Level A** -- the most basic conformance requirement
there is. On six screens, no link in the application can be followed from the
keyboard, including *every link in the primary navigation*. There is no fallback
for a link the way `Space` is for a button.

It is worse than a dead key. On `/works` -- the screen a cataloger opens nine times
in ten -- Enter on the nav bar navigates to a **work record**. The action a user
asked for is discarded and a different one is performed, with no indication that
anything went wrong. Someone mid-edit who tabs to a nav link is taken somewhere
they did not choose.

And it lands on the users who need the keyboard most. This app has a command
palette, thirteen `g <letter>` chords, a remappable keymap (`tasks/075`) and a `?`
overlay. Keyboard operation is not an afterthought here -- which is exactly why the
dispatcher swallowing the two keys the platform reserves for activation is a
regression against the product's own intent, not a missing feature.

Note `tasks/014` ("Formal accessibility audit") audited the **Hugo module**. The
SPA has never had one. `probe_admin_keyboard.mjs` is the beginning of that.

## Expected

Do not cancel a key that the focused element natively consumes. Concretely, in
`onKeydown`, before dispatching a plain-key binding:

```ts
const ACTIVATES: Record<string, string> = {
  Enter: 'a[href], button, summary, [role=button], [role=link], [role=menuitem]',
  ' ':   'button, summary, [role=button], input[type=checkbox], input[type=radio]',
};
const native = ACTIVATES[ev.key];
if (native && target?.closest?.(native) && !chord.startsWith('mod+')) return;
```

Two things to get right:

- **Do not simply add `a, button` to the existing form-control selector.** That
  would also silence `j`/`k`/`g` while focus rests on a link, and those are the
  bindings that make this app pleasant. Only the element's *activation* keys should
  be surrendered, and only to the element that actually consumes them.
- `mod+` chords must keep firing anywhere; that is stated in the module comment and
  is correct.

Arrow keys are a judgement call and not part of this bug's minimum fix: `ArrowDown`
over a focused link is a scroll in the platform, but a row-list app reasonably
claims it. If it is claimed, it should be claimed everywhere, not only where a
`RowList` is mounted.

A unit test belongs in `keyboard.test.ts`: bind `Enter`, dispatch a keydown whose
`target` is an `<a href>`, and assert `defaultPrevented === false` and that the
handler did not run. The same for `Space` over a `<button>`.

## Repro

```
cd ~/libcat-e2e && node harness/probe_admin_keyboard.mjs   # 3/7; A1 fails
node harness/retest.mjs                                     # t319
```

Or by hand: open `:8481/#/works`, press Tab five times (brand, Works, Authorities,
Vocabularies, Batch), press Enter. You arrive at a work record.

### Controls, because this is the kind of finding that is usually the harness

- Playwright's `Enter` **does** activate anchors: on the static OPAC at `:8482`, a
  focused `<a href="/works/">` navigates from `/` to `/works/`.
- A synthetic `.click()` on the very same nav link **does** navigate to `#/batch`,
  so the router and the href are fine.
- Seven of thirteen screens activate the link on Enter, and they are exactly the
  screens with no `Enter` binding.
- `Space` on a `<button>` works on all thirteen, because nothing binds `Space`.

The first version of this probe reported four other findings, every one of which
was its own bug: a 90-stop tab-walk cap read as "unreachable controls" and as a
"keyboard trap" on the two screens with more than 90 controls (`/duplicates` has
202); `input[type=month]` read as "no focus indicator" when focusing it directly
gives the declared 3px ring; `a[href^="#"]` matched the brand link `<a href="#/">`
and reported a skip link that does not exist; and `/copycat` reported "0 header tab
stops" because it autofocuses an input inside `<main>`.

## Outcome

Shipped in **v0.140.4** (`6ecfe71`). Took your suggested fix, verbatim in spirit.
`backend/ui/src/lib/keyboard.ts`, `onKeydown`, just before the plain-key dispatch:

```ts
const ACTIVATES: Record<string, string> = {
  Enter: "a[href], button, summary, [role=button], [role=link], [role=menuitem]",
  " ": "button, summary, [role=button], input[type=checkbox], input[type=radio]",
};
function surrendersToTarget(ev, chord, target) {
  const native = ACTIVATES[ev.key];
  return !!native && !chord.startsWith("mod+") && !!target?.closest?.(native);
}
// ...
if (surrendersToTarget(ev, chord, target)) return;   // before `const b = lookup(chord)`
```

Both things you said to get right are respected:

- **It is not folded into the form-control selector.** It is a separate, key-specific
  guard: only the key that natively activates the focused element is surrendered, and
  only to the element that consumes it. `j`/`k`/`g` and the sequence prefixes still
  fire with a link focused -- a unit test pins that (`j` over an `<a href>` still
  fires and still `preventDefault`s).
- **`mod+` chords keep firing everywhere** -- the `!chord.startsWith("mod+")` clause,
  and a test presses `mod+Enter` over a link and asserts the binding runs.

`NumpadEnter` reports `ev.key === "Enter"`, so it is covered by the same entry -- the
report's third row (`NumpadEnter -> a work record`) is fixed by the same line.

The guard sits after the `?` and sequence-prefix handling and before both the
plain-key dispatch and the sequence-arming `preventDefault`, so a surrendered key
never arms a sequence either. On the seven screens that already worked (no `Enter`
binding), behaviour is unchanged -- the key was reaching the browser before and still
does; those screens are now correct by rule rather than by the absence of a binding.

### Tests

Five cases added to `keyboard.test.ts` (`describe "native activation keys
(tasks/319)"`), the two you named plus three controls:

```
Enter over <a href>   -> defaultPrevented false, binding NOT called   (A1)
Space over <button>   -> defaultPrevented false, binding NOT called
Enter over <body>     -> defaultPrevented true,  binding called       (feature intact)
j     over <a href>   -> defaultPrevented true,  binding called       (not over-broadened)
mod+Enter over <a>    -> defaultPrevented true,  binding called       (mod exempt)
```

Mutation-checked: stubbing `surrendersToTarget` to `return false` fails exactly the
two surrender cases and leaves the three controls green, so the tests exercise the
guard and not something incidental. Full UI suite 318/318, `svelte-check` 0 errors.

### Verified end to end on live :8481

Drove the real login, then on each of the **six** screens you measured as blocked,
focused the primary-nav **Batch** link directly and pressed Enter:

```
/works  /authorities  /macros  /exports  /duplicates  /queue   -> #/batch, every one
```

(was: a work/authority record on two of them, nothing on the other four). Enter on
the theme `<button>` toggles it. The row-list Enter feature is intact -- Enter over a
non-activatable target still fires the "open selected" binding, which is the
`<body>` control test above and is why the `/works` list still opens on Enter when a
row (not a link) holds the selection.

### Left out of scope, as you flagged

The Arrow-key question (`ArrowDown` over a link scrolls in the platform but a
row-list app reasonably claims it) is a judgement call you separated from the minimum
fix. I left `j`/`k`/`Arrow*` firing as they were rather than half-fix it only where a
`RowList` is mounted; if you want arrows surrendered too, that is its own task with a
consistent everywhere-or-nowhere rule.
