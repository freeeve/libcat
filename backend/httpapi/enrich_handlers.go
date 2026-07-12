package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/freeeve/libcat/ingest"

	"github.com/freeeve/libcat/backend/auth"
	"github.com/freeeve/libcat/backend/enrich"
)

// registerEnrich mounts the admin enrichment surface: list configured
// sources and kick a run. Runs execute synchronously in-request (sources are
// batched and bounded); a scheduled/worker execution path is a deployment
// concern layered on the same service.
func registerEnrich(mux *http.ServeMux, svc *enrich.Service, verifier auth.TokenVerifier, logger *slog.Logger) {
	admin := auth.Require(verifier, auth.RoleAdmin)
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}

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
		source := r.PathValue("source")
		result, err := svc.Run(r.Context(), source, keep)
		if err != nil {
			writeEnrichRunError(w, logger, source, err)
			return
		}
		result.Scope = filters.String()
		writeJSON(w, http.StatusOK, result)
	})))
}

// writeEnrichRunError maps a run failure to the status its cause deserves,
// so automation retries transient upstream faults (5xx) and gives up on its
// own mistakes (4xx). The raw error -- which can carry upstream URLs and
// storage paths -- goes to the server log; the client gets a generic message.
func writeEnrichRunError(w http.ResponseWriter, logger *slog.Logger, source string, err error) {
	switch {
	case errors.Is(err, enrich.ErrUnknownSource):
		writeError(w, http.StatusNotFound, "unknown enrichment source")
	case errors.Is(err, context.DeadlineExceeded):
		logger.Error("enrichment run timed out", "source", source, "err", err)
		writeError(w, http.StatusGatewayTimeout, "enrichment upstream timed out")
	case errors.Is(err, ingest.ErrEnricher):
		logger.Error("enrichment upstream failed", "source", source, "err", err)
		writeError(w, http.StatusBadGateway, "enrichment upstream failed")
	default:
		// Storage faults and source misconfiguration alike: the
		// deployment's problem, never the caller's.
		logger.Error("enrichment run failed", "source", source, "err", err)
		writeError(w, http.StatusInternalServerError, "enrichment run failed")
	}
}
