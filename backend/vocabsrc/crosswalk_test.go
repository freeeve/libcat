package vocabsrc

import (
	"errors"
	"testing"

	"github.com/freeeve/libcat/ingest"
	"github.com/freeeve/libcat/storage/blob"

	"github.com/freeeve/libcat/backend/vocab"
)

// Two schemes side by side: a homosaurus term closeMatch-linked to an LCSH
// heading, plus an exactMatch pair, plus a retired LCSH target.
const crosswalkNT = `<https://homosaurus.org/v4/homoit1> <http://www.w3.org/2004/02/skos/core#prefLabel> "Gay men"@en <authority:homosaurus> .
<https://homosaurus.org/v4/homoit1> <http://www.w3.org/2004/02/skos/core#closeMatch> <http://id.loc.gov/authorities/subjects/sh1> <authority:homosaurus> .
<https://homosaurus.org/v4/homoit2> <http://www.w3.org/2004/02/skos/core#prefLabel> "Zines"@en <authority:homosaurus> .
<https://homosaurus.org/v4/homoit2> <http://www.w3.org/2004/02/skos/core#exactMatch> <http://id.loc.gov/authorities/subjects/sh2> <authority:homosaurus> .
<https://homosaurus.org/v4/homoit3> <http://www.w3.org/2004/02/skos/core#prefLabel> "Retired link"@en <authority:homosaurus> .
<https://homosaurus.org/v4/homoit3> <http://www.w3.org/2004/02/skos/core#closeMatch> <http://id.loc.gov/authorities/subjects/sh3> <authority:homosaurus> .
<http://id.loc.gov/authorities/subjects/sh1> <http://www.w3.org/2004/02/skos/core#prefLabel> "Gay men"@en <authority:lcsh> .
<http://id.loc.gov/authorities/subjects/sh1> <http://www.w3.org/2004/02/skos/core#broader> <http://id.loc.gov/authorities/subjects/shParent> <authority:lcsh> .
<http://id.loc.gov/authorities/subjects/shParent> <http://www.w3.org/2004/02/skos/core#prefLabel> "Gay people"@en <authority:lcsh> .
<http://id.loc.gov/authorities/subjects/shParent> <http://www.w3.org/2004/02/skos/core#broader> <http://id.loc.gov/authorities/subjects/shGrand> <authority:lcsh> .
<http://id.loc.gov/authorities/subjects/shGrand> <http://www.w3.org/2004/02/skos/core#prefLabel> "Sexual minorities"@en <authority:lcsh> .
<http://id.loc.gov/authorities/subjects/sh2> <http://www.w3.org/2004/02/skos/core#prefLabel> "Zines"@en <authority:lcsh> .
<http://id.loc.gov/authorities/subjects/sh3> <http://www.w3.org/2004/02/skos/core#prefLabel> "Old heading"@en <authority:lcsh> .
<http://id.loc.gov/authorities/subjects/sh3> <https://github.com/freeeve/libcat/ns#mergedInto> <http://id.loc.gov/authorities/subjects/sh2> <authority:lcsh> .
`

func crosswalkIndex(t *testing.T) *vocab.Index {
	t.Helper()
	bs := blob.NewMem()
	if _, err := bs.Put(t.Context(), "data/authorities/x.nq", []byte(crosswalkNT), blob.PutOptions{}); err != nil {
		t.Fatal(err)
	}
	ix, err := vocab.Load(t.Context(), bs, "data/authorities/", nil)
	if err != nil {
		t.Fatal(err)
	}
	return ix
}

func TestCrosswalkEnricher(t *testing.T) {
	ix := crosswalkIndex(t)
	e := NewCrosswalk(ix, "lcsh")
	if e.Name() != "crosswalk-lcsh" {
		t.Fatalf("name = %s", e.Name())
	}
	works := []ingest.WorkSummary{
		// closeMatch walks with 0.85 confidence.
		{WorkID: "w1", Subjects: []string{"https://homosaurus.org/v4/homoit1"}},
		// exactMatch walks with 1.0; the work already carrying the target
		// gains nothing.
		{WorkID: "w2", Subjects: []string{"https://homosaurus.org/v4/homoit2", "http://id.loc.gov/authorities/subjects/sh2"}},
		// A retired target is never suggested.
		{WorkID: "w3", Subjects: []string{"https://homosaurus.org/v4/homoit3"}},
		// LCSH subjects do not crosswalk into themselves.
		{WorkID: "w4", Subjects: []string{"http://id.loc.gov/authorities/subjects/sh1"}},
	}
	out, err := e.Enrich(t.Context(), works)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("enrichments = %+v", out)
	}
	got := out[0]
	if got.WorkID != "w1" || len(got.Subjects) != 1 || got.Confidence != confCloseMatch {
		t.Fatalf("w1 enrichment = %+v", got)
	}
	if got.Subjects[0].URI != "http://id.loc.gov/authorities/subjects/sh1" || got.Subjects[0].Labels["en"] != "Gay men" {
		t.Fatalf("w1 subject = %+v", got.Subjects[0])
	}
	// The candidate's transitive broader chain rides along as standalone
	// term metadata, nearer ancestor first.
	if len(got.Terms) != 2 ||
		got.Terms[0].URI != "http://id.loc.gov/authorities/subjects/shParent" || got.Terms[0].Labels["en"] != "Gay people" ||
		got.Terms[1].URI != "http://id.loc.gov/authorities/subjects/shGrand" || got.Terms[1].Labels["en"] != "Sexual minorities" {
		t.Fatalf("w1 ancestor terms = %+v", got.Terms)
	}
	if len(got.Terms[0].Broader) != 1 || got.Terms[0].Broader[0] != "http://id.loc.gov/authorities/subjects/shGrand" {
		t.Fatalf("parent term broader = %+v", got.Terms[0].Broader)
	}
}

func TestCacheTermResolvesForever(t *testing.T) {
	s := newService(t)
	ctx := t.Context()
	ix, err := vocab.Load(ctx, s.Blob, s.AuthoritiesPrefix, nil)
	if err != nil {
		t.Fatal(err)
	}
	s.Index = ix

	sugg := Suggestion{
		Source: "wikidata", Scheme: "wikidata",
		ID: "http://www.wikidata.org/entity/Q69990794", Label: "non-binary gender",
		Description: "gender identity outside the binary",
		ExactMatch:  []string{"https://homosaurus.org/v4/homoit0000505"},
	}
	if err := s.CacheTerm(ctx, sugg); err != nil {
		t.Fatal(err)
	}
	term, ok := ix.Resolve("http://www.wikidata.org/entity/Q69990794")
	if !ok || term.Scheme != "wikidata" || term.Label("en") != "non-binary gender" {
		t.Fatalf("cached term = %+v (ok=%v)", term, ok)
	}
	if len(term.ExactMatch) != 1 || term.Definition["en"] == "" {
		t.Fatalf("cached term details = %+v", term)
	}
	// Idempotent: re-caching is a no-op, not an error.
	if err := s.CacheTerm(ctx, sugg); err != nil {
		t.Fatal(err)
	}
	// A configured scheme filter picks the cached scheme up automatically.
	s.BaseSchemes = []string{"local"}
	schemes, err := s.Schemes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, scheme := range schemes {
		if scheme == "wikidata" {
			found = true
		}
	}
	if !found {
		t.Fatalf("schemes = %v, want wikidata included", schemes)
	}
	// Survives a full reload from the blob store.
	if err := s.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	if _, ok := ix.Resolve("http://www.wikidata.org/entity/Q69990794"); !ok {
		t.Fatal("cached term lost on reload")
	}
	// Validation floor.
	if err := s.CacheTerm(ctx, Suggestion{Scheme: "x", ID: "not-a-uri", Label: "y"}); !errors.Is(err, ErrValidation) {
		t.Fatalf("bad id err = %v", err)
	}
	if err := s.CacheTerm(ctx, Suggestion{Scheme: "", ID: "https://x", Label: "y"}); !errors.Is(err, ErrValidation) {
		t.Fatalf("no scheme err = %v", err)
	}
}

// TestCrosswalkPivotReachesHomosaurusFromFAST is the task-418 regression:
// works cataloged in FAST, homosaurus linking only to LCSH, LCSH itself NOT
// loaded. The pivot through the shared (unloaded) LCSH URI must suggest the
// homosaurus term at pivot confidence -- before this, FAST collections got
// structurally nothing from the crosswalk enricher.
func TestCrosswalkPivotReachesHomosaurusFromFAST(t *testing.T) {
	const pivotNT = `<http://id.worldcat.org/fast/995592> <http://www.w3.org/2004/02/skos/core#prefLabel> "Lesbians"@en <authority:fast> .
<http://id.worldcat.org/fast/995592> <http://www.w3.org/2004/02/skos/core#exactMatch> <http://id.loc.gov/authorities/subjects/sh85076160> <authority:fast> .
<https://homosaurus.org/v4/homoit1> <http://www.w3.org/2004/02/skos/core#prefLabel> "Lesbian"@en <authority:homosaurus> .
<https://homosaurus.org/v4/homoit1> <http://www.w3.org/2004/02/skos/core#closeMatch> <http://id.loc.gov/authorities/subjects/sh85076160> <authority:homosaurus> .
<http://id.worldcat.org/fast/900000> <http://www.w3.org/2004/02/skos/core#prefLabel> "Linkless"@en <authority:fast> .
`
	bs := blob.NewMem()
	if _, err := bs.Put(t.Context(), "data/authorities/p.nq", []byte(pivotNT), blob.PutOptions{}); err != nil {
		t.Fatal(err)
	}
	ix, err := vocab.Load(t.Context(), bs, "data/authorities/", nil)
	if err != nil {
		t.Fatal(err)
	}
	e := NewCrosswalk(ix, "homosaurus")
	out, err := e.Enrich(t.Context(), []ingest.WorkSummary{
		{WorkID: "wfast000001a", Subjects: []string{"http://id.worldcat.org/fast/995592"}},
		// A FAST term with no links still reaches nothing: pivots need a
		// first hop, never a guess.
		{WorkID: "wfast000001b", Subjects: []string{"http://id.worldcat.org/fast/900000"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("enrichments = %+v, want the pivot hit only", out)
	}
	got := out[0]
	if got.WorkID != "wfast000001a" || len(got.Subjects) != 1 || got.Subjects[0].URI != "https://homosaurus.org/v4/homoit1" {
		t.Fatalf("pivot enrichment = %+v", got)
	}
	// exact (fast->lcsh) + close (homosaurus->lcsh): the weaker hop grades it.
	if got.Confidence != confPivotClose {
		t.Fatalf("confidence = %v, want pivot-close %v", got.Confidence, confPivotClose)
	}
}

// TestCrosswalkTiersKeepDemotionVisible pins task 428 on queerbooks' exact
// Homosaurus v5 shape: "Women" and "Womyn" are skos:related (NOT
// broader/narrower, so the ancestor drop cannot fire) and BOTH exactMatch
// the bare LCSH "Women" node -- and, per task 430, FAST folds "Womyn" into
// "Women" as an ALT label, which must read as see-also grade rather than
// granting the full-match exemption. The demotion must survive into the
// emitted confidences: one enrichment per tier, Women at pivot-exact,
// Womyn materially below it, never folded into one per-work number.
func TestCrosswalkTiersKeepDemotionVisible(t *testing.T) {
	const relatedNT = `<urn:fast:women> <http://www.w3.org/2004/02/skos/core#prefLabel> "Women"@en <authority:fast> .
<urn:fast:women> <http://www.w3.org/2004/02/skos/core#exactMatch> <urn:lcsh:women> <authority:fast> .
<urn:homo:women> <http://www.w3.org/2004/02/skos/core#prefLabel> "Women"@en <authority:homosaurus> .
<urn:homo:women> <http://www.w3.org/2004/02/skos/core#exactMatch> <urn:lcsh:women> <authority:homosaurus> .
<urn:homo:womyn> <http://www.w3.org/2004/02/skos/core#prefLabel> "Womyn"@en <authority:homosaurus> .
<urn:homo:womyn> <http://www.w3.org/2004/02/skos/core#exactMatch> <urn:lcsh:women> <authority:homosaurus> .
<urn:homo:womyn> <http://www.w3.org/2004/02/skos/core#related> <urn:homo:women> <authority:homosaurus> .
<urn:homo:women> <http://www.w3.org/2004/02/skos/core#related> <urn:homo:womyn> <authority:homosaurus> .
`
	bs := blob.NewMem()
	if _, err := bs.Put(t.Context(), "data/authorities/rel/vocab.nq", []byte(relatedNT), blob.PutOptions{}); err != nil {
		t.Fatal(err)
	}
	ix, err := vocab.Load(t.Context(), bs, "data/authorities/", nil)
	if err != nil {
		t.Fatal(err)
	}
	e := NewCrosswalk(ix, "homosaurus")
	out, err := e.Enrich(t.Context(), []ingest.WorkSummary{
		{WorkID: "w428a", Subjects: []string{"urn:fast:women"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("enrichments = %+v, want two tiers", out)
	}
	byURI := map[string]float64{}
	for _, enr := range out {
		for _, s := range enr.Subjects {
			byURI[s.URI] = enr.Confidence
		}
	}
	if byURI["urn:homo:women"] != confPivotExact {
		t.Errorf("Women confidence = %v, want pivot-exact %v", byURI["urn:homo:women"], confPivotExact)
	}
	if byURI["urn:homo:womyn"] != confPivotClose {
		t.Errorf("Womyn confidence = %v, want demoted pivot-close %v", byURI["urn:homo:womyn"], confPivotClose)
	}
	if byURI["urn:homo:womyn"] >= byURI["urn:homo:women"] {
		t.Errorf("demotion invisible: Womyn %v >= Women %v", byURI["urn:homo:womyn"], byURI["urn:homo:women"])
	}
	// Strongest tier emits first.
	if out[0].Confidence < out[1].Confidence {
		t.Errorf("tier order = %v then %v, want strongest first", out[0].Confidence, out[1].Confidence)
	}
}
