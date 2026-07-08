package project

import (
	"reflect"
	"testing"
)

// TestMergeFirstFeedWins checks the multi-feed union: a work id claimed by an
// earlier catalog keeps that catalog's projection, unique works from every
// feed survive, and the result is sorted by id.
func TestMergeFirstFeedWins(t *testing.T) {
	primary := &Catalog{Version: SchemaVersion, Works: []Work{
		{ID: "w2", Title: "Shared (rich)"},
		{ID: "w1", Title: "Primary only"},
	}}
	sidecar := &Catalog{Version: SchemaVersion, Works: []Work{
		{ID: "w2", Title: "Shared (sparse)"},
		{ID: "w3", Title: "Sidecar only"},
	}}
	merged := Merge([]*Catalog{primary, sidecar})
	if merged.Version != SchemaVersion {
		t.Fatalf("version = %d, want %d", merged.Version, SchemaVersion)
	}
	var ids, titles []string
	for _, w := range merged.Works {
		ids = append(ids, w.ID)
		titles = append(titles, w.Title)
	}
	if !reflect.DeepEqual(ids, []string{"w1", "w2", "w3"}) {
		t.Fatalf("ids = %v", ids)
	}
	if titles[1] != "Shared (rich)" {
		t.Fatalf("shared work kept %q, want the first feed's projection", titles[1])
	}
}

// TestMergeEmpty checks that merging no catalogs yields an empty, versioned catalog.
func TestMergeEmpty(t *testing.T) {
	merged := Merge(nil)
	if merged.Version != SchemaVersion || len(merged.Works) != 0 {
		t.Fatalf("merged = %+v", merged)
	}
}

// TestMergeTermSideband covers tasks/180: Merge carries Catalog.Terms
// (dropping it hid every sideband-labeled ancestor from CLI-built
// deployments, since lcat project always routes through Merge). Shared term
// ids merge field-wise -- labels fill per language with earlier catalogs
// winning, broader edges union, first non-empty scheme sticks -- the inputs
// stay unmutated, and the output is sorted by id.
func TestMergeTermSideband(t *testing.T) {
	primary := &Catalog{Version: SchemaVersion, Terms: []Term{
		{ID: "t:b", Labels: map[string]string{"en": "Primary EN"}, Broader: []string{"t:z"}},
		{ID: "t:a", Labels: map[string]string{"en": "Alpha"}, Scheme: "homosaurus"},
	}}
	sidecar := &Catalog{Version: SchemaVersion, Terms: []Term{
		{ID: "t:b", Labels: map[string]string{"en": "Sidecar EN", "es": "Sidecar ES"}, Broader: []string{"t:y"}, Scheme: "fast"},
	}}
	merged := Merge([]*Catalog{primary, sidecar})
	if len(merged.Terms) != 2 || merged.Terms[0].ID != "t:a" || merged.Terms[1].ID != "t:b" {
		t.Fatalf("terms = %+v", merged.Terms)
	}
	b := merged.Terms[1]
	if b.Labels["en"] != "Primary EN" || b.Labels["es"] != "Sidecar ES" {
		t.Fatalf("labels = %+v, want primary en + sidecar es", b.Labels)
	}
	if !reflect.DeepEqual(b.Broader, []string{"t:y", "t:z"}) {
		t.Fatalf("broader = %v, want sorted union", b.Broader)
	}
	if b.Scheme != "fast" {
		t.Fatalf("scheme = %q, want the first non-empty", b.Scheme)
	}
	// Inputs untouched: the merge filled es/y into its own copies.
	if _, ok := primary.Terms[0].Labels["es"]; ok || len(primary.Terms[0].Broader) != 1 {
		t.Fatalf("primary input mutated: %+v", primary.Terms[0])
	}
}

// TestSanitizeSources checks allowlist rewriting: disallowed names are
// stripped and counted, values compare trimmed, kept values re-join ", ",
// the key is deleted when nothing public remains, and works without the
// extra are untouched.
func TestSanitizeSources(t *testing.T) {
	cat := &Catalog{Works: []Work{
		{ID: "w1", Extra: map[string]string{"sources": "loc, mombian, QLL"}},
		{ID: "w2", Extra: map[string]string{"sources": "mombian"}},
		{ID: "w3", Extra: map[string]string{"cover": "x.jpg"}},
		{ID: "w4"},
	}}
	stripped := SanitizeSources(cat, SourceSet("loc, QLL"))
	if stripped != 2 {
		t.Fatalf("stripped = %d, want 2", stripped)
	}
	if got := cat.Works[0].Extra["sources"]; got != "loc, QLL" {
		t.Fatalf("w1 sources = %q", got)
	}
	if _, ok := cat.Works[1].Extra["sources"]; ok {
		t.Fatalf("w2 sources should be deleted, got %q", cat.Works[1].Extra["sources"])
	}
	if cat.Works[2].Extra["cover"] != "x.jpg" {
		t.Fatalf("unrelated extras must survive")
	}
}

// TestSourceSet checks csv parsing: trimming, empty entries, empty input.
func TestSourceSet(t *testing.T) {
	got := SourceSet(" loc , ,QLL,")
	want := map[string]bool{"loc": true, "QLL": true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SourceSet = %v", got)
	}
	if len(SourceSet("")) != 0 {
		t.Fatalf("empty csv must give empty set")
	}
}
