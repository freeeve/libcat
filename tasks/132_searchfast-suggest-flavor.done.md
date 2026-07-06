# 132 -- vocabsrc: a `searchfast` suggest flavor for OCLC FAST typeahead

Filed from queerbooks-demo (2026-07-06). Do not let a queerbooks session edit
this repo -- implement here.

## Why

queerbooks-demo's lcatd instance (Lambda, scale-to-zero) needs librarians to
assign FAST terms that are NOT in the corpus subset. Full-FAST residency is
not Lambda-shaped (~2M concepts; multi-GB index rebuilt every cold start; Go
has no SnapStart; provisioned concurrency costs more than a small EC2). LCSH
already solves this with live suggest2 typeahead to id.loc.gov -- FAST needs
the same, but OCLC's API is not one of the implemented dialects
(suggest2 | wikidata | viaf in backend/vocabsrc/suggest.go).

## What

- A `searchfast` SuggestFlavor: OCLC's FAST suggest API
  (https://fast.oclc.org/searchfast/fastsuggest?query=...&fl=suggestall,idroot
  &rows=N -- verify current endpoint/params; assignFAST is the sibling
  service). Parse to []Suggestion with the `fst`-prefixed idroot mapped to
  the canonical URI form http://id.worldcat.org/fast/<id-without-fst-and-
  zero-padding> (matches the coll ingest provider's fastURI mapping in
  queerbooks-demo).
- A `fast` builtin Source (or documented drop-in registry entry):
  Scheme "fast", suggest-only (the full dump stays uninstalled; a corpus
  subset snapshot supplies display labels), license/homepage per OCLC.
- The existing enrichment registration (suggest-capable sources register as
  moderated enrichment targets at boot) should just work; note it in tests.

## Verify first

OCLC has been restructuring research services -- confirm searchFAST's
availability and terms of use before building; if it is gone, the fallbacks
are (a) full-FAST on a t4g.medium container instead of Lambda, or (b) a
self-hosted FAST suggest microservice fed from the dump.
