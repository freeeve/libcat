# 067 -- Built-in public authority sources (click-to-download vocabularies)

## Context

The authority module (tasks/046) is SKOS-native with local->established
promotion, and `ingest/locsh` already demonstrates a live-suggest client
against the public id.loc.gov suggest2 API. The vocab index (tasks/032)
supports atomic snapshot swap. Ship preconfigured public sources on both
patterns: live typeahead APIs and bulk snapshots installed from a
click-to-download list.

## Scope

1. **Source registry**: config shape `{name, kind: suggest|snapshot,
   endpoint, dataset, license, homepage}`. Built-ins: id.loc.gov datasets
   (LCSH, LCGFT, LCNAF, Children's Subjects), Wikidata entity search, VIAF
   AutoSuggest. GND (lobid-gnd), Getty AAT/TGN/ULAN, MeSH, and Homosaurus
   documented as drop-in configs, not code.
2. **Live suggest clients**: generalize the locsh client shape to the
   registry; VocabPicker offers registered suggest sources with per-source
   badges; enrichment reconciliation can target any registered source.
3. **Snapshot download manager**: `GET /v1/vocabsources` lists downloadable
   vocabularies with size, license, and installed version; `POST
   /v1/vocabsources/{id}/download` runs a job (reuse the tasks/038 job
   lifecycle) that fetches the bulk distribution, converts to the vocab
   index format, and snapshot-swaps on completion; refresh and remove
   actions. SPA screen with click-to-download and progress. LCSH/LCGFT are
   snapshot-sized; LCNAF (~11M) stays live-API only.
4. **Cross-references**: promotion works against any installed source;
   record `skos:exactMatch` when a picked term carries sibling ids (VIAF
   cluster -> LCNAF/GND/Wikidata).

## Acceptance

- A fresh install lists available vocabularies; clicking download on LCGFT
  yields working offline typeahead in VocabPicker.
- Names typeahead returns LCNAF, VIAF, and Wikidata live results with
  source badges.
- An installed snapshot refreshes and removes cleanly; the index swap is
  atomic under concurrent reads.
