// GET /v1/audit/diversity?simulate=queue -- the read-only "what would the
// catalog look like if we accepted the queue" projection (task 473 part 1). It
// unions each work's current subjects with its pending ADD suggestions and runs
// the same crosswalk, touching no grains and no queue statuses.
package httpapi

import (
	"net/http"
	"testing"

	"github.com/freeeve/libcat/bibframe"
	"github.com/freeeve/libcat/storage/blob"

	"github.com/freeeve/libcat/backend/suggest"
	"github.com/freeeve/libcat/backend/vocab"
)

// TestAuditSimulateQueue projects the pending ADD queue into the audit, proves
// the projection differs from the live report exactly where the queue holds
// terms the works lack, and proves the call mutates nothing.
func TestAuditSimulateQueue(t *testing.T) {
	h, bs, queue := newRecordsAPIWithQueue(t)
	// a: a controlled subject already categorized (lgbtqia via the FAST heading).
	seedAuditWork(t, bs, "waudsim0001a", "http://id.worldcat.org/fast/995592", "Lesbians", "", nil)
	// b: no aboutness at all -- uncovered until a suggestion lands.
	seedAuditWork(t, bs, "waudsim0001b", "", "", "", nil)
	// c: an uncontrolled tag in a different category.
	seedAuditWork(t, bs, "waudsim0001c", "", "", "Immigrants", nil)

	// Pending pipeline ADDs: a Homosaurus term (lgbtqia by scheme) for the
	// uncovered work and for the immigrant work, at 0.8 confidence.
	homoit := func(id string) vocab.TermRef {
		return vocab.TermRef{Scheme: "homosaurus", ID: "https://homosaurus.org/v5/" + id, Label: "Queer people"}
	}
	if err := queue.PipelineSuggest(t.Context(), "waudsim0001b", homoit("homoit0000900"), 0.8); err != nil {
		t.Fatal(err)
	}
	if err := queue.PipelineSuggest(t.Context(), "waudsim0001c", homoit("homoit0000901"), 0.8); err != nil {
		t.Fatal(err)
	}

	// Baseline: the plain audit sees only the current corpus.
	live := getAudit(t, h, "")
	if live.CoveredWorks != 2 || auditCat(live, "lgbtqia") != 1 {
		t.Fatalf("live = %d covered / lgbtqia %d, want 2 / 1", live.CoveredWorks, auditCat(live, "lgbtqia"))
	}

	// Capture the suggestion works' grain etags to prove the projection writes nothing.
	etagB := grainETagFor(t, bs, "waudsim0001b")
	etagC := grainETagFor(t, bs, "waudsim0001c")

	sim := getAudit(t, h, "simulate=queue")
	// The top-level report is still the CURRENT corpus.
	if sim.CoveredWorks != 2 || auditCat(sim, "lgbtqia") != 1 {
		t.Errorf("simulate top-level report drifted from live: %d covered / lgbtqia %d", sim.CoveredWorks, auditCat(sim, "lgbtqia"))
	}
	if sim.Simulation == nil {
		t.Fatal("simulate=queue must carry a simulation block")
	}
	if sim.Simulation.Applied != 2 || sim.Simulation.Works != 2 {
		t.Errorf("simulation = %d applied / %d works, want 2 / 2", sim.Simulation.Applied, sim.Simulation.Works)
	}
	// Projected: both suggested works become lgbtqia; b is now covered.
	if sim.Simulation.Projected.CoveredWorks != 3 {
		t.Errorf("projected covered = %d, want 3 (the empty work gains a subject)", sim.Simulation.Projected.CoveredWorks)
	}
	if got := projCat(sim, "lgbtqia"); got != 3 {
		t.Errorf("projected lgbtqia = %d, want 3 (fast + two homosaurus)", got)
	}
	if got := projCat(sim, "immigrant-diaspora"); got != 1 {
		t.Errorf("projected immigrant-diaspora = %d, want 1 (unchanged)", got)
	}

	// Read-only: no grain etag moved, and the queue still holds both rows PENDING.
	if grainETagFor(t, bs, "waudsim0001b") != etagB || grainETagFor(t, bs, "waudsim0001c") != etagC {
		t.Error("simulate=queue changed a grain etag -- it must be read-only")
	}
	if q, err := queue.Queue(t.Context(), suggest.QueueQuery{Status: suggest.StatusPending}); err != nil {
		t.Fatal(err)
	} else if q.Total != 2 {
		t.Errorf("pending queue total = %d after simulate, want 2 (nothing approved)", q.Total)
	}
}

// TestAuditSimulateQueueConfidenceFloor: the projection honours the review
// screen's confidence floor, so "everything above X" is answerable without
// approving anything.
func TestAuditSimulateQueueConfidenceFloor(t *testing.T) {
	h, bs, queue := newRecordsAPIWithQueue(t)
	seedAuditWork(t, bs, "waudsim0002a", "", "", "", nil)
	if err := queue.PipelineSuggest(t.Context(), "waudsim0002a",
		vocab.TermRef{Scheme: "homosaurus", ID: "https://homosaurus.org/v5/homoit0000902", Label: "Queer people"}, 0.8); err != nil {
		t.Fatal(err)
	}

	// Floor above the suggestion's confidence: it is filtered out, so the
	// projection equals the (empty) current corpus.
	sim := getAudit(t, h, "simulate=queue&minConfidence=0.9")
	if sim.Simulation == nil || sim.Simulation.Applied != 0 {
		t.Fatalf("floor 0.9 should filter the 0.8 row: %+v", sim.Simulation)
	}
	if sim.Simulation.Projected.CoveredWorks != 0 {
		t.Errorf("projected covered = %d, want 0 (row below the floor)", sim.Simulation.Projected.CoveredWorks)
	}

	// Floor at or below it: the row projects in.
	sim = getAudit(t, h, "simulate=queue&minConfidence=0.8")
	if sim.Simulation.Applied != 1 || projCat(sim, "lgbtqia") != 1 {
		t.Errorf("floor 0.8 = %d applied / lgbtqia %d, want 1 / 1", sim.Simulation.Applied, projCat(sim, "lgbtqia"))
	}
}

// TestAuditSimulateRejectsBadMode: only ?simulate=queue is valid.
func TestAuditSimulateRejectsBadMode(t *testing.T) {
	h, _, _ := newRecordsAPIWithQueue(t)
	rec := request(t, h, http.MethodGet, "/v1/audit/diversity?simulate=everything", "lib-token", "", nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("simulate=everything = %d, want 400", rec.Code)
	}
}

// grainETagFor reads a work grain's current etag straight from the blob store,
// for a mutation check by work id (the API-based grainETag is fixed to one id).
func grainETagFor(t *testing.T, bs blob.Store, workID string) string {
	t.Helper()
	_, etag, err := bs.Get(t.Context(), bibframe.GrainPath(workID))
	if err != nil {
		t.Fatal(err)
	}
	return etag
}
