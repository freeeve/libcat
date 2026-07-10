# 296 -- more-like-this rail renders raw authority IRIs when a shared concept is not among the page's own subjects

Filed from queerbooks-demo on 2026-07-10 (cross-repo ask).

Found adopting v0.118.0 (tasks/284's rail) over a 62,602-work catalog.

## What a visitor sees

    Shares: Juvenile Literature, Juvenile Fiction, Fantasy, Transgender people,
            https://homosaurus.org/v5/homoit0001643

A bare authority URL, in the middle of a human-readable list, on the public page.

Across the full render, both locales:

    pages with a rail:         113,988
    pages showing a raw IRI:    17,494   (15.3%)
    "Shares" spans:            844,710
    spans showing a raw IRI:    59,076   (7.0%)

## Why

`page.html` resolves shared subject IRIs against **the page's own** `subjectList`:

```gotemplate
{{- range $.Params.subjectList -}}
  {{- $labels = merge $labels (dict .id $l) -}}
{{- end -}}
```

The comment says "This page already carries the labels for its own subjects", and
that assumption is what breaks: `SimilarNeighbor.Shared` can name a concept the
page does **not** carry. Your own struct comment already says so --

    // A neighbour reached only through the concept tree or a flat bonus can
    // legitimately share nothing verbatim.

-- but the rail prints `shared[]` regardless. Checked directly: for every raw IRI
sampled, the concept is in the *neighbour's* subject list and absent from the
page's own.

    work w0094pvukkq59m -> neighbour wu01ue6cnt7o6o
      shared: https://homosaurus.org/v5/homoit0001643
      in this work's subjects?     False
      in the neighbour's subjects? True

FAST IRIs mostly resolve because both works tend to carry them verbatim;
Homosaurus concepts, reached through the tree, mostly do not. So the bug reads as
"Homosaurus URLs leak", but the cause is the tree hop, not the scheme.

## A second, smaller flaw in the same span

One concept expressed in two schemes prints its label twice:

    Shares: Elledge, Jim, Gay men, Lesbians, Gay men, Lesbians

`shared` for that pair is `["Elledge, Jim", fast/939117, fast/996540,
homoit0000506, homoit0000556]` -- the FAST and Homosaurus IRIs for "Gay men" and
"Lesbians". Both resolve, both render. The reader sees a stutter, not two facts.

## Ask

The sidecar is language-neutral by design, and the page cannot label a concept it
does not hold -- so the page is the wrong place to resolve these. Either:

1. **Have the projector resolve `shared` to labels** as it writes similar.json --
   it holds the whole catalog and every authority label. This costs size (the
   sidecar is already 103MB at 8 neighbours × 62.6k works here) unless labels are
   interned, and it needs one entry per site language, which is what the current
   design was avoiding. Or:
2. **Emit `shared` as `{id, label}` pairs, or a parallel `sharedLabels`**, letting
   the template render the label and keep the IRI in a `title`/`data-` attribute.
   Or:
3. **Drop from `shared` any value the page cannot label** -- the cheapest fix, and
   defensible: a "shares" line exists to explain the recommendation, and a bare
   URL explains nothing. A card that shares only tree-reached concepts would then
   show no reason, which is the honest outcome.

Whichever route, please also dedupe by resolved label so cross-scheme synonyms
collapse to one term.

## Adopter mitigation meanwhile

`[project] similar = 0` removes the sidecar and the rail entirely. That is the
switch we will use if we deploy before this lands, since the rail is otherwise a
good feature and we would rather not ship URLs to readers.

## Outcome

Fixed in **v0.121.1** (patch: no sidecar format change, no reproject), commit
`76b0f37`. Do not set `similar = 0`.

Every number in the report reproduces exactly against
`~/queerbooks-demo/site/assets` (read-only): 113,988 rail pages, 17,494 showing a
raw IRI (15.3%); 844,710 spans, 59,076 with a raw IRI (7.0%). The diagnosis is
right too, including that the cause is the tree hop and not the scheme.

### The route taken -- none of the three

The ask offered three, and correctly ruled out the first two on sidecar size and
per-language cost. But the premise underneath all three -- "the page cannot label
a concept it does not hold" -- is true only of the *page*. The **catalog** can.

`catalog.json` already carries a `terms` sideband (tasks/178): every referenced
term plus its **transitive `skos:broader` ancestors**, with labels per language.
That is precisely the set the tree hop reaches. It was added so the browse-artifact
builder could name hierarchy nodes no Work carries -- the same problem, already
solved, one file away.

So the content adapter resolves `shared` there instead. Cost: nothing. No sidecar
growth, no per-language sidecar, no format change.

Measured on the reporter's own catalog: **the terms sideband labels all 698,114
shared IRIs. Zero misses.** And zero shared IRIs fall outside the union of the
works' own subjects, which is the construction argument -- `scorer.sharedWith`
only ever reports a value the *neighbour* holds verbatim, so every shared IRI is
some Work's subject. The adapter therefore builds its map from both: the works'
inline subject labels (definitional, so a catalog with no sideband still resolves)
and the terms sideband (a superset, so it wins ties).

Option 3 survives as the residue: an IRI the catalog cannot label **in any
language** is dropped, never printed raw. A card whose shared concepts are all
unlabelable shows no reason -- the honest outcome the reporter named.

### The second flaw

Deduped by resolved label, so cross-scheme synonyms collapse. It was not a rare
polish item: **24.1% of the rail's 422,355 spans repeated a term** (110,221
redundant terms per language).

### Where it lives

The adapter (`content/works/_content.gotmpl`), not `page.html` -- the label map is
built once per language, not once per page. Labels resolve site-language, then
English, then the lexically first tag (a template ranges a map in sorted key
order, so it is deterministic; Homosaurus has nl/sv-only concepts). `page.html`
now just prints `.shared`.

Two scoping bugs of my own, caught before they shipped: inside `{{ with .labels }}`
the dot is the labels map, so `.id` was nil in both loops.

### Coverage

`hugo/similar_seam_test.cjs`, in `npm run test:js`. It builds the site for real
against a catalog whose every branch is deliberate, and reads the rendered line
the way a visitor does -- no hand-written adapter input, which is why nothing
caught this from inside a template. Branches: a tree-reached concept **no Work
carries**, a cross-scheme synonym, an undescribed IRI, a label in no site
language, free text, and a page with **no subjects of its own**.

Against the old templates 7 of its 8 checks fail, reproducing both reported
symptoms verbatim:

    FAIL - a concept reached through the tree, carried by neither page, is labeled
           got ["Transgender people","https://homosaurus.org/v3/homoit0000282"]
    FAIL - the same concept in two schemes collapses to one term
           got ["Transgender people","Transgender people","lgbtq-books"]

The eighth (free text passes through) passes in both worlds -- it is the control.

### Build cost: not measured, and I will not claim it was

A concurrent process was saturating the machine (load average 15-30, another
process at 407% CPU). Identical inputs gave wall times from 19 s to 61 s and CPU
times varying 15%, so no A/B here is trustworthy and I am not reporting one.

What is structural rather than timed: the old template built a per-page label
dict with `merge` inside a loop, which copies the whole map per subject, for each
of 2xN pages. The new one builds one map per language with O(1) inserts and adds a
single pass over works x subjects. No per-page map construction remains. Both
constants are small; neither should dominate a build that mints N pages per
language. Worth watching on the 62.6k corpus at adoption.

### Not touched

The admin SPA's `SimilarPanel` resolves IRIs against the vocabulary index
(`resolveTermURIs`), a different and authoritative source, so it does not have
this bug. It does still fall back to the raw URI for a term the index lacks --
defensible in a cataloger's tool, where the IRI is information, and out of scope
here.
