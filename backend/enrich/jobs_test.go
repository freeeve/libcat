package enrich

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/freeeve/libcat/ingest"

	"github.com/freeeve/libcat/backend/store"
)

// statsEnricher enriches nothing but reports counters, so a job's live
// stats mirror has something to snapshot.
type statsEnricher struct{ st ingest.EnrichStats }

func (e statsEnricher) Name() string { return "stats" }
func (e statsEnricher) Enrich(ctx context.Context, works []ingest.WorkSummary) ([]ingest.Enrichment, error) {
	return nil, nil
}
func (e statsEnricher) RunStats() ingest.EnrichStats { return e.st }

// failingEnricher fails with the configured error.
type failingEnricher struct{ err error }

func (e failingEnricher) Name() string { return "fail" }
func (e failingEnricher) Enrich(ctx context.Context, works []ingest.WorkSummary) ([]ingest.Enrichment, error) {
	return nil, e.err
}

func jobService(t *testing.T) *Service {
	t.Helper()
	return &Service{
		Blob: fixtureStore(t), DB: store.NewMem(), GrainPrefix: "data/works/",
		Sources: map[string]Source{
			"stub":  {Enricher: stubEnricher{}, Mode: ModeDirect},
			"stats": {Enricher: statsEnricher{st: ingest.EnrichStats{Batches: 7, ResolvedCreators: 3}}, Mode: ModeDirect},
			"fail":  {Enricher: failingEnricher{err: errors.New("dial upstream.internal: refused")}, Mode: ModeDirect},
		},
	}
}

// TestJobLifecycle covers kick -> drain -> done: the job record ends DONE
// with the run's result and final stats, and a second drain finds nothing
// queued.
func TestJobLifecycle(t *testing.T) {
	svc := jobService(t)
	job, err := svc.CreateJob(t.Context(), "admin@example.org", "stats", [][2]string{})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	if job.Status != JobQueued || job.ID == "" {
		t.Fatalf("created = %+v, want QUEUED with an id", job)
	}

	ran, err := svc.RunQueuedJobs(t.Context())
	if err != nil || ran != 1 {
		t.Fatalf("RunQueuedJobs = %d, %v; want 1 run", ran, err)
	}
	got, err := svc.GetJob(t.Context(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != JobDone || got.Result == nil || got.FinishedAt.IsZero() || got.StartedAt.IsZero() {
		t.Fatalf("after drain = %+v, want DONE with result and timestamps", got)
	}
	if got.Stats == nil || got.Stats.Batches != 7 {
		t.Fatalf("stats = %+v, want the enricher's reported counters", got.Stats)
	}

	if ran, _ := svc.RunQueuedJobs(t.Context()); ran != 0 {
		t.Fatalf("second drain ran %d, want 0", ran)
	}
}

// TestJobFailureClassified proves a failed run lands FAILED with the same
// generic client-facing classification the synchronous endpoint uses -- no
// raw upstream detail in the record.
func TestJobFailureClassified(t *testing.T) {
	svc := jobService(t)
	job, err := svc.CreateJob(t.Context(), "admin@example.org", "fail", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RunQueuedJobs(t.Context()); err != nil {
		t.Fatal(err)
	}
	got, _ := svc.GetJob(t.Context(), job.ID)
	if got.Status != JobFailed {
		t.Fatalf("status = %s, want FAILED", got.Status)
	}
	if got.Error != "enrichment upstream failed" {
		t.Fatalf("error = %q, want the generic upstream classification", got.Error)
	}
}

// TestJobValidation: unknown sources refuse at kick time, jobs need the
// record store, and unknown ids are ErrJobNotFound.
func TestJobValidation(t *testing.T) {
	svc := jobService(t)
	if _, err := svc.CreateJob(t.Context(), "a", "zz-nope", nil); !errors.Is(err, ErrUnknownSource) {
		t.Fatalf("unknown source err = %v", err)
	}
	if _, err := svc.GetJob(t.Context(), "deadbeef"); !errors.Is(err, ErrJobNotFound) {
		t.Fatalf("unknown id err = %v", err)
	}
	noDB := &Service{Sources: map[string]Source{"stub": {Enricher: stubEnricher{}, Mode: ModeDirect}}}
	if _, err := noDB.CreateJob(t.Context(), "a", "stub", nil); !errors.Is(err, ErrMisconfigured) {
		t.Fatalf("no-DB err = %v", err)
	}
}

// TestJobClaimContention: a job already RUNNING is not double-run.
func TestJobClaimContention(t *testing.T) {
	svc := jobService(t)
	job, err := svc.CreateJob(t.Context(), "a", "stub", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.claimJob(t.Context(), job.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.claimJob(t.Context(), job.ID); !errors.Is(err, errJobClaimed) {
		t.Fatalf("second claim err = %v, want claimed", err)
	}
	if ran, err := svc.RunQueuedJobs(t.Context()); err != nil || ran != 0 {
		t.Fatalf("drain over a RUNNING job = %d, %v; want 0 runs", ran, err)
	}
}

// TestJobList returns newest first with a deterministic clock.
func TestJobList(t *testing.T) {
	svc := jobService(t)
	base := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	tick := 0
	svc.Now = func() time.Time { tick++; return base.Add(time.Duration(tick) * time.Minute) }
	first, _ := svc.CreateJob(t.Context(), "a", "stub", nil)
	second, _ := svc.CreateJob(t.Context(), "a", "stats", nil)
	jobs, err := svc.ListJobs(t.Context())
	if err != nil || len(jobs) != 2 {
		t.Fatalf("ListJobs = %d, %v", len(jobs), err)
	}
	if jobs[0].CreatedAt.Before(jobs[1].CreatedAt) {
		t.Fatalf("list order = %s then %s, want newest first", jobs[0].ID, jobs[1].ID)
	}
	_ = first
	_ = second
}
