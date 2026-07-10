# 250 -- a click that unmounts its own button drops focus out of the modal, killing Escape and the Tab trap

Filed from libcat-e2e on 2026-07-09 (cross-repo ask).

## Symptom

Open the WorkEditor's **Add subject…** picker, choose the `lcsh` tab, search `cats`,
and click a neighbor in the panel under the details pane (e.g. the narrower term
`Kittens`). The panel walks to that term correctly -- and the dialog stops responding
to the keyboard.

Measured, with a control that isolates it:

```
CONTROL after clicking a tab   : {"tag":"INPUT","inDialog":true}
CONTROL Escape closed?         : true
```

and then, walking to three different neighbors of `Cats`:

```
walk "Kittens"         : focus=BODY inDialog=false | Escape closed=false
                         after Tab -> BUTTON "Look up subjects at targets…" inDialog=false
walk "Domestic animals": focus=BODY inDialog=false | Escape closed=false
                         after Tab -> BUTTON "Cats"                          inDialog=true
walk "Feral cats"      : focus=BODY inDialog=false | Escape closed=false
                         after Tab -> BUTTON "Look up subjects at targets…" inDialog=false
```

Two facts, of different strength. **Reliably (3 of 3): after the neighbor click
`document.activeElement` is `<body>` and Escape no longer closes the dialog.** Once
focus is on `<body>` the Tab cycle is no longer governed by the trap at all, so where
the next Tab lands is whatever document order happens to give -- in 2 of the 3 walks
above it was `Look up subjects at targets…`, a button of the page **behind the scrim**,
visually covered by the modal while it held focus. That second effect is incidental to
DOM order; the first is not.

The control matters: clicking a **scheme tab** also re-renders, but the tab button
survives the render and `setTab()` calls `inputEl?.focus()`
(`VocabPicker.svelte:83`), so focus stays inside and Escape still closes. The
difference is not "clicking" -- it is clicking a control that **unmounts itself**.

## Root cause

`backend/ui/src/components/Modal.svelte:62` binds the trap to the panel element:

```svelte
<div class="panel" role="dialog" aria-modal="true" tabindex="-1"
     bind:this={panel} onkeydown={onKeydown} …>
```

`onKeydown` (`:33-50`) handles both Escape and the Tab cycle. It is a DOM listener on
the panel, so it only runs for keydowns whose target is inside the panel. Once focus
has left for `<body>`, no keydown ever reaches it: Escape is dead **and the Tab trap
that would pull focus back never runs either**. The trap cannot recapture focus it has
already lost, because losing focus is exactly what disables the trap.

There is no global fallback. `backend/ui/src/lib/keyboard.ts:423` installs a
`window` keydown listener, but it dispatches registered chords only -- it has no
Escape-to-close for modals (the sole match for `Escape` in that file is a label
formatter at `:290`).

Focus reaches `<body>` because the clicked control destroys itself:
`backend/ui/src/components/NeighborhoodPanel.svelte:23-25`

```ts
function walk(t: Term): void {
  trail = [...trail, t];
}
```

`trail` feeds `current` (`:14`), which feeds the `{#each g.ids}` list the clicked
`button.linkish` lives in. Svelte removes the focused button, and the browser resets
focus to `<body>`. Nothing refocuses. `back()` (`:27-29`) unmounts the breadcrumb
button the same way.

`Modal` has `tabindex="-1"` on the panel precisely so it *can* hold focus, but nothing
returns focus to it.

## Why it matters

The neighborhood walk is the vocab picker's primary interaction -- it is how a
cataloger browses from a search hit to the term they actually want. Doing it once
strands a keyboard user: Escape will not close the dialog, and because the trap has
stopped governing, Tab can move focus onto controls hidden behind the scrim, where a
keystroke reaches the record underneath. For a screen-reader user the dialog is
`aria-modal="true"`, so focus resting on `<body>` outside it is incoherent.

This is a `Modal` defect, not only a `NeighborhoodPanel` one: **any** modal content
that unmounts the control the user just clicked loses the trap. `NeighborhoodPanel` is
where it reproduces today; the shell should not be this fragile.

## Expected

`Modal` should keep the trap working regardless of what its content unmounts. Either:

- listen for `keydown` on `document` (or `window`) while the dialog is open, so Escape
  works no matter where focus is; **and/or**
- listen for `focusout`/`focusin` and pull focus back to the panel when it escapes,
  which also restores the Tab cycle.

Belt and braces, and better for screen readers: `NeighborhoodPanel.walk()` and `back()`
should move focus deliberately after the trail changes -- to the new `.here`
breadcrumb, or the panel heading -- so the new context is announced rather than
silently dropped.

A regression test belongs next to `modal.test.ts`: mount a Modal whose content removes
the clicked button, click it, and assert Escape still calls `onclose`.

## Repro

```bash
cd ~/libcat-e2e && node harness/probe_vocabpicker.mjs   # V6, V7, V8
cd ~/libcat-e2e && node harness/retest.mjs              # check t250
```

By hand: sign in to :8481, open `#/works/w1dh6vtir43o8i`, click **Add subject…**,
select the `lcsh` tab, type `cats`, click `Kittens` in the panel's NARROWER list, then
press Escape. The dialog stays open. Press Tab and inspect `document.activeElement`:
it is outside `[role=dialog]`.

## Outcome

Shipped in **v0.140.6** (`e5c5c60`). Fixed as a `Modal` defect first -- you were
right that the shell should not be this fragile -- with a source-level focus move in
`NeighborhoodPanel` as the polite second half.

### The shell (`Modal.svelte`)

The trap was a `onkeydown` bound to the panel, so it went deaf the instant focus left
for `<body>`. Moved it to a **window-level capture listener** installed while the
dialog is open:

- **Escape closes from anywhere.** The listener is on `window`, so a keydown reaches
  it no matter where focus sits -- the reliable defect (3 of 3 in your table) is gone.
- **Tab recaptures focus that has escaped.** When `!panel.contains(document.activeElement)`,
  Tab now `preventDefault`s and pulls focus to the first (or, with Shift, last)
  focusable in the panel, so the cycle can never wander onto a control behind the
  scrim.

The panel's `onkeydown` is removed (the window listener supersedes it), and the
capture phase + `stopPropagation` on Escape keep a page-level listener from seeing it,
as before.

### The source (`NeighborhoodPanel.svelte`)

`walk()` and `back()` now move focus deliberately after the trail changes -- to the
new `.here` breadcrumb (given `tabindex="-1"`), or the `.hood` panel when the trail
returns to the root -- via `tick()` so the target exists. Focus never rests on
`<body>` between the click and the next key, and a screen reader announces the term
just walked to rather than silently dropping it. Both `.here` and `.hood` are
`tabindex="-1"`, so they are programmatic targets only and never become Tab stops.

### Tests

Two regressions in `modal.test.ts`, exactly the shape you asked for: mount a Modal,
remove the focused child (the unmount), then assert **Escape still calls `onclose`**
and **Tab recaptures focus into the dialog**. Mutation-checked -- rebinding the
listener to the panel (the old behaviour) fails both and leaves the four original
Modal tests green. Full UI suite 320/320, `svelte-check` 0 errors.

### Verified end to end on live :8481

Drove the real flow -- open the work editor, **Add subject…**, `lcsh` tab, search
`cats`, walk to a narrower term (`Vashti (Cat)`):

```
after the walk : focus is INSIDE [role=dialog]   (was <body>)
Escape         : closes the dialog                (was: stayed open)
```

Your control holds: clicking a scheme tab (which re-renders but keeps its button) was
never affected, and still is not.
