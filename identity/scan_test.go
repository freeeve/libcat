package identity

import (
	"sort"
	"testing"
)

// TestScanGrain recovers Work and Instance identity from a grain with distinct
// Work and Instance ids, a primary contribution + title + language (for the
// cluster key), and two typed identifiers.
func TestScanGrain(t *testing.T) {
	grain := []byte(`<#w1Work> <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <http://id.loc.gov/ontologies/bibframe/Work> <feed:overdrive> .
<#w1Work> <http://id.loc.gov/ontologies/bibframe/title> _:t <feed:overdrive> .
<#w1Work> <http://id.loc.gov/ontologies/bibframe/language> <http://id.loc.gov/vocabulary/languages/eng> <feed:overdrive> .
<#w1Work> <http://id.loc.gov/ontologies/bibframe/contribution> _:c <feed:overdrive> .
_:t <http://id.loc.gov/ontologies/bibframe/mainTitle> "Herculine" <feed:overdrive> .
_:c <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <http://id.loc.gov/ontologies/bflc/PrimaryContribution> <feed:overdrive> .
_:c <http://id.loc.gov/ontologies/bibframe/agent> _:ag <feed:overdrive> .
_:ag <http://www.w3.org/2000/01/rdf-schema#label> "Byron, Grace" <feed:overdrive> .
<#i1Instance> <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <http://id.loc.gov/ontologies/bibframe/Instance> <feed:overdrive> .
<#i1Instance> <http://id.loc.gov/ontologies/bibframe/instanceOf> <#w1Work> <feed:overdrive> .
<#i1Instance> <http://id.loc.gov/ontologies/bibframe/identifiedBy> _:a <feed:overdrive> .
<#i1Instance> <http://id.loc.gov/ontologies/bibframe/identifiedBy> _:b <feed:overdrive> .
_:a <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <http://id.loc.gov/ontologies/bibframe/Isbn> <feed:overdrive> .
_:a <http://www.w3.org/1999/02/22-rdf-syntax-ns#value> "9780000000001" <feed:overdrive> .
_:b <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <http://id.loc.gov/ontologies/bibframe/Identifier> <feed:overdrive> .
_:b <http://www.w3.org/1999/02/22-rdf-syntax-ns#value> "od-42" <feed:overdrive> .
`)

	gi, err := ScanGrain(grain)
	if err != nil {
		t.Fatalf("ScanGrain: %v", err)
	}
	if len(gi.Works) != 1 {
		t.Fatalf("got %d works, want 1", len(gi.Works))
	}
	if gi.Works[0].WorkID != "w1" {
		t.Errorf("WorkID = %q, want w1", gi.Works[0].WorkID)
	}
	if want := WorkKey("Byron, Grace", "Herculine", "eng"); gi.Works[0].ClusterKey != want {
		t.Errorf("ClusterKey = %q, want %q", gi.Works[0].ClusterKey, want)
	}

	if len(gi.Instances) != 1 {
		t.Fatalf("got %d instances, want 1", len(gi.Instances))
	}
	got := gi.Instances[0]
	if got.InstanceID != "i1" || got.WorkID != "w1" {
		t.Errorf("instance ids = %+v, want i1/w1", got)
	}
	want := []string{"id:od-42", "isbn:9780000000001"}
	keys := append([]string(nil), got.ProviderKeys...)
	sort.Strings(keys)
	if len(keys) != len(want) || keys[0] != want[0] || keys[1] != want[1] {
		t.Errorf("ProviderKeys = %v, want %v", got.ProviderKeys, want)
	}
}

// TestScanGrainSkipsRelationStubs: the crosswalk types 76X-78X related
// resources as bf:Work/bf:Instance on blank (or external) nodes. They carry
// no minted identity, so the scan must not surface them -- seeded stub keys
// and identifiers would capture unrelated incoming records, and they showed
// up as bogus "c14n" rows in the duplicates worklist.
func TestScanGrainSkipsRelationStubs(t *testing.T) {
	grain := []byte(`<#w1Work> <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <http://id.loc.gov/ontologies/bibframe/Work> <feed:marc> .
<#w1Work> <http://id.loc.gov/ontologies/bibframe/relation> _:rel <feed:marc> .
_:rel <http://id.loc.gov/ontologies/bibframe/associatedResource> _:c14n10 <feed:marc> .
_:c14n10 <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <http://id.loc.gov/ontologies/bibframe/Work> <feed:marc> .
_:c14n11 <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <http://id.loc.gov/ontologies/bibframe/Instance> <feed:marc> .
_:c14n11 <http://id.loc.gov/ontologies/bibframe/identifiedBy> _:sid <feed:marc> .
_:sid <http://www.w3.org/1999/02/22-rdf-syntax-ns#value> "9780000000009" <feed:marc> .
<https://example.org/otherWork> <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <http://id.loc.gov/ontologies/bibframe/Work> <feed:marc> .
`)
	gi, err := ScanGrain(grain)
	if err != nil {
		t.Fatalf("ScanGrain: %v", err)
	}
	if len(gi.Works) != 1 || gi.Works[0].WorkID != "w1" {
		t.Fatalf("works = %+v, want only w1", gi.Works)
	}
	if len(gi.Instances) != 0 {
		t.Fatalf("instances = %+v, want none", gi.Instances)
	}
}
