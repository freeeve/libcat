package tlc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/freeeve/libcat/ingest"
)

// dnsDoer fails every request as an unresolvable host.
type dnsDoer struct {
	mu    sync.Mutex
	calls int
}

func (d *dnsDoer) Do(req *http.Request) (*http.Response, error) {
	d.mu.Lock()
	d.calls++
	d.mu.Unlock()
	return nil, &net.DNSError{Err: "no such host", Name: req.URL.Hostname(), IsNotFound: true}
}

// rejectDoer answers every request with HTTP 404 -- a host that resolves but
// rejects each request rather than failing to connect.
type rejectDoer struct {
	mu    sync.Mutex
	calls int
}

func (d *rejectDoer) Do(req *http.Request) (*http.Response, error) {
	d.mu.Lock()
	d.calls++
	d.mu.Unlock()
	return &http.Response{StatusCode: http.StatusNotFound, Status: "404 Not Found",
		Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header), Request: req}, nil
}

func manyTerms(n int) []Term {
	terms := make([]Term, n)
	for i := range terms {
		terms[i] = Term{URI: fmt.Sprintf("https://homosaurus.org/v5/u%d", i), Labels: map[string]string{"en": "x"}, Query: fmt.Sprintf("q%d", i)}
	}
	return terms
}

// TestTLCCircuitBreaksOnUnreachable pins the fast-fail (task 469): an
// unresolvable host aborts after a bounded number of consecutive connection
// failures, naming the host, not after every driver term.
func TestTLCCircuitBreaksOnUnreachable(t *testing.T) {
	doer := &dnsDoer{}
	e := New([]string{"zzdead"}, manyTerms(200), WithClient(doer), WithDelay(0))
	_, err := e.Enrich(context.Background(), nil)
	if !errors.Is(err, ingest.ErrPeerUnreachable) {
		t.Fatalf("err = %v, want ErrPeerUnreachable", err)
	}
	if !strings.Contains(err.Error(), "zzdead") {
		t.Fatalf("err = %v, want the host named", err)
	}
	if doer.calls > ingest.UnreachableAbortAfter+2 {
		t.Fatalf("calls = %d, want ~%d (aborted early)", doer.calls, ingest.UnreachableAbortAfter)
	}
}

// TestTLCCircuitBreaksOnWholesaleRejection pins the second guard (task 471): a
// host that 404s every term -- never a connection error -- still aborts fast
// with ErrPeerRejected, naming the host and status, not after every term.
func TestTLCCircuitBreaksOnWholesaleRejection(t *testing.T) {
	doer := &rejectDoer{}
	e := New([]string{"zze2edead"}, manyTerms(200), WithClient(doer), WithDelay(0))
	_, err := e.Enrich(context.Background(), nil)
	if !errors.Is(err, ingest.ErrPeerRejected) {
		t.Fatalf("err = %v, want ErrPeerRejected", err)
	}
	if !strings.Contains(err.Error(), "zze2edead") || !strings.Contains(err.Error(), "HTTP 404") {
		t.Fatalf("err = %v, want the host and status named", err)
	}
	if doer.calls > ingest.RejectAbortAfter+2 {
		t.Fatalf("calls = %d, want ~%d (aborted early, not all 200 terms)", doer.calls, ingest.RejectAbortAfter)
	}
}
