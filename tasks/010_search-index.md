# 010 -- Search index: roaringrange wiring (lexical, per-language)

## Context

With `catalog.json` available (`lcat project`), build the search index the Hugo
module's browser reader queries (ARCHITECTURE §8). Parallelizable with `tasks/009`.

## Scope

A `search/` package wiring roaringrange (`github.com/freeeve/roaringrange`):

1. **Lexical / BM25 default** -- the `terms` path; no embeddings, no paid AI in
   the default build. The vector arm stays a build flag, off by default.
2. **Per-language stemmed indexes** -- an `RRTI` index carries one stemmer
   language, so build one stemmed index per corpus language, routed by each Work's
   `languages` (from catalog.json / MARC 041). Emit the small **language->index
   map** the query side uses. The 18 Snowball languages get stemmed indexes;
   unsupported languages get an unstemmed word-level index; unsegmented scripts
   (CJK/Thai/...) route to the trigram (`RRS`) index; no language signal -> the
   unstemmed fallback.
3. **Go-side stemmer wiring** -- the Go projector currently wires only English;
   the other 17 Snowball languages need Go-side wiring (or building via the Rust
   builder). Coordinate with roaringrange `tasks/055` (language-keyed stop words)
   and the Snowball coverage note in §8.
4. **Fields** -- index title, contributors, subjects (incl. curated/editorial);
   store the Work id for result -> page linking.

## Acceptance

- [x] `lcat` builds per-language indexes + the routing map from the projected data.
- [ ] The roaringrange WASM reader answers a query in the browser over the emitted
      index (smoke test in the Hugo module, `tasks/009`).
- [x] Embeddings remain opt-in (build flag), off by default -- v1 is lexical only;
      no vector arm is wired.

## Build notes (v1 shipped)

The `search/` package builds `lcat index --catalog catalog.json --out <dir>`:
one `term-<lang>.rrt` (roaringrange `WriteTermIndexFull`) + `term-<lang>.docs.json`
(dense doc-id -> Work id) per corpus language, plus `search-manifest.json` routing
language -> index with the tokenizer settings (termLanguage, stemmed, stopwords) so
the reader tokenizes queries identically. Indexed text: title, subtitle, contributor
names, subject labels. Doc ids are dense from 0 in projected (sorted) order.

Validated on the corpus: 5659 Works -> `eng(5286)` + `spa(373)` indexes (488K/79K
`.rrt`); no `und` index (every Work carries a language). `go test ./search` passes.

### Constrained by roaringrange (Go build side) -- filed for roaringrange

1. **Plain boolean index, not BM25.** `WriteTermIndexFull` is wired; the BM25
   *impact* path (`WriteImpacts`) needs per-term head byte-offsets roaringrange does
   not expose Go-side, so v1 indexes term *presence* (postings dedup'd per doc), not
   frequency. Ranking is therefore boolean/positional, not BM25.
2. **English-only stemming.** roaringrange wires a Snowball stemmer Go-side only for
   English; `iso639` maps the other 17 Snowball languages to their `TermLanguage`
   byte but they index word-level (stop words still apply) until wired. `spa` above
   is unstemmed for this reason.
3. **No trigram (`RRS`) arm yet** for unsegmented scripts (CJK/Thai) -- those would
   currently fall to word-level; wire once (1)/(2) land.

These three are Go-build-side gaps in roaringrange (BM25 head-offset exposure +
non-English Snowball wiring + trigram builder). Tracked in roaringrange
`tasks/073` (uncommitted, per the cross-repo boundary). Verified against
roaringrange source: `WriteTermIndexFull` returns only `error` (never the per-term
`head_off` `WriteImpacts` needs, and no reader enumerates the dict); go-stemmers
already ships all 18 languages but `NewTermTokenizerFull` wires only English.
Manifest carries `version` + per-index tokenizer flags so the reader can adapt as
these land without a reformat.
