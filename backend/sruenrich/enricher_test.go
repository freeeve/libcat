package sruenrich

import (
	"context"
	"strings"
	"testing"

	"github.com/freeeve/libcat/ingest"
	"github.com/freeeve/libcat/storage/blob"

	"github.com/freeeve/libcat/backend/copycat"
	"github.com/freeeve/libcat/backend/marcview"
	"github.com/freeeve/libcat/backend/vocab"
)

const homoGayMen = "https://homosaurus.org/v4/homoit0000505"

// homosaurusFixture is a minimal loaded vocabulary: one term with an English
// label the harvest can reconcile against, by identifier or by heading.
const homosaurusFixture = `<` + homoGayMen + `> <http://www.w3.org/2004/02/skos/core#prefLabel> "Gay men"@en <authority:homosaurus> .
<` + homoGayMen + `> <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <http://www.w3.org/2004/02/skos/core#Concept> <authority:homosaurus> .
`

// fakeSearcher returns canned target records and logs the queries it saw.
type fakeSearcher struct {
	results []copycat.SearchResult
	fails   map[string]string
	queries [][]copycat.FieldTerm
}

func (f *fakeSearcher) SearchAll(ctx context.Context, q string, fields []copycat.FieldTerm, names []string) ([]copycat.SearchResult, map[string]string, map[string]string, error) {
	f.queries = append(f.queries, fields)
	return f.results, f.fails, nil, nil
}

func subjectField(tag, ind2, heading, source, id string) marcview.Field {
	f := marcview.Field{Tag: tag, Ind2: ind2, Subfields: []marcview.Subfield{{Code: "a", Value: heading}}}
	if source != "" {
		f.Subfields = append(f.Subfields, marcview.Subfield{Code: "2", Value: source})
	}
	if id != "" {
		f.Subfields = append(f.Subfields, marcview.Subfield{Code: "0", Value: id})
	}
	return f
}

func newEnricher(t *testing.T, s Searcher) *Enricher {
	t.Helper()
	st := blob.NewMem()
	if _, err := st.Put(t.Context(), "data/authorities/ho/homosaurus.nq", []byte(homosaurusFixture), blob.PutOptions{}); err != nil {
		t.Fatal(err)
	}
	ix, err := vocab.Load(t.Context(), st, "data/authorities/", nil)
	if err != nil {
		t.Fatal(err)
	}
	return &Enricher{Search: s, Vocab: ix}
}

// TestEnrichHarvestsReconciledSubjectsOnly is the contract: only 6XX access
// points that reconcile to a loaded controlled term come back (identifier
// matches above label matches), never titles/contributors, never headings
// the work already carries, and works without an ISBN are skipped.
func TestEnrichHarvestsReconciledSubjectsOnly(t *testing.T) {
	search := &fakeSearcher{results: []copycat.SearchResult{{
		Target: "loc",
		Record: marcview.RecordDoc{Fields: []marcview.Field{
			{Tag: "245", Subfields: []marcview.Subfield{{Code: "a", Value: "A Title That Must Not Leak"}}},
			subjectField("650", "7", "Gay men", "homoit", homoGayMen), // identifier match
			subjectField("650", "0", "Something unmapped", "", ""),    // reconciles nowhere
			subjectField("650", "7", "Zines", "local-vocab", ""),      // work already tags it
		}},
	}}}
	e := newEnricher(t, search)

	works := []ingest.WorkSummary{
		{WorkID: "wsru0000001a", ISBNs: []string{"9780000000001"}, Tags: []string{"Zines"}},
		{WorkID: "wsru0000001b"}, // no ISBN: skipped, never queried
	}
	out, err := e.Enrich(t.Context(), works)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("enrichments = %+v, want exactly one (id-matched tier)", out)
	}
	res := out[0]
	if res.WorkID != "wsru0000001a" || res.Confidence != confIDMatch {
		t.Fatalf("result = %+v", res)
	}
	if len(res.Subjects) != 1 || res.Subjects[0].URI != homoGayMen || res.Subjects[0].Labels["en"] != "Gay men" {
		t.Fatalf("subjects = %+v, want the reconciled homosaurus term", res.Subjects)
	}
	for _, s := range res.Subjects {
		if strings.Contains(s.Labels["en"], "Title") {
			t.Fatal("a non-subject field leaked into the harvest")
		}
	}
	if len(search.queries) != 1 || search.queries[0][0].Index != "isbn" {
		t.Fatalf("queries = %+v, want one ISBN query (the ISBN-less work skipped)", search.queries)
	}
	st := e.RunStats()
	if st.Batches != 1 || st.SkippedBatches != 1 {
		t.Fatalf("stats = %+v, want 1 query / 1 skip", st)
	}
}

// TestEnrichLabelMatchTier: a heading with no identifier that whole-heading
// matches a loaded term lands in the lower-confidence tier, and a work
// already carrying the term's URI never re-suggests it.
func TestEnrichLabelMatchTier(t *testing.T) {
	search := &fakeSearcher{results: []copycat.SearchResult{{
		Target: "dnb",
		Record: marcview.RecordDoc{Fields: []marcview.Field{
			subjectField("650", "7", "Gay men.", "gnd", ""), // label match (punctuation-blind)
		}},
	}}}
	e := newEnricher(t, search)

	out, err := e.Enrich(t.Context(), []ingest.WorkSummary{{WorkID: "wsru0000002a", ISBNs: []string{"9780000000002"}}})
	if err != nil || len(out) != 1 || out[0].Confidence != confLabelMatch {
		t.Fatalf("out = %+v, %v; want one label-tier enrichment", out, err)
	}

	// Same harvest against a work already carrying the term: nothing.
	out, err = e.Enrich(t.Context(), []ingest.WorkSummary{{
		WorkID: "wsru0000002b", ISBNs: []string{"9780000000002"}, Subjects: []string{homoGayMen},
	}})
	if err != nil || len(out) != 0 {
		t.Fatalf("re-suggested a carried subject: %+v, %v", out, err)
	}
}

// TestEnrichAllTargetsDownSkips: a query where every target failed counts
// the work as skipped (a re-run backfills) rather than concluding "no
// subjects out there".
func TestEnrichAllTargetsDownSkips(t *testing.T) {
	search := &fakeSearcher{fails: map[string]string{"loc": "timeout", "dnb": "500"}}
	e := newEnricher(t, search)
	out, err := e.Enrich(t.Context(), []ingest.WorkSummary{{WorkID: "wsru0000003a", ISBNs: []string{"9780000000003"}}})
	if err != nil || len(out) != 0 {
		t.Fatalf("out = %+v, %v", out, err)
	}
	if st := e.RunStats(); st.SkippedBatches != 1 {
		t.Fatalf("stats = %+v, want the all-targets-down work skipped", st)
	}
}
