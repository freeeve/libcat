// Package enrich executes enrichment sources against the deployment's grain
// store in one of two modes: direct (auto-approve -- assertions land in the
// source's enrichment:<name> graph) or queue (candidates become
// PIPELINE-provenance suggestions for moderation). The mode is a per-source
// deployment decision; the enrichers themselves are mode-blind.
package enrich

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/freeeve/libcat/ingest"
	"github.com/freeeve/libcat/project"
	"github.com/freeeve/libcat/storage/blob"

	"github.com/freeeve/libcat/backend/store"
	"github.com/freeeve/libcat/backend/suggest"
	"github.com/freeeve/libcat/backend/vocab"
)

// Run error classes, so the HTTP surface can map failure cause to status:
// the caller's mistake (ErrUnknownSource), the deployment's mistake
// (ErrMisconfigured), the upstream service's fault (ingest.ErrEnricher), or
// -- unwrapped -- a storage fault.
var (
	// ErrUnknownSource names a source the deployment has not configured.
	ErrUnknownSource = errors.New("unknown enrichment source")
	// ErrMisconfigured marks a source whose configuration cannot run
	// (invalid mode, queue mode without the suggestion service).
	ErrMisconfigured = errors.New("enrichment source misconfigured")
)

// Mode selects how a source's results land.
type Mode string

const (
	// ModeQueue routes candidates through moderation (the approval gate).
	ModeQueue Mode = "queue"
	// ModeDirect writes the source's enrichment graph outright
	// (auto-approve on import).
	ModeDirect Mode = "direct"
)

// Source pairs an enricher with its deployment mode.
type Source struct {
	Enricher ingest.Enricher
	Mode     Mode
	// Scheme keys the queued TermRefs (e.g. "lcsh"); queue mode only.
	Scheme string
}

// Service runs configured sources.
type Service struct {
	Blob        blob.Store
	GrainPrefix string
	Queue       *suggest.Service
	Sources     map[string]Source
	// Summaries, when set, is the shared maintained summary source
	// (workindex) queue-mode runs read instead of a per-run
	// corpus walk; nil falls back to ScanSummaries.
	Summaries ingest.SummarySource
	// DB, when set, enables the async job surface (jobs.go): kick returns a
	// job id, a worker drains, GET polls live progress. Nil keeps runs
	// synchronous-only.
	DB store.Store
	// Now overrides the job clock (tests).
	Now func() time.Time
}

// Result summarizes one run.
type Result struct {
	Source string `json:"source"`
	Mode   Mode   `json:"mode"`
	// Works is the number of Works enriched (direct) or with candidates
	// queued (queue).
	Works int `json:"works"`
	// Scope names the run's filter ("" when the whole corpus).
	Scope string `json:"scope,omitempty"`
	// Stats carries the enricher's own run counters (batches, skips,
	// resolved, elapsed) when the source reports them.
	Stats *ingest.EnrichStats `json:"stats,omitempty"`
}

// Run executes one configured source by name. A non-nil keep scopes the run
// to the summaries it accepts: only those works are handed to the enricher
// (an external-service source queries for exactly the scoped set) and only
// their grains gain statements; out-of-scope works keep what they have.
func (s *Service) Run(ctx context.Context, name string, keep func(*ingest.WorkSummary) bool) (Result, error) {
	src, ok := s.Sources[name]
	if !ok {
		return Result{}, fmt.Errorf("%w: %q", ErrUnknownSource, name)
	}
	stats := func() *ingest.EnrichStats {
		if sr, ok := src.Enricher.(ingest.StatsReporter); ok {
			st := sr.RunStats()
			return &st
		}
		return nil
	}
	switch src.Mode {
	case ModeDirect:
		n, err := ingest.RunEnrichScoped(ctx, s.Blob, s.GrainPrefix, src.Enricher, keep)
		return Result{Source: name, Mode: src.Mode, Works: n, Stats: stats()}, err
	case ModeQueue:
		n, err := s.runQueued(ctx, src, keep)
		return Result{Source: name, Mode: src.Mode, Works: n, Stats: stats()}, err
	}
	return Result{}, fmt.Errorf("%w: source %q has invalid mode %q", ErrMisconfigured, name, src.Mode)
}

// Names lists the configured sources.
func (s *Service) Names() []string {
	names := make([]string, 0, len(s.Sources))
	for name := range s.Sources {
		names = append(names, name)
	}
	return names
}

func (s *Service) runQueued(ctx context.Context, src Source, keep func(*ingest.WorkSummary) bool) (int, error) {
	if s.Queue == nil {
		return 0, fmt.Errorf("%w: queue mode needs the suggestion service", ErrMisconfigured)
	}
	summaries, _, err := ingest.SummariesOf(ctx, s.Summaries, s.Blob, s.GrainPrefix)
	if err != nil {
		return 0, err
	}
	if keep != nil {
		kept := summaries[:0:0]
		for i := range summaries {
			if keep(&summaries[i]) {
				kept = append(kept, summaries[i])
			}
		}
		summaries = kept
	}
	results, err := src.Enricher.Enrich(ctx, summaries)
	if err != nil {
		return 0, fmt.Errorf("%w: %w", ingest.ErrEnricher, err)
	}
	queued := 0
	for _, res := range results {
		landed := false
		for _, subj := range res.Subjects {
			label := subj.URI
			if l := vocab.PickLabel(subj.Labels); l != "" {
				label = l
			}
			// A single-vocabulary source names its scheme once; a source
			// spanning vocabularies (SRU subject harvest) leaves it empty
			// and each term's scheme derives from its URI namespace.
			scheme := src.Scheme
			if scheme == "" {
				scheme = project.SchemeForURI(subj.URI)
			}
			term := vocab.TermRef{Scheme: scheme, ID: subj.URI, Label: label}
			err := s.Queue.PipelineSuggest(ctx, res.WorkID, term, res.Confidence)
			if err != nil {
				return queued, err
			}
			landed = true
		}
		if landed {
			queued++
		}
	}
	return queued, nil
}
