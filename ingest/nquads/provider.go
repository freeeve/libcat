// Package nquads is the generic mapped N-Quads ingest provider (tasks/172):
// it streams a dcterms-shaped .nq export into ingest records driven entirely
// by a declarative TOML mapping -- work-IRI prefix, predicate->field map,
// identifier URN schemes, and source-attestation tiers -- so a deployment
// sideloads an RDF export the way Aspen Discovery sideloads MARC: with a
// profile, not code. Works sharing identifier keys (e.g. ISBNs) with a
// primary feed merge in the shared clustering pipeline; unshared works mint
// as their own. Generalized from the queerbooks-demo collnq provider.
package nquads

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/freeeve/libcat/ingest"
	"github.com/freeeve/libcodex/rdf"
)

// ProviderName is the registry key and default provenance feed (feed:nquads).
const ProviderName = "nquads"

// Provider streams a catalog .nq file into ingest records, one per work IRI.
type Provider struct {
	feed          string
	path          string
	m             *Mapping
	idScheme      string
	dropTentative bool
}

// New builds the provider from an ingest.Config: Source is the .nq path,
// Params["mapping"] the mapping TOML path, Feed overrides the provenance feed
// name, and Params["tentative"]="drop" drops works whose only attestation is
// a tentative source instead of ingesting them.
func New(cfg ingest.Config) (ingest.Provider, error) {
	if cfg.Source == "" {
		return nil, fmt.Errorf("nquads: Source (.nq path) is required")
	}
	mappingPath := cfg.Params["mapping"]
	if mappingPath == "" {
		return nil, fmt.Errorf("nquads: Params[\"mapping\"] (mapping TOML path) is required")
	}
	m, err := LoadMapping(mappingPath)
	if err != nil {
		return nil, err
	}
	feed := cfg.Feed
	if feed == "" {
		feed = ProviderName
	}
	idScheme := m.IDScheme
	if idScheme == "" {
		idScheme = feed
	}
	drop := false
	switch v := cfg.Params["tentative"]; v {
	case "", "keep":
	case "drop":
		drop = true
	default:
		return nil, fmt.Errorf("nquads: unknown tentative param %q (keep|drop)", v)
	}
	return &Provider{feed: feed, path: cfg.Source, m: m, idScheme: idScheme, dropTentative: drop}, nil
}

// Name is the provenance feed the run writes (feed:<name>).
func (p *Provider) Name() string { return p.feed }

// Role marks this an ingest-role provider.
func (p *Provider) Role() ingest.Role { return ingest.RoleIngest }

// Records parses the export and returns one record per work, ordered per the
// mapping's id-order so ingest runs are deterministic.
func (p *Provider) Records(ctx context.Context) ([]ingest.Record, error) {
	f, err := os.Open(p.path)
	if err != nil {
		return nil, fmt.Errorf("nquads: open %s: %w", p.path, err)
	}
	defer f.Close()

	fieldFor := p.m.fieldFor()
	tentative := map[string]bool{}
	for _, iri := range p.m.Sources.Tentative {
		tentative[iri] = true
	}
	works := map[string]*work{}
	labels := map[string]string{}
	get := func(iri string) *work {
		id := strings.TrimPrefix(iri, p.m.WorkPrefix)
		w := works[id]
		if w == nil {
			w = &work{id: id}
			works[id] = w
		}
		return w
	}
	dec := rdf.NewDecoder(f, rdf.NQuads)
	defer dec.Close()
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		q, err := dec.DecodeQuad()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("nquads: parse %s: %w", p.path, err)
		}
		field := fieldFor[q.P.Value]
		if !strings.HasPrefix(q.S.Value, p.m.WorkPrefix) {
			// Authority labels ride on the concept IRI itself, outside the
			// work prefix.
			if field == "prefLabel" && labels[q.S.Value] == "" {
				labels[q.S.Value] = q.O.Value
			}
			continue
		}
		w := get(q.S.Value)
		switch field {
		case "title":
			if w.title == "" {
				w.title = q.O.Value
			}
		case "creator":
			w.creators = append(w.creators, q.O.Value)
		case "identifier":
			for prefix, scheme := range p.m.Identifiers {
				if v, ok := strings.CutPrefix(q.O.Value, prefix); ok {
					if scheme == "isbn" {
						w.isbns = append(w.isbns, v)
					} else {
						w.ids = append(w.ids, schemedID{scheme: scheme, value: v})
					}
					break
				}
			}
		case "subject":
			w.subjectURIs = append(w.subjectURIs, q.O.Value)
		case "language":
			if w.lang == "" {
				w.lang = p.m.language(q.O.Value)
			}
		case "source":
			w.sources = append(w.sources, strings.TrimPrefix(q.O.Value, p.m.Sources.Prefix))
			if !tentative[q.O.Value] {
				w.confident = true
			}
		}
	}

	ids := make([]string, 0, len(works))
	dropped := 0
	for id, w := range works {
		if w.title == "" {
			dropped++
			continue
		}
		if p.dropTentative && !w.confident {
			dropped++
			continue
		}
		ids = append(ids, id)
	}
	if p.m.IDOrder == "numeric" {
		sort.Sort(byNumericID(ids))
	} else {
		sort.Strings(ids)
	}
	if dropped > 0 {
		fmt.Fprintf(os.Stderr, "nquads: dropped %d works (untitled or tentative-only with tentative=drop)\n", dropped)
	}
	recs := make([]ingest.Record, 0, len(ids))
	for _, id := range ids {
		recs = append(recs, record{w: works[id], labels: labels, m: p.m, idScheme: p.idScheme})
	}
	return recs, nil
}

// byNumericID orders work ids numerically when possible (unpadded decimal
// ids), falling back to lexical order for non-numeric ids.
type byNumericID []string

func (s byNumericID) Len() int      { return len(s) }
func (s byNumericID) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s byNumericID) Less(i, j int) bool {
	if len(s[i]) != len(s[j]) && isDigits(s[i]) && isDigits(s[j]) {
		return len(s[i]) < len(s[j])
	}
	return s[i] < s[j]
}

func isDigits(v string) bool {
	for i := 0; i < len(v); i++ {
		if v[i] < '0' || v[i] > '9' {
			return false
		}
	}
	return len(v) > 0
}
