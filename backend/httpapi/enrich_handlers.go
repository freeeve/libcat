package httpapi

import (
	"github.com/freeeve/libcat/ingest"

	"net/http"

	"github.com/freeeve/libcat/backend/auth"
	"github.com/freeeve/libcat/backend/enrich"
)

// registerEnrich mounts the admin enrichment surface: list configured
// sources and kick a run. Runs execute synchronously in-request (sources are
// batched and bounded); a scheduled/worker execution path is a deployment
// concern layered on the same service.
func registerEnrich(mux *http.ServeMux, svc *enrich.Service, verifier auth.TokenVerifier) {
	admin := auth.Require(verifier, auth.RoleAdmin)

	mux.Handle("GET /v1/enrich", admin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"sources": svc.Names()})
	})))

	// ?filter=key=value (repeatable, ANDed; comma-joined extras match per
	// element) and ?source= scope the run to matching works -- the same
	// predicate the diversity audit uses. An external-service source then
	// queries for exactly the scoped set, which is both quota hygiene and
	// data minimization: sensitive enrichment (creator demographics) can be
	// generated for a curated sub-collection only.
	mux.Handle("POST /v1/enrich/{source}/run", admin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filters, err := auditFilters(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		var keep func(*ingest.WorkSummary) bool
		if len(filters) > 0 {
			keep = func(s *ingest.WorkSummary) bool { return filters.match(s.Extras) }
		}
		result, err := svc.Run(r.Context(), r.PathValue("source"), keep)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		result.Scope = filters.String()
		writeJSON(w, http.StatusOK, result)
	})))
}
