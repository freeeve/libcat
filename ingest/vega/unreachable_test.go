package vega

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

// rejectDoer answers every request with HTTP 404 -- a wildcard-DNS host reached
// with a mistyped siteCode, which resolves and rejects each request rather than
// failing to connect.
type rejectDoer struct {
	mu    sync.Mutex
	calls int
}

func (d *rejectDoer) Do(req *http.Request) (*http.Response, error) {
	d.mu.Lock()
	d.calls++
	d.mu.Unlock()
	return &http.Response{
		StatusCode: http.StatusNotFound,
		Status:     "404 Not Found",
		Body:       io.NopCloser(strings.NewReader("")),
		Header:     make(http.Header),
		Request:    req,
	}, nil
}

func manyTerms(n int) []Term {
	terms := make([]Term, n)
	for i := range terms {
		terms[i] = Term{URI: fmt.Sprintf("https://homosaurus.org/v5/u%d", i), Labels: map[string]string{"en": "x"}, Query: fmt.Sprintf("q%d", i)}
	}
	return terms
}

// TestVegaCircuitBreaksOnUnreachable pins the fast-fail (task 469): an
// unresolvable region host aborts after a bounded number of consecutive
// connection failures, naming the tenant, not after every driver term.
func TestVegaCircuitBreaksOnUnreachable(t *testing.T) {
	doer := &dnsDoer{}
	e := New([]Tenant{{SiteCode: "zzdead", Region: "na2"}}, manyTerms(200), WithClient(doer), WithDelay(0))
	_, err := e.Enrich(context.Background(), nil)
	if !errors.Is(err, ingest.ErrPeerUnreachable) {
		t.Fatalf("err = %v, want ErrPeerUnreachable", err)
	}
	if !strings.Contains(err.Error(), "zzdead.na2") {
		t.Fatalf("err = %v, want the tenant named", err)
	}
	if doer.calls > ingest.UnreachableAbortAfter+2 {
		t.Fatalf("calls = %d, want ~%d (aborted early)", doer.calls, ingest.UnreachableAbortAfter)
	}
}

// TestVegaCircuitBreaksOnWholesaleRejection pins the second guard (task 471):
// a bogus siteCode resolves via III's wildcard DNS and 404s every term -- never
// a connection error -- so the run must still abort fast with ErrPeerRejected,
// naming the tenant and the rejecting status, not grind every driver term.
func TestVegaCircuitBreaksOnWholesaleRejection(t *testing.T) {
	doer := &rejectDoer{}
	e := New([]Tenant{{SiteCode: "zze2edead", Region: "na2"}}, manyTerms(200), WithClient(doer), WithDelay(0))
	_, err := e.Enrich(context.Background(), nil)
	if !errors.Is(err, ingest.ErrPeerRejected) {
		t.Fatalf("err = %v, want ErrPeerRejected", err)
	}
	if errors.Is(err, ingest.ErrPeerUnreachable) {
		t.Fatalf("err = %v, must not classify a rejecting host as unreachable", err)
	}
	if !strings.Contains(err.Error(), "zze2edead.na2") || !strings.Contains(err.Error(), "HTTP 404") {
		t.Fatalf("err = %v, want the tenant and status named", err)
	}
	if doer.calls > ingest.RejectAbortAfter+2 {
		t.Fatalf("calls = %d, want ~%d (aborted early, not all 200 terms)", doer.calls, ingest.RejectAbortAfter)
	}
}
