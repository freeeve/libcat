package httpapi

import (
	"encoding/json"
	"maps"
	"net/http"
	"sort"
	"strings"

	"github.com/freeeve/libcatalog/bibframe"
	"github.com/freeeve/libcatalog/identity"
	"github.com/freeeve/libcatalog/ingest"
	"github.com/freeeve/libcatalog/storage/blob"

	"github.com/freeeve/libcatalog/backend/auth"
	"github.com/freeeve/libcatalog/backend/copycat"
	"github.com/freeeve/libcatalog/backend/marcview"
	"github.com/freeeve/libcatalog/backend/vocab"
)

// subjectCandidate is one external heading a cataloger can pull in: the
// heading text, its MARC tag and source vocabulary, how many target records
// carried it, and -- when it whole-heading-matches a loaded vocabulary --
// the controlled term to add instead of a tag.
type subjectCandidate struct {
	Heading string         `json:"heading"`
	Tag     string         `json:"tag"`
	Source  string         `json:"source,omitempty"`
	Count   int            `json:"count"`
	Targets []string       `json:"targets"`
	Term    *vocab.TermRef `json:"term,omitempty"`
}

// registerSubjectLookup mounts the tasks/073 external-subject fetch: the
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
		grain, _, err := bs.Get(r.Context(), bibframe.GrainPath(workID))
		if err != nil {
			writeError(w, http.StatusNotFound, "no such work")
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
				existingLabels[normHeading(tag)] = true
			}
		}
		if len(isbns) > 5 {
			isbns = isbns[:5]
		}
		if len(isbns) == 0 {
			writeError(w, http.StatusBadRequest, "this work carries no ISBNs to search by")
			return
		}
		byKey := map[string]*subjectCandidate{}
		failures := map[string]string{}
		for _, isbn := range isbns {
			results, fails, err := cc.SearchAll(r.Context(), isbn, req.Targets)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			maps.Copy(failures, fails)
			for _, res := range results {
				collectSubjects(byKey, res.Target, res.Record)
			}
		}
		candidates := make([]subjectCandidate, 0, len(byKey))
		for _, c := range byKey {
			if existingLabels[normHeading(c.Heading)] {
				continue
			}
			if ix != nil {
				c.Term = reconcileHeading(ix, c.Heading)
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
		writeJSON(w, http.StatusOK, map[string]any{"candidates": candidates, "failures": failures})
	})))

	// Identifier kinds (tasks/073): each bf:identifiedBy value mapped to its
	// BIBFRAME type so the editor can badge ISBN vs ISSN vs provider id.
	mux.Handle("GET /v1/works/{id}/identifiers", librarian(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		workID := r.PathValue("id")
		if !workIDPattern.MatchString(workID) {
			writeError(w, http.StatusBadRequest, "bad work id")
			return
		}
		grain, _, err := bs.Get(r.Context(), bibframe.GrainPath(workID))
		if err != nil {
			writeError(w, http.StatusNotFound, "no such work")
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

// subjectTags are the MARC 6XX fields worth harvesting; ind2 names the
// source vocabulary (7 defers to $2).
var subjectTags = map[string]bool{
	"600": true, "610": true, "611": true, "630": true,
	"648": true, "650": true, "651": true, "655": true,
}

var ind2Sources = map[string]string{"0": "lcsh", "1": "lcshac", "2": "mesh", "5": "cash", "6": "rvm"}

func collectSubjects(byKey map[string]*subjectCandidate, target string, rec marcview.RecordDoc) {
	for _, f := range rec.Fields {
		if !subjectTags[f.Tag] {
			continue
		}
		heading, source := headingOf(f)
		if heading == "" {
			continue
		}
		key := f.Tag + "|" + normHeading(heading)
		c := byKey[key]
		if c == nil {
			c = &subjectCandidate{Heading: heading, Tag: f.Tag, Source: source}
			byKey[key] = c
		}
		c.Count++
		found := false
		for _, t := range c.Targets {
			if t == target {
				found = true
			}
		}
		if !found {
			c.Targets = append(c.Targets, target)
		}
	}
}

// headingOf joins a 6XX field into a display heading: name/title subfields
// space-joined, subdivisions ($v$x$y$z) double-dash-joined, trailing
// punctuation trimmed. Returns the heading and its source vocabulary.
func headingOf(f marcview.Field) (string, string) {
	var main []string
	var subs []string
	source := ind2Sources[f.Ind2]
	for _, sf := range f.Subfields {
		switch sf.Code {
		case "a", "b", "c", "d", "t":
			main = append(main, strings.TrimSpace(sf.Value))
		case "v", "x", "y", "z":
			subs = append(subs, strings.TrimSpace(sf.Value))
		case "2":
			if source == "" {
				source = sf.Value
			}
		}
	}
	heading := strings.Join(main, " ")
	if len(subs) > 0 {
		heading += "--" + strings.Join(subs, "--")
	}
	return strings.TrimRight(strings.TrimSpace(heading), ".,"), source
}

func normHeading(s string) string {
	return strings.TrimSuffix(strings.Join(strings.Fields(strings.ToLower(s)), " "), ".")
}

// reconcileHeading whole-heading-matches against every loaded scheme: the
// full heading first, then its pre-subdivision head.
func reconcileHeading(ix *vocab.Index, heading string) *vocab.TermRef {
	tries := []string{heading}
	if head, _, ok := strings.Cut(heading, "--"); ok {
		tries = append(tries, head)
	}
	for _, try := range tries {
		for _, scheme := range ix.Schemes() {
			for _, m := range ix.MatchLabel(scheme, try) {
				if m.Term.MergedInto == "" {
					return &vocab.TermRef{Scheme: scheme, ID: m.Term.ID, Label: m.Term.Label("en")}
				}
			}
		}
	}
	return nil
}
