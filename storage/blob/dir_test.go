package blob

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDirListPicksUpExternalWrites pins the ETag cache's invalidation: a file
// rewritten behind the store's back (new mtime/size) must list with its new
// content ETag, not the cached one.
func TestDirListPicksUpExternalWrites(t *testing.T) {
	root := t.TempDir()
	d := NewDir(root)
	ctx := t.Context()
	if _, err := d.Put(ctx, "data/works/ab/w1.nq", []byte("v1\n"), PutOptions{}); err != nil {
		t.Fatal(err)
	}
	// Prime the cache with a first listing.
	for range d.List(ctx, "data/works/") {
	}
	if err := os.WriteFile(filepath.Join(root, "data/works/ab/w1.nq"), []byte("rewritten v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, wantTag, err := d.Get(ctx, "data/works/ab/w1.nq")
	if err != nil {
		t.Fatal(err)
	}
	for entry, err := range d.List(ctx, "data/works/") {
		if err != nil {
			t.Fatal(err)
		}
		if entry.ETag != wantTag {
			t.Fatalf("List ETag = %s, want the rewritten content's %s", entry.ETag, wantTag)
		}
	}
}

// TestDirListUsesETagCache pins that List answers from the stat-signature
// cache instead of re-reading content: a rewrite that preserves mtime and
// size (below the cache's resolution, the documented best-effort boundary)
// serves the cached ETag, proving no content read happened.
func TestDirListUsesETagCache(t *testing.T) {
	root := t.TempDir()
	d := NewDir(root)
	ctx := t.Context()
	oldTag, err := d.Put(ctx, "data/works/ab/w1.nq", []byte("aaaa\n"), PutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	full := filepath.Join(root, "data/works/ab/w1.nq")
	info, err := os.Stat(full)
	if err != nil {
		t.Fatal(err)
	}
	// Same length, same mtime: the stat signature is unchanged.
	if err := os.WriteFile(full, []byte("bbbb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(full, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	for entry, err := range d.List(ctx, "data/works/") {
		if err != nil {
			t.Fatal(err)
		}
		if entry.ETag != oldTag {
			t.Fatalf("List read content instead of using the cache: ETag = %s, want cached %s", entry.ETag, oldTag)
		}
	}
}

// TestDirListScopedToPrefixSubtree pins the walk scoping: listing under a
// deep prefix must not descend into sibling subtrees (an unreadable sibling
// directory would fail an unscoped walk).
func TestDirListScopedToPrefixSubtree(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("directory permissions do not bind root")
	}
	root := t.TempDir()
	d := NewDir(root)
	ctx := t.Context()
	if _, err := d.Put(ctx, "data/works/ab/w1.nq", []byte("v1\n"), PutOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Put(ctx, "data/exports/job1/out.csv", []byte("a,b\n"), PutOptions{}); err != nil {
		t.Fatal(err)
	}
	locked := filepath.Join(root, "data/exports/job1")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	n := 0
	for entry, err := range d.List(ctx, "data/works/") {
		if err != nil {
			t.Fatalf("scoped List hit sibling subtree: %v", err)
		}
		if entry.Path != "data/works/ab/w1.nq" {
			t.Fatalf("unexpected entry %q", entry.Path)
		}
		n++
	}
	if n != 1 {
		t.Fatalf("entries = %d, want 1", n)
	}
}

// TestDirListMissingSubtree pins that a prefix naming a directory that does
// not exist lists empty rather than erroring.
func TestDirListMissingSubtree(t *testing.T) {
	d := NewDir(t.TempDir())
	for _, err := range d.List(t.Context(), "data/works/") {
		t.Fatalf("missing subtree: %v", err)
	}
}
