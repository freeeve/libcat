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
	"strings"
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
	// MaxParallel caps how many jobs a single drain runs at once across
	// distinct sources; 0 is unlimited (one per queued source). The drain
	// never runs two jobs of the SAME source concurrently regardless.
	MaxParallel int
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
	// Suggestions is the exact count of NEW queue rows this run created
	// (queue mode): pairs already suggested, tombstoned, or resolved are
	// silent no-ops and do not count.
	Suggestions int `json:"suggestions,omitempty"`
	// Scope names the run's filter ("" when the whole corpus).
	Scope string `json:"scope,omitempty"`
	// Stats carries the enricher's own run counters (batches, skips,
	// resolved, elapsed) when the source reports them.
	Stats *ingest.EnrichStats `json:"stats,omitempty"`
}

// Describer is an optional Enricher capability: Describe names what the
// source talks to, for humans -- the bibliocommons peer subdomains, a
// crosswalk's target vocabulary -- so a job card can say which library a
// three-hour run is pulling from.
type Describer interface {
	Describe() string
}

// HostScoped is an optional Enricher capability: ForHosts returns a
// per-run view of the enricher scoped to the given peer hosts (the
// bibliocommons harvest), so one job can sweep a different peer list
// without a restart.
type HostScoped interface {
	ForHosts(hosts []string) ingest.Enricher
}

// ErrValidation reports a caller mistake in a run/job request (bad host,
// hosts on a source that takes none); the HTTP layer answers 400.
var ErrValidation = errors.New("enrich: invalid request")

// Run executes one configured source by name. A non-nil keep scopes the run
// to the summaries it accepts: only those works are handed to the enricher
// (an external-service source queries for exactly the scoped set) and only
// their grains gain statements; out-of-scope works keep what they have.
func (s *Service) Run(ctx context.Context, name string, keep func(*ingest.WorkSummary) bool) (Result, error) {
	return s.RunHosted(ctx, name, keep, nil)
}

// RunHosted is Run with an optional per-run peer-host override for sources
// that take one.
func (s *Service) RunHosted(ctx context.Context, name string, keep func(*ingest.WorkSummary) bool, hosts []string) (Result, error) {
	src, ok := s.Sources[name]
	if !ok {
		return Result{}, fmt.Errorf("%w: %q", ErrUnknownSource, name)
	}
	enr, err := s.scopedEnricher(src, name, hosts)
	if err != nil {
		return Result{}, err
	}
	stats := func() *ingest.EnrichStats {
		if sr, ok := enr.(ingest.StatsReporter); ok {
			st := sr.RunStats()
			return &st
		}
		return nil
	}
	switch src.Mode {
	case ModeDirect:
		n, err := ingest.RunEnrichScoped(ctx, s.Blob, s.GrainPrefix, enr, keep)
		return Result{Source: name, Mode: src.Mode, Works: n, Stats: stats()}, err
	case ModeQueue:
		n, sugg, err := s.runQueued(ctx, name, src, enr, keep)
		return Result{Source: name, Mode: src.Mode, Works: n, Suggestions: sugg, Stats: stats()}, err
	}
	return Result{}, fmt.Errorf("%w: source %q has invalid mode %q", ErrMisconfigured, name, src.Mode)
}

// scopedEnricher resolves the run's enricher: the configured one, or a
// per-run host-scoped view when hosts are named and the source takes them.
func (s *Service) scopedEnricher(src Source, name string, hosts []string) (ingest.Enricher, error) {
	if len(hosts) == 0 {
		return src.Enricher, nil
	}
	hs, ok := src.Enricher.(HostScoped)
	if !ok {
		return nil, fmt.Errorf("%w: source %q does not take hosts", ErrValidation, name)
	}
	return hs.ForHosts(hosts), nil
}

// Targets maps each configured source to its human descriptor (what it
// talks to), for the sources that expose one.
func (s *Service) Targets() map[string]string {
	out := map[string]string{}
	for name, src := range s.Sources {
		if d, ok := src.Enricher.(Describer); ok {
			if t := d.Describe(); t != "" {
				out[name] = t
			}
		}
	}
	return out
}

// Names lists the configured sources.
func (s *Service) Names() []string {
	names := make([]string, 0, len(s.Sources))
	for name := range s.Sources {
		names = append(names, name)
	}
	return names
}

func (s *Service) runQueued(ctx context.Context, name string, src Source, enricher ingest.Enricher, keep func(*ingest.WorkSummary) bool) (int, int, error) {
	if s.Queue == nil {
		return 0, 0, fmt.Errorf("%w: queue mode needs the suggestion service", ErrMisconfigured)
	}
	summaries, _, err := ingest.SummariesOf(ctx, s.Summaries, s.Blob, s.GrainPrefix)
	if err != nil {
		return 0, 0, err
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
	results, err := enricher.Enrich(ctx, summaries)
	if err != nil {
		return 0, 0, fmt.Errorf("%w: %w", ingest.ErrEnricher, err)
	}
	queued, suggestions := 0, 0
	for _, res := range results {
		landed := false
		for si, subj := range res.Subjects {
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
			// Every machine row says where it came from: "<source>" or
			// "<source>: <origin>" (the tag/heading/subject that produced
			// the candidate). A subject arriving with an endorsement
			// additionally carries peer consensus -- the supporter count
			// ranks it, the sources say which libraries corroborated, and
			// each attribution is the moderator's verifiable evidence.
			sourceRef := name
			if si < len(res.Origins) && res.Origins[si] != "" {
				sourceRef = name + ": " + res.Origins[si]
			}
			var created bool
			var err error
			if si < len(res.Endorsements) && res.Endorsements[si].Count > 0 {
				e := res.Endorsements[si]
				attrs := make([]suggest.Attribution, 0, len(e.Attributions))
				for _, a := range e.Attributions {
					attrs = append(attrs, suggest.Attribution{Source: a.Source, Basis: a.Basis, Key: a.Key, Ref: a.Ref})
				}
				created, err = s.Queue.PipelineSuggestVouched(ctx, res.WorkID, term, res.Confidence,
					e.Count, name+": "+strings.Join(e.Sources, ", "), attrs)
			} else {
				created, err = s.Queue.PipelineSuggestFrom(ctx, res.WorkID, term, res.Confidence, sourceRef)
			}
			if err != nil {
				return queued, suggestions, err
			}
			if created {
				suggestions++
			}
			landed = true
		}
		if landed {
			queued++
		}
	}
	return queued, suggestions, nil
}
