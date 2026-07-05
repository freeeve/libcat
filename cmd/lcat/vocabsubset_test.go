package main

import (
	"strings"
	"testing"
)

func TestSubsetFromNT(t *testing.T) {
	const uri = "http://id.loc.gov/authorities/subjects/sh85118629"
	// The shape id.loc.gov returns for <uri>.skos.nt: kept SKOS statements plus
	// noise (marcKey) the converter must drop.
	body := strings.Join([]string{
		`<` + uri + `> <http://www.w3.org/2004/02/skos/core#prefLabel> "Science fiction"@en .`,
		`<` + uri + `> <http://www.w3.org/2004/02/skos/core#broader> <http://id.loc.gov/authorities/subjects/sh85048050> .`,
		`<` + uri + `> <http://id.loc.gov/ontologies/bflc/marcKey> "150  $aScience fiction" .`,
		``,
	}, "\n")

	const ns = "http://id.loc.gov/authorities/subjects/"
	out, terms := subsetFromNT("lcsh", ns, []string{uri, "http://id.loc.gov/authorities/subjects/shMISSING"},
		map[string][]byte{uri: []byte(body)})
	s := string(out)

	if terms != 1 {
		t.Fatalf("terms = %d, want 1 (one prefLabel-bearing concept)", terms)
	}
	if !strings.Contains(s, "<authority:lcsh>") {
		t.Fatalf("output not re-graphed to authority:lcsh:\n%s", s)
	}
	if !strings.Contains(s, `"Science fiction"@en`) {
		t.Fatalf("prefLabel dropped:\n%s", s)
	}
	if !strings.Contains(s, "sh85048050") {
		t.Fatalf("broader dropped:\n%s", s)
	}
	if strings.Contains(s, "marcKey") {
		t.Fatalf("non-SKOS noise kept:\n%s", s)
	}
	// The missing URI (no fetched body) is simply skipped, not fatal.
	if strings.Contains(s, "shMISSING") {
		t.Fatalf("emitted a term with no fetched concept:\n%s", s)
	}
}

// TestSubsetFromNTHTTPS covers tasks/100: an https catalog URI must still count
// and must be emitted under https, matching the exact-match index -- id.loc.gov
// serves the concept keyed on the canonical http URI.
func TestSubsetFromNTHTTPS(t *testing.T) {
	const httpsURI = "https://id.loc.gov/authorities/subjects/sh85118629"
	const canon = "http://id.loc.gov/authorities/subjects/sh85118629"
	const ns = "https://id.loc.gov/authorities/subjects/"
	// The fetched payload uses the canonical http URI; a broader in-namespace and
	// an out-of-namespace exactMatch (wikidata) exercise the re-scheme rule.
	body := strings.Join([]string{
		`<` + canon + `> <http://www.w3.org/2004/02/skos/core#prefLabel> "Science fiction"@en .`,
		`<` + canon + `> <http://www.w3.org/2004/02/skos/core#broader> <http://id.loc.gov/authorities/subjects/sh85048050> .`,
		`<` + canon + `> <http://www.w3.org/2004/02/skos/core#closeMatch> <http://www.wikidata.org/entity/Q24925> .`,
		``,
	}, "\n")

	out, terms := subsetFromNT("lcsh", ns, []string{httpsURI}, map[string][]byte{httpsURI: []byte(body)})
	s := string(out)

	if terms != 1 {
		t.Fatalf("terms = %d, want 1 (https URI must still count)", terms)
	}
	// The concept and its in-namespace broader are emitted under https.
	if !strings.Contains(s, "<"+httpsURI+">") {
		t.Fatalf("concept not re-schemed to the catalog's https URI:\n%s", s)
	}
	if !strings.Contains(s, "<https://id.loc.gov/authorities/subjects/sh85048050>") {
		t.Fatalf("in-namespace broader not re-schemed to https:\n%s", s)
	}
	// The canonical http concept URI must not leak through.
	if strings.Contains(s, "<"+canon+">") {
		t.Fatalf("canonical http URI leaked; index would not match:\n%s", s)
	}
	// An out-of-namespace URI keeps its own scheme.
	if !strings.Contains(s, "<http://www.wikidata.org/entity/Q24925>") {
		t.Fatalf("out-of-namespace URI wrongly re-schemed:\n%s", s)
	}
}
