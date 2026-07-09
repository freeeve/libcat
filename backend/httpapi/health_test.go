package httpapi

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func probe(t *testing.T, h http.Handler, path string) int {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec.Code
}

// The whole point of separating the probes: a draining replica must stop
// receiving traffic without being restarted. If liveness followed readiness,
// the orchestrator would kill the pod in the middle of its own graceful drain
// and the in-flight requests we delayed shutdown to protect would die anyway.
func TestDrainFailsReadinessButNotLiveness(t *testing.T) {
	health := &Health{}
	h := New(Deps{Health: health})

	if code := probe(t, h, "/v1/readyz"); code != http.StatusOK {
		t.Fatalf("readyz before drain = %d, want 200", code)
	}
	if code := probe(t, h, "/v1/healthz"); code != http.StatusOK {
		t.Fatalf("healthz before drain = %d, want 200", code)
	}

	health.Drain()

	if code := probe(t, h, "/v1/readyz"); code != http.StatusServiceUnavailable {
		t.Fatalf("readyz while draining = %d, want 503", code)
	}
	if code := probe(t, h, "/v1/healthz"); code != http.StatusOK {
		t.Fatalf("healthz while draining = %d, want 200: a draining server is not a wedged one", code)
	}
}

// A deployment that never wires Health -- every test, and any non-orchestrated
// run -- must still answer readiness, not panic on a nil pointer.
func TestNilHealthIsAlwaysReady(t *testing.T) {
	h := New(Deps{})
	if code := probe(t, h, "/v1/readyz"); code != http.StatusOK {
		t.Fatalf("readyz with no Health = %d, want 200", code)
	}
	var nilHealth *Health
	if nilHealth.Draining() {
		t.Fatal("a nil Health reported draining")
	}
	nilHealth.Drain() // must not panic
}

// Drain is called from the signal goroutine while requests are still being
// served, so the flag is read and written concurrently.
func TestDrainIsRaceFree(t *testing.T) {
	health := &Health{}
	h := New(Deps{Health: health})
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 50 {
				probe(t, h, "/v1/readyz")
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		health.Drain()
	}()
	wg.Wait()
	if !health.Draining() {
		t.Fatal("drain did not stick")
	}
}

// Draining is monotonic: nothing un-drains a replica that has been told to stop.
func TestDrainIsMonotonic(t *testing.T) {
	health := &Health{}
	health.Drain()
	health.Drain()
	if !health.Draining() {
		t.Fatal("a second Drain cleared the flag")
	}
}
