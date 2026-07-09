package bibframe

import (
	"strings"
	"testing"

	"github.com/freeeve/libcodex/rdf"
)

// cloneSourceGrain builds a grain with the shapes CloneGrain must handle:
// feed description, an identifier subgraph with a blank-node child, admin
// metadata, an item, an editorial tag, and a second instance.
func cloneSourceGrain(t *testing.T) []byte {
	t.Helper()
	ds := &rdf.Dataset{}
	feed := FeedGraph("overdrive")
	bf := "http://id.loc.gov/ontologies/bibframe/"
	work := rdf.NewIRI(WorkIRI("w1"))
	inst1, inst2 := rdf.NewIRI(InstanceIRI("i1")), rdf.NewIRI(InstanceIRI("i2"))
	// Node-shaped title, the real grain shape -- its blank node must
	// skolemize so the clone's title stays editable.
	titleNode := rdf.NewBlank("t0")
	ds.Add(work, rdf.NewIRI(bf+"title"), titleNode, feed)
	ds.Add(titleNode, rdf.NewIRI(bf+"mainTitle"), rdf.NewLiteral("A Book", "", ""), feed)
	ds.Add(work, rdf.NewIRI(bf+"hasInstance"), inst1, feed)
	ds.Add(work, rdf.NewIRI(bf+"hasInstance"), inst2, feed)
	ds.Add(inst1, rdf.NewIRI(bf+"instanceOf"), work, feed)
	ds.Add(inst2, rdf.NewIRI(bf+"instanceOf"), work, feed)
	// Identifier subgraph: instance -> blank id node -> value + source node.
	idNode, srcNode := rdf.NewBlank("id0"), rdf.NewBlank("src0")
	ds.Add(inst1, rdf.NewIRI(bf+"identifiedBy"), idNode, feed)
	ds.Add(idNode, rdf.NewIRI("http://www.w3.org/1999/02/22-rdf-syntax-ns#value"), rdf.NewLiteral("od-12345", "", ""), feed)
	ds.Add(idNode, rdf.NewIRI(bf+"source"), srcNode, feed)
	ds.Add(srcNode, rdf.NewIRI("http://www.w3.org/2000/01/rdf-schema#label"), rdf.NewLiteral("OverDrive", "", ""), feed)
	// Admin metadata subgraph.
	adminNode := rdf.NewBlank("adm0")
	ds.Add(inst1, rdf.NewIRI(bf+"adminMetadata"), adminNode, feed)
	ds.Add(adminNode, rdf.NewIRI("http://www.w3.org/2000/01/rdf-schema#label"), rdf.NewLiteral("DLC", "", ""), feed)
	// Subjects: a controlled term (IRI, carries) and an uncontrolled
	// provider heading (blank node, stays with the source -- skolemizing it
	// would forge a controlled term).
	ds.Add(work, rdf.NewIRI(bf+"subject"), rdf.NewIRI("http://id.loc.gov/authorities/subjects/sh85056595"), feed)
	headingNode := rdf.NewBlank("subj0")
	ds.Add(work, rdf.NewIRI(bf+"subject"), headingNode, feed)
	ds.Add(headingNode, rdf.NewIRI("http://www.w3.org/2000/01/rdf-schema#label"), rdf.NewLiteral("Statesmen -- Virginia", "", ""), feed)
	nq, err := ds.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	// Editorial curation on the source: a tag, an item, suppression.
	nq, err = ApplyEditorialPatch(nq, Patch{Add: []rdf.Quad{TagQuad("w1", "book-club")}})
	if err != nil {
		t.Fatal(err)
	}
	nq, err = SetItems(nq, "i1", []Item{{CallNumber: "FIC UNG", Barcode: "31234"}})
	if err != nil {
		t.Fatal(err)
	}
	return nq
}

func TestCloneGrain(t *testing.T) {
	src := cloneSourceGrain(t)
	out, newID, err := CloneGrain(src, "w1")
	if err != nil {
		t.Fatalf("CloneGrain: %v", err)
	}
	if newID == "w1" || !strings.HasPrefix(newID, "w") || len(newID) != 14 {
		t.Fatalf("newID = %q", newID)
	}
	text := string(out)
	// Fresh ids: the old work and instance fragments are gone, the new work
	// node carries the title, and both instances re-minted distinctly.
	for _, old := range []string{"#w1Work", "#i1Instance", "#i2Instance"} {
		if strings.Contains(text, old) {
			t.Fatalf("clone still references %s:\n%s", old, text)
		}
	}
	if !strings.Contains(text, WorkIRI(newID)) || !strings.Contains(text, "A Book") {
		t.Fatalf("clone lost the work description:\n%s", text)
	}
	ds, err := rdf.ParseNQuads(out)
	if err != nil {
		t.Fatal(err)
	}
	insts := map[string]bool{}
	editorial := EditorialGraph()
	for i := range ds.Quads {
		q := &ds.Quads[i]
		if q.G != editorial {
			t.Fatalf("non-editorial statement in clone: %v", q)
		}
		// Skolemized: blank nodes would make the clone's structure fields
		// unpatchable in the editor.
		if q.S.IsBlank() || q.O.IsBlank() {
			t.Fatalf("blank node survived cloning: %v", q)
		}
		if q.S.IsIRI() && strings.HasSuffix(q.S.Value, "Instance") {
			insts[q.S.Value] = true
		}
	}
	if len(insts) != 2 {
		t.Fatalf("instances = %v, want 2 fresh ones", insts)
	}
	// Provider keys, admin metadata, holdings, curation markers, and
	// uncontrolled headings stayed with the source; the controlled subject
	// carried over.
	for _, gone := range []string{"od-12345", "OverDrive", "DLC", "31234", "FIC UNG", "book-club", "Statesmen"} {
		if strings.Contains(text, gone) {
			t.Fatalf("clone carried %q:\n%s", gone, text)
		}
	}
	if !strings.Contains(text, "sh85056595") {
		t.Fatalf("clone lost the controlled subject:\n%s", text)
	}
	// Born suppressed (the draft state), nothing else set.
	vis, err := Visibility(out, newID)
	if err != nil {
		t.Fatal(err)
	}
	if !vis.Suppressed || vis.Tombstoned || vis.Withdrawn != "" {
		t.Fatalf("visibility = %+v, want suppressed only", vis)
	}
	// Two clones of one source never share ids.
	_, newID2, err := CloneGrain(src, "w1")
	if err != nil || newID2 == newID {
		t.Fatalf("second clone id %q (err %v)", newID2, err)
	}
	// Describes-guard: a grain that does not describe the id refuses.
	if _, _, err := CloneGrain(src, "wzzz999zzz999z"); err == nil {
		t.Fatal("clone of undescribed work id succeeded")
	}
}
