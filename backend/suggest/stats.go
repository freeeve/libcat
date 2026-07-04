package suggest

import (
	"context"
	"sort"
	"time"
)

// sessionGap is the idle span that ends one editing session and starts the
// next: two of a cataloger's actions more than this far apart belong to
// different sittings.
const sessionGap = 30 * time.Minute

// Session is one contiguous run of a cataloger's activity -- actions no more
// than sessionGap apart -- with the works it touched.
type Session struct {
	Start   time.Time `json:"start"`
	End     time.Time `json:"end"`
	Actions int       `json:"actions"`
	Works   int       `json:"works"`
}

// ActorStats rolls up one cataloger's audit activity for the month.
type ActorStats struct {
	Actor      string         `json:"actor"`
	Total      int            `json:"total"`
	ByAction   map[string]int `json:"byAction"`
	Works      int            `json:"works"`
	ActiveDays int            `json:"activeDays"`
	First      time.Time      `json:"first"`
	Last       time.Time      `json:"last"`
	Sessions   []Session      `json:"sessions"`
}

// MonthStats is the editing-activity rollup for a single YYYY-MM audit
// partition: overall totals plus a per-cataloger breakdown.
type MonthStats struct {
	Month    string         `json:"month"`
	Total    int            `json:"total"`
	Actors   int            `json:"actors"`
	Works    int            `json:"works"`
	ByAction map[string]int `json:"byAction"`
	PerActor []ActorStats   `json:"perActor"`
}

// Stats aggregates a month's audit trail into editing statistics and session
// summaries. It is a read-only rollup over the same entries Audit returns;
// entries without an actor are ignored (they carry no editorial attribution).
func (s *Service) Stats(ctx context.Context, month string) (MonthStats, error) {
	entries, err := s.Audit(ctx, month)
	if err != nil {
		return MonthStats{}, err
	}
	out := MonthStats{Month: month, ByAction: map[string]int{}, PerActor: []ActorStats{}}
	byActor := map[string][]AuditEntry{}
	works := map[string]struct{}{}
	for _, e := range entries {
		if e.Actor == "" {
			continue
		}
		out.Total++
		out.ByAction[e.Action]++
		if e.WorkID != "" {
			works[e.WorkID] = struct{}{}
		}
		byActor[e.Actor] = append(byActor[e.Actor], e)
	}
	out.Works = len(works)
	out.Actors = len(byActor)
	for actor, es := range byActor {
		out.PerActor = append(out.PerActor, actorStats(actor, es))
	}
	sort.SliceStable(out.PerActor, func(i, j int) bool {
		if out.PerActor[i].Total != out.PerActor[j].Total {
			return out.PerActor[i].Total > out.PerActor[j].Total
		}
		return out.PerActor[i].Actor < out.PerActor[j].Actor
	})
	return out, nil
}

// actorStats builds one cataloger's rollup. Entries arrive newest-first (Audit
// order); it sorts them ascending, then walks them splitting a new session
// whenever the gap since the previous action exceeds sessionGap.
func actorStats(actor string, es []AuditEntry) ActorStats {
	sort.SliceStable(es, func(i, j int) bool { return es[i].At.Before(es[j].At) })
	st := ActorStats{
		Actor:    actor,
		Total:    len(es),
		ByAction: map[string]int{},
		First:    es[0].At,
		Last:     es[len(es)-1].At,
		Sessions: []Session{},
	}
	works := map[string]struct{}{}
	days := map[string]struct{}{}
	var cur *Session
	var curWorks map[string]struct{}
	var prev time.Time
	flush := func() {
		if cur != nil {
			cur.Works = len(curWorks)
			st.Sessions = append(st.Sessions, *cur)
		}
	}
	for i, e := range es {
		st.ByAction[e.Action]++
		if e.WorkID != "" {
			works[e.WorkID] = struct{}{}
		}
		days[e.At.UTC().Format("2006-01-02")] = struct{}{}
		if cur == nil || e.At.Sub(prev) > sessionGap {
			flush()
			cur = &Session{Start: e.At}
			curWorks = map[string]struct{}{}
		}
		cur.End = e.At
		cur.Actions++
		if e.WorkID != "" {
			curWorks[e.WorkID] = struct{}{}
		}
		prev = e.At
		if i == len(es)-1 {
			flush()
		}
	}
	st.Works = len(works)
	st.ActiveDays = len(days)
	return st
}
