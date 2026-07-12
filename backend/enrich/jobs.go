package enrich

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/freeeve/libcat/ingest"

	"github.com/freeeve/libcat/backend/store"
)

// JobStatus is the async-run lifecycle.
type JobStatus string

const (
	JobQueued  JobStatus = "QUEUED"
	JobRunning JobStatus = "RUNNING"
	JobDone    JobStatus = "DONE"
	JobFailed  JobStatus = "FAILED"
)

// jobTTL bounds how long finished jobs stay listable.
const jobTTL = 7 * 24 * time.Hour

// statsInterval is how often a running job's record refreshes with the
// enricher's live counters.
const statsInterval = 3 * time.Second

// Job is one asynchronous enrichment run: kicked with a source and scope,
// drained by the worker, its record carrying live batch counters while it
// runs so a poller can show progress on an hours-long corpus pass.
type Job struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	// Filters are the run's [key, value] scope terms (ingest.MatchExtras
	// semantics -- the same scoping the synchronous run and the audit use).
	Filters   [][2]string `json:"filters,omitempty"`
	Requester string      `json:"requester"`
	Status    JobStatus   `json:"status"`
	// Stats is the live progress while RUNNING (updated per statsInterval
	// when the source reports counters) and the final tallies after.
	Stats *ingest.EnrichStats `json:"stats,omitempty"`
	// Result is the completed run's summary (DONE only).
	Result *Result `json:"result,omitempty"`
	// Error is the failure, classified the same way the synchronous
	// endpoint classifies it (FAILED only; generic, detail in the log).
	Error      string    `json:"error,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
	StartedAt  time.Time `json:"startedAt,omitzero"`
	FinishedAt time.Time `json:"finishedAt,omitzero"`
}

// ErrJobNotFound reports an unknown job id.
var ErrJobNotFound = errors.New("enrichment job not found")

// errJobClaimed reports a job another worker already picked up.
var errJobClaimed = errors.New("enrichment job already claimed")

func jobKey(id string) store.Key { return store.Key{PK: "JOB#ENRICH", SK: id} }

// CreateJob queues an asynchronous run. The source must exist (the caller's
// mistake surfaces at kick time, not first drain); execution happens on the
// worker via RunQueuedJobs.
func (s *Service) CreateJob(ctx context.Context, requester, source string, filters [][2]string) (Job, error) {
	if s.DB == nil {
		return Job{}, fmt.Errorf("%w: async jobs need the record store", ErrMisconfigured)
	}
	if _, ok := s.Sources[source]; !ok {
		return Job{}, fmt.Errorf("%w: %q", ErrUnknownSource, source)
	}
	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		return Job{}, err
	}
	job := Job{
		ID: hex.EncodeToString(suffix), Source: source, Filters: filters,
		Requester: requester, Status: JobQueued, CreatedAt: s.jobNow().UTC(),
	}
	if err := s.putJob(ctx, job, store.CondIfAbsent); err != nil {
		return Job{}, err
	}
	return job, nil
}

// GetJob returns one job. The surface is admin-gated, so there is no
// requester scoping.
func (s *Service) GetJob(ctx context.Context, id string) (Job, error) {
	if s.DB == nil {
		return Job{}, ErrJobNotFound
	}
	rec, err := s.DB.Get(ctx, jobKey(id))
	if errors.Is(err, store.ErrNotFound) {
		return Job{}, ErrJobNotFound
	}
	if err != nil {
		return Job{}, err
	}
	var job Job
	if err := json.Unmarshal(rec.Data, &job); err != nil {
		return Job{}, err
	}
	return job, nil
}

// ListJobs returns every live job, newest first.
func (s *Service) ListJobs(ctx context.Context) ([]Job, error) {
	jobs := []Job{}
	if s.DB == nil {
		return jobs, nil
	}
	for rec, err := range s.DB.Query(ctx, "JOB#ENRICH", "", store.QueryOpt{Limit: 200}) {
		if err != nil {
			return nil, err
		}
		var job Job
		if json.Unmarshal(rec.Data, &job) == nil {
			jobs = append(jobs, job)
		}
	}
	// Keys are random ids, so store order is meaningless; newest first.
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].CreatedAt.After(jobs[j].CreatedAt) })
	return jobs, nil
}

// RunQueuedJobs drains QUEUED jobs once -- the worker-loop body for container
// deployments (a ticker) and scheduled serverless drains alike. Job failures
// land in the job record; the returned error is store trouble only.
func (s *Service) RunQueuedJobs(ctx context.Context) (int, error) {
	if s.DB == nil {
		return 0, nil
	}
	ran := 0
	for rec, err := range s.DB.Query(ctx, "JOB#ENRICH", "", store.QueryOpt{}) {
		if err != nil {
			return ran, err
		}
		var job Job
		if json.Unmarshal(rec.Data, &job) != nil || job.Status != JobQueued {
			continue
		}
		if err := s.runJob(ctx, job.ID); errors.Is(err, errJobClaimed) {
			continue
		} else if err != nil {
			return ran, err
		}
		ran++
	}
	return ran, nil
}

// runJob claims one QUEUED job and executes it, refreshing the record with
// live enricher counters while the run is in flight.
func (s *Service) runJob(ctx context.Context, id string) error {
	job, err := s.claimJob(ctx, id)
	if err != nil {
		return err
	}
	var keep func(*ingest.WorkSummary) bool
	if len(job.Filters) > 0 {
		filters := job.Filters
		keep = func(sum *ingest.WorkSummary) bool { return ingest.MatchExtras(filters, sum.Extras) }
	}

	// While Run executes, mirror the source's live counters into the job
	// record so GET shows batches advancing. Best effort: a lost update is
	// the next tick's problem.
	src := s.Sources[job.Source]
	reporter, reports := src.Enricher.(ingest.StatsReporter)
	stopStats := make(chan struct{})
	statsDone := make(chan struct{})
	go func() {
		defer close(statsDone)
		if !reports {
			return
		}
		ticker := time.NewTicker(statsInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stopStats:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				st := reporter.RunStats()
				job.Stats = &st
				_ = s.putJob(ctx, *job, store.CondNone)
			}
		}
	}()

	result, runErr := s.Run(ctx, job.Source, keep)
	close(stopStats)
	<-statsDone

	job.FinishedAt = s.jobNow().UTC()
	if runErr != nil {
		job.Status = JobFailed
		job.Error = classifyJobError(runErr)
		if reports {
			st := reporter.RunStats()
			job.Stats = &st
		}
		return s.putJob(ctx, *job, store.CondNone)
	}
	job.Status = JobDone
	job.Result = &result
	job.Stats = result.Stats
	return s.putJob(ctx, *job, store.CondNone)
}

// classifyJobError mirrors the synchronous endpoint's client-facing
// classification: generic per class, raw detail stays server-side.
func classifyJobError(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "enrichment upstream timed out"
	case errors.Is(err, ingest.ErrEnricher):
		return "enrichment upstream failed"
	default:
		return "enrichment run failed"
	}
}

// claimJob flips QUEUED -> RUNNING under the record's version so concurrent
// workers cannot double-run.
func (s *Service) claimJob(ctx context.Context, id string) (*Job, error) {
	rec, err := s.DB.Get(ctx, jobKey(id))
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrJobNotFound
	}
	if err != nil {
		return nil, err
	}
	var job Job
	if err := json.Unmarshal(rec.Data, &job); err != nil {
		return nil, err
	}
	if job.Status != JobQueued {
		return nil, errJobClaimed
	}
	job.Status = JobRunning
	job.StartedAt = s.jobNow().UTC()
	data, err := json.Marshal(job)
	if err != nil {
		return nil, err
	}
	rec.Data = data
	if _, err := s.DB.Put(ctx, rec, store.CondIfVersion); err != nil {
		if errors.Is(err, store.ErrConditionFailed) {
			return nil, errJobClaimed
		}
		return nil, err
	}
	return &job, nil
}

func (s *Service) putJob(ctx context.Context, job Job, cond store.Cond) error {
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	rec := store.Record{Key: jobKey(job.ID), Data: data, ExpireAt: s.jobNow().Add(jobTTL)}
	_, err = s.DB.Put(ctx, rec, cond)
	return err
}

// jobNow is the service clock (tests may override Now).
func (s *Service) jobNow() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}
