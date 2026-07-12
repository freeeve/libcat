// POST /v1/enrich/{source}/run -- failure classes map to distinct status
// codes (unknown source 404, upstream fault 502, timeout 504, storage or
// misconfiguration 500) and the client body never carries raw internal
// error strings; the detail goes to the server log.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/freeeve/libcat/ingest"
	"github.com/freeeve/libcat/storage/blob"

	"github.com/freeeve/libcat/backend/auth"
	"github.com/freeeve/libcat/backend/enrich"
	"github.com/freeeve/libcat/backend/store"
)

// errEnricher fails every Enrich call with the configured error.
type errEnricher struct{ err error }

func (e errEnricher) Name() string { return "err" }
func (e errEnricher) Enrich(ctx context.Context, works []ingest.WorkSummary) ([]ingest.Enrichment, error) {
	return nil, e.err
}

// okEnricher enriches nothing, successfully.
type okEnricher struct{}

func (okEnricher) Name() string { return "ok" }
func (okEnricher) Enrich(ctx context.Context, works []ingest.WorkSummary) ([]ingest.Enrichment, error) {
	return nil, nil
}

func newEnrichAPI(t *testing.T) (http.Handler, *enrich.Service) {
	t.Helper()
	bs := blob.NewMem()
	// One work in the store, so direct-mode runs actually call the enricher.
	seedAuditWork(t, bs, "wenrich0001a", "", "", "zines", nil)
	svc := &enrich.Service{
		Blob:        bs,
		DB:          store.NewMem(),
		GrainPrefix: "data/works/",
		Sources: map[string]enrich.Source{
			"ok":      {Enricher: okEnricher{}, Mode: enrich.ModeDirect},
			"flaky":   {Enricher: errEnricher{err: errors.New("get https://upstream.internal/sparql: 429 Too Many Requests")}, Mode: enrich.ModeDirect},
			"slow":    {Enricher: errEnricher{err: context.DeadlineExceeded}, Mode: enrich.ModeDirect},
			"badmode": {Enricher: okEnricher{}, Mode: enrich.Mode("nonsense")},
			"noqueue": {Enricher: okEnricher{}, Mode: enrich.ModeQueue},
		},
	}
	verifier := staffVerifier{"admin-token": {Email: "admin@example.org", Roles: []auth.Role{auth.RoleAdmin}}}
	mux := http.NewServeMux()
	registerEnrich(mux, svc, verifier, nil)
	return mux, svc
}

func TestEnrichRunErrorClasses(t *testing.T) {
	h, _ := newEnrichAPI(t)
	cases := map[string]struct {
		source string
		status int
	}{
		"success":             {"ok", http.StatusOK},
		"unknown source":      {"zz-no-such-source", http.StatusNotFound},
		"upstream fault":      {"flaky", http.StatusBadGateway},
		"upstream timeout":    {"slow", http.StatusGatewayTimeout},
		"invalid mode":        {"badmode", http.StatusInternalServerError},
		"queue misconfigured": {"noqueue", http.StatusInternalServerError},
	}
	for name, tc := range cases {
		rec := request(t, h, http.MethodPost, "/v1/enrich/"+tc.source+"/run", "admin-token", "", nil)
		if rec.Code != tc.status {
			t.Errorf("%s: status = %d, want %d (%s)", name, rec.Code, tc.status, rec.Body)
		}
		body := rec.Body.String()
		// No leaked internals: package prefixes, upstream URLs, Go error
		// chains stay in the server log.
		for _, leak := range []string{"enrich:", "upstream.internal", "context deadline"} {
			if strings.Contains(body, leak) {
				t.Errorf("%s: body leaks %q: %s", name, leak, body)
			}
		}
	}
}

// TestEnrichJobSurface covers the async path over HTTP: kick returns 202
// QUEUED, the drained job polls DONE with its result, the list serves it,
// and unknown source/job ids are 404s with clean bodies.
func TestEnrichJobSurface(t *testing.T) {
	h, svc := newEnrichAPI(t)

	rec := request(t, h, http.MethodPost, "/v1/enrich/ok/jobs?filter=inQll=true", "admin-token", "", nil)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("kick = %d (%s)", rec.Code, rec.Body)
	}
	var job struct {
		ID      string      `json:"id"`
		Status  string      `json:"status"`
		Filters [][2]string `json:"filters"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &job)
	if job.Status != "QUEUED" || job.ID == "" || len(job.Filters) != 1 {
		t.Fatalf("kicked job = %+v", job)
	}

	if _, err := svc.RunQueuedJobs(t.Context()); err != nil {
		t.Fatal(err)
	}
	rec = request(t, h, http.MethodGet, "/v1/enrich/jobs/"+job.ID, "admin-token", "", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"DONE"`) {
		t.Fatalf("poll = %d %s, want DONE", rec.Code, rec.Body)
	}

	rec = request(t, h, http.MethodGet, "/v1/enrich/jobs", "admin-token", "", nil)
	var list struct{ Jobs []struct{ ID string } }
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list.Jobs) != 1 || list.Jobs[0].ID != job.ID {
		t.Fatalf("list = %s", rec.Body)
	}

	if rec := request(t, h, http.MethodPost, "/v1/enrich/zz-nope/jobs", "admin-token", "", nil); rec.Code != http.StatusNotFound {
		t.Errorf("kick unknown source = %d, want 404", rec.Code)
	}
	if rec := request(t, h, http.MethodGet, "/v1/enrich/jobs/deadbeef", "admin-token", "", nil); rec.Code != http.StatusNotFound {
		t.Errorf("poll unknown job = %d, want 404", rec.Code)
	}
}
