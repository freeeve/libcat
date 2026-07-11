package oaipmh

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/freeeve/libcat/ingest"
)

const page1 = `<OAI-PMH><ListRecords>` +
	`<record><header><identifier>oai:x:1</identifier></header><metadata>` +
	`<record xmlns="http://www.loc.gov/MARC21/slim"><leader>00000nam a2200000 a 4500</leader>` +
	`<controlfield tag="001">ctrl1</controlfield>` +
	`<datafield tag="245" ind1="0" ind2="0"><subfield code="a">First Work</subfield></datafield></record>` +
	`</metadata></record>` +
	`<record><header status="deleted"><identifier>oai:x:2</identifier></header></record>` +
	`<resumptionToken>TOKEN2</resumptionToken>` +
	`</ListRecords></OAI-PMH>`

const page2 = `<OAI-PMH><ListRecords>` +
	`<record><header><identifier>oai:x:3</identifier></header><metadata>` +
	`<record xmlns="http://www.loc.gov/MARC21/slim"><leader>00000nam a2200000 a 4500</leader>` +
	`<controlfield tag="001">ctrl3</controlfield>` +
	`<datafield tag="245" ind1="0" ind2="0"><subfield code="a">Third Work</subfield></datafield></record>` +
	`</metadata></record>` +
	`</ListRecords></OAI-PMH>`

// stubClient records each request's query and returns the fixture pages.
type stubClient struct{ calls []string }

func (s *stubClient) Do(req *http.Request) (*http.Response, error) {
	s.calls = append(s.calls, req.URL.RawQuery)
	body := page1
	if req.URL.Query().Get("resumptionToken") == "TOKEN2" {
		body = page2
	}
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
}

func newProvider(t *testing.T, params map[string]string) (*Provider, *stubClient) {
	t.Helper()
	p, err := New(ingest.Config{Source: "http://ils.example/oai", Params: params})
	if err != nil {
		t.Fatal(err)
	}
	prov := p.(*Provider)
	c := &stubClient{}
	prov.SetClient(c)
	return prov, c
}

func TestHarvestFollowsResumptionSkipsDeleted(t *testing.T) {
	p, c := newProvider(t, map[string]string{"set": "biblios", "from": "2026-01-01"})
	recs, err := p.Records(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 2 {
		t.Fatalf("got %d records, want 2 (the deleted record is skipped, both pages harvested)", len(recs))
	}
	if recs[0].Identity().Title == "" {
		t.Error("harvested record has no title in its identity -- the MARCXML crosswalk did not run")
	}
	// Page 1 carries the selectors; page 2 carries only the token (OAI rule).
	if !strings.Contains(c.calls[0], "metadataPrefix=marc21") || !strings.Contains(c.calls[0], "set=biblios") || !strings.Contains(c.calls[0], "from=2026-01-01") {
		t.Errorf("first request missing selectors: %s", c.calls[0])
	}
	if len(c.calls) != 2 || !strings.Contains(c.calls[1], "resumptionToken=TOKEN2") || strings.Contains(c.calls[1], "metadataPrefix") {
		t.Errorf("second request should carry only the resumption token: %v", c.calls)
	}
}

// flakyClient fails the first failFirst requests with a transient network error
// (a connection reset, as a recycled plack worker gives mid-harvest), then serves.
type flakyClient struct {
	failFirst int
	attempts  int
}

func (f *flakyClient) Do(*http.Request) (*http.Response, error) {
	f.attempts++
	if f.attempts <= f.failFirst {
		return nil, fmt.Errorf("read: connection reset by peer")
	}
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(page2))}, nil
}

func TestFetchRetriesTransientFailure(t *testing.T) {
	p, _ := newProvider(t, nil)
	c := &flakyClient{failFirst: 2}
	p.SetClient(c)
	recs, err := p.Records(context.Background())
	if err != nil {
		t.Fatalf("harvest should survive transient resets via retry: %v", err)
	}
	if len(recs) != 1 {
		t.Errorf("got %d records, want 1", len(recs))
	}
	if c.attempts != 3 {
		t.Errorf("attempts = %d, want 3 (2 failures + 1 success)", c.attempts)
	}
}

type errClient struct{ body string }

func (e errClient) Do(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(e.body))}, nil
}

func TestNoRecordsMatchIsEmptyNotError(t *testing.T) {
	p, _ := newProvider(t, nil)
	p.SetClient(errClient{`<OAI-PMH><error code="noRecordsMatch">nothing new</error></OAI-PMH>`})
	recs, err := p.Records(context.Background())
	if err != nil {
		t.Fatalf("noRecordsMatch must be an empty harvest, not an error: %v", err)
	}
	if len(recs) != 0 {
		t.Errorf("got %d records, want 0", len(recs))
	}
}

func TestOAIErrorIsFatal(t *testing.T) {
	p, _ := newProvider(t, nil)
	p.SetClient(errClient{`<OAI-PMH><error code="badArgument">bad</error></OAI-PMH>`})
	if _, err := p.Records(context.Background()); err == nil {
		t.Error("a badArgument OAI error must fail the harvest")
	}
}
