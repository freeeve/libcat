package httpapi

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"testing"

	"github.com/freeeve/libcat/storage/blob"

	"github.com/freeeve/libcat/backend/vocab"
)

// TestTermsResolveBatch drives the chip-resolver: URIs resolve to
// terms across schemes without the caller naming one; unresolvable URIs are
// silently absent.
func TestTermsResolveBatch(t *testing.T) {
	data, err := os.ReadFile("../vocab/testdata/authorities.nq")
	if err != nil {
		t.Fatal(err)
	}
	bs := blob.NewMem()
	_, _ = bs.Put(t.Context(), "a/x.nq", data, blob.PutOptions{})
	ix, err := vocab.Load(t.Context(), bs, "a/", nil)
	if err != nil {
		t.Fatal(err)
	}
	h := New(Deps{Vocab: ix})

	q := url.Values{}
	q.Add("id", "https://homosaurus.org/v4/homoit0001235")
	q.Add("id", "http://id.loc.gov/authorities/subjects/sh85118553")
	q.Add("id", "https://example.org/not-a-term")
	rec := request(t, h, http.MethodGet, "/v1/terms/resolve?"+q.Encode(), "", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("resolve = %d %s", rec.Code, rec.Body)
	}
	var res struct {
		Terms map[string]vocab.Term `json:"terms"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Terms) != 2 {
		t.Fatalf("resolved %d terms, want 2: %+v", len(res.Terms), res.Terms)
	}
	trans := res.Terms["https://homosaurus.org/v4/homoit0001235"]
	if trans.Scheme != "homosaurus" || trans.Labels["en"] != "Transgender people" || len(trans.Broader) != 1 {
		t.Fatalf("trans term = %+v", trans)
	}
	if res.Terms["http://id.loc.gov/authorities/subjects/sh85118553"].Scheme != "lcsh" {
		t.Fatalf("lcsh term = %+v", res.Terms["http://id.loc.gov/authorities/subjects/sh85118553"])
	}

	// Bounds.
	if rec := request(t, h, http.MethodGet, "/v1/terms/resolve", "", "", nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("no ids = %d", rec.Code)
	}
}

// TestTermsSearchPath drives the picker breadcrumb: search hits
// carry their broader-chain path; root terms simply omit it.
func TestTermsSearchPath(t *testing.T) {
	data, err := os.ReadFile("../vocab/testdata/authorities.nq")
	if err != nil {
		t.Fatal(err)
	}
	bs := blob.NewMem()
	_, _ = bs.Put(t.Context(), "a/x.nq", data, blob.PutOptions{})
	ix, err := vocab.Load(t.Context(), bs, "a/", nil)
	if err != nil {
		t.Fatal(err)
	}
	h := New(Deps{Vocab: ix})

	rec := request(t, h, http.MethodGet, "/v1/terms?scheme=homosaurus&q=trans", "", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("search = %d %s", rec.Code, rec.Body)
	}
	var res struct {
		Terms []struct {
			vocab.Term
			Path []vocab.TermRef `json:"path"`
		} `json:"terms"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Terms) != 1 || res.Terms[0].Labels["en"] != "Transgender people" {
		t.Fatalf("hits = %+v", res.Terms)
	}
	path := res.Terms[0].Path
	if len(path) != 1 || path[0].Label != "Gender identity" || path[0].ID != "https://homosaurus.org/v4/homoit0000508" {
		t.Fatalf("path = %+v", path)
	}
	// A root term has no path key at all.
	rec = request(t, h, http.MethodGet, "/v1/terms?scheme=homosaurus&q=gender", "", "", nil)
	var raw struct {
		Terms []map[string]json.RawMessage `json:"terms"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if len(raw.Terms) != 1 {
		t.Fatalf("root hits = %+v", raw.Terms)
	}
	if _, ok := raw.Terms[0]["path"]; ok {
		t.Fatalf("root term carries path: %s", raw.Terms[0]["path"])
	}
}

// TestTermsEquivalents drives the cross-scheme equivalents endpoint over the
// shared fixture: the homosaurus term's gnd exactMatch surfaces as a direct
// (unloaded) equivalent, an unknown URI is a 404, a missing id a 400.
func TestTermsEquivalents(t *testing.T) {
	data, err := os.ReadFile("../vocab/testdata/authorities.nq")
	if err != nil {
		t.Fatal(err)
	}
	bs := blob.NewMem()
	_, _ = bs.Put(t.Context(), "a/x.nq", data, blob.PutOptions{})
	ix, err := vocab.Load(t.Context(), bs, "a/", nil)
	if err != nil {
		t.Fatal(err)
	}
	h := New(Deps{Vocab: ix})

	rec := request(t, h, http.MethodGet,
		"/v1/terms/equivalents?id="+url.QueryEscape("https://homosaurus.org/v4/homoit0001235"), "", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("equivalents = %d %s", rec.Code, rec.Body)
	}
	var res struct {
		Equivalents []vocab.Equivalent `json:"equivalents"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Equivalents) == 0 {
		t.Fatal("expected at least the gnd exactMatch")
	}
	found := false
	for _, e := range res.Equivalents {
		if e.ID == "https://d-nb.info/gnd/4121991-6" {
			found = true
			if e.Strength != "exact" || e.Known {
				t.Errorf("gnd equivalent = %+v, want direct exact, not Known", e)
			}
		}
	}
	if !found {
		t.Errorf("gnd link missing from %+v", res.Equivalents)
	}

	if rec := request(t, h, http.MethodGet, "/v1/terms/equivalents?id=https%3A%2F%2Fexample.org%2Fnope", "", "", nil); rec.Code != http.StatusNotFound {
		t.Errorf("unknown term = %d, want 404", rec.Code)
	}
	if rec := request(t, h, http.MethodGet, "/v1/terms/equivalents", "", "", nil); rec.Code != http.StatusBadRequest {
		t.Errorf("missing id = %d, want 400", rec.Code)
	}
}
