# 251 -- the vocab picker's details pane keeps describing the highlighted term after the neighborhood walks away from it

Filed from libcat-e2e on 2026-07-09 (cross-repo ask).

## Symptom

In the vocab picker, the details pane's heading, URI, definition and "Also known as"
line describe the *highlighted search result*. The `NeighborhoodPanel` mounted beneath
them keeps its own walk trail. Walk one step and the two disagree: the pane identifies
one term, the button under it picks a different one.

Search `cats` on the `lcsh` tab, then click the narrower term `Kittens`:

```
details pane says  : "Biology, Economic › Zoology, Economic › Domestic animals › Cats"
details pane URI   : http://id.loc.gov/authorities/subjects/sh85021262
breadcrumb current : "Kittens"

pending chip after clicking "Use this term": ["Kittens  LCSH  adds on save  ✕ undo"]
```

Everything above the breadcrumb still describes **Cats** -- including the URI
`sh85021262` and the variants `Felis domestica; Cat, Domestic; Felis catus; Felis
silvestris catus`. "Use this term" correctly stages **Kittens**.

So the button is right and the identity block above it is stale. A cataloger reading
top-to-bottom sees `Cats`, its authority URI and its variant labels, and clicks the one
button in that pane; the record gets `Kittens`.

## Root cause

Two components each derive a "current term" and they are never reconciled.

`backend/ui/src/components/VocabPicker.svelte:58`

```ts
const current = $derived(results[highlight]);
```

`:218-230` renders the whole identity block from that `current` -- `h3` with
`current.path` and `bestLabel(current)`, `<p class="opt-id">{current.id}</p>`,
`bestDefinition(current)`, `allAltLabels(current)`.

`backend/ui/src/components/NeighborhoodPanel.svelte:13-14`

```ts
let trail = $state<Term[]>([]);
const current = $derived(trail[trail.length - 1] ?? term);
```

`walk()` (`:23-25`) pushes onto `trail`, so the panel's `current` moves while the
picker's `current` does not. The panel emits the walked term on
`onselect?.(current)` (`:38`), which is why the pick is correct.

`VocabPicker.svelte:241-242` mounts the panel keyed on the *highlighted* term:

```svelte
{#key current.scheme + " " + current.id}
  <NeighborhoodPanel term={current} {onselect} />
{/key}
```

The key correctly resets the trail when the highlight moves, but nothing carries the
walk back up, so the pane above the panel never learns that the panel walked.

The panel does render the walked term's own definition inside its breadcrumb block
(`NeighborhoodPanel.svelte:41-43`), which makes the disagreement worse rather than
better: after a walk the pane can show *two* definitions, one per term, with no
indication that they describe different subjects.

## Why it matters

This is a subject-authority tool, so identity is the entire product. The URI on screen
is the thing a cataloger checks before committing a heading -- it is the only
unambiguous identifier in the pane -- and after a walk it is the wrong one. The
variants make it worse: "Also known as: Felis catus" is a strong signal that the
highlighted term is what will be used.

The mitigation that exists (the breadcrumb bolds `Kittens`) is a small line below a
large heading that says `Cats`. It is exactly the kind of near-miss that produces a
mis-assigned heading nobody notices until the OPAC facet looks wrong.

Note this is not the same defect as **248**: there the panel offered `Add` for a term
already on the record. Here the panel is correct and the surrounding pane is stale.

## Expected

The details pane and the neighborhood must describe the same term. Either:

- lift the trail into `VocabPicker` (pass `term` and an `onwalk` callback, or bind the
  panel's `current`), so the heading, URI, definition and alt-labels follow the walk;
  **or**
- render the identity block *inside* `NeighborhoodPanel` from its own `current`, so one
  component owns "which term am I describing".

Whichever way, `Use this term` and the URI printed above it must never name different
terms. A unit test should walk one step and assert the rendered `opt-id` equals the id
passed to `onselect`.

## Repro

```bash
cd ~/libcat-e2e && node harness/probe_vocabpicker.mjs   # V9
cd ~/libcat-e2e && node harness/retest.mjs              # check t251
```

By hand: sign in to :8481, open `#/works/w1dh6vtir43o8i`, click **Add subject…**,
select the `lcsh` tab, type `cats`, click `Kittens` under NARROWER. The heading still
reads `… › Cats` and the URI still reads `…/sh85021262`, while the breadcrumb reads
`Kittens`. Clicking **Use this term** stages `Kittens`. Undo the staged add and delete
the autosaved draft (`GET /v1/drafts` then `DELETE /v1/drafts/{id}`) -- this report's
staging was cleaned up that way.

## Outcome

Shipped in **v0.140.7** (`c790df0`). Took your second option -- **one component owns
"which term am I describing."** The whole identity block (heading with path, URI,
definition, "Also known as") now lives inside `NeighborhoodPanel`, rendered from *its*
`current`, so it and the walk buttons can never name different terms. There is no
cross-component state to keep in sync, which is what made the lift option (option 1)
fragile at the edges -- a stale describe-term on tab switch or highlight change.

`VocabPicker.svelte`: the shared identity block moved into the **live-tab branch
only** (live tabs have no walk, so they keep describing the highlighted suggestion,
exactMatch and all). The non-live branch is now just the keyed `NeighborhoodPanel`.

`NeighborhoodPanel.svelte`: renders `h3` (path + label), the URI (`.ident-uri`),
definition and alt-labels from `current` at the top of the panel, above the
breadcrumb and relation groups. The old bolded `.here` breadcrumb label is gone --
redundant with the heading -- and `refocus()` (the tasks/250 focus move) now lands on
that `.ident` heading, so a screen reader announces the walked-to term's full identity
rather than a lone breadcrumb.

### The test you asked for

`neighborhoodpanel.test.ts` walks one step and asserts the rendered `.ident-uri`
equals `onselect.mock.calls[0][0].id` -- the URI shown above the button is exactly the
id the button stages. Mutation-checked: pointing `.ident-uri` at the original `term`
instead of the walked `current` (the reported bug) fails it. A second test pins the
pre-walk state (describes the highlighted term). Full UI suite 322/322, `svelte-check`
0 errors.

### Verified end to end on live :8481

Your exact flow -- **Add subject…**, `lcsh`, search `cats`, walk to a narrower term
(`Vashti (Cat)`):

```
before walk : heading "… › Cats"        URI .../sh85021262
after walk  : heading "Vashti (Cat)"    URI .../sh99014663
```

The URI follows the walk (was frozen on `sh85021262` before), the heading names the
walked term, and the tasks/250 behaviour is intact -- focus stays inside the dialog
and Escape still closes. Read-only: the picker was opened and closed, nothing staged
or saved.

### Not touched

Live-source tabs are unchanged -- they have no `NeighborhoodPanel`, so their identity
block (with the "Same as" exactMatch list) still renders in `VocabPicker` from the
highlighted suggestion, which is correct because there is no walk to disagree with.
