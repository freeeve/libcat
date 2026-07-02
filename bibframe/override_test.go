package bibframe

import (
	"bytes"
	"strings"
	"testing"

	"github.com/freeeve/libcodex/rdf"
)

func TestOverrideLifecycle(t *testing.T) {
	grain := sampleGrain(t)
	workIRI := WorkIRI("w1")

	// Claim ownership of bf:subject and assert a replacement value.
	patch := OverridePatch(workIRI, bfSubjectIRI)
	patch.Add = append(patch.Add, SubjectQuad("w1", "https://homosaurus.org/v4/better"))
	claimed, err := ApplyEditorialPatch(grain, patch)
	if err != nil {
		t.Fatal(err)
	}
	ds, err := rdf.ParseNQuads(claimed)
	if err != nil {
		t.Fatal(err)
	}
	overrides := ScanOverrides(ds)
	if !overrides.Shadows("#w1Work", bfSubjectIRI) {
		t.Fatalf("marker not scanned: %v", overrides)
	}
	if overrides.Shadows("#w1Work", "http://id.loc.gov/ontologies/bibframe/title") {
		t.Fatal("unclaimed predicate shadowed")
	}

	// The marker lives in editorial:, so re-ingest preserves it.
	preserved, err := preservedQuads(claimed, FeedGraph("overdrive"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(preserved), PredOverrides) {
		t.Fatal("marker not preserved across reingest")
	}

	// Shadow filtering drops the claimed (subject, predicate) feed triples
	// and nothing else.
	feedGraph := ds.Graph(FeedGraph("overdrive"))
	feedGraph.Add(rdf.NewIRI(workIRI), rdf.NewIRI(bfSubjectIRI), rdf.NewIRI("https://feed.example/old"))
	filtered := ApplyShadow(feedGraph, overrides)
	if got := filtered.Objects(rdf.NewIRI(workIRI), bfSubjectIRI); len(got) != 0 {
		t.Fatalf("shadowed feed subjects survived: %v", got)
	}
	if _, ok := filtered.Object(rdf.NewIRI(workIRI), "http://id.loc.gov/ontologies/bibframe/title"); !ok {
		t.Fatal("unshadowed property filtered")
	}

	// Revert: drop the marker and the editorial value -> byte-identical to
	// the original grain.
	revert := RevertPatch(workIRI, bfSubjectIRI)
	revert.Remove = append(revert.Remove, SubjectQuad("w1", "https://homosaurus.org/v4/better"))
	restored, err := ApplyEditorialPatch(claimed, revert)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(restored, grain) {
		t.Fatalf("revert did not restore the grain:\n%s\nvs\n%s", restored, grain)
	}
}

func TestApplyShadowNilSafety(t *testing.T) {
	if got := ApplyShadow(nil, Overrides{"x": {"y": true}}); got != nil {
		t.Fatal("nil graph not passed through")
	}
	g := &rdf.Graph{}
	g.Add(rdf.NewIRI("#w1Work"), rdf.NewIRI(bfSubjectIRI), rdf.NewIRI("https://x"))
	if got := ApplyShadow(g, nil); got != g {
		t.Fatal("empty overrides should be a no-op passthrough")
	}
}
