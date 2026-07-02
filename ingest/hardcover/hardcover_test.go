package hardcover_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/freeeve/libcatalog/ingest"
	"github.com/freeeve/libcatalog/ingest/hardcover"
	"github.com/freeeve/libcatalog/project"
)

const fixture = "testdata/read-shelf.json"

// TestRecordsExplodeByFormat proves a captured shelf explodes into one record per
// collapsed edition format, with per-format instance keys and the shared reading-log
// extras carried on each (tasks/026).
func TestRecordsExplodeByFormat(t *testing.T) {
	prov, err := hardcover.New(ingest.Config{Source: fixture})
	if err != nil {
		t.Fatal(err)
	}
	recs, err := prov.Records(context.Background())
	if err != nil {
		t.Fatalf("Records: %v", err)
	}
	// Herculine -> ebook + audiobook (2), Left Hand -> physical (1), Ambient -> ebook (1).
	if len(recs) != 4 {
		t.Fatalf("records = %d, want 4", len(recs))
	}

	// The two Herculine records share a Work-clustering identity (author/title/lang) but
	// carry distinct instance keys, and both expose the same extras.
	var herculine []ingest.Record
	for _, r := range recs {
		if r.Work().Titles[0].MainTitle == "Herculine" {
			herculine = append(herculine, r)
		}
	}
	if len(herculine) != 2 {
		t.Fatalf("Herculine records = %d, want 2", len(herculine))
	}
	a, b := herculine[0].Identity(), herculine[1].Identity()
	if a.Author != b.Author || a.Title != b.Title || a.Lang != b.Lang {
		t.Errorf("Herculine format records must share cluster fields: %+v vs %+v", a, b)
	}
	if a.Author != "Byron, Grace" {
		t.Errorf("author = %q, want %q", a.Author, "Byron, Grace")
	}
	if reflect.DeepEqual(a.ProviderKeys, b.ProviderKeys) {
		t.Errorf("Herculine formats must have distinct instance keys, both = %v", a.ProviderKeys)
	}
	ep, ok := herculine[0].(ingest.ExtraProvider)
	if !ok {
		t.Fatal("record does not implement ingest.ExtraProvider")
	}
	wantExtra := map[string]string{
		"cover":       "https://covers.example.org/herculine.jpg",
		"rating":      "5",
		"dateRead":    "2026-01-15",
		"description": "A haunting debut novel.",
	}
	if got := ep.Extras(); !reflect.DeepEqual(got, wantExtra) {
		t.Errorf("extras = %v, want %v", got, wantExtra)
	}
}

// TestEndToEndProjection runs the fixture through the real ingest.Run -> project path
// and asserts the catalog matches the demo's pipeline: clustered formats, normalized
// contributors, genre tags (from both object and string cached_tags), and the
// cover/rating/dateRead extras (tasks/026). No network.
func TestEndToEndProjection(t *testing.T) {
	prov, err := hardcover.New(ingest.Config{Source: fixture})
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	if _, err := ingest.Run(prov, out); err != nil {
		t.Fatalf("Run: %v", err)
	}
	nq, err := os.ReadFile(filepath.Join(out, "catalog.nq"))
	if err != nil {
		t.Fatal(err)
	}
	cat, err := project.Project(nq, "hardcover")
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if len(cat.Works) != 3 {
		t.Fatalf("works = %d, want 3", len(cat.Works))
	}
	byTitle := map[string]project.Work{}
	for _, w := range cat.Works {
		byTitle[w.Title] = w
	}

	// Herculine: an ebook and an audiobook edition cluster into one Work with both
	// formats and two instances; primary author leads; genres become tags; extras ride.
	h := byTitle["Herculine"]
	if !reflect.DeepEqual(h.Formats, []string{"audiobook", "ebook"}) {
		t.Errorf("Herculine formats = %v, want [audiobook ebook]", h.Formats)
	}
	if len(h.Instances) != 2 {
		t.Errorf("Herculine instances = %d, want 2", len(h.Instances))
	}
	wantContribs := []project.Contributor{
		{Name: "Byron, Grace", Role: "author"},
		{Name: "Endres, Nicky", Role: "narrator"},
	}
	if !reflect.DeepEqual(h.Contributors, wantContribs) {
		t.Errorf("Herculine contributors = %v, want %v", h.Contributors, wantContribs)
	}
	if !reflect.DeepEqual(h.Tags, []string{"Fiction", "Horror", "LGBTQ"}) {
		t.Errorf("Herculine tags = %v, want [Fiction Horror LGBTQ]", h.Tags)
	}
	wantExtra := map[string]string{
		"cover":       "https://covers.example.org/herculine.jpg",
		"rating":      "5",
		"dateRead":    "2026-01-15",
		"description": "A haunting debut novel.",
	}
	if !reflect.DeepEqual(h.Extra, wantExtra) {
		t.Errorf("Herculine extra = %v, want %v", h.Extra, wantExtra)
	}
	if !hasHardcoverProvenance(h) {
		t.Errorf("Herculine instances missing a hardcover source-tagged id: %+v", h.Instances)
	}

	// Left Hand: physical edition -> "print"; comma-bearing name passes through; the
	// genre came from a string-wrapped cached_tags value; rating keeps its half star.
	l := byTitle["The Left Hand of Darkness"]
	if !reflect.DeepEqual(l.Formats, []string{"print"}) {
		t.Errorf("Left Hand formats = %v, want [print]", l.Formats)
	}
	if len(l.Contributors) != 1 || l.Contributors[0].Name != "Le Guin, Ursula K." {
		t.Errorf("Left Hand contributors = %v, want [{Le Guin, Ursula K. author}]", l.Contributors)
	}
	if !reflect.DeepEqual(l.Tags, []string{"Science Fiction"}) {
		t.Errorf("Left Hand tags = %v, want [Science Fiction] (string-wrapped cached_tags)", l.Tags)
	}
	if l.Extra["rating"] != "4.5" || l.Extra["dateRead"] != "2025-06-01" {
		t.Errorf("Left Hand extra = %v, want rating 4.5 / dateRead 2025-06-01", l.Extra)
	}

	// Ambient: text-format fallback (Kindle) -> ebook; no contributors; no extras.
	a := byTitle["Ambient Novel"]
	if !reflect.DeepEqual(a.Formats, []string{"ebook"}) {
		t.Errorf("Ambient formats = %v, want [ebook] (edition_format fallback)", a.Formats)
	}
	if len(a.Contributors) != 0 {
		t.Errorf("Ambient contributors = %v, want none", a.Contributors)
	}
	if a.Extra != nil {
		t.Errorf("Ambient extra = %v, want nil", a.Extra)
	}
}

// hasHardcoverProvenance reports whether every instance carries a hardcover-source id.
func hasHardcoverProvenance(w project.Work) bool {
	for _, inst := range w.Instances {
		found := false
		for _, pid := range inst.ProviderIDs {
			if pid.Source == hardcover.SourceHardcover && pid.Value != "" {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return len(w.Instances) > 0
}
