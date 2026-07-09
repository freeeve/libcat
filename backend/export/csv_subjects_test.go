package export

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"

	"github.com/freeeve/libcodex/rdf"

	"github.com/freeeve/libcat/bibframe"
	"github.com/freeeve/libcat/storage/blob"

	"github.com/freeeve/libcat/backend/store"
	"github.com/freeeve/libcat/backend/vocab"
)

// csvSubjectAuthorities knows two of the fixture's three subjects. The third
// resolves nowhere, standing in for a term whose authority was never loaded.
const csvSubjectAuthorities = `<https://example.org/auth/t1> <http://www.w3.org/2004/02/skos/core#prefLabel> "Zines"@en <authority:local> .
<https://example.org/auth/g1> <http://www.w3.org/2004/02/skos/core#prefLabel> "Graphic novels"@en <authority:local> .
`

const (
	csvSubjWorkID   = "wcsv123subj456"
	csvSubjInGrain  = "https://example.org/auth/t1"
	csvSubjInIndex  = "https://example.org/auth/g1"
	csvSubjNowhere  = "https://example.org/auth/unknown"
	skosPrefLabel   = "http://www.w3.org/2004/02/skos/core#prefLabel"
	rdfTypeIRI      = "http://www.w3.org/1999/02/22-rdf-syntax-ns#type"
	bfNS            = "http://id.loc.gov/ontologies/bibframe/"
	csvSubjectsCell = 4
)

// seedCSVSubjectGrain writes a work with three controlled subjects: one whose
// skos:prefLabel rides the grain, one labeled only in the term index, one no
// index resolves.
func seedCSVSubjectGrain(t *testing.T, bs blob.Store) {
	t.Helper()
	ds := &rdf.Dataset{}
	feed := bibframe.FeedGraph("overdrive")
	work := rdf.NewIRI(bibframe.WorkIRI(csvSubjWorkID))
	ds.Add(work, rdf.NewIRI(rdfTypeIRI), rdf.NewIRI(bfNS+"Work"), feed)
	titleNode := rdf.NewBlank("t0")
	ds.Add(work, rdf.NewIRI(bfNS+"title"), titleNode, feed)
	ds.Add(titleNode, rdf.NewIRI(bfNS+"mainTitle"), rdf.NewLiteral("River of Teeth", "", ""), feed)
	for _, id := range []string{csvSubjInGrain, csvSubjInIndex, csvSubjNowhere} {
		ds.Add(work, rdf.NewIRI(bfNS+"subject"), rdf.NewIRI(id), feed)
	}
	// Only the first subject carries its label in the grain.
	ds.Add(rdf.NewIRI(csvSubjInGrain), rdf.NewIRI(skosPrefLabel), rdf.NewLiteral("Zines", "en", ""), feed)
	nq, err := ds.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bs.Put(t.Context(), bibframe.GrainPath(csvSubjWorkID), nq, blob.PutOptions{}); err != nil {
		t.Fatal(err)
	}
}

// exportedSubjects runs a one-work CSV export and returns the subjects cell,
// split back out of its "; " join.
func exportedSubjects(t *testing.T, svc *Service) []string {
	t.Helper()
	job, err := svc.Create(t.Context(), "lib@example.org", FormatCSV, Selection{WorkIDs: []string{csvSubjWorkID}})
	if err != nil || job.Status != StatusDone {
		t.Fatalf("csv job = %+v, %v", job, err)
	}
	out, err := svc.Open(t.Context(), job)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(bytes.NewReader(out)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("csv rows = %d, want header + one work", len(rows))
	}
	return strings.Split(rows[1][csvSubjectsCell], "; ")
}

// TestExportCSVResolvesSubjectLabels covers tasks/233: the human-facing CSV
// renders a controlled subject as a word whether its label rides the grain or
// only the loaded term index, and an unresolvable term stays visible as its
// IRI rather than being dropped from the row.
func TestExportCSVResolvesSubjectLabels(t *testing.T) {
	bs := blob.NewMem()
	seedCSVSubjectGrain(t, bs)
	if _, err := bs.Put(t.Context(), "data/authorities/ab/fixture.nq", []byte(csvSubjectAuthorities), blob.PutOptions{}); err != nil {
		t.Fatal(err)
	}
	ix, err := vocab.Load(t.Context(), bs, "data/authorities/", nil)
	if err != nil {
		t.Fatal(err)
	}
	svc, err := New(store.NewMem(), bs, "overdrive", []byte("0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	svc.Vocab = ix

	got := exportedSubjects(t, svc)
	want := map[string]bool{"Zines": true, "Graphic novels": true, csvSubjNowhere: true}
	if len(got) != len(want) {
		t.Fatalf("subjects = %q, want %d values", got, len(want))
	}
	for _, v := range got {
		if !want[v] {
			t.Errorf("subjects = %q: %q is not a label the index or grain provides", got, v)
		}
		if strings.HasPrefix(v, "https://example.org/auth/") && v != csvSubjNowhere {
			t.Errorf("subject %q exported as a raw IRI though it resolves", v)
		}
	}

	// With no index loaded the export still works, falling back to the grain's
	// own label and the IRI -- deployments that load no vocabulary keep the
	// old behavior rather than failing.
	svc.Vocab = nil
	bare := exportedSubjects(t, svc)
	if len(bare) != 3 {
		t.Fatalf("subjects without an index = %q", bare)
	}
	for _, v := range bare {
		if v == "Graphic novels" {
			t.Fatalf("no index was loaded, yet %q resolved", v)
		}
	}
	if !strings.Contains(strings.Join(bare, "; "), "Zines") {
		t.Fatalf("the grain's own label must survive without an index: %q", bare)
	}
}
