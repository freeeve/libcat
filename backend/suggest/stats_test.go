package suggest

import (
	"testing"
	"time"
)

// writeAt stamps one audit entry at a fixed instant by pinning the clock.
func writeAt(t *testing.T, svc *Service, at time.Time, e AuditEntry) {
	t.Helper()
	svc.SetClock(func() time.Time { return at })
	svc.WriteAudit(t.Context(), e)
}

func TestStatsEmptyMonth(t *testing.T) {
	svc, _ := newService(t)
	st, err := svc.Stats(t.Context(), "2026-07")
	if err != nil {
		t.Fatal(err)
	}
	if st.Total != 0 || st.Actors != 0 || st.Works != 0 || len(st.PerActor) != 0 {
		t.Fatalf("expected zero-value stats, got %+v", st)
	}
}

func TestStatsAggregates(t *testing.T) {
	svc, _ := newService(t)
	base := time.Date(2026, 7, 2, 9, 0, 0, 0, time.UTC)

	// eve: two sittings on day 1 (a 40-min gap splits them), one on day 2.
	writeAt(t, svc, base, AuditEntry{Actor: "eve", Action: "MARC_EDIT", WorkID: "w1"})
	writeAt(t, svc, base.Add(5*time.Minute), AuditEntry{Actor: "eve", Action: "MARC_EDIT", WorkID: "w1"})
	writeAt(t, svc, base.Add(15*time.Minute), AuditEntry{Actor: "eve", Action: "REVIEW_APPROVE", WorkID: "w2"})
	writeAt(t, svc, base.Add(55*time.Minute), AuditEntry{Actor: "eve", Action: "MARC_EDIT", WorkID: "w3"})
	writeAt(t, svc, base.Add(24*time.Hour), AuditEntry{Actor: "eve", Action: "MERGE", WorkID: "w1"})
	// amir: a single action, no work id.
	writeAt(t, svc, base.Add(30*time.Minute), AuditEntry{Actor: "amir", Action: "PUBLISH_DONE"})
	// an actorless publish event must not appear in the rollup.
	writeAt(t, svc, base.Add(30*time.Minute), AuditEntry{Action: "PUBLISH_DONE"})

	st, err := svc.Stats(t.Context(), "2026-07")
	if err != nil {
		t.Fatal(err)
	}
	if st.Total != 6 {
		t.Fatalf("Total = %d, want 6 (actorless entry excluded)", st.Total)
	}
	if st.Actors != 2 {
		t.Fatalf("Actors = %d, want 2", st.Actors)
	}
	if st.Works != 3 { // w1, w2, w3
		t.Fatalf("Works = %d, want 3", st.Works)
	}
	if st.ByAction["MARC_EDIT"] != 3 {
		t.Fatalf("ByAction[MARC_EDIT] = %d, want 3", st.ByAction["MARC_EDIT"])
	}

	// PerActor is sorted by Total desc: eve (5) before amir (1).
	if st.PerActor[0].Actor != "eve" || st.PerActor[1].Actor != "amir" {
		t.Fatalf("PerActor order = [%s %s], want [eve amir]", st.PerActor[0].Actor, st.PerActor[1].Actor)
	}
	eve := st.PerActor[0]
	if eve.Total != 5 {
		t.Fatalf("eve.Total = %d, want 5", eve.Total)
	}
	if eve.Works != 3 {
		t.Fatalf("eve.Works = %d, want 3", eve.Works)
	}
	if eve.ActiveDays != 2 {
		t.Fatalf("eve.ActiveDays = %d, want 2", eve.ActiveDays)
	}
	// Sittings: [base, +5, +15] (span 15m), [+55] (40m gap), [+24h].
	if len(eve.Sessions) != 3 {
		t.Fatalf("eve.Sessions = %d, want 3", len(eve.Sessions))
	}
	if eve.Sessions[0].Actions != 3 || eve.Sessions[0].Works != 2 {
		t.Fatalf("session 0 = %+v, want 3 actions / 2 works", eve.Sessions[0])
	}
	if !eve.Sessions[0].Start.Equal(base) || !eve.Sessions[0].End.Equal(base.Add(15*time.Minute)) {
		t.Fatalf("session 0 span = %s..%s", eve.Sessions[0].Start, eve.Sessions[0].End)
	}
	if !eve.First.Equal(base) || !eve.Last.Equal(base.Add(24*time.Hour)) {
		t.Fatalf("eve first/last = %s..%s", eve.First, eve.Last)
	}
}
