package httpapi

import (
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strings"

	"github.com/freeeve/libcat/diversity"
	"github.com/freeeve/libcat/ingest"
	"github.com/freeeve/libcat/project"

	"github.com/freeeve/libcat/backend/auth"
	"github.com/freeeve/libcat/backend/workindex"
)

// auditResponse is the diversity report plus what it was computed over, matching
// the `lcat audit` JSON shape so a saved report reads the same either way. The
// creators block aggregates the cached wikidata claims when any exist.
type auditResponse struct {
	Input string `json:"input"`
	Scope string `json:"scope,omitempty"`
	diversity.Report
	Creators *creatorAudit `json:"creators,omitempty"`
}

// creatorAudit is the aggregate creator-demographics report: match rate first,
// then per-property value distributions over DISTINCT resolved creators. It
// never names a person -- distributions and counts only.
type creatorAudit struct {
	// TotalWorks mirrors the content report's denominator; MatchedWorks is
	// how many carry at least one resolved creator identity.
	TotalWorks   int     `json:"totalWorks"`
	MatchedWorks int     `json:"matchedWorks"`
	MatchRate    float64 `json:"matchRate"`
	// ResolvedCreators is the number of distinct creator entities.
	ResolvedCreators int               `json:"resolvedCreators"`
	Properties       []creatorProperty `json:"properties"`
}

// creatorProperty is one demographic property's distribution. Unknown is the
// resolved creators with NO stated value for this property -- reported
// alongside every distribution because it is usually the honest majority.
type creatorProperty struct {
	Property string         `json:"property"`
	Label    string         `json:"label"`
	Known    int            `json:"known"`
	Unknown  int            `json:"unknown"`
	Values   []creatorValue `json:"values,omitempty"`
}

// creatorValue is one claim value and how many distinct creators carry it.
type creatorValue struct {
	Label    string `json:"label"`
	QID      string `json:"qid"`
	Creators int    `json:"creators"`
}

// creatorPropLabels names the audited properties for display.
var creatorPropLabels = []struct{ id, label string }{
	{"P21", "Sex or gender"},
	{"P27", "Country of citizenship"},
	{"P91", "Sexual orientation"},
	{"P172", "Ethnic group"},
}

// aggregateCreators folds the summaries' cached creator claims into the
// aggregate report; nil when the corpus carries no creator data at all (the
// source is opt-in, and "not enabled" must read differently from "0% match").
func aggregateCreators(sums []ingest.WorkSummary, include func(*ingest.WorkSummary) bool) *creatorAudit {
	ca := &creatorAudit{}
	creators := map[string]map[string][]string{} // QID -> property -> value QIDs
	valueLabels := map[string]string{}
	for i := range sums {
		s := &sums[i]
		if !include(s) {
			continue
		}
		ca.TotalWorks++
		if len(s.Creators) == 0 {
			continue
		}
		ca.MatchedWorks++
		for _, c := range s.Creators {
			props := creators[c.QID]
			if props == nil {
				props = map[string][]string{}
				creators[c.QID] = props
			}
			for _, cl := range c.Claims {
				if !slices.Contains(props[cl.Property], cl.ValueQID) {
					props[cl.Property] = append(props[cl.Property], cl.ValueQID)
				}
				if cl.ValueLabel != "" {
					valueLabels[cl.ValueQID] = cl.ValueLabel
				}
			}
		}
	}
	if len(creators) == 0 {
		return nil
	}
	ca.ResolvedCreators = len(creators)
	if ca.TotalWorks > 0 {
		ca.MatchRate = float64(ca.MatchedWorks) / float64(ca.TotalWorks)
	}
	for _, p := range creatorPropLabels {
		cp := creatorProperty{Property: p.id, Label: p.label}
		counts := map[string]int{}
		for _, props := range creators {
			vals := props[p.id]
			if len(vals) == 0 {
				continue
			}
			cp.Known++
			for _, v := range vals {
				counts[v]++
			}
		}
		cp.Unknown = ca.ResolvedCreators - cp.Known
		for qid, n := range counts {
			label := valueLabels[qid]
			if label == "" {
				label = qid
			}
			cp.Values = append(cp.Values, creatorValue{Label: label, QID: qid, Creators: n})
		}
		sort.Slice(cp.Values, func(i, j int) bool {
			if cp.Values[i].Creators != cp.Values[j].Creators {
				return cp.Values[i].Creators > cp.Values[j].Creators
			}
			return cp.Values[i].Label < cp.Values[j].Label
		})
		ca.Properties = append(ca.Properties, cp)
	}
	return ca
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
		include := func(s *ingest.WorkSummary) bool {
			return !s.Tombstoned && filters.match(s.Extras)
		}
		for i := range sums {
			s := &sums[i]
			if !include(s) {
				continue
			}
			a.Add(summaryRefs(s))
		}
		writeJSON(w, http.StatusOK, auditResponse{
			Input:    "work index (cataloging corpus: suppressed included, tombstoned excluded)",
			Scope:    filters.String(),
			Report:   a.Report(),
			Creators: aggregateCreators(sums, include),
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
