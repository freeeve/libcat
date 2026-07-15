package overdrive

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// sampleItem mirrors a real cached OverDrive audiobook record. It is the shared
// fixture for the package's tests (notably the direct BIBFRAME crosswalk in
// bibframe_test.go).
func sampleItem() Item {
	return Item{
		ID:          "11682058",
		ReserveID:   "24760f5d-028e-4749-968f-85a458a79ad2",
		Title:       "Herculine",
		Subtitle:    "A Novel",
		Edition:     "Unabridged",
		Description: "<p><b>A stunning debut.</b></p><p>Herculine leaves the city&#8212;and her past.<br />What follows is&nbsp;unforgettable.</p>",
		Type:        NamedID{ID: "audiobook", Name: "Audiobook"},
		Publisher:   &NamedID{ID: "36805", Name: "Simon & Schuster Audio"},
		PublishDate: "2025-10-07T00:00:00Z",
		Creators: []Creator{
			{Name: "Grace Byron", Role: "Author", SortName: "Byron, Grace"},
			{Name: "Nicky Endres", Role: "Narrator", SortName: "Endres, Nicky"},
		},
		Languages: []NamedID{{ID: "en", Name: "English"}},
		Subjects:  []NamedID{{ID: "26", Name: "Fiction"}, {ID: "1224", Name: "LGBTQIA+ (Fiction)"}},
		BISAC:     []BISAC{{Code: "FIC073000", Description: "Fiction / LGBTQ+ / Transgender"}},
		Formats:   []Format{{Identifiers: []Identifier{{Type: "ISBN", Value: "9781668128251"}, {Type: "ASIN", Value: "B0CXYZ1234"}}}},
	}
}

// TestItemParsesAvailability pins that the availability fields the feed carries
// (previously dropped by json.Unmarshal) now decode onto the Item.
func TestItemParsesAvailability(t *testing.T) {
	const raw = `{"id":"1","title":"T","isOwned":true,"ownedCopies":3,"holdsCount":7}`
	var it Item
	if err := json.Unmarshal([]byte(raw), &it); err != nil {
		t.Fatal(err)
	}
	if !it.IsOwned || it.OwnedCopies != 3 || it.HoldsCount != 7 {
		t.Fatalf("parsed = %+v, want isOwned/3/7", it)
	}
	if !it.owned() {
		t.Error("an owned title should read as owned")
	}
}

// TestItemExtras checks the ownership signal surfaces as extras: owned marks
// collection membership, ownedCopies is the held quantity, holds appears only
// when a queue exists. A title held via ownedCopies alone still reads owned.
func TestItemExtras(t *testing.T) {
	owned := Item{OwnedCopies: 2, HoldsCount: 5}.Extras()
	if owned["owned"] != "true" || owned["ownedCopies"] != "2" || owned["holds"] != "5" {
		t.Errorf("owned extras = %v, want owned=true ownedCopies=2 holds=5", owned)
	}
	unowned := Item{}.Extras()
	if unowned["owned"] != "false" || unowned["ownedCopies"] != "0" {
		t.Errorf("unowned extras = %v, want owned=false ownedCopies=0", unowned)
	}
	if _, ok := unowned["holds"]; ok {
		t.Errorf("no queue should omit holds, got %v", unowned)
	}
}

// TestReadCacheRejectsMissingOrEmptyDir covers a mistyped --cache
// path must error, not read as an empty feed.
func TestReadCacheRejectsMissingOrEmptyDir(t *testing.T) {
	if _, err := ReadCache(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Error("missing cache dir should error")
	}
	if _, err := ReadCache(t.TempDir()); err == nil {
		t.Error("cache dir without page files should error")
	}
}
