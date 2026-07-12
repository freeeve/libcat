package main

import (
	"os"
	"strings"
	"testing"

	"github.com/freeeve/libcodex/rdf"
)

// TestFastRDFXMLToNT decodes a captured live response from
// https://id.worldcat.org/fast/995592 (task 418): labels, the labeled
// broader parent, and -- the crosswalk pivot's first hop -- the
// schema:sameAs link to the source LCSH heading emitted as skos:exactMatch.
func TestFastRDFXMLToNT(t *testing.T) {
	body, err := os.ReadFile("testdata/fast995592.rdf.xml")
	if err != nil {
		t.Fatal(err)
	}
	const uri = "http://id.worldcat.org/fast/995592"
	nt, err := fastRDFXMLToNT(uri, body)
	if err != nil {
		t.Fatal(err)
	}
	// The output must parse as the N-Triples the subset pipeline reads.
	if _, err := rdf.ParseNQuads(nt); err != nil {
		t.Fatalf("emitted NT does not parse: %v\n%s", err, nt)
	}
	text := string(nt)
	for _, want := range []string{
		`<` + uri + `> <http://www.w3.org/2004/02/skos/core#prefLabel> "Legends"@en .`,
		`<` + uri + `> <http://www.w3.org/2004/02/skos/core#altLabel> "Urban legends"@en .`,
		`<` + uri + `> <http://www.w3.org/2004/02/skos/core#broader> <http://id.worldcat.org/fast/930306> .`,
		`<http://id.worldcat.org/fast/930306> <http://www.w3.org/2004/02/skos/core#prefLabel> "Folklore"@en .`,
		`<` + uri + `> <http://www.w3.org/2004/02/skos/core#exactMatch> <http://id.loc.gov/authorities/subjects/sh85075780> .`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing statement:\n  %s\nin decoded output:\n%s", want, text)
		}
	}
	// Wikidata relatedMatch targets and license URIs never masquerade as
	// exactMatch: only the LCSH source heading is definitional identity.
	if strings.Contains(text, "wikidata.org") || strings.Contains(text, "opendatacommons") {
		t.Errorf("non-LCSH link leaked into the SKOS slice:\n%s", text)
	}
	if got := strings.Count(text, "exactMatch"); got != 1 {
		t.Errorf("exactMatch statements = %d, want exactly the LCSH source link", got)
	}
}

// TestFastRDFXMLToNTRejectsWrongShape: a JSON error body (the dead
// fast.oclc.org routes answer that way) refuses instead of emitting nothing.
func TestFastRDFXMLToNTRejectsWrongShape(t *testing.T) {
	if _, err := fastRDFXMLToNT("http://id.worldcat.org/fast/1", []byte(`{"message":"no Route matched"}`)); err == nil {
		t.Fatal("want an error for a non-RDF body")
	}
}
