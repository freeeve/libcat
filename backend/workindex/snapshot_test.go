package workindex

import (
	"testing"

	"github.com/freeeve/libcatalog/bibframe"
	"github.com/freeeve/libcatalog/storage/blob"
)

// TestSnapshotRoundTrip: a saved snapshot lets a fresh index serve the same
// summaries, barcodes, and provider lookups without re-reading any grain.
func TestSnapshotRoundTrip(t *testing.T) {
	ctx := t.Context()
	cs := &countingStore{Store: blob.NewMem()}
	seed(t, cs, "w1", grain("w1", "A Wizard of Earthsea", "Le Guin, Ursula K.", "9780547773742", "B-0001"))
	seed(t, cs, "w2", grain("w2", "The Tombs of Atuan", "Le Guin, Ursula K.", "9780689845369", "B-0002"))
	src := New(cs, "data/works/")
	want, err := src.Summaries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := src.Save(ctx); err != nil {
		t.Fatal(err)
	}

	dst := New(cs, "data/works/")
	if err := dst.LoadSnapshot(ctx); err != nil {
		t.Fatal(err)
	}
	dst.feedActive = false // isolate snapshot load; the feed has its own tests
	cs.gets.Store(0)
	cs.lists.Store(0)
	got, err := dst.Summaries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) || got[0].WorkID != want[0].WorkID || got[1].Title != want[1].Title {
		t.Fatalf("summaries after load = %+v, want %+v", got, want)
	}
	// The reconcile Lists once but re-reads zero grains: the snapshot's ETags
	// all match, so the corpus scan is skipped.
	if cs.gets.Load() != 0 {
		t.Fatalf("load served %d grain gets, want 0 (snapshot only)", cs.gets.Load())
	}
	taken, _ := dst.Barcodes(ctx)
	if !taken["B-0001"] || !taken["B-0002"] {
		t.Fatalf("barcodes after load = %v", taken)
	}
	owners, _ := dst.ProviderOwners(ctx, "isbn:9780547773742")
	if len(owners) != 1 || owners[0].WorkID != "w1" {
		t.Fatalf("provider owners after load = %+v", owners)
	}
}

// TestSnapshotStaleReconciles: an out-of-date snapshot is still correct -- the
// ETag diff re-reads only the grains changed since it, and drops deletions.
func TestSnapshotStaleReconciles(t *testing.T) {
	ctx := t.Context()
	cs := &countingStore{Store: blob.NewMem()}
	seed(t, cs, "w1", grain("w1", "The Left Hand of Darkness", "Le Guin, Ursula K.", "9780441478125", ""))
	seed(t, cs, "w2", grain("w2", "The Dispossessed", "Le Guin, Ursula K.", "9780061054884", ""))
	src := New(cs, "data/works/")
	if _, err := src.Summaries(ctx); err != nil {
		t.Fatal(err)
	}
	if err := src.Save(ctx); err != nil {
		t.Fatal(err)
	}

	// Mutate the corpus after the snapshot: change w1, add w3, delete w2.
	seed(t, cs, "w1", grain("w1", "The Left Hand of Darkness (rev)", "Le Guin, Ursula K.", "9780441478125", ""))
	seed(t, cs, "w3", grain("w3", "Always Coming Home", "Le Guin, Ursula K.", "9780520227354", ""))
	if err := cs.Delete(ctx, bibframe.GrainPath("w2")); err != nil {
		t.Fatal(err)
	}

	dst := New(cs, "data/works/")
	if err := dst.LoadSnapshot(ctx); err != nil {
		t.Fatal(err)
	}
	dst.feedActive = false // isolate snapshot reconcile; the feed has its own tests
	cs.gets.Store(0)
	sums, err := dst.Summaries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(sums) != 2 || sums[0].Title != "The Left Hand of Darkness (rev)" || sums[1].WorkID != "w3" {
		t.Fatalf("summaries after stale load = %+v", sums)
	}
	if got := cs.gets.Load(); got != 2 {
		t.Fatalf("reconcile gets = %d, want 2 (changed + new only)", got)
	}
}

// TestSnapshotMissing: no snapshot is not an error; the index warms from the
// store as before.
func TestSnapshotMissing(t *testing.T) {
	ctx := t.Context()
	cs := &countingStore{Store: blob.NewMem()}
	seed(t, cs, "w1", grain("w1", "A Wizard of Earthsea", "Le Guin, Ursula K.", "9780547773742", ""))
	ix := New(cs, "data/works/")
	if err := ix.LoadSnapshot(ctx); err != nil {
		t.Fatalf("missing snapshot should be nil, got %v", err)
	}
	sums, err := ix.Summaries(ctx)
	if err != nil || len(sums) != 1 {
		t.Fatalf("warm from store after missing snapshot = %+v, %v", sums, err)
	}
}

// TestSnapshotCorruptFallsBack: a garbage or wrong-version snapshot errors so
// the caller can fall back to a full scan -- it never corrupts the index.
func TestSnapshotCorruptFallsBack(t *testing.T) {
	ctx := t.Context()
	cs := &countingStore{Store: blob.NewMem()}
	seed(t, cs, "w1", grain("w1", "A Wizard of Earthsea", "Le Guin, Ursula K.", "9780547773742", ""))

	if _, err := cs.Put(ctx, DefaultSnapshotPath, []byte("not gzip"), blob.PutOptions{}); err != nil {
		t.Fatal(err)
	}
	ix := New(cs, "data/works/")
	if err := ix.LoadSnapshot(ctx); err == nil {
		t.Fatal("corrupt snapshot should return an error")
	}
	// The index is untouched, so a normal read still scans the store correctly.
	sums, err := ix.Summaries(ctx)
	if err != nil || len(sums) != 1 || sums[0].WorkID != "w1" {
		t.Fatalf("scan after corrupt snapshot = %+v, %v", sums, err)
	}
}
