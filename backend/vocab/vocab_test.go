package vocab

import (
	"errors"
	"os"
	"strings"
	"testing"
	"unicode"

	"github.com/freeeve/libcatalog/storage/blob"
)

func loadFixture(t *testing.T, schemes []string) *Index {
	t.Helper()
	data, err := os.ReadFile("testdata/authorities.nq")
	if err != nil {
		t.Fatal(err)
	}
	st := blob.NewMem()
	if _, err := st.Put(t.Context(), "data/authorities/ho/vocab.nq", data, blob.PutOptions{}); err != nil {
		t.Fatal(err)
	}
	ix, err := Load(t.Context(), st, "data/authorities/", schemes)
	if err != nil {
		t.Fatal(err)
	}
	return ix
}

func TestLoadAndLookup(t *testing.T) {
	ix := loadFixture(t, nil)
	if got := ix.Schemes(); len(got) != 2 || got[0] != "homosaurus" || got[1] != "lcsh" {
		t.Fatalf("schemes = %v", got)
	}
	term, ok := ix.Lookup("homosaurus", "https://homosaurus.org/v4/homoit0001235")
	if !ok {
		t.Fatal("transgender people not found")
	}
	if term.Labels["en"] != "Transgender people" || term.Labels["es"] != "Personas transgénero" {
		t.Fatalf("labels = %v", term.Labels)
	}
	if len(term.Broader) != 1 || term.Broader[0] != "https://homosaurus.org/v4/homoit0000508" {
		t.Fatalf("broader = %v", term.Broader)
	}
	if term.AltLabels["en"][0] != "Trans people" {
		t.Fatalf("alt = %v", term.AltLabels)
	}
	if !strings.HasPrefix(term.Definition["en"], "People whose gender identity") {
		t.Fatalf("definition = %v", term.Definition)
	}
	if term.Label("es") != "Personas transgénero" || term.Label("fr") != "Transgender people" {
		t.Fatalf("Label() fallbacks: es=%q fr=%q", term.Label("es"), term.Label("fr"))
	}
	parent, ok := ix.Lookup("homosaurus", "https://homosaurus.org/v4/homoit0000508")
	if !ok || len(parent.Narrower) != 1 {
		t.Fatalf("parent narrower = %+v", parent)
	}
	// rdfs:label is a fallback, prefLabel wins.
	lcsh, ok := ix.Lookup("lcsh", "http://id.loc.gov/authorities/subjects/sh85118553")
	if !ok || lcsh.Labels["en"] != "Science fiction" {
		t.Fatalf("lcsh = %+v", lcsh)
	}
	// Quads outside authority: graphs never load.
	if _, ok := ix.Lookup("homosaurus", "http://example.org/not-authority"); ok {
		t.Fatal("feed-graph noise indexed")
	}
	// Unknown scheme/id fail closed.
	if _, ok := ix.Lookup("fast", "anything"); ok {
		t.Fatal("unknown scheme resolved")
	}
	if _, ok := ix.Lookup("homosaurus", "https://homosaurus.org/v4/nope"); ok {
		t.Fatal("unknown term resolved")
	}
}

func TestSchemeFilter(t *testing.T) {
	ix := loadFixture(t, []string{"homosaurus"})
	if got := ix.Schemes(); len(got) != 1 || got[0] != "homosaurus" {
		t.Fatalf("schemes = %v", got)
	}
	if _, ok := ix.Lookup("lcsh", "http://id.loc.gov/authorities/subjects/sh85118553"); ok {
		t.Fatal("filtered scheme loaded")
	}
}

func TestSearch(t *testing.T) {
	ix := loadFixture(t, nil)
	// Prefix match on prefLabel, case-insensitive.
	hits := ix.Search("homosaurus", "trans", 10)
	if len(hits) != 1 || hits[0].ID != "https://homosaurus.org/v4/homoit0001235" {
		t.Fatalf("search trans = %v", hits)
	}
	// Alt labels searchable, result deduped with the pref hit.
	hits = ix.Search("homosaurus", "Trans people", 10)
	if len(hits) != 1 {
		t.Fatalf("alt search = %v", hits)
	}
	// Multilingual.
	hits = ix.Search("homosaurus", "personas", 10)
	if len(hits) != 1 || hits[0].Labels["es"] != "Personas transgénero" {
		t.Fatalf("es search = %v", hits)
	}
	// Limit respected.
	if hits := ix.Search("homosaurus", "", 10); hits != nil {
		t.Fatalf("empty query = %v", hits)
	}
	all := ix.Search("homosaurus", "q", 1)
	if len(all) != 1 {
		t.Fatalf("limit = %v", all)
	}
	if hits := ix.Search("lcsh", "science", 10); len(hits) != 1 {
		t.Fatalf("lcsh search = %v", hits)
	}
	if hits := ix.Search("nope", "x", 10); hits != nil {
		t.Fatalf("unknown scheme search = %v", hits)
	}
}

func TestNormalizeFolk(t *testing.T) {
	good := map[string]string{
		"Cozy Fantasy":      "cozy fantasy",
		"  found\tfamily  ": "found family",
		"SAPPHIC":           "sapphic",
		"enemies to lovers": "enemies to lovers",
		"ＦＵＬＬＷＩＤＴＨ":         "fullwidth", // NFKC folds fullwidth forms
		"ace rep é":         "ace rep é",
	}
	for raw, want := range good {
		got, err := NormalizeFolk(raw)
		if err != nil || got != want {
			t.Errorf("NormalizeFolk(%q) = %q, %v; want %q", raw, got, err, want)
		}
	}
	bad := []string{
		"", "a", strings.Repeat("x", 61),
		"see http://spam.example/x", "www.spam.example",
		"<script>alert(1)</script>", "tag\x00null", "tab\ttag\ncontrol\x1b",
		"{template}",
	}
	for _, raw := range bad {
		if got, err := NormalizeFolk(raw); !errors.Is(err, ErrBadFolkTerm) {
			t.Errorf("NormalizeFolk(%q) = %q, %v; want ErrBadFolkTerm", raw, got, err)
		}
	}
}

func FuzzNormalizeFolk(f *testing.F) {
	for _, seed := range []string{"cozy fantasy", "É", "a\x00b", "http://x", strings.Repeat("y", 80)} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		out, err := NormalizeFolk(raw)
		if err != nil {
			return
		}
		// Invariants of any accepted term.
		if out != strings.ToLower(out) {
			t.Fatalf("not lowercase: %q", out)
		}
		if strings.Contains(out, "  ") || out != strings.TrimSpace(out) {
			t.Fatalf("whitespace not collapsed: %q", out)
		}
		for _, r := range out {
			if unicode.IsControl(r) {
				t.Fatalf("control char survived: %q", out)
			}
		}
		if n := len([]rune(out)); n < folkMinLen || n > folkMaxLen {
			t.Fatalf("length out of bounds: %q", out)
		}
		// Idempotent.
		again, err := NormalizeFolk(out)
		if err != nil || again != out {
			t.Fatalf("not idempotent: %q -> %q (%v)", out, again, err)
		}
	})
}
