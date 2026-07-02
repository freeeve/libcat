package httpapi

import (
	"net/http"
	"strconv"

	"github.com/freeeve/libcatalog/backend/suggest"
	"github.com/freeeve/libcatalog/backend/vocab"
)

const termsDefaultLimit = 20

// registerTerms mounts the public autocomplete endpoint over the vocabulary
// index. With the suggestion service present, scheme=folk serves ACCEPTED
// community tags (PROPOSED and BLOCKED terms stay invisible).
func registerTerms(mux *http.ServeMux, ix *vocab.Index, folk *suggest.Service) {
	mux.HandleFunc("GET /v1/terms", func(w http.ResponseWriter, r *http.Request) {
		scheme := r.URL.Query().Get("scheme")
		q := r.URL.Query().Get("q")
		if scheme == "" {
			schemes := ix.Schemes()
			if folk != nil {
				schemes = append(schemes, vocab.FolkScheme)
			}
			writeJSON(w, http.StatusOK, map[string]any{"schemes": schemes})
			return
		}
		limit := termsDefaultLimit
		if raw := r.URL.Query().Get("limit"); raw != "" {
			if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 100 {
				limit = n
			}
		}
		if scheme == vocab.FolkScheme {
			if folk == nil {
				writeJSON(w, http.StatusOK, map[string]any{"terms": []any{}})
				return
			}
			norm, err := vocab.NormalizeFolk(q)
			if err != nil {
				norm = ""
			}
			names, err := folk.AcceptedFolkTerms(r.Context(), norm, limit)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "folk lookup failed")
				return
			}
			terms := make([]vocab.TermRef, 0, len(names))
			for _, name := range names {
				terms = append(terms, vocab.TermRef{Scheme: vocab.FolkScheme, ID: name, Label: name})
			}
			writeJSON(w, http.StatusOK, map[string]any{"terms": terms})
			return
		}
		terms := ix.Search(scheme, q, limit)
		if terms == nil {
			terms = []*vocab.Term{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"terms": terms})
	})
}
