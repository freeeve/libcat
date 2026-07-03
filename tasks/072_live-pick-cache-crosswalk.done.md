# 072 -- Cache live picks; crosswalk equivalents (exactMatch/closeMatch)

## Context

A subject picked from a live source (Wikidata QID) renders unresolved after
save: the label only ever existed in the picker, the grain stores the URI,
and the live scheme has no local index entry. Separately, homosaurus's full
dump links terms to LCSH via skos:exactMatch and (mostly) skos:closeMatch --
walking those relationships is the automatic homosaurus->LCSH crosswalk, but
the index does not load closeMatch and nothing surfaces equivalents. (The
current dev homosaurus subset carries no match links; a full dump installed
through tasks/067 does.)

## Scope

1. **closeMatch**: the vocab index loads skos:closeMatch (Term.CloseMatch)
   and the tasks/067 snapshot converter keeps it.
2. **Cache live picks**: picking a term from a live source writes a minimal
   term grain (prefLabel, definition, exactMatch) into the authorities tree
   under cache/<scheme>/ and reloads the index -- the pick labels forever,
   across restarts, and its exactMatch siblings enter the crosswalk data.
   POST /v1/vocabcache (librarian); the picker fires it on live picks.
3. **Equivalents in the neighborhood panel**: exactMatch + closeMatch URIs
   resolve scheme-agnostically and render as an "Equivalents" group with
   scheme badges and the same Replace/Add actions -- the one-click manual
   crosswalk. Unresolved equivalents show muted with the install hint.
4. **Crosswalk enrichment**: a per-target-scheme enricher
   (crosswalk-<scheme>, queue mode) walks every work subject's
   exactMatch/closeMatch into the target scheme and queues moderated
   add-subject suggestions -- the batch crosswalk over the whole corpus.

## Acceptance

- Picking a Wikidata term, saving, and reloading shows its label chip, not
  the QID.
- A term with a closeMatch into an installed scheme shows it under
  Equivalents with Replace/Add.
- POST /v1/enrich/crosswalk-lcsh/run queues suggestions for works whose
  homosaurus subjects match LCSH terms (given both vocabularies loaded).
