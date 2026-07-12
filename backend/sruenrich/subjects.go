// Package sruenrich harvests SUBJECT access points from copycat's SRU/Z39.50
// targets: the extraction and reconciliation the editor's per-work subject
// lookup uses, plus an ingest.Enricher that batches it over a scoped corpus
// as moderated suggestions -- "ask the wider cataloging ecosystem what these
// works are about" without whole-record copy cataloging.
package sruenrich

import (
	"slices"
	"strings"

	"github.com/freeeve/libcat/backend/marcview"
	"github.com/freeeve/libcat/backend/vocab"
)

// Candidate is one external heading: the heading text, its MARC tag and
// source vocabulary, the $0 identifier URIs it carried, how many target
// records carried it, and -- when an identifier or the whole heading matches
// a loaded vocabulary -- the controlled term to add instead of a tag.
type Candidate struct {
	Heading string         `json:"heading"`
	Tag     string         `json:"tag"`
	Source  string         `json:"source,omitempty"`
	IDs     []string       `json:"ids,omitempty"`
	Count   int            `json:"count"`
	Targets []string       `json:"targets"`
	Term    *vocab.TermRef `json:"term,omitempty"`
}

// subjectTags are the MARC 6XX fields worth harvesting; ind2 names the
// source vocabulary (7 defers to $2).
var subjectTags = map[string]bool{
	"600": true, "610": true, "611": true, "630": true,
	"648": true, "650": true, "651": true, "655": true,
}

var ind2Sources = map[string]string{"0": "lcsh", "1": "lcshac", "2": "mesh", "5": "cash", "6": "rvm"}

// Collect folds one target record's 6XX fields into the candidate map,
// deduping by (tag, normalized heading) and accumulating identifiers,
// counts, and carrying targets.
func Collect(byKey map[string]*Candidate, target string, rec marcview.RecordDoc) {
	for _, f := range rec.Fields {
		if !subjectTags[f.Tag] {
			continue
		}
		heading, source, ids := HeadingOf(f)
		if heading == "" {
			continue
		}
		key := f.Tag + "|" + NormHeading(heading)
		c := byKey[key]
		if c == nil {
			c = &Candidate{Heading: heading, Tag: f.Tag, Source: source}
			byKey[key] = c
		}
		for _, id := range ids {
			if !slices.Contains(c.IDs, id) {
				c.IDs = append(c.IDs, id)
			}
		}
		c.Count++
		if !slices.Contains(c.Targets, target) {
			c.Targets = append(c.Targets, target)
		}
	}
}

// HeadingOf joins a 6XX field into a display heading: name/title subfields
// space-joined, subdivisions ($v$x$y$z) double-dash-joined, trailing
// punctuation trimmed. Returns the heading, its source vocabulary, and the
// resolvable identifier URIs its $0 subfields carry.
func HeadingOf(f marcview.Field) (string, string, []string) {
	var main []string
	var subs []string
	var ids []string
	source := ind2Sources[f.Ind2]
	for _, sf := range f.Subfields {
		switch sf.Code {
		case "a", "b", "c", "d", "t":
			main = append(main, strings.TrimSpace(sf.Value))
		case "v", "x", "y", "z":
			subs = append(subs, strings.TrimSpace(sf.Value))
		case "0":
			if id := SubjectIDURI(sf.Value); id != "" && !slices.Contains(ids, id) {
				ids = append(ids, id)
			}
		case "2":
			if source == "" {
				source = sf.Value
			}
		}
	}
	heading := strings.Join(main, " ")
	if len(subs) > 0 {
		heading += "--" + strings.Join(subs, "--")
	}
	return strings.TrimRight(strings.TrimSpace(heading), ".,"), source, ids
}

// SubjectIDURI folds a 6XX $0 value into a resolvable identifier URI: full
// http(s) URIs pass through, the "(DE-588)X" GND parenthetical form becomes
// its d-nb.info URI. Other control numbers (local PPNs, bare LCCNs) return
// "" -- nothing loadable claims them.
func SubjectIDURI(v string) string {
	v = strings.TrimSpace(v)
	if strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://") {
		return v
	}
	if id, ok := strings.CutPrefix(v, "(DE-588)"); ok && id != "" {
		return "https://d-nb.info/gnd/" + strings.TrimSpace(id)
	}
	return ""
}

// NormHeading lowercases, whitespace-normalizes, and strips a trailing
// period so headings compare across targets' punctuation habits.
func NormHeading(s string) string {
	return strings.TrimSuffix(strings.Join(strings.Fields(strings.ToLower(s)), " "), ".")
}

// ReconcileIdentifiers resolves the first $0 identifier any loaded term
// claims as its own URI or a skos exact/close match sibling. Identifier
// matches outrank label matches: they survive the language gap (a German
// GND heading still lands on the English-labeled term).
func ReconcileIdentifiers(ix *vocab.Index, ids []string) *vocab.TermRef {
	for _, id := range ids {
		if t, ok := ix.MatchIdentifier(id); ok {
			return &vocab.TermRef{Scheme: t.Scheme, ID: t.ID, Label: t.Label("en")}
		}
	}
	return nil
}

// ReconcileHeading whole-heading-matches against every loaded scheme: the
// full heading first, then its pre-subdivision head.
func ReconcileHeading(ix *vocab.Index, heading string) *vocab.TermRef {
	tries := []string{heading}
	if head, _, ok := strings.Cut(heading, "--"); ok {
		tries = append(tries, head)
	}
	for _, try := range tries {
		for _, scheme := range ix.Schemes() {
			for _, m := range ix.MatchLabel(scheme, try) {
				if m.Term.MergedInto == "" {
					return &vocab.TermRef{Scheme: scheme, ID: m.Term.ID, Label: m.Term.Label("en")}
				}
			}
		}
	}
	return nil
}
