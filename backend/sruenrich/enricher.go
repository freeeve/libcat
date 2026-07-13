package sruenrich

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/freeeve/libcat/bibframe"
	"github.com/freeeve/libcat/ingest"

	"github.com/freeeve/libcat/backend/copycat"
	"github.com/freeeve/libcat/backend/vocab"
)

// Name is the enrichment source name the run endpoints address.
const Name = "sru-subjects"

// Suggestion confidence by match kind, mirroring the auto-linker's scale:
// an identifier match is near-certain, a whole-heading label match is
// likelier to need review.
const (
	confIDMatch    = 0.9
	confLabelMatch = 0.75
)

// maxISBNsPerWork bounds the target fan-out one work costs -- quota hygiene
// against shared public SRU endpoints.
const maxISBNsPerWork = 2

// Searcher is the copycat seam: fan one fielded query out to the configured
// targets. *copycat.Service satisfies it.
type Searcher interface {
	SearchAll(ctx context.Context, query string, fields []copycat.FieldTerm, names []string) ([]copycat.SearchResult, map[string]string, map[string]string, error)
}

// Enricher asks the copycat targets what a work is about: ISBN Bath-profile
// searches, 6XX extraction, reconciliation against the loaded vocabularies,
// and ONLY reconciled controlled terms come back -- as moderated
// suggestions, never direct writes (queue mode). Works without an ISBN are
// skipped; a work's existing subjects and tags never re-suggest.
type Enricher struct {
	Search Searcher
	Vocab  *vocab.Index
	// Targets restricts the fan-out to named targets (nil = all configured).
	Targets []string
	// Delay is the politeness pause between per-work queries.
	Delay time.Duration
	Log   *slog.Logger

	statsMu sync.Mutex
	stats   ingest.EnrichStats
}

// RunStats implements ingest.StatsReporter; safe to poll mid-run. Batches
// counts target queries, SkippedBatches works skipped (no ISBN) plus
// queries every target failed.
func (e *Enricher) RunStats() ingest.EnrichStats {
	e.statsMu.Lock()
	defer e.statsMu.Unlock()
	return e.stats
}

func (e *Enricher) setStats(st ingest.EnrichStats) {
	e.statsMu.Lock()
	e.stats = st
	e.statsMu.Unlock()
}

// Name implements ingest.Enricher.
func (e *Enricher) Name() string { return Name }

// Enrich implements ingest.Enricher over a batch of scoped works.
func (e *Enricher) Enrich(ctx context.Context, works []ingest.WorkSummary) ([]ingest.Enrichment, error) {
	started := time.Now()
	var st ingest.EnrichStats
	publish := func() {
		st.ElapsedMS = time.Since(started).Milliseconds()
		e.setStats(st)
	}
	publish()

	var out []ingest.Enrichment
	first := true
	for i := range works {
		w := &works[i]
		isbns := w.ISBNs
		if len(isbns) > maxISBNsPerWork {
			isbns = isbns[:maxISBNsPerWork]
		}
		if len(isbns) == 0 {
			st.SkippedBatches++
			publish()
			continue
		}
		if !first && e.Delay > 0 {
			select {
			case <-ctx.Done():
				return out, ctx.Err()
			case <-time.After(e.Delay):
			}
		}
		first = false

		existingSubjects := map[string]bool{}
		for _, s := range w.Subjects {
			existingSubjects[s] = true
		}
		existingLabels := map[string]bool{}
		for _, tag := range w.Tags {
			existingLabels[NormHeading(tag)] = true
		}
		for _, h := range w.Headings {
			existingLabels[NormHeading(h)] = true
		}

		byKey := map[string]*Candidate{}
		answered := false
		for _, isbn := range isbns {
			if ctx.Err() != nil {
				return out, ctx.Err()
			}
			results, failures, _, err := e.Search.SearchAll(ctx, "", []copycat.FieldTerm{{Index: "isbn", Term: isbn}}, e.Targets)
			st.Batches++
			if err != nil {
				// A malformed query is ours, not weather; surface it.
				return out, err
			}
			if len(results) > 0 || len(failures) == 0 {
				answered = true
			}
			for name, msg := range failures {
				if e.Log != nil {
					e.Log.Warn("sru subject target failed", "work", w.WorkID, "target", name, "err", msg)
				}
			}
			for _, res := range results {
				Collect(byKey, res.Target, res.Record)
			}
			publish()
		}
		if !answered {
			st.SkippedBatches++
			publish()
			continue
		}

		var idMatched, labelMatched []bibframe.AuthoritySubject
		var idOrigins, labelOrigins []string
		for _, c := range byKey {
			if existingLabels[NormHeading(c.Heading)] {
				continue
			}
			term := ReconcileIdentifiers(e.Vocab, c.IDs)
			byID := term != nil
			if term == nil {
				term = ReconcileHeading(e.Vocab, c.Heading)
			}
			if term == nil || existingSubjects[term.ID] {
				continue
			}
			subj := bibframe.AuthoritySubject{URI: term.ID, Labels: map[string]string{"en": term.Label}}
			if byID {
				idMatched = append(idMatched, subj)
				idOrigins = append(idOrigins, "heading "+c.Heading)
			} else {
				labelMatched = append(labelMatched, subj)
				labelOrigins = append(labelOrigins, "heading "+c.Heading)
			}
		}
		if len(idMatched) > 0 {
			out = append(out, ingest.Enrichment{WorkID: w.WorkID, Confidence: confIDMatch, Subjects: idMatched, Origins: idOrigins})
		}
		if len(labelMatched) > 0 {
			out = append(out, ingest.Enrichment{WorkID: w.WorkID, Confidence: confLabelMatch, Subjects: labelMatched, Origins: labelOrigins})
		}
		if e.Log != nil && len(idMatched)+len(labelMatched) > 0 {
			e.Log.Info("sru subjects suggested", "work", w.WorkID,
				"byIdentifier", len(idMatched), "byLabel", len(labelMatched))
		}
	}
	publish()
	return out, nil
}
