# 307 -- collapse the more-like-this rail on skos:exactMatch rather than the English label, so a cross-scheme homograph does not merge silently

Opened 2026-07-10. Deferred deliberately; the evidence for deferring is below.

## Background

tasks/302 made the rail's synonym collapse language-independent by grouping
concepts on their **English label** rather than on the label the page renders. The
known cost, inherited from tasks/141's term-page policy: two vocabularies that
agree on an English string are treated as one concept. Right for a synonym pair,
wrong for a cross-scheme homograph.

`hugo/content/works/_content.gotmpl` builds `$termKeys` from the `en` label. The
correct key is a concept identity, and the graph already has the vocabulary for
one: `skos:exactMatch` / `skos:closeMatch`.

## Why it is not done

queerbooks-demo measured their 62,602-work catalog (their tasks/070, our
tasks/306) and libcat reproduced every figure independently against their sidecar:

| measure | value |
|---|---|
| terms in the `terms` sideband | 10,050 |
| English labels carried by more than one IRI | 55 |
| of those, cross-scheme | 43 (every one a FAST <-> Homosaurus pair) |
| pairs where at least one work carries **both** IRIs | 26 |
| pairs with **zero** co-assignment (the homograph signature) | **0** |
| pairs where one IRI has no subject usage, so the test is silent | 17 |

Zero homographs. The collapse fires on 87,439 of 422,355 rail blocks (20.7%) and
is correct in every case anyone has looked at.

## The test case, when someone does this

**`Sapphics`.** FAST `1105395` and Homosaurus `homoit0002277`, 1,063 rail blocks.
"Sapphics" is also the classical verse form, so it has exactly the homograph shape.
It is not one *in that corpus*: 795 works carry the FAST term, 2,504 carry the
Homosaurus term, and **all 795 of the FAST works also carry the Homosaurus one** --
total containment, not mere overlap. Both mean the people.

Merge a poetry or classics collection into the same catalog and `Sapphics` becomes
a real homograph. The English-label collapse would fold the verse form into the
people, silently, with nothing on the page to reveal it. That is the regression
test this task exists to make pass.

## Shape of the work

1. **Projector.** `terms` (tasks/178) carries `id`, `labels`, `broader`. It would
   need `exactMatch` (and probably `closeMatch`), harvested from the vocabulary
   graph the same way `broader` is. Schema version bump on `catalog.json`.
2. **Adapter.** `$termKeys` becomes the canonical member of each match set --
   lexically-least IRI in the set, so it is stable and language-free -- falling
   back to the English label for a term with no mapping, which is most of them.
   `$termNative` and the render are unchanged.
3. **Seam test.** `hugo/similar_seam_test.cjs` gains a fixture with two IRIs that
   share an English label and are **not** `exactMatch`, and must render twice; plus
   the existing synonym pair, now joined by `exactMatch`, which must still collapse.
   Both cases must be red against the label-collapse implementation.

## Cost of not doing it

A silent merge on a corpus nobody has measured. The failure is invisible from the
page: the rail simply names one concept where it should name two. Any adopter
merging a general collection into a specialised one inherits this, and there is no
warning. Worth revisiting whenever a second corpus of a different subject reports
in -- not before, because 43 cross-scheme collisions and 0 homographs is not the
evidence base for a schema change.

The limitation is documented in `hugo/README.md` under "More like this", including
the `Sapphics` example, so an adopter meets it rather than discovers it.
