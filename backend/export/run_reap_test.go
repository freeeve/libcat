package export

import (
	"testing"
	"time"

	"github.com/freeeve/libcat/backend/store"
)

// TestOrphanedRunningExportIsReaped pins the restart-orphan reaper (task
// 440): a claim persists RUNNING, so a process dying mid-export left a
// record badged RUNNING until the TTL. The drain fails a stale-heartbeat
// RUNNING record and leaves a fresh one (a live worker) alone.
func TestOrphanedRunningExportIsReaped(t *testing.T) {
	bs, workIDs := buildFixtureTree(t)
	svc := newService(t, bs)
	now := time.Now().UTC()
	svc.now = func() time.Time { return now }

	// Big selections stay QUEUED at create; claim them to simulate workers.
	many := append([]string{}, workIDs...)
	for len(many) <= InRequestCutoff {
		many = append(many, many...)
	}
	orphan, err := svc.Create(t.Context(), "lib@example.org", FormatNQuads, Selection{WorkIDs: many})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.claim(t.Context(), orphan.ID); err != nil {
		t.Fatalf("claim: %v", err)
	}
	live, err := svc.Create(t.Context(), "lib@example.org", FormatCSV, Selection{WorkIDs: many})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.claim(t.Context(), live.ID); err != nil {
		t.Fatalf("claim: %v", err)
	}

	// The orphan's worker died 2 minutes ago; the live one heartbeat since.
	now = now.Add(2 * time.Minute)
	fresh, err := svc.Get(t.Context(), "lib@example.org", live.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	fresh.HeartbeatAt = now
	if err := svc.put(t.Context(), &fresh, store.CondNone); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.RunQueued(t.Context()); err != nil {
		t.Fatalf("drain: %v", err)
	}
	reaped, err := svc.Get(t.Context(), "lib@example.org", orphan.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if reaped.Status != StatusFailed || reaped.Error != "interrupted by a restart" || reaped.FinishedAt.IsZero() {
		t.Fatalf("orphan after drain = %+v, want FAILED interrupted-by-restart", reaped)
	}
	alive, err := svc.Get(t.Context(), "lib@example.org", live.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if alive.Status != StatusRunning {
		t.Fatalf("live job after drain = %+v, want still RUNNING", alive)
	}
}
