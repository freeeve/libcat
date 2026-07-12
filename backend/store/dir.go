package store

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Dir is the persistent local document store: the in-memory store fronted by
// an append-only JSONL journal, replayed (and compacted) on open. It exists
// so a container/laptop deployment without DynamoDB keeps its moderation
// queue, promotions, review decisions, audit trail, drafts, and job records
// across restarts. Write rates on this surface are human-scale, so every
// append syncs; a crash loses at most the op whose append never landed.
type Dir struct {
	mu    sync.Mutex
	inner *Mem
	f     *os.File
	w     *bufio.Writer
	path  string
}

// journalEntry is one persisted mutation. Put entries carry the STORED
// record (post-condition, version assigned), and counter entries the
// absolute value -- so replay is a plain state rebuild, never a re-run of
// conditional logic.
type journalEntry struct {
	Op       string    `json:"op"` // "put" | "delete" | "counter"
	PK       string    `json:"pk"`
	SK       string    `json:"sk"`
	Data     []byte    `json:"data,omitempty"`
	Version  int64     `json:"version,omitempty"`
	Value    int64     `json:"value,omitempty"`
	ExpireAt time.Time `json:"expireAt,omitzero"`
}

// NewDir opens (or creates) the journal-backed store rooted at dir. The
// journal is replayed into memory and rewritten compacted, so growth is
// bounded by live data times the churn since the last open.
func NewDir(dir string) (*Dir, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("store: create %s: %w", dir, err)
	}
	d := &Dir{inner: NewMem(), path: filepath.Join(dir, "journal.jsonl")}
	if err := d.replay(); err != nil {
		return nil, err
	}
	if err := d.compact(); err != nil {
		return nil, err
	}
	return d, nil
}

// replay loads the journal into the inner store. A truncated final line --
// the crash-mid-append case -- is tolerated and dropped; anything malformed
// earlier is an error, because silently skipping history would resurrect
// deleted records.
func (d *Dir) replay() error {
	f, err := os.Open(d.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 16<<20)
	var pending []journalEntry
	lastComplete := true
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var e journalEntry
		if err := json.Unmarshal(line, &e); err != nil {
			lastComplete = false
			continue
		}
		if !lastComplete {
			return fmt.Errorf("store: journal %s is corrupt mid-file (not just a truncated tail)", d.path)
		}
		pending = append(pending, e)
	}
	if err := sc.Err(); err != nil && err != io.EOF {
		return err
	}
	for _, e := range pending {
		k := Key{PK: e.PK, SK: e.SK}
		switch e.Op {
		case "put":
			d.inner.records[k] = memRecord{data: e.Data, version: e.Version, expireAt: e.ExpireAt}
		case "delete":
			delete(d.inner.records, k)
		case "counter":
			d.inner.counters[k] = memCounter{value: e.Value, expireAt: e.ExpireAt}
		}
	}
	return nil
}

// compact rewrites the journal as one entry per live record and counter,
// atomically (write aside, rename over), then reopens it for appends.
func (d *Dir) compact() error {
	tmp := d.path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	enc := json.NewEncoder(w)
	now := time.Now()
	for k, rec := range d.inner.records {
		if expired(rec.expireAt, now) {
			continue
		}
		if err := enc.Encode(journalEntry{Op: "put", PK: k.PK, SK: k.SK, Data: rec.data, Version: rec.version, ExpireAt: rec.expireAt}); err != nil {
			f.Close()
			return err
		}
	}
	for k, c := range d.inner.counters {
		if expired(c.expireAt, now) {
			continue
		}
		if err := enc.Encode(journalEntry{Op: "counter", PK: k.PK, SK: k.SK, Value: c.value, ExpireAt: c.expireAt}); err != nil {
			f.Close()
			return err
		}
	}
	if err := w.Flush(); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, d.path); err != nil {
		return err
	}
	live, err := os.OpenFile(d.path, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	d.f = live
	d.w = bufio.NewWriter(live)
	return nil
}

// append journals one entry durably.
func (d *Dir) append(e journalEntry) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	enc := json.NewEncoder(d.w)
	if err := enc.Encode(e); err != nil {
		return err
	}
	if err := d.w.Flush(); err != nil {
		return err
	}
	return d.f.Sync()
}

// Close syncs and closes the journal.
func (d *Dir) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.w.Flush(); err != nil {
		return err
	}
	if err := d.f.Sync(); err != nil {
		return err
	}
	return d.f.Close()
}

// SetClock overrides the inner store's clock (tests).
func (d *Dir) SetClock(now func() time.Time) { d.inner.SetClock(now) }

// Get returns the record at k, or ErrNotFound.
func (d *Dir) Get(ctx context.Context, k Key) (Record, error) {
	return d.inner.Get(ctx, k)
}

// Put writes r subject to cond, journaling the stored result.
func (d *Dir) Put(ctx context.Context, r Record, cond Cond) (Record, error) {
	stored, err := d.inner.Put(ctx, r, cond)
	if err != nil {
		return Record{}, err
	}
	if err := d.append(journalEntry{
		Op: "put", PK: stored.Key.PK, SK: stored.Key.SK,
		Data: stored.Data, Version: stored.Version, ExpireAt: stored.ExpireAt,
	}); err != nil {
		return Record{}, fmt.Errorf("store: journal append: %w", err)
	}
	return stored, nil
}

// Delete removes the record at r.Key subject to cond.
func (d *Dir) Delete(ctx context.Context, r Record, cond Cond) error {
	if err := d.inner.Delete(ctx, r, cond); err != nil {
		return err
	}
	if err := d.append(journalEntry{Op: "delete", PK: r.Key.PK, SK: r.Key.SK}); err != nil {
		return fmt.Errorf("store: journal append: %w", err)
	}
	return nil
}

// Query yields the partition pk's records whose SK starts with skPrefix.
func (d *Dir) Query(ctx context.Context, pk, skPrefix string, opt QueryOpt) iter.Seq2[Record, error] {
	return d.inner.Query(ctx, pk, skPrefix, opt)
}

// Increment atomically adds delta to the counter at k, journaling the
// resulting absolute value so replay never re-adds.
func (d *Dir) Increment(ctx context.Context, k Key, delta int64, expireAt time.Time) (int64, error) {
	v, err := d.inner.Increment(ctx, k, delta, expireAt)
	if err != nil {
		return 0, err
	}
	if err := d.append(journalEntry{Op: "counter", PK: k.PK, SK: k.SK, Value: v, ExpireAt: expireAt}); err != nil {
		return 0, fmt.Errorf("store: journal append: %w", err)
	}
	return v, nil
}
