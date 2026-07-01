package bibframe

import (
	"bytes"
	"testing"

	codex "github.com/freeeve/libcodex"
)

// titleRecord builds a small BIBFRAME-representable record, adding its fields in
// the given order so the same logical record can be constructed differently to
// exercise canonicalization.
func titleRecord(order []int) *codex.Record {
	fields := []codex.Field{
		codex.NewControlField("001", "lcat-test-1"),
		codex.NewDataField("100", '1', ' ', codex.NewSubfield('a', "Nelson, Maggie")),
		codex.NewDataField("245", '1', '0', codex.NewSubfield('a', "The Argonauts")),
		codex.NewDataField("020", ' ', ' ', codex.NewSubfield('a', "9781555977351")),
	}
	r := codex.NewRecord()
	r.SetLeader("00000nam a2200000 a 4500")
	for _, i := range order {
		r.AddField(fields[i])
	}
	return r
}

func TestGrainCanonicalStable(t *testing.T) {
	feed := FeedGraph("overdrive")

	g, err := Grain(titleRecord([]int{0, 1, 2, 3}), feed)
	if err != nil {
		t.Fatalf("Grain: %v", err)
	}
	if len(g) == 0 {
		t.Fatal("empty grain")
	}
	if !bytes.Contains(g, []byte("The Argonauts")) {
		t.Errorf("grain missing title:\n%s", g)
	}
	if !bytes.Contains(g, []byte("feed:overdrive")) {
		t.Errorf("grain not tagged with the feed graph:\n%s", g)
	}

	// Canonicalization is order-independent: the same logical record built with
	// its fields in a different order yields a byte-identical grain (no-op diff).
	shuffled, err := Grain(titleRecord([]int{3, 2, 1, 0}), feed)
	if err != nil {
		t.Fatalf("Grain (shuffled): %v", err)
	}
	if !bytes.Equal(g, shuffled) {
		t.Errorf("canonical grain not order-independent:\n--- ordered ---\n%s\n--- shuffled ---\n%s", g, shuffled)
	}
}
