package vocabsrc

import (
	"context"
	"slices"
	"sort"

	"github.com/freeeve/libcat/bibframe"
	"github.com/freeeve/libcat/ingest"

	"github.com/freeeve/libcat/backend/vocab"
)

// Crosswalk confidence by link strength: exactMatch is definitional
// identity, closeMatch (how homosaurus links most LCSH headings) is
// near-equivalence, and a pivot (two terms linking the same intermediate
// URI -- the FAST -> LCSH <- Homosaurus shape) is only as good as its
// weakest hop -- and pivot-close sits well below pivot-exact so a guard
// demotion is VISIBLE in the queue, not folded into a near-identical number.
const (
	confExactMatch = 1.0
	confCloseMatch = 0.85
	confPivotExact = 0.8
	confPivotClose = 0.6
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
		// One enrichment PER CONFIDENCE TIER, not per work: an Enrichment
		// carries a single confidence, and folding a work's candidates into
		// one (the old min) erased exactly the distinction the pivot guards
		// compute -- a demoted "Womyn" queued at the same number as its
		// matched "Women" counterpart. Tiers keep each candidate's own
		// strength visible in the moderation queue.
		byConf := map[float64]*ingest.Enrichment{}
		seen := map[string]bool{}
		tier := func(confidence float64) *ingest.Enrichment {
			enr := byConf[confidence]
			if enr == nil {
				enr = &ingest.Enrichment{WorkID: work.WorkID, Confidence: confidence}
				byConf[confidence] = enr
			}
			return enr
		}
		for _, uri := range work.Subjects {
			src, ok := e.Index.Resolve(uri)
			if !ok || src.Scheme == e.Target {
				continue
			}
			origin := src.Label("en") + " (" + src.Scheme + ")"
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
				enr := tier(confidence)
				enr.Subjects = append(enr.Subjects, bibframe.AuthoritySubject{
					URI: target.ID, Labels: target.Labels, Broader: target.Broader,
				})
				enr.Origins = append(enr.Origins, origin)
				for _, a := range e.Index.Ancestors(e.Target, target.ID) {
					if seen[a.ID] {
						continue
					}
					seen[a.ID] = true
					enr.Terms = append(enr.Terms, bibframe.AuthoritySubject{
						URI: a.ID, Labels: a.Labels, Broader: a.Broader,
					})
				}
			}
		}
		// Strongest tier first, deterministically.
		confs := make([]float64, 0, len(byConf))
		for c := range byConf {
			confs = append(confs, c)
		}
		sort.Sort(sort.Reverse(sort.Float64Slice(confs)))
		for _, c := range confs {
			out = append(out, *byConf[c])
		}
	}
	return out, nil
}
