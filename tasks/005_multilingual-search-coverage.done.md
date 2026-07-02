# 005 -- Multilingual search coverage (stemmer / tokenizer / segmentation)

## Problem
The "multilingual-aware" claim was oversold. Measured against roaringrange as it
stands (ARCHITECTURE.md §8):

- **Tokenizer** (Go builder + Rust/WASM reader): Unicode-correct alphanumeric
  runs + full Unicode lowercasing, but **no word segmentation**. CJK, Thai,
  Khmer, Lao, Burmese collapse an unbroken run into one term -> word-level BM25
  fails for them.
- **Stemmer**: Snowball. The on-disk format + Rust build/reader define **18
  languages** (en es ar da nl fi fr de el hu it no pt ro ru sv ta tr), but the
  **Go projector wires only English** (`terms.go`: `TermLanguageEnglish`). Since
  `lcat` is Go, the default build path yields English or unstemmed.
- **Stop words**: optional, **English-only** list. No per-language sets.
- **Structural limit**: an `RRTI` header carries **one stemmer language**. No
  per-document language detection, no per-field stemmer -> a mixed-language
  corpus cannot stem each record in its own language within one term index.

## Scope
1. **Wire the remaining Snowball languages into the Go builder** (so `lcat` can
   emit a stemmed index in any of the 18), *or* have `lcat` shell to the Rust
   native `terms_build` for stemmed builds. Decide which; the Rust builder
   already covers all 18.
2. **Mixed-language strategy (decided: per-language indexes + routing map).**
   Build per-language stemmed indexes for the 18 Snowball languages, routed by the
   record's declared language (`dcterms:language` / MARC 008/041). **Unsupported
   (non-Snowball) languages get their own index**, not the shared fallback:
   unstemmed word-level for space-delimited scripts (Polish, Vietnamese, Hebrew,
   ...), and the trigram (`RRS`) n-gram index for unsegmented scripts (CJK, Thai,
   Khmer, Lao). Records with no language signal fall back to the unstemmed index.
   A small **language -> index routing map** (a manifest built at projection time
   from the languages actually present) lets the query path search only the
   relevant index(es). No per-document language *detection* -- routing keys on the
   declared language.
   Open sub-question: query-time language *selection* -- how the router picks
   which mapped index(es) to hit (detect query language vs fan-out-and-merge vs a
   UI language scope; the map makes any of these cheap).
3. **CJK / substring path.** Ensure the projector emits the trigram (`RRS`)
   index alongside `RRTI`, and the UI queries `RRS` for CJK/scriptio-continua and
   substring/fuzzy. Confirm recall on a CJK sample.
4. **Non-English stop words** (optional, low priority): per-language stop sets,
   or document that stop-word removal is English-only and off for other corpora.
   Tracked upstream in roaringrange `tasks/055` (language-keyed stop words).

## Acceptance
- [x] A non-English (e.g. Spanish or French) corpus builds a correctly stemmed index
  via `lcat`, verified against the Rust reader (build/query stem symmetry). **Done**
  (see Delivered below).
- [x] A CJK sample is searchable via the trigram index with reasonable recall.
  **Done** -- unsegmented-script languages route to a trigram (`RRSI`) index; a
  Chinese sample's substring query recalls its doc through roaringrange's Go RRSI
  reader (`TestTrigramRecall`). See Delivered below.
- [x] ARCHITECTURE.md §8 and any deployment docs state the coverage accurately
  (§8 updated: all 18 stem Go-side + Rust reader as of roaringrange v0.27.0).

## Delivered -- all-18 Snowball stemming Go-side (roaringrange `tasks/073`, v0.27.0)

Scope item 1 (wire the remaining Snowball languages into the Go builder) and the
Go-side stemmer wiring for the mixed-language strategy (item 2) are done. roaringrange
v0.27.0's `NewTermTokenizerFull` now builds a Snowball stemmer for all 18 supported
languages Go-side (byte-exact vs the Rust reader, proven by its
`TestTokenizerStemMatchesRustGolden`), so no shelling to the Rust builder is needed --
`lcat index` builds stemmed indexes natively in Go. libcatalog's change was minimal, as
`073`'s consumer note predicted: `search.termLanguage` now returns `stem = tl != None`
(was English-only), and the existing `iso639` map (already all 18) + the per-language
index/routing structure from `tasks/010` did the rest. Validated: the corpus's `spa`
index now builds stemmed (`stemmed:true`, 373 works); manifest records per-index
`termLanguage`/`stemmed` so the reader tokenizes queries identically.

## Delivered -- trigram (RRSI) arm for unsegmented scripts (item 3)

`search/search.go` now routes by script: a language whose script is scriptio-continua
(`unsegmented` set: Chinese chi/zho, Japanese jpn, Thai tha, Khmer khm, Lao lao,
Burmese mya/bur, Tibetan bod/tib -- Korean excluded, Hangul is space-delimited) builds
a **trigram (`RRSI`, `.rrs`) index** via `rr.NewTrigramMonolithBuilder` instead of a
word-level RRTI, since word tokenization collapses an unbroken run into one useless
token. `buildLangIndex` dispatches to `buildTrigramIndex` or `buildTermIndex`; the
manifest records `kind` ("terms"|"trigram") + `gramSize` so the reader picks the query
path (`NgramKeys` for trigram). **Manifest SchemaVersion 2 -> 3.** Recall proven
Go-side (no browser): `TestTrigramRecall` builds a Chinese corpus, opens the `.rrs`
through roaringrange's `rr.Open` RRSI reader, and asserts the trigram keys of a
substring query recall the matching doc and reject the other. The real eng/spa corpus
has no CJK, so every index there stays `kind:terms` (v3 manifest confirmed).

**Remaining:** per-language stop words (item 4, roaringrange `tasks/055`) -- non-English
`stopwords=true` currently applies whatever roaringrange has (English list until 055
lands language-keyed sets). The query-time index-selection sub-question (item 2) stays
open, tied to the `tasks/009` browser reader (which is also `010`'s last gate).

## Notes
Upstream drift flagged in roaringrange: `TERMS.md` still says the header
`language` byte is "`1` = English; `0` otherwise," but `terms.rs` defines all 18.
The TERMS.md sync and language-keyed stop words are tracked in roaringrange
`tasks/055`.
