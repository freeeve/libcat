package vocabsrc

import (
	"testing"
	"time"

	"github.com/freeeve/libcat/backend/store"
)

// TestOrphanedRunningDownloadIsReaped pins the restart-orphan reaper (task
// 440): a claim persists RUNNING, so a process dying mid-install left a
// record badged RUNNING until the TTL. The drain fails a stale-heartbeat
// RUNNING record and leaves a fresh one (a live worker) alone.
func TestOrphanedRunningDownloadIsReaped(t *testing.T) {
	s := newService(t)
	ctx := t.Context()
	now := time.Now().UTC()
	s.Now = func() time.Time { return now }
	// The URL is never fetched: the jobs are claimed, not installed.
	if err := s.PutSource(ctx, Source{Name: "lcgft", Scheme: "lcgft", SnapshotURL: "http://localhost:1/x.nt.gz"}); err != nil {
		t.Fatal(err)
	}

	orphan, err := s.CreateDownload(ctx, "eve@example.com", "lcgft")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.claim(ctx, orphan.ID); err != nil {
		t.Fatalf("claim: %v", err)
	}
	live, err := s.CreateDownload(ctx, "eve@example.com", "lcgft")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.claim(ctx, live.ID); err != nil {
		t.Fatalf("claim: %v", err)
	}

	// The orphan's worker died 2 minutes ago; the live one heartbeat since.
	now = now.Add(2 * time.Minute)
	fresh, err := s.GetJob(ctx, live.ID)
	if err != nil {
		t.Fatal(err)
	}
	fresh.HeartbeatAt = now
	if err := s.putJob(ctx, &fresh, store.CondNone); err != nil {
		t.Fatal(err)
	}

	if _, err := s.RunQueued(ctx); err != nil {
		t.Fatalf("drain: %v", err)
	}
	reaped, err := s.GetJob(ctx, orphan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reaped.Status != StatusFailed || reaped.Error != "interrupted by a restart" || reaped.FinishedAt.IsZero() {
		t.Fatalf("orphan after drain = %+v, want FAILED interrupted-by-restart", reaped)
	}
	alive, err := s.GetJob(ctx, live.ID)
	if err != nil {
		t.Fatal(err)
	}
	if alive.Status != StatusRunning {
		t.Fatalf("live job after drain = %+v, want still RUNNING", alive)
	}
}
