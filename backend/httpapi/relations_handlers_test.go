package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestWorkRelationsAPI covers tasks/221: linking writes both directions,
// listing resolves titles, unlinking retracts both sides, and phantom /
// self links refuse before anything is written.
func TestWorkRelationsAPI(t *testing.T) {
	h, bs := newRecordsAPI(t)
	seedWorkGrain(t, bs)                  // editWorkID, "A Book"
	seedISBNGrain(t, bs, "9781250313195") // isbnWorkID
	link := map[string]string{"kind": "hasPart", "target": isbnWorkID}

	// Self-link and phantom target refuse.
	if rec := request(t, h, http.MethodPost, "/v1/works/"+editWorkID+"/relations", "lib-token", "", map[string]string{"kind": "hasPart", "target": editWorkID}); rec.Code != http.StatusBadRequest {
		t.Fatalf("self link = %d", rec.Code)
	}
	if rec := request(t, h, http.MethodPost, "/v1/works/"+editWorkID+"/relations", "lib-token", "", map[string]string{"kind": "hasPart", "target": "wzzzz00phantom"}); rec.Code != http.StatusNotFound {
		t.Fatalf("phantom target = %d", rec.Code)
	}
	if rec := request(t, h, http.MethodPost, "/v1/works/"+editWorkID+"/relations", "lib-token", "", map[string]string{"kind": "sideways", "target": isbnWorkID}); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad kind = %d", rec.Code)
	}

	// Link, then both sides list it (the inverse on the target).
	if rec := request(t, h, http.MethodPost, "/v1/works/"+editWorkID+"/relations", "lib-token", "", link); rec.Code != http.StatusNoContent {
		t.Fatalf("link = %d %s", rec.Code, rec.Body)
	}
	var got struct{ HasPart, PartOf []relationEntry }
	rec := request(t, h, http.MethodGet, "/v1/works/"+editWorkID+"/relations", "lib-token", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list = %d", rec.Code)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.HasPart) != 1 || got.HasPart[0].WorkID != isbnWorkID || len(got.PartOf) != 0 {
		t.Fatalf("source relations = %+v", got)
	}
	if got.HasPart[0].Title != "Companion Volume" {
		t.Fatalf("target title should resolve from the index: %+v", got.HasPart[0])
	}
	rec = request(t, h, http.MethodGet, "/v1/works/"+isbnWorkID+"/relations", "lib-token", "", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.PartOf) != 1 || got.PartOf[0].WorkID != editWorkID {
		t.Fatalf("target relations = %+v", got)
	}

	// Unlink retracts both sides.
	if rec := request(t, h, http.MethodDelete, "/v1/works/"+editWorkID+"/relations", "lib-token", "", link); rec.Code != http.StatusNoContent {
		t.Fatalf("unlink = %d %s", rec.Code, rec.Body)
	}
	for _, id := range []string{editWorkID, isbnWorkID} {
		rec = request(t, h, http.MethodGet, "/v1/works/"+id+"/relations", "lib-token", "", nil)
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if len(got.HasPart)+len(got.PartOf) != 0 {
			t.Fatalf("%s still related after unlink: %+v", id, got)
		}
	}

	// Anonymous refuses.
	if rec := request(t, h, http.MethodPost, "/v1/works/"+editWorkID+"/relations", "", "", link); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anon link = %d", rec.Code)
	}
}
