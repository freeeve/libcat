// Snapshot persistence for the work index (tasks/155): a cold start loads the
// projection from one blob instead of GETting every grain. The snapshot is the
// in-memory projection (identity + merges + barcodes + summaries per grain),
// serialized as gzipped JSON -- portable, inspectable, and readable by the
// Rust/WASM side, unlike a Go-only gob stream. It is a disposable cache: a
// missing, corrupt, or wrong-version snapshot costs a full scan on the next
// boot, never correctness, because refreshLocked's ETag diff always reconciles.
package workindex

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/freeeve/libcatalog/identity"
	"github.com/freeeve/libcatalog/ingest"
	"github.com/freeeve/libcatalog/storage/blob"
)

// DefaultSnapshotPath is where the persisted projection lives -- outside the
// grain prefix the index lists, so refreshLocked never mistakes it for a grain.
const DefaultSnapshotPath = "data/workindex.snapshot"

// snapshotVersion is the on-disk schema version; a mismatch falls back to a full
// scan rather than risk decoding an incompatible layout.
const snapshotVersion = 1

// snapshotEntry is one grain's projected state, the JSON-portable mirror of the
// unexported grainEntry. Shared shape with the change feed (tasks/156).
type snapshotEntry struct {
	Path      string                 `json:"path"`
	ETag      string                 `json:"etag"`
	Identity  identity.GrainIdentity `json:"identity"`
	Merges    []identity.Merge       `json:"merges,omitempty"`
	Barcodes  []string               `json:"barcodes,omitempty"`
	Summaries []ingest.WorkSummary   `json:"summaries,omitempty"`
}

// snapshotFile is the whole persisted projection: a version tag and the grain
// entries in path order.
type snapshotFile struct {
	Version int             `json:"version"`
	Entries []snapshotEntry `json:"entries"`
}

// Save serializes the current projection to the snapshot blob (gzipped JSON). It
// reads straight from memory -- no grain reads -- so it is cheap to call after a
// warm-up or a publish. Entries are written in path order so the artifact is
// deterministic.
func (ix *Index) Save(ctx context.Context) error {
	ix.mu.Lock()
	paths := make([]string, 0, len(ix.grains))
	for p := range ix.grains {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	file := snapshotFile{Version: snapshotVersion, Entries: make([]snapshotEntry, 0, len(paths))}
	for _, p := range paths {
		e := ix.grains[p]
		file.Entries = append(file.Entries, snapshotEntry{
			Path:      p,
			ETag:      e.etag,
			Identity:  e.identity,
			Merges:    e.merges,
			Barcodes:  e.barcodes,
			Summaries: e.summaries,
		})
	}
	ix.mu.Unlock()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if err := json.NewEncoder(gz).Encode(file); err != nil {
		return fmt.Errorf("workindex: encode snapshot: %w", err)
	}
	if err := gz.Close(); err != nil {
		return fmt.Errorf("workindex: gzip snapshot: %w", err)
	}
	if _, err := ix.bs.Put(ctx, ix.snapshotPath, buf.Bytes(), blob.PutOptions{ContentType: "application/gzip"}); err != nil {
		return fmt.Errorf("workindex: put snapshot: %w", err)
	}
	return nil
}

// LoadSnapshot primes the grain entries from the snapshot blob so a cold start
// skips the corpus scan. It leaves the refresh clock unset, so the next read
// still runs the ETag-diff reconcile -- re-reading only grains changed since the
// snapshot and dropping any deleted since. A missing snapshot is not an error
// (first boot). A corrupt or wrong-version one returns an error so the caller
// can log and fall back to the full scan.
func (ix *Index) LoadSnapshot(ctx context.Context) error {
	data, _, err := ix.bs.Get(ctx, ix.snapshotPath)
	if errors.Is(err, blob.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("workindex: get snapshot: %w", err)
	}
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("workindex: open snapshot gzip: %w", err)
	}
	defer gz.Close()
	var file snapshotFile
	if err := json.NewDecoder(gz).Decode(&file); err != nil {
		return fmt.Errorf("workindex: decode snapshot: %w", err)
	}
	if file.Version != snapshotVersion {
		return fmt.Errorf("workindex: snapshot version %d, want %d", file.Version, snapshotVersion)
	}
	ix.mu.Lock()
	defer ix.mu.Unlock()
	ix.grains = make(map[string]*grainEntry, len(file.Entries))
	for _, e := range file.Entries {
		ix.grains[e.Path] = &grainEntry{
			etag:      e.ETag,
			identity:  e.Identity,
			merges:    e.Merges,
			barcodes:  e.Barcodes,
			summaries: e.Summaries,
		}
	}
	ix.dirty = true
	ix.at = time.Time{} // force the next refresh to reconcile the delta
	return nil
}
