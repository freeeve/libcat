# 148 -- slim the negative-filter button markup (13GB site regression)

Filed from queerbooks-demo (2026-07-06). Do not let a queerbooks session
edit this repo -- implement here.

## Measured

queerbooks' full build went 5.9GB -> 13GB when tasks/144 landed. Term pages
tripled (20KB -> 65KB): the sidebar renders ~137 facet rows and each
exclude button repeats data the row's anchor already carries --

    <button type="button" class="lcat-facet-not" data-lcat-exclude
      data-lcat-taxonomy="subjects" data-lcat-term="fast-fiction"
      data-lcat-label="Fiction" aria-pressed="false"
      aria-label="Exclude Fiction" title="Exclude Fiction">&#x2212;</button>

data-lcat-taxonomy/term are derivable from the sibling <a href>; the label
appears 3x (data-lcat-label + aria-label + title) and once more as the
anchor's text. ~280 redundant bytes/row x rows x every sidebar-bearing page
(~300k at queerbooks' scale) ~= +7GB of HTML.

## Suggested

Emit `<button class="lcat-facet-not" aria-pressed="false">&#x2212;</button>`
and let the exclude JS derive taxonomy/term/label from the sibling anchor
(parse href + textContent), setting aria-label at hydration time. Keep a
data-attr only if the no-JS story needs it (it doesn't -- the button is
JS-only anyway; without JS it should probably be display:none). gzip hides
much of this on the wire, but storage/build-time/deploy-diff all scale with
the raw bytes, and the fix is mechanical.

## Done

As suggested, with one correction: the term key is NOT always derivable
from the href -- Hugo slugs the URL segment while x-params and the cards'
data attributes carry the exact indexed key (contributors "Byron, Grace"
-> byron-grace, classifications FIC073000 -> fic073000). So:

- The button ships as `<button type="button" class="lcat-facet-not"
  hidden>&#x2212;</button>`; `data-lcat-term` is emitted ONLY on rows whose
  key differs from the URL's last segment (the partial compares) --
  subjects/tags/languages/extra facets, i.e. the bulk at catalog scale,
  ship it attribute-free.
- lcat-negatives.js hydrates everything from the row anchor: taxonomy =
  second-to-last path segment (language prefixes and subpath deploys both
  precede it), term = data-lcat-term or the last segment, label = the
  anchor's facet-value text; sets aria-label/title/aria-pressed and
  unhides. Rows without a term-page link keep their button hidden -- and
  no JS now honestly means no visible button (the `hidden` attribute also
  keeps the nameless button out of the axe audit).
- Config JSON gained the "exclude" label template (localized aria-labels
  at hydration; verified "Excluir Inglés" on the es site).

Measured on the exampleSite term page: 251 -> 71 avg bytes/button (8 of 22
rows carry the term attr), page 17.8KB -> 13.8KB. At queerbooks' ~137
mostly-subject rows that is ~25KB back per sidebar-bearing page -- the
bulk of the 20KB->65KB regression; the remainder is the cards' data
attributes, which are load-bearing (exclusion matching). jsdom suite grew
to 7 tests (hydration, language-prefixed hrefs, key-vs-slug); links +
a11y audits pass; verified a classification exclusion end-to-end on the
built site (exact key in the x-param, display label in the chip).
