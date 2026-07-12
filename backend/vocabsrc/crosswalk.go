package vocabsrc

import (
	"context"
	"slices"

	"github.com/freeeve/libcat/bibframe"
	"github.com/freeeve/libcat/ingest"

	"github.com/freeeve/libcat/backend/vocab"
)

// Crosswalk confidence by link strength: exactMatch is definitional
// identity, closeMatch (how homosaurus links most LCSH headings) is
// near-equivalence, and a pivot (two terms linking the same intermediate
// URI -- the FAST -> LCSH <- Homosaurus shape) is only as good as its
// weakest hop.
const (
	confExactMatch = 1.0
	confCloseMatch = 0.85
	confPivotExact = 0.8
	confPivotClose = 0.7
)

// strengthConfidence maps an equivalent's strength to its suggestion
// confidence; unknown strengths suggest nothing.
var strengthConfidence = map[string]float64{
	"exact":       confExactMatch,
	"close":       confCloseMatch,
	"pivot-exact": confPivotExact,
	"pivot-close": confPivotClose,
}

// CrosswalkEnricher resolves every work subject's cross-scheme equivalents
// into one target scheme: direct skos:exactMatch/closeMatch links in either
// direction, plus one-hop pivots through a shared intermediate URI -- so a
// FAST-cataloged work reaches Homosaurus through the LCSH heading both
// vocabularies link, with neither LCSH loaded nor any direct FAST-Homosaurus
// edge. Purely local (the pivot URI need not resolve); suggestions are
// moderated.
type CrosswalkEnricher struct {
	Index *vocab.Index
	// Target is the scheme equivalents resolve into.
	Target string
}

// NewCrosswalk wraps the index as a crosswalk enrichment source for one
// target scheme.
func NewCrosswalk(ix *vocab.Index, target string) *CrosswalkEnricher {
	return &CrosswalkEnricher{Index: ix, Target: target}
}

// Name implements ingest.Enricher; the registry key and enrichment graph.
func (e *CrosswalkEnricher) Name() string { return "crosswalk-" + e.Target }

// Enrich implements ingest.Enricher: for each work, every controlled
// subject's match links resolving in the target scheme (and not already on
// the work) become subject candidates, and each candidate's skos:broader
// ancestor chain rides along as standalone term metadata so
// hierarchy nodes stay labeled without a work carrying them.
func (e *CrosswalkEnricher) Enrich(_ context.Context, works []ingest.WorkSummary) ([]ingest.Enrichment, error) {
	var out []ingest.Enrichment
	for _, work := range works {
		enrichment := ingest.Enrichment{WorkID: work.WorkID, Confidence: 1}
		seen := map[string]bool{}
		for _, uri := range work.Subjects {
			if term, ok := e.Index.Resolve(uri); !ok || term.Scheme == e.Target {
				continue
			}
			eqs, ok := e.Index.Equivalents(uri)
			if !ok {
				continue
			}
			for _, eq := range eqs {
				// Only terms the target scheme actually holds suggest; a
				// link target outside any loaded vocabulary (the pivot URI
				// itself, typically LCSH) is a bridge, not a candidate.
				if !eq.Known || eq.Scheme != e.Target {
					continue
				}
				confidence, graded := strengthConfidence[eq.Strength]
				if !graded || seen[eq.ID] || slices.Contains(work.Subjects, eq.ID) {
					continue
				}
				target, ok := e.Index.Lookup(e.Target, eq.ID)
				if !ok || target.MergedInto != "" {
					continue
				}
				seen[eq.ID] = true
				enrichment.Subjects = append(enrichment.Subjects, bibframe.AuthoritySubject{
					URI: target.ID, Labels: target.Labels, Broader: target.Broader,
				})
				for _, a := range e.Index.Ancestors(e.Target, target.ID) {
					if seen[a.ID] {
						continue
					}
					seen[a.ID] = true
					enrichment.Terms = append(enrichment.Terms, bibframe.AuthoritySubject{
						URI: a.ID, Labels: a.Labels, Broader: a.Broader,
					})
				}
				if confidence < enrichment.Confidence {
					enrichment.Confidence = confidence
				}
			}
		}
		if len(enrichment.Subjects) > 0 {
			out = append(out, enrichment)
		}
	}
	return out, nil
}
