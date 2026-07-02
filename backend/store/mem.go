package store

import (
	"context"
	"iter"
	"sort"
	"strings"
	"sync"
	"time"
)

// Mem is an in-memory Store with exact conditional semantics and strict TTL
// (expired records are invisible immediately) -- the reference implementation
// for tests and local development.
type Mem struct {
	mu       sync.Mutex
	records  map[Key]memRecord
	counters map[Key]memCounter
	now      func() time.Time
}

type memRecord struct {
	data     []byte
	version  int64
	expireAt time.Time
}

type memCounter struct {
	value    int64
	expireAt time.Time
}

// NewMem returns an empty Mem.
func NewMem() *Mem {
	return &Mem{
		records:  make(map[Key]memRecord),
		counters: make(map[Key]memCounter),
		now:      time.Now,
	}
}

// SetClock overrides the store's clock; tests use it to step through TTL
// windows deterministically.
func (m *Mem) SetClock(now func() time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.now = now
}

// Get returns the record at k, or ErrNotFound.
func (m *Mem) Get(ctx context.Context, k Key) (Record, error) {
	if err := validateKey(k); err != nil {
		return Record{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.records[k]
	if !ok || expired(rec.expireAt, m.now()) {
		return Record{}, ErrNotFound
	}
	return m.export(k, rec), nil
}

func (m *Mem) export(k Key, rec memRecord) Record {
	data := make([]byte, len(rec.data))
	copy(data, rec.data)
	return Record{Key: k, Data: data, Version: rec.version, ExpireAt: rec.expireAt}
}

// Put writes r subject to cond.
func (m *Mem) Put(ctx context.Context, r Record, cond Cond) (Record, error) {
	if err := validateKey(r.Key); err != nil {
		return Record{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	current, exists := m.records[r.Key]
	if exists && expired(current.expireAt, m.now()) {
		exists = false
		current = memRecord{}
	}
	switch cond {
	case CondIfAbsent:
		if exists {
			return Record{}, ErrConditionFailed
		}
	case CondIfVersion:
		if r.Version == 0 && exists {
			return Record{}, ErrConditionFailed
		}
		if r.Version != 0 && (!exists || current.version != r.Version) {
			return Record{}, ErrConditionFailed
		}
	}
	stored := memRecord{
		data:     append([]byte(nil), r.Data...),
		version:  current.version + 1,
		expireAt: r.ExpireAt,
	}
	m.records[r.Key] = stored
	return m.export(r.Key, stored), nil
}

// Delete removes the record at r.Key subject to cond.
func (m *Mem) Delete(ctx context.Context, r Record, cond Cond) error {
	if err := validateKey(r.Key); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	current, exists := m.records[r.Key]
	if !exists || expired(current.expireAt, m.now()) {
		return ErrNotFound
	}
	if cond == CondIfVersion && current.version != r.Version {
		return ErrConditionFailed
	}
	delete(m.records, r.Key)
	return nil
}

// Query yields the partition's records in SK order.
func (m *Mem) Query(ctx context.Context, pk, skPrefix string, opt QueryOpt) iter.Seq2[Record, error] {
	return func(yield func(Record, error) bool) {
		m.mu.Lock()
		now := m.now()
		var matched []Record
		for k, rec := range m.records {
			if k.PK != pk || !strings.HasPrefix(k.SK, skPrefix) || expired(rec.expireAt, now) {
				continue
			}
			matched = append(matched, m.export(k, rec))
		}
		m.mu.Unlock()
		sort.Slice(matched, func(i, j int) bool {
			if opt.Descending {
				return matched[i].Key.SK > matched[j].Key.SK
			}
			return matched[i].Key.SK < matched[j].Key.SK
		})
		count := 0
		for _, rec := range matched {
			if opt.StartAfter != "" {
				if !opt.Descending && rec.Key.SK <= opt.StartAfter {
					continue
				}
				if opt.Descending && rec.Key.SK >= opt.StartAfter {
					continue
				}
			}
			if opt.Limit > 0 && count >= opt.Limit {
				return
			}
			count++
			if !yield(rec, nil) {
				return
			}
		}
	}
}

// Increment atomically adds delta to the counter at k.
func (m *Mem) Increment(ctx context.Context, k Key, delta int64, expireAt time.Time) (int64, error) {
	if err := validateKey(k); err != nil {
		return 0, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.counters[k]
	if !ok || expired(c.expireAt, m.now()) {
		c = memCounter{}
	}
	c.value += delta
	if !expireAt.IsZero() {
		c.expireAt = expireAt
	}
	m.counters[k] = c
	return c.value, nil
}
