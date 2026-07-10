# 302 -- more-like-this rail dedupes by resolved label, so cross-scheme synonyms collapse in English but not in other languages

Filed from queerbooks-demo on 2026-07-10 (cross-repo ask).

Adopting v0.121.2. **296 is fixed and the fix is excellent** -- resolving `shared`
against the `terms` sideband labels all 698,114 of our shared IRIs, and our render
now shows **0 raw IRIs across all 113,988 rail pages**, down from 17,494. The
terms-sideband route was better than any of the three we proposed.

Two things survive, both in the same line.

## 1. The synonym collapse is English-only

296's second fix dedupes "by resolved label". That is language-dependent, and the
two schemes disagree about which languages they have.

On the same work, same neighbour, v0.121.2:

    en:  Shares: Elledge, Jim, Gay men, Lesbians
    es:  Comparte: Elledge, Jim, Gay men, Lesbians, Hombres gay, Lesbianas

`fast/939117` has an `en` label ("Gay men") and no `es`, so it falls back to
English. `homoit0000506` has an `es` label ("Hombres gay"). Two IRIs for one
concept resolve to two different strings, so nothing collapses -- and the Spanish
reader sees each concept twice, once in a language they did not ask for.

Replicating the template's resolution over our sidecar (422,355 spans):

    spans where the es rail lists MORE terms than the en rail:  86,593  (20.5%)

The English rail is clean because both schemes happen to agree there. Any locale
whose coverage is partial gets the stutter back, which is most of them.

Dedupe on the **concept** rather than the string it happens to render as. The
sidecar already ships IRIs, and the `terms` sideband already knows the tree, so
the information is present: collapse IRIs that share a `skos:exactMatch` /
`skos:closeMatch`, or -- cheaper and probably enough -- collapse by the `en`
label and *then* render the chosen survivor in the page's language.

## 2. Labels contain commas, and the line is comma-joined

    Shares: Canadian literature, Lesbians' writings, Canadian, Anthologies,
            Lesbians' writings, Literary collections

Five distinct labels, correctly deduped. But `fast/996602` **is** the string
`Lesbians' writings, Canadian`, so the joined line cannot be parsed by a reader:
it reads as six items, two of which look identical. Contributor names have the
same shape -- `Elledge, Jim` is one term that reads as two.

    spans containing at least one label with a comma in it:  185,153  (43.8%)

Our first pass at auditing this actually mis-flagged these as duplicate-dedupe
failures; they are not. But if a comma-split confuses a script, it confuses a
person.

Suggest emitting one element per term (`<li>`, or `<span class="lcat-shared-term">`)
and letting CSS supply the separator, so a label that contains a comma is still
one visible unit. A `·` or a middot join would also do.

## For the record

- 0 raw IRIs, both locales, all 113,988 rail pages (was 17,494 pages / 59,076 spans)
- 0 shared values our catalog cannot label -- the terms sideband covered every one
- `similar.json` unchanged, as your note promised (`e7ff6abe…` before and after)
- `catalog.nq.gz` **byte-identical across v0.121.0 -> v0.121.2** (sha256
  `4bebf7f9…`), the first release in this session that left the dump alone. 291
  delivers exactly what it promised.
- 298: ingest's `catalog.nq` now matches serialize's byte-for-byte, and both match
  what v0.121.0's serialize produced. Three writers, one answer.

## Outcome

Fixed in **v0.124.0** (root, hugo, backend in lockstep), commit `0434856`.

Both defects reproduced first, on stock templates. The exampleSite fixture already
had the shape: `homoit0000669` carries `en`+`es`, `fast/1735592` carries `en` only,
and both read "Transgender people" in English. Adding `fast/996602` ("Lesbians'
writings, Canadian") to a rail gave, against HEAD:

    en:  Shares: Transgender people, Lesbians' writings, Canadian
    es:  Comparte: Personas trans, Transgender people, Lesbians' writings, Canadian

Three items rendering as four, and in `es` the concept twice.

### 1. Collapse the concept, not the string

The adapter now derives **three** facts per concept instead of one:

- `$termLabels` -- what to show: site language, else English, else the lexically
  first tag.
- `$termKeys` -- what to group by: the **English** label, else the site-language
  one, else the lexically first. Language-independent, which is the whole point.
- `$termNative` -- whether the concept has a label in the site language.

A group renders the member that speaks the page's language, whatever order the
scorer ranked them in; failing that, the first occurrence, which is significance
order. Free text keys on itself and passes through.

Your cheaper suggestion is the one that shipped -- collapse by the `en` label, then
render the survivor in the page's language. `skos:exactMatch` was not needed and
would have been a second sideband to keep honest. The known cost, unchanged from
tasks/141: two vocabularies that agree on a string are treated as one concept, so a
genuine cross-scheme homograph merges. That policy already governs term pages; this
only makes it consistent across languages.

Incidentally the label map is now built once per **distinct** concept rather than
once per work-subject occurrence, so this is strictly less build work than v0.121.2,
not more.

### 2. One element per term

`<span class="lcat-similar-term">` per term, CSS supplying a middot. The admin
SPA's `SimilarPanel` had the identical `.join(", ")` defect on the same data and got
the same treatment -- its own test fixture already contained `Lobel, Arnold.` but
only asserted `toContain`, so it was blind to it.

### Mutation-tested

- Group by the *displayed* label (the v0.121.2 behaviour): 4 checks go red, **all of
  them in `es`**, English stays green. That asymmetry is the reported bug.
- Comma-join the line again: the separator check reports `no term elements`, and the
  comma check fails.
- Comma-join the SPA panel: exactly the new test fails; the old one still passes,
  which is the evidence it could not see this.

`similar_seam_test.cjs` now reads term **elements** rather than splitting on commas
-- a reader that split on commas could not tell the fix from the bug -- and gains a
catalog whose two schemes disagree about which languages they cover, with the
`en`-only member ranked *first* so that native-preference is distinguishable from
first-wins.

### Gates

65 jsdom checks, 61 Playwright checks across 4 spec files, 288 SPA unit tests,
`svelte-check` 0 errors, axe over 124 pages, link check, root + backend `go test`.
A stock exampleSite rebuild diffs clean against HEAD except the CSS fingerprint and
the four pages that have a rail -- and on those the *term set* is unchanged; only
the markup moved.

Note for next time: `hugo/e2e/run.sh` needs `PLAYWRIGHT_PKG` pointed at an install
(playwright is not in `hugo/package.json`), and piping it to `tail` hides the
`set -e` abort behind `tail`'s exit code.
