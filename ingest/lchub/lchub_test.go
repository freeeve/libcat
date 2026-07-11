package lchub

import (
	"context"
	"testing"

	"github.com/freeeve/libcat/identity"
	"github.com/freeeve/libcat/ingest"
)

func TestKeyForUsesPrimaryCreatorTitleLanguage(t *testing.T) {
	s := ingest.WorkSummary{
		Title:        "The House of the Spirits",
		Contributors: []string{"Allende, Isabel", "Bogin, Magda"}, // primary first
		Languages:    []string{"eng"},
	}
	want := identity.WorkKey("Allende, Isabel", "The House of the Spirits", "eng")
	if got := KeyFor(s); got != want {
		t.Errorf("KeyFor = %q, want %q", got, want)
	}
	// A title-less Work has no access point.
	if KeyFor(ingest.WorkSummary{Contributors: []string{"X"}}) != "" {
		t.Error("a title-less Work must have no key")
	}
}

func TestEnrichMatchesAccessPoint(t *testing.T) {
	const hubURI = "http://id.loc.gov/resources/hubs/abc123"
	author, title, lang := "Allende, Isabel", "The House of the Spirits", "eng"
	e := New(map[string]string{identity.WorkKey(author, title, lang): hubURI})

	got, err := e.Enrich(context.Background(), []ingest.WorkSummary{
		{WorkID: "w1", Title: title, Contributors: []string{author}, Languages: []string{lang}},
		{WorkID: "w2", Title: "Some Other Book", Contributors: []string{author}, Languages: []string{lang}}, // no hit
		{WorkID: "w3", Contributors: []string{author}, Languages: []string{lang}},                           // no title -> skipped
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].WorkID != "w1" {
		t.Fatalf("got %+v, want only w1 enriched", got)
	}
	if len(got[0].Identities) != 1 || got[0].Identities[0].URI != hubURI || got[0].Identities[0].Scheme != Scheme {
		t.Errorf("identities = %+v, want one %s -> %s", got[0].Identities, Scheme, hubURI)
	}
}

func TestEnrichLanguageDisambiguates(t *testing.T) {
	// Same creator+title in two languages are two access points: an English index
	// entry must not link the Spanish Work (the language corroboration at work).
	author, title := "Allende, Isabel", "The House of the Spirits"
	e := New(map[string]string{identity.WorkKey(author, title, "eng"): "http://id.loc.gov/resources/hubs/eng1"})

	got, err := e.Enrich(context.Background(), []ingest.WorkSummary{
		{WorkID: "wes", Title: title, Contributors: []string{author}, Languages: []string{"spa"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("a different-language Work must not match an English hub, got %+v", got)
	}
}
