package store_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/freeeve/libcat/backend/store"
	"github.com/freeeve/libcat/backend/store/storetest"
)

func TestDirConformance(t *testing.T) {
	d, err := store.NewDir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	storetest.Run(t, d, storetest.Options{StrictTTL: true})
}

// TestDirSurvivesReopen is the store's reason to exist: records, their
// versions (optimistic concurrency must not reset), deletions, and counters
// all survive close-and-reopen; expired records do not come back.
func TestDirSurvivesReopen(t *testing.T) {
	dir := t.TempDir()
	d, err := store.NewDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	keep := store.Key{PK: "QUEUE#x", SK: "s1"}
	gone := store.Key{PK: "QUEUE#x", SK: "s2"}
	brief := store.Key{PK: "QUEUE#x", SK: "s3"}
	if _, err := d.Put(t.Context(), store.Record{Key: keep, Data: []byte("v1")}, store.CondIfAbsent); err != nil {
		t.Fatal(err)
	}
	kept, err := d.Put(t.Context(), store.Record{Key: keep, Data: []byte("v2")}, store.CondNone)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Put(t.Context(), store.Record{Key: gone, Data: []byte("x")}, store.CondNone); err != nil {
		t.Fatal(err)
	}
	if err := d.Delete(t.Context(), store.Record{Key: gone}, store.CondNone); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Put(t.Context(), store.Record{Key: brief, Data: []byte("ttl"), ExpireAt: time.Now().Add(-time.Minute)}, store.CondNone); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Increment(t.Context(), store.Key{PK: "CTR#x", SK: "c"}, 7, time.Time{}); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}

	re, err := store.NewDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer re.Close()
	got, err := re.Get(t.Context(), keep)
	if err != nil || string(got.Data) != "v2" {
		t.Fatalf("kept record after reopen = %+v, %v", got, err)
	}
	if got.Version != kept.Version {
		t.Fatalf("version after reopen = %d, want %d (optimistic concurrency must survive)", got.Version, kept.Version)
	}
	// A stale-version write still refuses after reopen.
	if _, err := re.Put(t.Context(), store.Record{Key: keep, Data: []byte("stale"), Version: kept.Version - 1}, store.CondIfVersion); !errors.Is(err, store.ErrConditionFailed) {
		t.Fatalf("stale write err = %v, want condition failed", err)
	}
	if _, err := re.Get(t.Context(), gone); !errors.Is(err, store.ErrNotFound) {
		t.Fatal("deleted record resurrected by replay")
	}
	if _, err := re.Get(t.Context(), brief); !errors.Is(err, store.ErrNotFound) {
		t.Fatal("expired record resurrected by replay")
	}
	// Counter: replay restores the absolute value; the next add continues.
	if v, err := re.Increment(t.Context(), store.Key{PK: "CTR#x", SK: "c"}, 1, time.Time{}); err != nil || v != 8 {
		t.Fatalf("counter after reopen = %d, %v; want 8", v, err)
	}
}

// TestDirToleratesTruncatedTail: a crash mid-append leaves a partial final
// line; open drops it and keeps everything before it. Corruption anywhere
// else refuses to open rather than resurrecting deleted history.
func TestDirToleratesTruncatedTail(t *testing.T) {
	dir := t.TempDir()
	d, err := store.NewDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	k := store.Key{PK: "P#1", SK: "a"}
	if _, err := d.Put(t.Context(), store.Record{Key: k, Data: []byte("ok")}, store.CondNone); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "journal.jsonl")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"op":"put","pk":"P#1","sk":"b","da`); err != nil {
		t.Fatal(err)
	}
	f.Close()

	re, err := store.NewDir(dir)
	if err != nil {
		t.Fatalf("truncated tail must open: %v", err)
	}
	if got, err := re.Get(t.Context(), k); err != nil || string(got.Data) != "ok" {
		t.Fatalf("record lost with the tail: %+v, %v", got, err)
	}
	re.Close()

	// Mid-file corruption refuses.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	corrupt := []byte("not json at all\n")
	if err := os.WriteFile(path, append(corrupt, data...), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.NewDir(dir); err == nil || !strings.Contains(err.Error(), "corrupt") {
		t.Fatalf("mid-file corruption err = %v, want refusal", err)
	}
}
