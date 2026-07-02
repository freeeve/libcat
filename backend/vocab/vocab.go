// Package vocab loads controlled vocabularies from SKOS authority grains and
// serves the in-memory term index behind term validation, the picker's
// autocomplete, and neighborhood browsing. A vocabulary's quads live in its
// authority:<vocab> named graph (ARCHITECTURE §5), so the loader routes terms
// to schemes by graph name -- one authorities tree can carry homosaurus, lcsh,
// and local terms side by side. This replaces qllpoc's embedded
// homosaurus-min.json with a vocabulary-agnostic store-backed load.
package vocab

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/freeeve/libcodex/rdf"

	"github.com/freeeve/libcatalog/storage/blob"
)

// SKOS predicate IRIs.
const (
	skosPrefLabel  = "http://www.w3.org/2004/02/skos/core#prefLabel"
	skosAltLabel   = "http://www.w3.org/2004/02/skos/core#altLabel"
	skosDefinition = "http://www.w3.org/2004/02/skos/core#definition"
	skosBroader    = "http://www.w3.org/2004/02/skos/core#broader"
	skosNarrower   = "http://www.w3.org/2004/02/skos/core#narrower"
	skosRelated    = "http://www.w3.org/2004/02/skos/core#related"
	rdfsLabel      = "http://www.w3.org/2000/01/rdf-schema#label"
	// authorityGraphPrefix matches bibframe.AuthorityGraph's naming.
	authorityGraphPrefix = "authority:"
)

// Term is one controlled-vocabulary concept.
type Term struct {
	Scheme     string              `json:"scheme"`
	ID         string              `json:"id"`                   // the authority URI
	Labels     map[string]string   `json:"labels"`               // lang -> prefLabel ("" key = untagged)
	AltLabels  map[string][]string `json:"altLabels,omitempty"`  // lang -> used-for labels
	Definition map[string]string   `json:"definition,omitempty"` // lang -> scope note
	Broader    []string            `json:"broader,omitempty"`
	Narrower   []string            `json:"narrower,omitempty"`
	Related    []string            `json:"related,omitempty"`
}

// Label returns the term's best label for lang: exact match, then English,
// then untagged, then any.
func (t *Term) Label(lang string) string {
	for _, k := range []string{lang, "en", ""} {
		if l, ok := t.Labels[k]; ok {
			return l
		}
	}
	for _, l := range t.Labels {
		return l
	}
	return t.ID
}

// Index is the loaded, immutable term index. Build once at startup (or on a
// reload signal); reads are lock-free.
type Index struct {
	schemes map[string]map[string]*Term
	// search holds, per scheme, entries sorted by normalized label for
	// prefix search across pref and alt labels in every language.
	search map[string][]searchEntry
}

type searchEntry struct {
	norm string
	uri  string
}

// Load reads every authority grain under prefix from the store and indexes
// the terms of the requested schemes (nil/empty schemes = all authority
// graphs found).
func Load(ctx context.Context, st blob.Store, prefix string, schemes []string) (*Index, error) {
	want := map[string]bool{}
	for _, s := range schemes {
		want[s] = true
	}
	ix := &Index{schemes: map[string]map[string]*Term{}, search: map[string][]searchEntry{}}
	for entry, err := range st.List(ctx, prefix) {
		if err != nil {
			return nil, fmt.Errorf("vocab: list authorities: %w", err)
		}
		if !strings.HasSuffix(entry.Path, ".nq") {
			continue
		}
		data, _, err := st.Get(ctx, entry.Path)
		if err != nil {
			return nil, fmt.Errorf("vocab: read %s: %w", entry.Path, err)
		}
		ds, err := rdf.ParseNQuads(data)
		if err != nil {
			return nil, fmt.Errorf("vocab: parse %s: %w", entry.Path, err)
		}
		ix.addDataset(ds, want)
	}
	ix.finish()
	return ix, nil
}

func (ix *Index) addDataset(ds *rdf.Dataset, want map[string]bool) {
	for _, q := range ds.Quads {
		scheme, ok := strings.CutPrefix(q.G.Value, authorityGraphPrefix)
		if !ok || !q.S.IsIRI() {
			continue
		}
		if len(want) > 0 && !want[scheme] {
			continue
		}
		t := ix.term(scheme, q.S.Value)
		switch q.P.Value {
		case skosPrefLabel:
			if q.O.IsLiteral() {
				t.Labels[q.O.Lang] = q.O.Value
			}
		case rdfsLabel:
			if q.O.IsLiteral() {
				if _, ok := t.Labels[q.O.Lang]; !ok {
					t.Labels[q.O.Lang] = q.O.Value
				}
			}
		case skosAltLabel:
			if q.O.IsLiteral() {
				if t.AltLabels == nil {
					t.AltLabels = map[string][]string{}
				}
				t.AltLabels[q.O.Lang] = append(t.AltLabels[q.O.Lang], q.O.Value)
			}
		case skosDefinition:
			if q.O.IsLiteral() {
				if t.Definition == nil {
					t.Definition = map[string]string{}
				}
				if _, ok := t.Definition[q.O.Lang]; !ok {
					t.Definition[q.O.Lang] = q.O.Value
				}
			}
		case skosBroader:
			if q.O.IsIRI() {
				t.Broader = appendUnique(t.Broader, q.O.Value)
			}
		case skosNarrower:
			if q.O.IsIRI() {
				t.Narrower = appendUnique(t.Narrower, q.O.Value)
			}
		case skosRelated:
			if q.O.IsIRI() {
				t.Related = appendUnique(t.Related, q.O.Value)
			}
		}
	}
}

func (ix *Index) term(scheme, uri string) *Term {
	byURI := ix.schemes[scheme]
	if byURI == nil {
		byURI = map[string]*Term{}
		ix.schemes[scheme] = byURI
	}
	t := byURI[uri]
	if t == nil {
		t = &Term{Scheme: scheme, ID: uri, Labels: map[string]string{}}
		byURI[uri] = t
	}
	return t
}

// finish sorts relation lists and builds the per-scheme search slices.
func (ix *Index) finish() {
	for scheme, byURI := range ix.schemes {
		var entries []searchEntry
		for uri, t := range byURI {
			sort.Strings(t.Broader)
			sort.Strings(t.Narrower)
			sort.Strings(t.Related)
			seen := map[string]bool{}
			for _, l := range t.Labels {
				if n := normLabel(l); n != "" && !seen[n] {
					seen[n] = true
					entries = append(entries, searchEntry{norm: n, uri: uri})
				}
			}
			for _, alts := range t.AltLabels {
				for _, l := range alts {
					if n := normLabel(l); n != "" && !seen[n] {
						seen[n] = true
						entries = append(entries, searchEntry{norm: n, uri: uri})
					}
				}
			}
		}
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].norm != entries[j].norm {
				return entries[i].norm < entries[j].norm
			}
			return entries[i].uri < entries[j].uri
		})
		ix.search[scheme] = entries
	}
}

// Schemes lists the loaded vocabulary keys, sorted.
func (ix *Index) Schemes() []string {
	out := make([]string, 0, len(ix.schemes))
	for s := range ix.schemes {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// Lookup returns the term by scheme and URI -- the validation gate: only
// terms that resolve here are accepted into suggestions or subject edits.
func (ix *Index) Lookup(scheme, id string) (*Term, bool) {
	t, ok := ix.schemes[scheme][id]
	return t, ok
}

// Search returns up to limit terms whose pref or alt label (any language)
// starts with q, deduped, ordered by label.
func (ix *Index) Search(scheme, q string, limit int) []*Term {
	entries := ix.search[scheme]
	norm := normLabel(q)
	if norm == "" || limit <= 0 {
		return nil
	}
	start := sort.Search(len(entries), func(i int) bool { return entries[i].norm >= norm })
	var out []*Term
	seen := map[string]bool{}
	for i := start; i < len(entries) && strings.HasPrefix(entries[i].norm, norm); i++ {
		uri := entries[i].uri
		if seen[uri] {
			continue
		}
		seen[uri] = true
		out = append(out, ix.schemes[scheme][uri])
		if len(out) >= limit {
			break
		}
	}
	return out
}

func normLabel(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

func appendUnique(list []string, v string) []string {
	if slices.Contains(list, v) {
		return list
	}
	return append(list, v)
}
