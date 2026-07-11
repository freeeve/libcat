package httpapi

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/freeeve/libcat/diversity"
	"github.com/freeeve/libcat/ingest"
	"github.com/freeeve/libcat/project"

	"github.com/freeeve/libcat/backend/auth"
	"github.com/freeeve/libcat/backend/workindex"
)

// auditResponse is the diversity report plus what it was computed over, matching
// the `lcat audit` JSON shape so a saved report reads the same either way.
type auditResponse struct {
	Input string `json:"input"`
	Scope string `json:"scope,omitempty"`
	diversity.Report
}

// registerAudit serves the content-diversity audit over the live work index
// : the same coverage-first report `lcat audit` computes, but against
// the cataloging corpus the editor sees -- suppressed works included (they are
// held, just not published), tombstoned works excluded (they are retired).
// Aggregation over the in-memory summaries is O(corpus) string matching and runs
// per request; at works-list scale that is milliseconds, and the index owns
// freshness.
//
// Query: filter=key=value (repeatable, ANDed; comma-joined extras match per
// element) and source=<name>, both matching the summaries' Extras -- the same
// semantics as `lcat audit --filter/--source`.
func registerAudit(mux *http.ServeMux, ix *workindex.Index, verifier auth.TokenVerifier) {
	librarian := auth.Require(verifier, auth.RoleLibrarian)

	mux.Handle("GET /v1/audit/diversity", librarian(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filters, err := auditFilters(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		sums, err := ix.Summaries(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "scan failed")
			return
		}
		cw := diversity.Default()
		a := diversity.NewAuditor(cw)
		for i := range sums {
			s := &sums[i]
			if s.Tombstoned || !filters.match(s.Extras) {
				continue
			}
			a.Add(summaryRefs(s))
		}
		writeJSON(w, http.StatusOK, auditResponse{
			Input:  "work index (cataloging corpus: suppressed included, tombstoned excluded)",
			Scope:  filters.String(),
			Report: a.Report(),
		})
	})))
}

// auditFilters parses ?filter=k=v (repeatable) and ?source=<name> into the same
// ANDed filter set the CLI uses.
func auditFilters(r *http.Request) (auditFilterSet, error) {
	var out auditFilterSet
	q := r.URL.Query()
	for _, raw := range q["filter"] {
		k, v, ok := strings.Cut(raw, "=")
		if !ok || k == "" || v == "" {
			return nil, fmt.Errorf("filter wants key=value, got %q", raw)
		}
		out = append(out, [2]string{k, v})
	}
	if s := q.Get("source"); s != "" {
		out = append(out, [2]string{"sources", s})
	}
	return out, nil
}

// auditFilterSet is the endpoint's ANDed extras filters.
type auditFilterSet [][2]string

// String renders the active filters for the response's scope field.
func (f auditFilterSet) String() string {
	parts := make([]string, 0, len(f))
	for _, p := range f {
		parts = append(parts, p[0]+"="+p[1])
	}
	return strings.Join(parts, " AND ")
}

// match reports whether a summary's extras satisfy every filter; a comma-joined
// extra (the sources convention) matches on any element.
func (f auditFilterSet) match(extra map[string]string) bool {
	for _, p := range f {
		got, ok := extra[p[0]]
		if !ok {
			return false
		}
		if got == p[1] {
			continue
		}
		found := false
		for _, part := range strings.Split(got, ",") {
			if strings.TrimSpace(part) == p[1] {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// summaryRefs turns a work summary's aboutness signal into audit refs: controlled
// subject IRIs (scheme from the URI namespace), their heading labels, and the
// uncontrolled tags -- the same three dimensions the CLI's json and graph inputs
// feed, so all three surfaces measure the same thing.
func summaryRefs(s *ingest.WorkSummary) []diversity.SubjectRef {
	refs := make([]diversity.SubjectRef, 0, len(s.Subjects)+len(s.Headings)+len(s.Tags))
	for _, uri := range s.Subjects {
		refs = append(refs, diversity.SubjectRef{URI: uri, Scheme: project.SchemeForURI(uri)})
	}
	for _, h := range s.Headings {
		refs = append(refs, diversity.SubjectRef{Labels: []string{h}})
	}
	for _, t := range s.Tags {
		refs = append(refs, diversity.SubjectRef{Labels: []string{t}})
	}
	return refs
}
