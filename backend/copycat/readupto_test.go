package copycat

import (
	"errors"
	"io"
	"testing"

	codex "github.com/freeeve/libcodex"
)

func rec(id string) *codex.Record {
	r := codex.NewRecord()
	r.AddField(codex.NewControlField("001", id))
	return r
}

// reads replays a scripted stream: records, then a terminal error that repeats,
// because libcodex's readers make an error sticky.
func reads(recs []*codex.Record, terminal error) func() (*codex.Record, error) {
	i := 0
	return func() (*codex.Record, error) {
		if i < len(recs) {
			i++
			return recs[i-1], nil
		}
		return nil, terminal
	}
}

// A complete stream is complete: no records lost, nothing to report.
func TestReadUpToCompleteStream(t *testing.T) {
	got, err := readUpTo(reads([]*codex.Record{rec("1"), rec("2")}, io.EOF), 20)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if len(got) != 2 {
		t.Fatalf("records = %d, want 2", len(got))
	}
}

// An error before any record is a failure, as it always was.
func TestReadUpToImmediateErrorIsAFailure(t *testing.T) {
	boom := errors.New("sru: parse response: XML syntax error")
	got, err := readUpTo(reads(nil, boom), 20)
	if got != nil {
		t.Fatalf("records = %v, want none", got)
	}
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the stream's error", err)
	}
	var partial *PartialError
	if errors.As(err, &partial) {
		t.Fatal("an immediate error must not be reported as partial results")
	}
}

// tasks/258: the same error one record later used to be swallowed whole. The
// records are still returned -- partial results beat none -- but the caller is
// now told the set is incomplete, and why.
func TestReadUpToMidStreamErrorReportsPartialResults(t *testing.T) {
	boom := errors.New("sru: parse response: XML syntax error")
	got, err := readUpTo(reads([]*codex.Record{rec("1")}, boom), 20)
	if len(got) != 1 {
		t.Fatalf("records = %d, want the one that arrived before the break", len(got))
	}
	var partial *PartialError
	if !errors.As(err, &partial) {
		t.Fatalf("err = %v, want a PartialError", err)
	}
	if partial.Got != 1 {
		t.Fatalf("partial.Got = %d, want 1", partial.Got)
	}
	if !errors.Is(err, boom) {
		t.Fatalf("the underlying stream error was lost: %v", err)
	}
	// Whether an error lands on page 1 or page 2 is the remote server's page
	// size, not a property of the error. Both must be visible to the caller.
	if partial.Error() == "" {
		t.Fatal("empty message")
	}
}

// Hitting the search limit is also an incomplete answer -- the caller cannot
// tell "20 matches" from "the first 20 of 4,113" without being told.
func TestReadUpToCapReportsTruncation(t *testing.T) {
	got, err := readUpTo(reads([]*codex.Record{rec("1"), rec("2"), rec("3")}, io.EOF), 2)
	if len(got) != 2 {
		t.Fatalf("records = %d, want the limit", len(got))
	}
	if !errors.Is(err, ErrCapped) {
		t.Fatalf("err = %v, want ErrCapped", err)
	}
	var partial *PartialError
	if errors.As(err, &partial) {
		t.Fatal("the cap is not a stream failure")
	}
}

// A stream that ends exactly at the limit is capped as far as anyone here can
// know: reading further would fetch another page. Saying "may have more" is the
// honest answer, and it is what the cap sentinel means.
func TestReadUpToExactlyAtLimitIsReportedAsCapped(t *testing.T) {
	_, err := readUpTo(reads([]*codex.Record{rec("1"), rec("2")}, io.EOF), 2)
	if !errors.Is(err, ErrCapped) {
		t.Fatalf("err = %v, want ErrCapped", err)
	}
}
