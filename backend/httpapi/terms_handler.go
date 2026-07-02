package httpapi

import (
	"net/http"
	"strconv"

	"github.com/freeeve/libcatalog/backend/vocab"
)

const termsDefaultLimit = 20

// registerTerms mounts the public autocomplete endpoint over the vocabulary
// index. Accepted folk terms join these results when the suggestion service
// lands (tasks/033).
func registerTerms(mux *http.ServeMux, ix *vocab.Index) {
	mux.HandleFunc("GET /v1/terms", func(w http.ResponseWriter, r *http.Request) {
		scheme := r.URL.Query().Get("scheme")
		q := r.URL.Query().Get("q")
		if scheme == "" {
			writeJSON(w, http.StatusOK, map[string]any{"schemes": ix.Schemes()})
			return
		}
		limit := termsDefaultLimit
		if raw := r.URL.Query().Get("limit"); raw != "" {
			if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 100 {
				limit = n
			}
		}
		terms := ix.Search(scheme, q, limit)
		if terms == nil {
			terms = []*vocab.Term{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"terms": terms})
	})
}
