package bibframe

import (
	"strings"
	"testing"

	"github.com/freeeve/libcodex/rdf"
)

// TestSplitCrossGraphBlanks: a fused node (typed in two graphs) splits into
// per-graph copies; a clean grain returns untouched with count 0; a repaired
// grain reports clean on the second pass.
func TestSplitCrossGraphBlanks(t *testing.T) {
	fused := []byte(`<#w1Work> <http://id.loc.gov/ontologies/bibframe/subject> _:c14n1 <feed:marc> .
_:c14n1 <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <http://id.loc.gov/ontologies/bibframe/Place> <feed:marc> .
_:c14n1 <http://www.w3.org/2000/01/rdf-schema#label> "Louisiana." <feed:marc> .
<#w1Work> <http://id.loc.gov/ontologies/bibframe/classification> _:c14n1 <feed:copycat> .
_:c14n1 <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <http://id.loc.gov/ontologies/bibframe/ClassificationDdc> <feed:copycat> .
_:c14n1 <http://www.w3.org/2000/01/rdf-schema#label> "813/.6" <feed:copycat> .
`)
	out, n, err := SplitCrossGraphBlanks(fused)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("split %d fused nodes, want 1", n)
	}
	ds, err := rdf.ParseNQuads(out)
	if err != nil {
		t.Fatal(err)
	}
	// After the split: two distinct blanks, each typed in exactly one graph,
	// each with only its own graph's label.
	typesOf := map[string]map[string]bool{}
	labelsOf := map[string][]string{}
	for _, q := range ds.Quads {
		if !q.S.IsBlank() {
			continue
		}
		switch {
		case strings.HasSuffix(q.P.Value, "#type"):
			set := typesOf[q.S.Value]
			if set == nil {
				set = map[string]bool{}
				typesOf[q.S.Value] = set
			}
			set[q.O.Value] = true
		case strings.HasSuffix(q.P.Value, "#label"):
			labelsOf[q.S.Value] = append(labelsOf[q.S.Value], q.O.Value)
		}
	}
	if len(typesOf) != 2 {
		t.Fatalf("blanks after split = %d, want 2 (one per graph): %v", len(typesOf), typesOf)
	}
	for label, types := range typesOf {
		if len(types) != 1 {
			t.Errorf("blank %s still typed %d ways: %v", label, len(types), types)
		}
		if len(labelsOf[label]) != 1 {
			t.Errorf("blank %s carries %d labels, want its own graph's only", label, len(labelsOf[label]))
		}
	}

	// Idempotence: the repaired grain is clean.
	again, n2, err := SplitCrossGraphBlanks(out)
	if err != nil {
		t.Fatal(err)
	}
	if n2 != 0 || string(again) != string(out) {
		t.Errorf("second pass split %d and changed bytes=%v; want clean no-op", n2, string(again) != string(out))
	}

	// A clean grain passes through untouched.
	clean := []byte(`<#w2Work> <http://id.loc.gov/ontologies/bibframe/subject> _:b1 <feed:marc> .
_:b1 <http://www.w3.org/2000/01/rdf-schema#label> "Cats." <feed:marc> .
`)
	same, n3, err := SplitCrossGraphBlanks(clean)
	if err != nil {
		t.Fatal(err)
	}
	if n3 != 0 || string(same) != string(clean) {
		t.Error("clean grain must return untouched")
	}
}
