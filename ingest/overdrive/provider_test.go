package overdrive

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/freeeve/libcat/ingest"
)

// TestProviderCleansTitles pins that the provider normalizes HTML character
// references and markup in transcribed titles at the source, so both the
// BIBFRAME titles and the identity clustering key see clean text.
func TestProviderCleansTitles(t *testing.T) {
	it := sampleItem()
	it.Title = "Incredible LEGO&#174; Creations"
	it.Subtitle = "Emperor Xuanzong of Qing&#8212;Min Ning"
	it.Series = "The <b>Big</b> Series"

	dir := t.TempDir()
	page := struct {
		Items []Item `json:"items"`
	}{Items: []Item{it}}
	data, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "page-0001.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	prov, err := New(ingest.Config{Source: dir})
	if err != nil {
		t.Fatal(err)
	}
	recs, err := prov.Records(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 {
		t.Fatalf("records = %d, want 1", len(recs))
	}
	work := recs[0].Work()
	if got, want := work.Titles[0].MainTitle, "Incredible LEGO® Creations"; got != want {
		t.Errorf("title = %q, want %q", got, want)
	}
	if got, want := work.Titles[0].Subtitle, "Emperor Xuanzong of Qing—Min Ning"; got != want {
		t.Errorf("subtitle = %q, want %q", got, want)
	}
	// The identity clustering title is cleaned too.
	if got, want := recs[0].Identity().Title, "Incredible LEGO® Creations"; got != want {
		t.Errorf("identity title = %q, want %q", got, want)
	}
}

// TestProviderOwnedOnlyFilter checks Params["ownedOnly"] drops titles the
// library does not hold, so the ingested collection is exactly the owned feed,
// while the default keeps every title.
func TestProviderOwnedOnlyFilter(t *testing.T) {
	held := sampleItem()
	held.ID, held.ReserveID, held.OwnedCopies = "owned1", "res-owned1", 2
	unheld := sampleItem()
	unheld.ID, unheld.ReserveID, unheld.IsOwned, unheld.OwnedCopies = "unowned1", "res-unowned1", false, 0

	dir := t.TempDir()
	page := struct {
		Items []Item `json:"items"`
	}{Items: []Item{held, unheld}}
	data, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "page-0001.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	// Default: both titles ingested.
	all, err := New(ingest.Config{Source: dir})
	if err != nil {
		t.Fatal(err)
	}
	recs, err := all.Records(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 2 {
		t.Fatalf("default records = %d, want 2 (no filter)", len(recs))
	}

	// ownedOnly: the unheld title is dropped.
	ownedProv, err := New(ingest.Config{Source: dir, Params: map[string]string{"ownedOnly": "true"}})
	if err != nil {
		t.Fatal(err)
	}
	recs, err = ownedProv.Records(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 {
		t.Fatalf("ownedOnly records = %d, want 1 (unheld dropped)", len(recs))
	}
	// The survivor is the held title, marked owned in its extras.
	if ep, ok := recs[0].(ingest.ExtraProvider); !ok || ep.Extras()["owned"] != "true" {
		t.Error("kept record should be the owned title")
	}
}
