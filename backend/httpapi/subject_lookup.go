package httpapi

import (
	"encoding/json"
	"maps"
	"net/http"
	"sort"
	"strings"

	"github.com/freeeve/libcat/identity"
	"github.com/freeeve/libcat/ingest"
	"github.com/freeeve/libcat/storage/blob"

	"github.com/freeeve/libcat/backend/auth"
	"github.com/freeeve/libcat/backend/copycat"
	"github.com/freeeve/libcat/backend/sruenrich"
	"github.com/freeeve/libcat/backend/vocab"
)

// registerSubjectLookup mounts the external-subject fetch: the
// work's ISBNs fan out to the copycat targets, 6XX headings come back
// deduped and reconciled against the local index. Explicitly button-driven
// -- target fan-out takes seconds.
func registerSubjectLookup(mux *http.ServeMux, cc *copycat.Service, bs blob.Store, ix *vocab.Index, verifier auth.TokenVerifier) {
	librarian := auth.Require(verifier, auth.RoleLibrarian)

	mux.Handle("POST /v1/works/{id}/subjects/lookup", librarian(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		workID := r.PathValue("id")
		if !workIDPattern.MatchString(workID) {
			writeError(w, http.StatusBadRequest, "bad work id")
			return
		}
		var req struct {
			Targets []string `json:"targets"`
		}
		if r.ContentLength > 0 {
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "bad request body")
				return
			}
		}
		grain, _, _, ok := readWorkGrain(w, r, bs)
		if !ok {
			return
		}
		summaries, err := ingest.SummarizeGrain(grain)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "grain parse failed")
			return
		}
		var isbns []string
		existingSubjects := map[string]bool{}
		existingLabels := map[string]bool{}
		for _, summary := range summaries {
			if summary.WorkID != workID {
				continue
			}
			isbns = append(isbns, summary.ISBNs...)
			for _, s := range summary.Subjects {
				existingSubjects[s] = true
			}
			for _, tag := range summary.Tags {
				existingLabels[sruenrich.NormHeading(tag)] = true
			}
		}
		if len(isbns) > 5 {
			isbns = isbns[:5]
		}
		if len(isbns) == 0 {
			writeError(w, http.StatusBadRequest, "this work carries no ISBNs to search by")
			return
		}
		byKey := map[string]*sruenrich.Candidate{}
		failures := map[string]string{}
		warnings := map[string]string{}
		for _, isbn := range isbns {
			results, fails, warns, err := cc.SearchAll(r.Context(), "", []copycat.FieldTerm{{Index: "isbn", Term: isbn}}, req.Targets)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			maps.Copy(failures, fails)
			maps.Copy(warnings, warns)
			for _, res := range results {
				sruenrich.Collect(byKey, res.Target, res.Record)
			}
		}
		candidates := make([]sruenrich.Candidate, 0, len(byKey))
		for _, c := range byKey {
			if existingLabels[sruenrich.NormHeading(c.Heading)] {
				continue
			}
			if ix != nil {
				c.Term = sruenrich.ReconcileIdentifiers(ix, c.IDs)
				if c.Term == nil {
					c.Term = sruenrich.ReconcileHeading(ix, c.Heading)
				}
				if c.Term != nil && existingSubjects[c.Term.ID] {
					continue
				}
			}
			sort.Strings(c.Targets)
			candidates = append(candidates, *c)
		}
		// Controlled matches first, then by prevalence.
		sort.Slice(candidates, func(i, j int) bool {
			if (candidates[i].Term != nil) != (candidates[j].Term != nil) {
				return candidates[i].Term != nil
			}
			if candidates[i].Count != candidates[j].Count {
				return candidates[i].Count > candidates[j].Count
			}
			return candidates[i].Heading < candidates[j].Heading
		})
		writeJSON(w, http.StatusOK, map[string]any{"candidates": candidates, "failures": failures, "warnings": warnings})
	})))

	// Identifier kinds: each bf:identifiedBy value mapped to its
	// BIBFRAME type so the editor can badge ISBN vs ISSN vs provider id.
	mux.Handle("GET /v1/works/{id}/identifiers", librarian(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		grain, _, workID, ok := readWorkGrain(w, r, bs)
		if !ok {
			return
		}
		gi, err := identity.ScanGrain(grain)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "grain parse failed")
			return
		}
		kinds := map[string]string{}
		for _, inst := range gi.Instances {
			for _, pk := range inst.ProviderKeys {
				if scheme, value, ok := strings.Cut(pk, ":"); ok {
					if _, taken := kinds[value]; !taken {
						kinds[value] = scheme
					}
				}
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"workId": workID, "kinds": kinds})
	})))
}
