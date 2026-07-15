package csvmap

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
id-scheme = "mylib"
multi-separator = ";"

[columns]
id = "Record ID"
title = "Title"
subtitle = "Subtitle"
creator = "Authors"
isbn = "ISBNs"
subject = "Genres"
language = "Lang"
summary = "Description"

[extras]
cover = "Cover URL"

[languages]
en = "eng"
de = "ger"
`

const testCSV = `Record ID,Title,Subtitle,Authors,ISBNs,Genres,Lang,Description,Cover URL
r1,First Book,A Subtitle,Ada Author;Bob Buddy,9780000000011;9780000000028,Romance;Fantasy,de,A summary.,https://x/1.jpg
r2,Second Book,,"Carol, City",,,,,
,Untitled Row Missing Title -> kept (title present),,,,,,,
r4,,,No Title,,,,,
`

// buildProvider writes the fixture mapping + csv and constructs the provider.
func buildProvider(t *testing.T, mapping, csv string) ingest.Provider {
	t.Helper()
	dir := t.TempDir()
	mp := filepath.Join(dir, "mapping.toml")
	cp := filepath.Join(dir, "export.csv")
	if err := os.WriteFile(mp, []byte(mapping), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cp, []byte(csv), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := New(ingest.Config{Source: cp, Params: map[string]string{"mapping": mp}})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// TestRecordsMappedColumns checks the mapping drives every field: id keys,
// multi-valued splits, subjects as uncontrolled labels, language table,
// extras, and the untitled-row drop.
func TestRecordsMappedColumns(t *testing.T) {
	recs, err := buildProvider(t, testMapping, testCSV).Records(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 3 {
		t.Fatalf("records = %d, want 3 (untitled row dropped)", len(recs))
	}

	id0 := recs[0].Identity()
	if id0.Title != "First Book" || len(id0.Langs) != 1 || id0.Langs[0] != "ger" {
		t.Fatalf("identity = %+v", id0)
	}
	if !strings.HasPrefix(id0.Author, "mylib:r1 ") || !strings.Contains(id0.Author, "Author, Ada") {
		t.Fatalf("author key = %q", id0.Author)
	}
	if !slices.Contains(id0.ProviderKeys, string(identity.ProviderKey(identity.SchemeID, "mylib:r1"))) {
		t.Fatalf("missing id key: %v", id0.ProviderKeys)
	}
	if !slices.Contains(id0.ProviderKeys, string(identity.ProviderKey(identity.SchemeISBN, "9780000000028"))) {
		t.Fatalf("missing second isbn key: %v", id0.ProviderKeys)
	}

	w := recs[0].Work()
	if len(w.Contributions) != 2 || w.Contributions[1].Label != "Buddy, Bob" || w.Contributions[1].Primary {
		t.Fatalf("contributions = %+v", w.Contributions)
	}
	if len(w.Subjects) != 2 || w.Subjects[0].Label != "Romance" {
		t.Fatalf("subjects = %+v", w.Subjects)
	}
	if len(w.Summary) != 1 || w.Titles[0].Subtitle != "A Subtitle" {
		t.Fatalf("summary/subtitle = %+v / %+v", w.Summary, w.Titles)
	}
	if e := recs[0].(record).Extras(); e["cover"] != "https://x/1.jpg" {
		t.Fatalf("extras = %v", e)
	}

	// Quoted comma-formed name passes through; default language applies.
	w1 := recs[1].Work()
	if id1 := recs[1].Identity(); w1.Contributions[0].Label != "Carol, City" || len(id1.Langs) != 1 || id1.Langs[0] != "eng" {
		t.Fatalf("row 2 = %+v", w1.Contributions)
	}

	// Row without an id column value still gets a line-scoped identity
	// namespace, so shared access points cannot re-merge rows.
	id2 := recs[2].Identity()
	if !strings.HasPrefix(id2.Author, "mylib:line4") {
		t.Fatalf("line-scoped author key = %q", id2.Author)
	}
	if len(id2.ProviderKeys) != 0 {
		t.Fatalf("idless row minted provider keys: %v", id2.ProviderKeys)
	}
}

// TestMappingValidation checks the loader rejects broken profiles and headers.
func TestMappingValidation(t *testing.T) {
	dir := t.TempDir()
	bad := map[string]string{
		"missing title": `[columns]
creator = "Authors"`,
		"unknown field": `[columns]
title = "Title"
banana = "X"`,
		"long delimiter": `delimiter = "ab"
[columns]
title = "Title"`,
	}
	for name, body := range bad {
		path := filepath.Join(dir, strings.ReplaceAll(name, " ", "_")+".toml")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadMapping(path); err == nil {
			t.Errorf("%s: mapping accepted, want error", name)
		}
	}

	// A mapped column missing from the header fails the run up front.
	p := buildProvider(t, testMapping, "Title,Other\nBook,x\n")
	if _, err := p.Records(context.Background()); err == nil || !strings.Contains(err.Error(), "not in header") {
		t.Fatalf("missing header column: %v", err)
	}
}

// TestTSVDelimiter checks the delimiter option.
func TestTSVDelimiter(t *testing.T) {
	mapping := "delimiter = \"\t\"\n[columns]\ntitle = \"Title\"\ncreator = \"Authors\"\n"
	csv := "Title\tAuthors\nTab Book\tAda Author\n"
	recs, err := buildProvider(t, mapping, csv).Records(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].Identity().Title != "Tab Book" {
		t.Fatalf("records = %+v", recs)
	}
}
