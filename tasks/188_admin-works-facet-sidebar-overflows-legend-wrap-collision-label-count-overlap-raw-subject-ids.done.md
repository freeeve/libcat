# 188 -- admin works facet sidebar overflows: legend wrap/collision, label-count overlap, raw subject ids

Opened 2026-07-08.

## Outcome

Fixed in 9ffd460, released v0.41.0; verified with a Playwright
screenshot of the playground's work-search rail (all facet groups
contained, no collisions, no raw ids).

Layout (WorkSearch.svelte): the root cause was the fieldset quirk --
fieldsets default to min-inline-size: min-content, so a long legend
("WIKIDATA (CONTROLLED VOCABULARY)") forced the whole facet box wider
than the 13rem rail column and under the results list; every symptom
in the screenshot (legends wrapping over borders, labels overlapping
counts, the box border vanishing) was that one overflow. Facet groups
now shrink (min-inline-size: 0), legends ellipsize on one line with a
title tooltip for the full text, and .facet-count gets flex-shrink: 0.

Raw subject ids (vocab.go): the "SUBJECT / homoit0000170" group was
Homosaurus release skew, not layout -- the demo store's feed data
carries /v4/ IRIs while the installed vocabulary indexes /v5/, so
Resolve missed and the facet fell into the schemeless group with
tail-segment labels. vocab.Resolve now bridges release segments
through the version-stable homoit id (probe capped at v12); Lookup
stays exact so subject edits keep storing the installed release's
canonical IRI. The rail now shows all four Homosaurus terms labeled
under one group, which correctly reads (SKOS VOCABULARY) now that the
resolved terms expose hierarchy.
