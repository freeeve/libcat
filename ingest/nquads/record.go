package nquads

import (
	"sort"
	"strings"

	"github.com/freeeve/libcat/identity"
	"github.com/freeeve/libcat/ingest"
	codexbf "github.com/freeeve/libcodex/bibframe"
)

// schemedID is one non-ISBN identifier with its mapping-declared scheme.
type schemedID struct{ scheme, value string }

// work is one accumulated export work: the record fields plus which source
// objects attested it.
type work struct {
	id          string
	title       string
	creators    []string
	isbns       []string
	ids         []schemedID
	lang        string // ISO 639-2/B, "" -> mapping default
	subjectURIs []string
	sources     []string
	confident   bool // attested by any non-tentative source
}

// record adapts a work to ingest.Record. One record per work -- a mapped
// export is expected to be pre-deduped, so there are no per-format buckets.
// labels is the shared authority-IRI -> prefLabel map parsed from the export.
type record struct {
	w        *work
	labels   map[string]string
	m        *Mapping
	idScheme string
}

// Identity namespaces the author key with the export work id: the export
// already deduped its works, so the computed author|title key must not
// re-merge distinct works that share an access point. Cross-feed merging with
// a primary feed happens only through the identifier keys (ISBNs and the
// mapping's non-ISBN schemes -- durable for isbn-less works whose export ids
// renumber between dumps).
func (r record) Identity() identity.Record {
	title := r.w.title
	if title == "" {
		title = "[untitled]"
	}
	author := ""
	if len(r.w.creators) > 0 {
		author = lastFirst(r.w.creators[0])
	}
	rec := identity.Record{
		Author: r.idScheme + ":" + r.w.id + " " + author,
		Title:  title,
		Lang:   r.lang(),
	}
	rec.ProviderKeys = append(rec.ProviderKeys, identity.ProviderKey(identity.SchemeID, r.providerID()))
	for _, isbn := range r.w.isbns {
		rec.ProviderKeys = append(rec.ProviderKeys, identity.ProviderKey(identity.SchemeISBN, isbn))
	}
	for _, id := range r.w.ids {
		rec.ProviderKeys = append(rec.ProviderKeys, identity.ProviderKey(identity.SchemeID, id.scheme+":"+id.value))
	}
	return rec
}

// lang is the work's ISO 639-2/B language, defaulting per the mapping when
// the export carries none.
func (r record) lang() string {
	if r.w.lang != "" {
		return r.w.lang
	}
	return r.m.DefaultLanguage
}

// Work returns the export work's BIBFRAME Work: the mapping's class, the
// title, one author contribution per creator, and the language.
func (r record) Work() codexbf.Work {
	w := codexbf.Work{Class: r.m.Class, Languages: []string{r.lang()}}
	if r.w.title != "" {
		w.Titles = append(w.Titles, codexbf.Title{MainTitle: r.w.title})
	}
	for i, c := range r.w.creators {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		w.Contributions = append(w.Contributions, codexbf.Contribution{
			Primary: i == 0,
			Class:   "Person",
			Label:   lastFirst(c),
			Roles:   []codexbf.Role{{Term: "author"}},
		})
	}
	return w
}

// Instance returns the single Instance: the ISBNs, the mapping's schemed
// identifiers, and the source-tagged provider id (which MUST equal the
// SchemeID key so re-ingest round-trips ids for isbn-less works).
func (r record) Instance() codexbf.Instance {
	var inst codexbf.Instance
	if r.w.title != "" {
		inst.Titles = append(inst.Titles, codexbf.Title{MainTitle: r.w.title})
	}
	for _, isbn := range r.w.isbns {
		inst.Identifiers = append(inst.Identifiers, codexbf.Identifier{Class: "Isbn", Value: isbn})
	}
	for _, id := range r.w.ids {
		inst.Identifiers = append(inst.Identifiers,
			codexbf.Identifier{Class: "Identifier", Value: id.scheme + ":" + id.value, Source: id.scheme})
	}
	inst.Identifiers = append(inst.Identifiers,
		codexbf.Identifier{Class: "Identifier", Value: r.providerID(), Source: r.idScheme})
	return inst
}

// Extras carries the export's provenance to catalog.json's extra object: the
// source slugs under the mapping's extra key (the one the public-sources
// allowlist governs), and a lower-confidence marker for tentative-only works.
func (r record) Extras() map[string]string {
	e := map[string]string{}
	if len(r.w.sources) > 0 {
		e[r.m.Sources.ExtraKey] = strings.Join(dedupeSorted(r.w.sources), ", ")
	}
	if !r.w.confident && len(r.w.sources) > 0 {
		e["tentative"] = "yes"
	}
	if len(e) == 0 {
		return nil
	}
	return e
}

// ControlledSubjects returns the export's subject URIs with the prefLabels
// the export carries; a URI the export left unlabeled emits label-less and
// the projector's corpus-wide label index covers it if any other feed knows
// it.
func (r record) ControlledSubjects() []ingest.AuthoritySubject {
	var subs []ingest.AuthoritySubject
	for _, uri := range dedupeSorted(r.w.subjectURIs) {
		s := ingest.AuthoritySubject{URI: uri}
		if l := r.labels[uri]; l != "" {
			s.Labels = map[string]string{"en": l}
		}
		subs = append(subs, s)
	}
	return subs
}

// providerID backs both the SchemeID resolution key and the Instance's
// source-tagged identifier; the two must be the same string.
func (r record) providerID() string {
	return r.idScheme + ":" + r.w.id
}

// lastFirst normalizes "First Middle Last" to "Last, First Middle" (already
// comma-formed names pass through), so shared works across feeds don't get
// double-listed contributors.
func lastFirst(name string) string {
	n := strings.TrimSpace(name)
	if n == "" || strings.Contains(n, ",") {
		return n
	}
	parts := strings.Fields(n)
	if len(parts) < 2 {
		return n
	}
	return parts[len(parts)-1] + ", " + strings.Join(parts[:len(parts)-1], " ")
}

// dedupeSorted returns the distinct values in sorted order.
func dedupeSorted(vals []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range vals {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}
