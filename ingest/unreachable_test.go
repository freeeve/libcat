package ingest_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"
	"testing"

	"github.com/freeeve/libcat/ingest"
)

// TestIsUnreachable pins the connection-class classifier (task 469): the
// failures that mean "this host is misconfigured" -- DNS miss, refused,
// unroutable, timeout -- count toward the circuit break; a content/parse
// failure or an HTTP status does not.
func TestIsUnreachable(t *testing.T) {
	unreachable := []error{
		&net.DNSError{Err: "no such host", Name: "zzdead.example", IsNotFound: true},
		fmt.Errorf("dial: %w", syscall.ECONNREFUSED),
		fmt.Errorf("route: %w", syscall.EHOSTUNREACH),
		context.DeadlineExceeded,
		&net.OpError{Op: "dial", Err: &timeoutErr{}},
		// Wrapped through the enricher's own error, as the providers return it.
		fmt.Errorf("%w: %w", ingest.ErrEnricher, &net.DNSError{Err: "no such host", IsNotFound: true}),
	}
	for i, e := range unreachable {
		if !ingest.IsUnreachable(e) {
			t.Fatalf("unreachable[%d] = %v classified reachable", i, e)
		}
	}
	reachable := []error{
		nil,
		errors.New("parse rss: unexpected EOF"),
		fmt.Errorf("%w: HTTP 404", ingest.ErrEnricher),
		fmt.Errorf("%w: totalHits null (request schema rejected)", ingest.ErrEnricher),
	}
	for i, e := range reachable {
		if ingest.IsUnreachable(e) {
			t.Fatalf("reachable[%d] = %v classified unreachable", i, e)
		}
	}
}

// TestBreakerRejectsWholesaleFailure pins the second guard (task 471): a peer
// that answers every request with an HTTP error -- a wildcard-DNS host reached
// with a mistyped siteCode -- never trips the connection guard, so the reject
// guard must trip it after RejectAbortAfter consecutive failures of any class
// with no candidate produced, naming the status and host.
func TestBreakerRejectsWholesaleFailure(t *testing.T) {
	br := ingest.NewBreaker("zze2edead")
	http404 := fmt.Errorf("%w: HTTP 404", ingest.ErrEnricher)
	var abort error
	for i := 0; i < ingest.RejectAbortAfter; i++ {
		if abort = br.Fail(http404); abort != nil {
			if i+1 < ingest.RejectAbortAfter {
				t.Fatalf("tripped early at %d, want %d", i+1, ingest.RejectAbortAfter)
			}
			break
		}
	}
	if !errors.Is(abort, ingest.ErrPeerRejected) {
		t.Fatalf("abort = %v, want ErrPeerRejected", abort)
	}
	if !strings.Contains(abort.Error(), "zze2edead") || !strings.Contains(abort.Error(), "HTTP 404") {
		t.Fatalf("abort = %v, want host and status named", abort)
	}
	if errors.Is(abort, ingest.ErrPeerUnreachable) {
		t.Fatalf("abort = %v, must not classify as unreachable (no connection failure)", abort)
	}
}

// TestBreakerConnectionClassStillUnreachable keeps the connection guard: a run
// of DNS misses trips ErrPeerUnreachable, the more specific classification, not
// ErrPeerRejected (both thresholds are equal, so the order must favor it).
func TestBreakerConnectionClassStillUnreachable(t *testing.T) {
	br := ingest.NewBreaker("zzdead.invalid")
	dns := &net.DNSError{Err: "no such host", Name: "zzdead.invalid", IsNotFound: true}
	var abort error
	for i := 0; i < ingest.UnreachableAbortAfter+5 && abort == nil; i++ {
		abort = br.Fail(dns)
	}
	if !errors.Is(abort, ingest.ErrPeerUnreachable) {
		t.Fatalf("abort = %v, want ErrPeerUnreachable", abort)
	}
}

// TestBreakerAnsweredTermResetsStreak keeps a healthy but sparse peer alive: a
// term the peer answers (Ok) resets the failure streak, so intermittent misses
// never trip either guard.
func TestBreakerAnsweredTermResetsStreak(t *testing.T) {
	br := ingest.NewBreaker("healthy")
	http404 := fmt.Errorf("%w: HTTP 404", ingest.ErrEnricher)
	for i := 0; i < ingest.RejectAbortAfter*4; i++ {
		if abort := br.Fail(http404); abort != nil {
			t.Fatalf("tripped at %d despite periodic answers", i)
		}
		if i%3 == 2 {
			br.Ok() // one term in three resolves cleanly
		}
	}
}

// TestBreakerCandidateDisarmsReject proves a peer that has produced real output
// is never called "rejecting every request": once a candidate lands, a later
// HTTP-error streak no longer trips the reject guard.
func TestBreakerCandidateDisarmsReject(t *testing.T) {
	br := ingest.NewBreaker("works-then-flaky")
	br.Ok()
	br.Candidate()
	http404 := fmt.Errorf("%w: HTTP 404", ingest.ErrEnricher)
	for i := 0; i < ingest.RejectAbortAfter*4; i++ {
		if abort := br.Fail(http404); abort != nil {
			t.Fatalf("tripped at %d after a candidate was produced", i)
		}
	}
}

// timeoutErr is a net.Error that reports a timeout.
type timeoutErr struct{}

func (timeoutErr) Error() string   { return "i/o timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }
