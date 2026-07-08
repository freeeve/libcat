package nquads

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/freeeve/libcat/identity"
	"github.com/freeeve/libcat/ingest"
)

const testMapping = `
work-prefix = "urn:demo:work:"
id-order = "numeric"
default-language = "eng"

[predicates]
title = "http://purl.org/dc/terms/title"
creator = "http://purl.org/dc/terms/creator"
identifier = "http://purl.org/dc/terms/identifier"
subject = "http://purl.org/dc/terms/subject"
source = "http://purl.org/dc/terms/source"
language = "http://purl.org/dc/terms/language"
prefLabel = "http://www.w3.org/2004/02/skos/core#prefLabel"

[identifiers]
"urn:isbn:" = "isbn"
"urn:demo:od:" = "overdrive"

[languages]
en = "eng"
fr = "fre"

[sources]
prefix = "urn:demo:src:"
tentative = ["urn:demo:src:scan-tier-2"]
`

const testNQ = `<urn:demo:work:2> <http://purl.org/dc/terms/title> "Second Work" .
<urn:demo:work:2> <http://purl.org/dc/terms/creator> "Ada Author" .
<urn:demo:work:2> <http://purl.org/dc/terms/identifier> <urn:isbn:9780000000011> .
<urn:demo:work:2> <http://purl.org/dc/terms/subject> <https://homosaurus.org/v5/hom1> .
<urn:demo:work:2> <http://purl.org/dc/terms/source> <urn:demo:src:loc> .
<urn:demo:work:2> <http://purl.org/dc/terms/language> "fr" .
<urn:demo:work:10> <http://purl.org/dc/terms/title> "Tenth Work" .
<urn:demo:work:10> <http://purl.org/dc/terms/identifier> <urn:demo:od:abc-123> .
<urn:demo:work:10> <http://purl.org/dc/terms/source> <urn:demo:src:scan-tier-2> .
<urn:demo:work:11> <http://purl.org/dc/terms/creator> "No Title" .
<https://homosaurus.org/v5/hom1> <http://www.w3.org/2004/02/skos/core#prefLabel> "Label One" .
`

// buildProvider writes the fixture mapping + export and constructs the provider.
func buildProvider(t *testing.T, params map[string]string) ingest.Provider {
	t.Helper()
	dir := t.TempDir()
	mapping := filepath.Join(dir, "mapping.toml")
	nq := filepath.Join(dir, "export.nq")
	if err := os.WriteFile(mapping, []byte(testMapping), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nq, []byte(testNQ), 0o644); err != nil {
		t.Fatal(err)
	}
	if params == nil {
		params = map[string]string{}
	}
	params["mapping"] = mapping
	p, err := New(ingest.Config{Source: nq, Params: params})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// TestRecordsMappedFields checks the mapping drives every field: titles,
// creators (last-first), identifier schemes, subjects with harvested labels,
// language table, source slugs with the tentative marker, and numeric order.
func TestRecordsMappedFields(t *testing.T) {
	recs, err := buildProvider(t, nil).Records(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 2 {
		t.Fatalf("records = %d, want 2 (untitled work dropped)", len(recs))
	}

	// Numeric id-order: 2 before 10.
	id0 := recs[0].Identity()
	if !strings.Contains(id0.Author, "nquads:2") || id0.Title != "Second Work" {
		t.Fatalf("first record identity = %+v", id0)
	}
	if id0.Lang != "fre" {
		t.Fatalf("language table not applied: %q", id0.Lang)
	}
	keys := map[string]bool{}
	for _, k := range id0.ProviderKeys {
		keys[string(k)] = true
	}
	if !keys[string(identity.ProviderKey(identity.SchemeISBN, "9780000000011"))] {
		t.Fatalf("missing isbn key: %v", id0.ProviderKeys)
	}

	w := recs[0].Work()
	if len(w.Contributions) != 1 || w.Contributions[0].Label != "Author, Ada" {
		t.Fatalf("contributions = %+v", w.Contributions)
	}
	subs := recs[0].(record).ControlledSubjects()
	if len(subs) != 1 || subs[0].Labels["en"] != "Label One" {
		t.Fatalf("subjects = %+v", subs)
	}
	extras := recs[0].(record).Extras()
	if extras["sources"] != "loc" || extras["tentative"] != "" {
		t.Fatalf("extras = %v", extras)
	}

	// The tier-2-only work: schemed id key, tentative extra.
	id1 := recs[1].Identity()
	if !slices.Contains(id1.ProviderKeys, string(identity.ProviderKey(identity.SchemeID, "overdrive:abc-123"))) {
		t.Fatalf("missing schemed id key: %v", id1.ProviderKeys)
	}
	extras1 := recs[1].(record).Extras()
	if extras1["tentative"] != "yes" || extras1["sources"] != "scan-tier-2" {
		t.Fatalf("tentative extras = %v", extras1)
	}
}

// TestTentativeDrop checks Params["tentative"]="drop" removes works whose
// only attestation is a tentative source.
func TestTentativeDrop(t *testing.T) {
	recs, err := buildProvider(t, map[string]string{"tentative": "drop"}).Records(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].Identity().Title != "Second Work" {
		t.Fatalf("records = %d, want only the confident work", len(recs))
	}
}

// TestMappingValidation checks the mapping loader rejects broken profiles.
func TestMappingValidation(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		"missing work-prefix": `[predicates]
title = "x"`,
		"no predicates": `work-prefix = "urn:w:"`,
		"unknown field": `work-prefix = "urn:w:"
[predicates]
banana = "x"`,
		"bad id-order": `work-prefix = "urn:w:"
id-order = "random"
[predicates]
title = "x"`,
	}
	for name, body := range cases {
		path := filepath.Join(dir, strings.ReplaceAll(name, " ", "_")+".toml")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadMapping(path); err == nil {
			t.Errorf("%s: mapping accepted, want error", name)
		}
	}
}

// TestMappingPredicateList checks a field may list several predicate IRIs.
func TestMappingPredicateList(t *testing.T) {
	path := filepath.Join(t.TempDir(), "m.toml")
	body := `work-prefix = "urn:w:"
[predicates]
title = ["http://purl.org/dc/terms/title", "http://example.org/title"]`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := LoadMapping(path)
	if err != nil {
		t.Fatal(err)
	}
	ff := m.fieldFor()
	if ff["http://purl.org/dc/terms/title"] != "title" || ff["http://example.org/title"] != "title" {
		t.Fatalf("fieldFor = %v", ff)
	}
}
