package trigger

import (
	"context"
	"sync"
	"testing"
	"time"
)

// capture records deliveries for assertions.
type capture struct {
	mu     sync.Mutex
	events []Event
}

func (c *capture) Notify(_ context.Context, e Event) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, e)
	return nil
}

func (c *capture) snapshot() []Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]Event(nil), c.events...)
}

func TestCoalesceBatchesBurst(t *testing.T) {
	sink := &capture{}
	c := &Coalesce{Next: sink, Window: 30 * time.Millisecond}
	for _, paths := range [][]string{{"a.nq"}, {"b.nq", "a.nq"}, {"c.nq"}} {
		if err := c.Notify(t.Context(), Event{Kind: "grains-changed", Paths: paths}); err != nil {
			t.Fatal(err)
		}
	}
	deadline := time.Now().Add(2 * time.Second)
	for len(sink.snapshot()) == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	events := sink.snapshot()
	if len(events) != 1 {
		t.Fatalf("deliveries = %d, want 1 coalesced batch", len(events))
	}
	got := map[string]bool{}
	for _, p := range events[0].Paths {
		got[p] = true
	}
	if len(events[0].Paths) != 3 || !got["a.nq"] || !got["b.nq"] || !got["c.nq"] {
		t.Fatalf("batch paths = %v, want deduped union of a/b/c", events[0].Paths)
	}
}

// TestCoalesceMaxDelayBoundsBusyStream: a stream that never goes quiet still
// flushes by MaxDelay after its first event.
func TestCoalesceMaxDelayBoundsBusyStream(t *testing.T) {
	sink := &capture{}
	c := &Coalesce{Next: sink, Window: 40 * time.Millisecond, MaxDelay: 120 * time.Millisecond}
	stop := time.Now().Add(400 * time.Millisecond)
	i := 0
	for time.Now().Before(stop) && len(sink.snapshot()) == 0 {
		i++
		if err := c.Notify(t.Context(), Event{Kind: "grains-changed", Paths: []string{string(rune('a' + i%20))}}); err != nil {
			t.Fatal(err)
		}
		time.Sleep(15 * time.Millisecond) // always inside Window, so quiet never happens
	}
	if len(sink.snapshot()) == 0 {
		t.Fatal("busy stream held the batch past MaxDelay")
	}
}
