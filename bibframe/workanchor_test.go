package bibframe

import (
	"strings"
	"testing"

	"github.com/freeeve/libcat/identity"
	codexbf "github.com/freeeve/libcodex/bibframe"
)

// TestWorkAnchorRoundTrip covers the full work-level anchor round-trip (task
// 492): a WorkGroup's anchor is emitted onto the Work node, recovered by
// identity.ScanGrain, and drives a fresh record onto the committed Work ahead of
// the fuzzy access-point key -- so a shared work id deduplicates across feeds
// even when the transcriptions differ.
func TestWorkAnchorRoundTrip(t *testing.T) {
	wg := WorkGroup{
		WorkID: "wanchor000001",
		Work: codexbf.Work{
			Class:     "Text",
			Titles:    []codexbf.Title{{MainTitle: "Gideon the Ninth"}},
			Languages: []string{"eng"},
		},
		Instances: []GroupInstance{{
			InstanceID: "ianchor00001",
			Instance:   codexbf.Instance{Identifiers: []codexbf.Identifier{{Class: "Isbn", Value: "9781250313195"}}},
		}},
		Anchors: []string{"oclcwork:1112223"},
	}
	grain, err := BuildWorkGrain(wg, "coll")
	if err != nil {
		t.Fatalf("build grain: %v", err)
	}
	// The anchor rides the Work node as a bf:source-labeled identifier.
	if !strings.Contains(string(grain), "identifiedBy") || !strings.Contains(string(grain), "oclcwork") {
		t.Fatalf("grain does not carry the work anchor:\n%s", grain)
	}

	gi, err := identity.ScanGrain(grain)
	if err != nil {
		t.Fatalf("scan grain: %v", err)
	}
	if len(gi.Works) != 1 || len(gi.Works[0].Anchors) == 0 {
		t.Fatalf("no anchor recovered: %+v", gi.Works)
	}

	// Seed a resolver from the scanned identity and resolve a fresh record that
	// shares only the OCLC work id (different provider id, no matching ISBN, a
	// different transcription) -- only the anchor can bridge it.
	r := identity.NewResolver()
	identity.SeedResolver(r, []identity.GrainIdentity{gi})
	got := r.Resolve(identity.Record{
		ProviderKeys: []string{"overdrive:9"},
		Author:       "Muir, Tamsyn",
		Title:        "A Wildly Different Transcription",
		Langs:        []string{"eng"},
		Anchors:      []string{"oclcwork:1112223"},
	})
	if got.WorkID != "wanchor000001" {
		t.Errorf("anchor round-trip should resolve onto the committed Work, got %s", got.WorkID)
	}
	if got.MintedWork {
		t.Errorf("anchor hit should not mint a Work: %+v", got)
	}

	// A different language under the same anchor stays distinct.
	other := r.Resolve(identity.Record{
		ProviderKeys: []string{"overdrive:10"},
		Author:       "Muir, Tamsyn",
		Title:        "Gideon la Novena",
		Langs:        []string{"spa"},
		Anchors:      []string{"oclcwork:1112223"},
	})
	if other.WorkID == got.WorkID {
		t.Errorf("a differing-language record must not cluster via the anchor: %s", other.WorkID)
	}
}
