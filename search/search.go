// Package search builds the catalog's lexical search index from the projected
// catalog (ARCHITECTURE §8). It emits one roaringrange term index (.rrt) per
// corpus language plus a manifest routing each language to its index -- the data
// the browser's WASM reader queries. Building is the only half done in Go:
// roaringrange has no Go term-index reader, so queries run in the Rust/WASM
// reader the Hugo module ships.
package search

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/RoaringBitmap/roaring/v2"
	"github.com/freeeve/libcatalog/project"
	"github.com/freeeve/libcatalog/storage"
	rr "github.com/freeeve/roaringrange"
)

// SchemaVersion is the search-manifest schema version, checked by the reader.
const SchemaVersion = 1

// undetermined is the index key for Works with no declared language.
const undetermined = "und"

// Manifest is the language->index routing map the browser reader loads: one entry
// per language present in the corpus (§8).
type Manifest struct {
	Version int         `json:"version"`
	Indexes []IndexInfo `json:"indexes"`
}

// IndexInfo describes one per-language term index and how it was tokenized, so
// the reader tokenizes queries identically.
type IndexInfo struct {
	Language     string `json:"language"`     // ISO 639-2 code, or "und" when undeclared
	TermLanguage uint8  `json:"termLanguage"` // roaringrange stemmer-language byte
	Stemmed      bool   `json:"stemmed"`
	Stopwords    bool   `json:"stopwords"`
	Index        string `json:"index"` // .rrt filename
	Docs         string `json:"docs"`  // JSON array: doc id (index) -> Work id
	DocCount     int    `json:"docCount"`
}

// BuildIndexes writes one term index per corpus language into sink, plus a
// doc-id->Work-id list per index and a search-manifest.json routing map. A Work
// is indexed once per language it declares; a Work with none goes to the
// undetermined index. Doc ids are dense from 0 in the projected (sorted) order.
func BuildIndexes(cat *project.Catalog, sink storage.Sink) (Manifest, error) {
	byLang := map[string][]project.Work{}
	for _, w := range cat.Works {
		if len(w.Languages) == 0 {
			byLang[undetermined] = append(byLang[undetermined], w)
			continue
		}
		for _, l := range w.Languages {
			byLang[l] = append(byLang[l], w)
		}
	}

	langs := make([]string, 0, len(byLang))
	for l := range byLang {
		langs = append(langs, l)
	}
	slices.Sort(langs)

	m := Manifest{Version: SchemaVersion}
	for _, lang := range langs {
		info, err := buildLangIndex(sink, lang, byLang[lang])
		if err != nil {
			return m, err
		}
		m.Indexes = append(m.Indexes, info)
	}
	if err := writeJSON(sink, "search-manifest.json", m); err != nil {
		return m, err
	}
	return m, nil
}

// buildLangIndex builds and writes the term index for one language group.
func buildLangIndex(sink storage.Sink, lang string, works []project.Work) (IndexInfo, error) {
	tl, stem := termLanguage(lang)
	stopwords := tl != rr.TermLanguageNone
	tok := rr.NewTermTokenizerFull(tl, stem, stopwords, true)

	postings := map[string]*roaring.Bitmap{}
	docIDs := make([]string, len(works))
	for i, w := range works {
		docIDs[i] = w.ID
		terms := tok.Tokenize(searchText(w))
		slices.Sort(terms)
		terms = slices.Compact(terms) // presence, not frequency: plain boolean index
		for _, t := range terms {
			bm := postings[t]
			if bm == nil {
				bm = roaring.New()
				postings[t] = bm
			}
			bm.Add(uint32(i))
		}
	}

	idxName := "term-" + lang + ".rrt"
	docsName := "term-" + lang + ".docs.json"
	if err := writeIndex(sink, idxName, postings, tl, stem, stopwords); err != nil {
		return IndexInfo{}, err
	}
	if err := writeJSON(sink, docsName, docIDs); err != nil {
		return IndexInfo{}, err
	}
	return IndexInfo{
		Language:     lang,
		TermLanguage: uint8(tl),
		Stemmed:      stem,
		Stopwords:    stopwords,
		Index:        idxName,
		Docs:         docsName,
		DocCount:     len(works),
	}, nil
}

// writeIndex writes a plain (boolean whole-word) term index. headBoundary is the
// default 65536; blockCap 0 selects roaringrange's default dict block size.
func writeIndex(sink storage.Sink, name string, postings map[string]*roaring.Bitmap, tl rr.TermLanguage, stem, stopwords bool) error {
	w, err := sink.Create(name)
	if err != nil {
		return err
	}
	if err := rr.WriteTermIndexFull(w, postings, 65536, tl, stem, stopwords, true, 0); err != nil {
		w.Close()
		return fmt.Errorf("write term index %s: %w", name, err)
	}
	return w.Close()
}

// searchText is the text a Work contributes to the index: its title, subtitle,
// contributor names, and subject labels.
func searchText(w project.Work) string {
	var b strings.Builder
	b.WriteString(w.Title)
	if w.Subtitle != "" {
		b.WriteByte(' ')
		b.WriteString(w.Subtitle)
	}
	for _, c := range w.Contributors {
		b.WriteByte(' ')
		b.WriteString(c.Name)
	}
	for _, s := range w.Subjects {
		b.WriteByte(' ')
		b.WriteString(s)
	}
	return b.String()
}

// termLanguage maps an ISO 639-2 language code to a roaringrange stemmer language
// and whether stemming is applied. roaringrange wires a Snowball stemmer on the Go
// build side only for English, so other languages index word-level (stop words
// still apply) until their stemmers are wired (see tasks/010).
func termLanguage(iso string) (rr.TermLanguage, bool) {
	if tl, ok := iso639[iso]; ok {
		return tl, tl == rr.TermLanguageEnglish
	}
	return rr.TermLanguageNone, false
}

// iso639 maps ISO 639-2 codes to roaringrange's 18 Snowball stemmer languages.
var iso639 = map[string]rr.TermLanguage{
	"eng": rr.TermLanguageEnglish, "spa": rr.TermLanguageSpanish, "ara": rr.TermLanguageArabic,
	"dan": rr.TermLanguageDanish, "dut": rr.TermLanguageDutch, "nld": rr.TermLanguageDutch,
	"fin": rr.TermLanguageFinnish, "fre": rr.TermLanguageFrench, "fra": rr.TermLanguageFrench,
	"ger": rr.TermLanguageGerman, "deu": rr.TermLanguageGerman, "gre": rr.TermLanguageGreek,
	"ell": rr.TermLanguageGreek, "hun": rr.TermLanguageHungarian, "ita": rr.TermLanguageItalian,
	"nor": rr.TermLanguageNorwegian, "por": rr.TermLanguagePortuguese, "rum": rr.TermLanguageRomanian,
	"ron": rr.TermLanguageRomanian, "rus": rr.TermLanguageRussian, "swe": rr.TermLanguageSwedish,
	"tam": rr.TermLanguageTamil, "tur": rr.TermLanguageTurkish,
}

// writeJSON marshals v as indented JSON through the sink.
func writeJSON(sink storage.Sink, name string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	w, err := sink.Create(name)
	if err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}
