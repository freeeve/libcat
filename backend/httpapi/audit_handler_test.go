// GET /v1/audit/diversity -- the live coverage-first content audit
// over the work index, matching `lcat audit` semantics: subject URIs match by
// scheme, heading labels and tags by keyword, extras drive ?filter/?source.
package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/freeeve/libcat/bibframe"
	"github.com/freeeve/libcat/storage/blob"
	"github.com/freeeve/libcodex/rdf"
)

// seedAuditWork writes a work grain with an optional controlled subject (uri +
// prefLabel), an optional uncontrolled tag (blank-node bf:subject), and optional
// extras.
func seedAuditWork(t *testing.T, bs blob.Store, workID, uri, prefLabel, tag string, extras map[string]string) {
	t.Helper()
	const (
		bfNS      = "http://id.loc.gov/ontologies/bibframe/"
		rdfType   = "http://www.w3.org/1999/02/22-rdf-syntax-ns#type"
		prefLbl   = "http://www.w3.org/2004/02/skos/core#prefLabel"
		rdfsLabel = "http://www.w3.org/2000/01/rdf-schema#label"
	)
	ds := &rdf.Dataset{}
	feed := bibframe.FeedGraph("coll")
	work := rdf.NewIRI(bibframe.WorkIRI(workID))
	ds.Add(work, rdf.NewIRI(rdfType), rdf.NewIRI(bfNS+"Work"), feed)
	titleNode := rdf.NewIRI("#" + workID + "Title")
	ds.Add(work, rdf.NewIRI(bfNS+"title"), titleNode, feed)
	ds.Add(titleNode, rdf.NewIRI(bfNS+"mainTitle"), rdf.NewLiteral("T "+workID, "", ""), feed)
	if uri != "" {
		subj := rdf.NewIRI(uri)
		ds.Add(work, rdf.NewIRI(bfNS+"subject"), subj, feed)
		if prefLabel != "" {
			ds.Add(subj, rdf.NewIRI(prefLbl), rdf.NewLiteral(prefLabel, "en", ""), feed)
		}
	}
	if tag != "" {
		topic := rdf.NewBlank("t1")
		ds.Add(work, rdf.NewIRI(bfNS+"subject"), topic, feed)
		ds.Add(topic, rdf.NewIRI(rdfsLabel), rdf.NewLiteral(tag, "", ""), feed)
	}
	for k, v := range extras {
		ds.Add(work, rdf.NewIRI(bibframe.ExtraPred+k), rdf.NewLiteral(v, "", ""), feed)
	}
	nq, err := ds.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bs.Put(t.Context(), bibframe.GrainPath(workID), nq, blob.PutOptions{}); err != nil {
		t.Fatal(err)
	}
}

type auditPage struct {
	Input        string `json:"input"`
	Scope        string `json:"scope"`
	TotalWorks   int    `json:"totalWorks"`
	CoveredWorks int    `json:"coveredWorks"`
	Categories   []struct {
		ID    string `json:"id"`
		Works int    `json:"works"`
	} `json:"categories"`
}

func getAudit(t *testing.T, h http.Handler, query string) auditPage {
	t.Helper()
	url := "/v1/audit/diversity"
	if query != "" {
		url += "?" + query
	}
	rec := request(t, h, http.MethodGet, url, "lib-token", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d, want 200 (%s)", url, rec.Code, rec.Body.String())
	}
	var page auditPage
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	return page
}

func auditCat(p auditPage, id string) int {
	for _, c := range p.Categories {
		if c.ID == id {
			return c.Works
		}
	}
	return -1
}

func TestAuditDiversity(t *testing.T) {
	h, bs := newRecordsAPI(t)
	// w1: Homosaurus URI with a keyword-less label -> lgbtqia via SCHEME.
	seedAuditWork(t, bs, "waudit00001a", "https://homosaurus.org/v5/homoit0000506", "Chosen family", "",
		map[string]string{"inQll": "true"})
	// w2: FAST URI whose HEADING label matches a keyword (plural-tolerant).
	seedAuditWork(t, bs, "waudit00001b", "http://id.worldcat.org/fast/995592", "Lesbians", "", nil)
	// w3: uncontrolled TAG only.
	seedAuditWork(t, bs, "waudit00001c", "", "", "Immigrants", nil)
	// w4: no aboutness signal at all -- dilutes coverage.
	seedAuditWork(t, bs, "waudit00001d", "", "", "", nil)

	p := getAudit(t, h, "")
	if p.TotalWorks != 4 || p.CoveredWorks != 3 {
		t.Errorf("totals = %d/%d, want 4 total / 3 covered", p.TotalWorks, p.CoveredWorks)
	}
	if got := auditCat(p, "lgbtqia"); got != 2 {
		t.Errorf("lgbtqia = %d, want 2 (scheme + heading-keyword paths)", got)
	}
	if got := auditCat(p, "immigrant-diaspora"); got != 1 {
		t.Errorf("immigrant-diaspora = %d, want 1 (tag path)", got)
	}
	if p.Input == "" {
		t.Error("response should name its input")
	}

	// ?filter scopes by extras and is named in the response.
	p = getAudit(t, h, "filter=inQll%3Dtrue")
	if p.TotalWorks != 1 || auditCat(p, "lgbtqia") != 1 {
		t.Errorf("filtered = %d works / lgbtqia %d, want 1/1", p.TotalWorks, auditCat(p, "lgbtqia"))
	}
	if p.Scope != "inQll=true" {
		t.Errorf("scope = %q, want inQll=true", p.Scope)
	}

	// A malformed filter is a 400, not a silent full-corpus report.
	rec := request(t, h, http.MethodGet, "/v1/audit/diversity?filter=nokey", "lib-token", "", nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("bad filter = %d, want 400", rec.Code)
	}
}
