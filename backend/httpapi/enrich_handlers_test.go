// POST /v1/enrich/{source}/run -- failure classes map to distinct status
// codes (unknown source 404, upstream fault 502, timeout 504, storage or
// misconfiguration 500) and the client body never carries raw internal
// error strings; the detail goes to the server log.
package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/freeeve/libcat/ingest"
	"github.com/freeeve/libcat/storage/blob"

	"github.com/freeeve/libcat/backend/auth"
	"github.com/freeeve/libcat/backend/enrich"
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

func newEnrichAPI(t *testing.T) (http.Handler, blob.Store) {
	t.Helper()
	bs := blob.NewMem()
	// One work in the store, so direct-mode runs actually call the enricher.
	seedAuditWork(t, bs, "wenrich0001a", "", "", "zines", nil)
	svc := &enrich.Service{
		Blob:        bs,
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
	return mux, bs
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
