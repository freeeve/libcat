package csvmap

import (
	"fmt"
	"strings"

	"github.com/freeeve/libcat/identity"
	codexbf "github.com/freeeve/libcodex/bibframe"
)

// record is one CSV row adapted to ingest.Record.
type record struct {
	m        *Mapping
	idScheme string
	line     int
	id       string
	title    string
	subtitle string
	summary  string
	creators []string
	isbns    []string
	subjects []string
	lang     string
	extras   map[string]string
}

// Identity resolves the row: the id column (namespaced by the id scheme) is
// the durable key when mapped, the ISBNs cluster cross-feed, and the
// author|title key is namespaced by row id (or line number) so distinct rows
// sharing an access point never re-merge -- the export's own rows are taken
// as already deduped, mirroring the nquads provider.
func (r record) Identity() identity.Record {
	rec := identity.Record{
		Author: r.idKey() + " " + firstAuthor(r.creators),
		Title:  r.title,
		Lang:   r.language(),
	}
	if r.id != "" {
		rec.ProviderKeys = append(rec.ProviderKeys, identity.ProviderKey(identity.SchemeID, r.providerID()))
	}
	for _, isbn := range r.isbns {
		rec.ProviderKeys = append(rec.ProviderKeys, identity.ProviderKey(identity.SchemeISBN, isbn))
	}
	return rec
}

// idKey is the namespacing prefix for the computed author|title identity.
func (r record) idKey() string {
	if r.id != "" {
		return r.idScheme + ":" + r.id
	}
	return fmt.Sprintf("%s:line%d", r.idScheme, r.line)
}

// language is the row's ISO 639-2/B language, defaulting per the mapping.
func (r record) language() string {
	if r.lang != "" {
		return r.lang
	}
	return r.m.DefaultLanguage
}

// Work returns the row's BIBFRAME Work: title/subtitle, one author
// contribution per creator, uncontrolled subjects as plain labels, summary,
// and language.
func (r record) Work() codexbf.Work {
	w := codexbf.Work{Class: r.m.Class, Languages: []string{r.language()}}
	w.Titles = append(w.Titles, codexbf.Title{MainTitle: r.title, Subtitle: r.subtitle})
	for i, c := range r.creators {
		w.Contributions = append(w.Contributions, codexbf.Contribution{
			Primary: i == 0,
			Class:   "Person",
			Label:   lastFirst(c),
			Roles:   []codexbf.Role{{Term: "author"}},
		})
	}
	for _, s := range r.subjects {
		w.Subjects = append(w.Subjects, codexbf.Subject{Class: "Topic", Label: s})
	}
	if r.summary != "" {
		w.Summary = []string{r.summary}
	}
	return w
}

// Instance returns the single Instance: title, ISBNs, and the source-tagged
// provider id when an id column is mapped (which MUST equal the SchemeID key
// so re-ingest round-trips ids for isbn-less rows).
func (r record) Instance() codexbf.Instance {
	var inst codexbf.Instance
	inst.Titles = append(inst.Titles, codexbf.Title{MainTitle: r.title, Subtitle: r.subtitle})
	for _, isbn := range r.isbns {
		inst.Identifiers = append(inst.Identifiers, codexbf.Identifier{Class: "Isbn", Value: isbn})
	}
	if r.id != "" {
		inst.Identifiers = append(inst.Identifiers,
			codexbf.Identifier{Class: "Identifier", Value: r.providerID(), Source: r.idScheme})
	}
	return inst
}

// Extras carries the mapped extra columns to catalog.json's extra object.
func (r record) Extras() map[string]string { return r.extras }

// providerID backs both the SchemeID resolution key and the Instance's
// source-tagged identifier; the two must be the same string.
func (r record) providerID() string {
	return r.idScheme + ":" + r.id
}

// firstAuthor normalizes the first creator for the identity key.
func firstAuthor(creators []string) string {
	if len(creators) == 0 {
		return ""
	}
	return lastFirst(creators[0])
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
