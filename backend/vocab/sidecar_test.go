package vocab

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/freeeve/libcat/storage/blob"
)

// sidecarFixture stores the shared fixture as an installed snapshot, builds
// sidecar artifacts for the requested schemes, and loads an Index over them.
func sidecarFixture(t *testing.T, arm []string) (*Index, blob.Store) {
	t.Helper()
	data, err := os.ReadFile("testdata/authorities.nq")
	if err != nil {
		t.Fatal(err)
	}
	st := blob.NewMem()
	source := "data/authorities/vocab/authorities.nq"
	if _, err := st.Put(t.Context(), source, data, blob.PutOptions{}); err != nil {
		t.Fatal(err)
	}
	for _, scheme := range arm {
		if _, err := BuildSidecar(t.Context(), st, "data/authorities/", scheme, source); err != nil {
			t.Fatal(err)
		}
	}
	ix, err := Load(t.Context(), st, "data/authorities/", nil)
	if err != nil {
		t.Fatal(err)
	}
	return ix, st
}

func asJSON(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestSidecarParity drives the whole read surface against the map-backed
// and sidecar-backed loads of the same fixture and requires identical
// results -- the sidecar's correctness contract.
func TestSidecarParity(t *testing.T) {
	maps := loadFixture(t, nil)
	side, _ := sidecarFixture(t, []string{"homosaurus", "lcsh"})
	if n := len(side.load().sidecar); n != 2 {
		t.Fatalf("sidecar armed for %d schemes, want 2", n)
	}

	if got, want := asJSON(t, side.Schemes()), asJSON(t, maps.Schemes()); got != want {
		t.Fatalf("Schemes: %s != %s", got, want)
	}
	for _, scheme := range maps.Schemes() {
		mTerms, sTerms := maps.Terms(scheme), side.Terms(scheme)
		if got, want := asJSON(t, sTerms), asJSON(t, mTerms); got != want {
			t.Fatalf("Terms(%s) diverge:\n side=%s\n maps=%s", scheme, got, want)
		}
		for _, mt := range mTerms {
			st, ok := side.Lookup(scheme, mt.ID)
			if !ok {
				t.Fatalf("Lookup(%s, %s) missing in sidecar", scheme, mt.ID)
			}
			if asJSON(t, st) != asJSON(t, mt) {
				t.Fatalf("Lookup(%s, %s) diverges", scheme, mt.ID)
			}
			mr, mok := maps.Resolve(mt.ID)
			sr, sok := side.Resolve(mt.ID)
			if mok != sok || asJSON(t, mr) != asJSON(t, sr) {
				t.Fatalf("Resolve(%s) diverges", mt.ID)
			}
			if got, want := asJSON(t, side.Path(scheme, mt.ID)), asJSON(t, maps.Path(scheme, mt.ID)); got != want {
				t.Fatalf("Path(%s, %s): %s != %s", scheme, mt.ID, got, want)
			}
			for _, id := range append([]string{mt.ID}, append(mt.ExactMatch, mt.CloseMatch...)...) {
				mm, mok := maps.MatchIdentifier(id)
				sm, sok := side.MatchIdentifier(id)
				if mok != sok || asJSON(t, mm) != asJSON(t, sm) {
					t.Fatalf("MatchIdentifier(%s) diverges", id)
				}
			}
			for _, l := range mt.Labels {
				if got, want := asJSON(t, side.MatchLabel(scheme, l)), asJSON(t, maps.MatchLabel(scheme, l)); got != want {
					t.Fatalf("MatchLabel(%s, %q): %s != %s", scheme, l, got, want)
				}
			}
		}
		for _, q := range []string{"t", "tr", "trans", "trans people", "s", "science f", "personas", "zzz", ""} {
			if got, want := asJSON(t, side.Search(scheme, q, 10)), asJSON(t, maps.Search(scheme, q, 10)); got != want {
				t.Fatalf("Search(%s, %q): %s != %s", scheme, q, got, want)
			}
		}
	}
	if _, ok := side.Lookup("homosaurus", "https://homosaurus.org/v4/nope"); ok {
		t.Fatal("unknown term resolved via sidecar")
	}
	if _, ok := side.Lookup("fast", "anything"); ok {
		t.Fatal("unknown scheme resolved via sidecar")
	}
}

// TestSidecarStaleSource edits the snapshot after the artifacts were built:
// the scheme must fall back to maps and serve the edited content.
func TestSidecarStaleSource(t *testing.T) {
	ix, st := sidecarFixture(t, []string{"homosaurus", "lcsh"})
	source := "data/authorities/vocab/authorities.nq"
	data, _, err := st.Get(t.Context(), source)
	if err != nil {
		t.Fatal(err)
	}
	edited := append([]byte(nil), data...)
	edited = append(edited, []byte("\n<http://id.loc.gov/authorities/subjects/shNEW1> <http://www.w3.org/2004/02/skos/core#prefLabel> \"Brand new heading\"@en <authority:lcsh> .\n")...)
	if _, err := st.Put(t.Context(), source, edited, blob.PutOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := ix.Reload(t.Context(), st, "data/authorities/", nil); err != nil {
		t.Fatal(err)
	}
	if n := len(ix.load().sidecar); n != 0 {
		t.Fatalf("stale sidecar still armed for %d schemes", n)
	}
	term, ok := ix.Lookup("lcsh", "http://id.loc.gov/authorities/subjects/shNEW1")
	if !ok || term.Labels["en"] != "Brand new heading" {
		t.Fatalf("edited snapshot not served: %+v ok=%v", term, ok)
	}
}

// TestSidecarLooseQuads adds a separate grain carrying quads for an armed
// scheme: the scheme must fall back to maps and index both files' terms.
func TestSidecarLooseQuads(t *testing.T) {
	ix, st := sidecarFixture(t, []string{"homosaurus", "lcsh"})
	grain := "<https://homosaurus.org/v4/homoitLOCAL> <http://www.w3.org/2004/02/skos/core#prefLabel> \"Locally merged concept\"@en <authority:homosaurus> .\n"
	if _, err := st.Put(t.Context(), "data/authorities/aa/grain.nq", []byte(grain), blob.PutOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := ix.Reload(t.Context(), st, "data/authorities/", nil); err != nil {
		t.Fatal(err)
	}
	// A loose quad no longer costs the scheme its sidecar. It used to: three
	// accessors read the sidecar alone, so the loader replayed the whole
	// snapshot into maps to keep them correct -- 513k LCSH headings for one
	// cached live pick (tasks/265). The accessors merge now, so the scheme
	// carries a sidecar and a small map overlay together.
	if n := len(ix.load().sidecar); n != 2 {
		t.Fatalf("a loose quad demoted a sidecar-backed scheme: %d armed, want 2", n)
	}
	if snap := ix.load(); len(snap.schemes["homosaurus"]) != 1 {
		t.Fatalf("overlay = %d terms, want just the loose one (the snapshot must not be resident)",
			len(snap.schemes["homosaurus"]))
	}

	// Both backends answer, through every accessor.
	if _, ok := ix.Lookup("homosaurus", "https://homosaurus.org/v4/homoitLOCAL"); !ok {
		t.Fatal("Lookup missed the loose grain term")
	}
	if _, ok := ix.Lookup("homosaurus", "https://homosaurus.org/v4/homoit0001235"); !ok {
		t.Fatal("Lookup missed the snapshot term")
	}
	if got := ix.Search("homosaurus", "Locally merged", 10); len(got) != 1 || got[0].ID != "https://homosaurus.org/v4/homoitLOCAL" {
		t.Fatalf("Search missed the overlay term: %v", termIDs(got))
	}
	if got := ix.Search("homosaurus", "Trans", 10); len(got) == 0 {
		t.Fatal("Search missed the sidecar's terms once an overlay existed")
	}
	if got := ix.MatchLabel("homosaurus", "locally merged concept"); len(got) != 1 {
		t.Fatalf("MatchLabel missed the overlay term: %d matches", len(got))
	}
	all := ix.Terms("homosaurus")
	if len(all) < 2 {
		t.Fatalf("Terms = %d, want the sidecar's terms plus the overlay", len(all))
	}
	seen := map[string]bool{}
	for _, t2 := range all {
		if seen[t2.ID] {
			t.Fatalf("Terms returned %s twice", t2.ID)
		}
		seen[t2.ID] = true
	}
	if !seen["https://homosaurus.org/v4/homoitLOCAL"] || !seen["https://homosaurus.org/v4/homoit0001235"] {
		t.Fatal("Terms did not merge the overlay with the sidecar")
	}
}

func termIDs(ts []*Term) []string {
	out := make([]string, 0, len(ts))
	for _, t := range ts {
		out = append(out, t.ID)
	}
	return out
}

// TestSidecarPartialArming builds artifacts for one scheme of a shared
// source file: the file cannot be skipped, so both schemes serve from maps.
func TestSidecarPartialArming(t *testing.T) {
	ix, _ := sidecarFixture(t, []string{"homosaurus"})
	if n := len(ix.load().sidecar); n != 0 {
		t.Fatalf("partially-armed shared source produced %d sidecar schemes", n)
	}
	if got := ix.Schemes(); len(got) != 2 {
		t.Fatalf("schemes = %v", got)
	}
	if _, ok := ix.Lookup("lcsh", "http://id.loc.gov/authorities/subjects/sh85118553"); !ok {
		t.Fatal("lcsh term missing")
	}
	if _, ok := ix.Lookup("homosaurus", "https://homosaurus.org/v4/homoit0001235"); !ok {
		t.Fatal("homosaurus term missing")
	}
}

// TestSidecarSearchIndex covers the RRTI search path through odd inputs
// (multi-byte labels, no-match prefixes) via the public surface.
func TestSidecarSearchIndex(t *testing.T) {
	side, _ := sidecarFixture(t, []string{"homosaurus", "lcsh"})
	got := side.Search("homosaurus", "personas transg", 5)
	if len(got) != 1 || got[0].Labels["es"] != "Personas transgénero" {
		t.Fatalf("multibyte prefix search = %v", asJSON(t, got))
	}
	if hits := side.Search("homosaurus", strings.Repeat("z", 40), 5); len(hits) != 0 {
		t.Fatalf("phantom hits: %v", asJSON(t, hits))
	}
}
