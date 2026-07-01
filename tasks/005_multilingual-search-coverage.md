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
- A non-English (e.g. Spanish or French) corpus builds a correctly stemmed index
  via `lcat`, verified against the Rust reader (build/query stem symmetry).
- A CJK sample is searchable via the trigram index with reasonable recall.
- ARCHITECTURE.md §8 and any deployment docs state the coverage accurately
  (done for §8).

## Notes
Upstream drift flagged in roaringrange: `TERMS.md` still says the header
`language` byte is "`1` = English; `0` otherwise," but `terms.rs` defines all 18.
The TERMS.md sync and language-keyed stop words are tracked in roaringrange
`tasks/055`.
