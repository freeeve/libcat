// Sidecar index reader (tasks/167): serves one scheme from the artifacts
// BuildSidecar wrote, holding only the small structures resident -- the URI
// and identifier RRILs, the LCVS search arena, and the RRSR offset index --
// while Term payloads range-fetch from the record store on demand (a
// bounded cache absorbs the editor's hot set). Reads are lock-free except
// the cache. Unlike the map path, sidecar reads can fail (the store is
// remote): failures log and report a miss, never an invented term.
package vocab

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strings"
	"sync"

	rr "github.com/freeeve/roaringrange"

	"github.com/freeeve/libcat/storage/blob"
)

// sidecarTermCacheCap bounds the materialized-Term cache; at the editor's
// pace this covers the working set, and eviction is wholesale for
// simplicity (the next fetch is one ranged read).
const sidecarTermCacheCap = 4096

// sidecarScheme is one artifact-backed scheme of a snapshot.
type sidecarScheme struct {
	scheme  string
	count   uint32
	uri     *rr.LookupIndex
	tiers   [identifierTiers]*rr.LookupIndex
	records *rr.RecordStore

	// search columns, decoded from the LCVS blob.
	arena []byte
	ends  []uint32
	docs  []uint32
	alt   []byte

	mu    sync.Mutex
	cache map[uint32]*Term
}

// openSidecar loads a scheme's resident artifacts and wires the record
// store over ranged reads of the .bin blob.
func openSidecar(ctx context.Context, st blob.Store, prefix string, m *SidecarManifest) (*sidecarScheme, error) {
	resident := func(suffix string) ([]byte, error) {
		data, _, err := st.Get(ctx, sidecarPath(prefix, m.Scheme, suffix))
		if err != nil {
			return nil, fmt.Errorf("vocab: sidecar %s%s: %w", m.Scheme, suffix, err)
		}
		return data, nil
	}
	s := &sidecarScheme{scheme: m.Scheme, count: uint32(m.Terms), cache: map[uint32]*Term{}}

	uriData, err := resident(".uri.rril")
	if err != nil {
		return nil, err
	}
	if s.uri, err = rr.OpenLookup(bytes.NewReader(uriData)); err != nil {
		return nil, fmt.Errorf("vocab: sidecar %s uri lookup: %w", m.Scheme, err)
	}
	for k := range identifierTiers {
		data, err := resident(fmt.Sprintf(".id%d.rril", k+1))
		if err != nil {
			return nil, err
		}
		if s.tiers[k], err = rr.OpenLookup(bytes.NewReader(data)); err != nil {
			return nil, fmt.Errorf("vocab: sidecar %s id tier %d: %w", m.Scheme, k+1, err)
		}
	}
	searchData, err := resident(".search.bin")
	if err != nil {
		return nil, err
	}
	if err := s.decodeSearch(searchData); err != nil {
		return nil, err
	}
	idxData, err := resident(".rrsr.idx")
	if err != nil {
		return nil, err
	}
	binRA, _, _, err := blob.ReaderAt(ctx, st, sidecarPath(prefix, m.Scheme, ".rrsr.bin"))
	if err != nil {
		return nil, fmt.Errorf("vocab: sidecar %s records: %w", m.Scheme, err)
	}
	if s.records, err = rr.OpenRecordStore(bytes.NewReader(idxData), binRA); err != nil {
		return nil, fmt.Errorf("vocab: sidecar %s record store: %w", m.Scheme, err)
	}
	return s, nil
}

func (s *sidecarScheme) decodeSearch(data []byte) error {
	r := bytes.NewReader(data)
	magic := make([]byte, 5)
	if _, err := io.ReadFull(r, magic); err != nil || string(magic[:4]) != searchMagic || magic[4] != sidecarVersion {
		return fmt.Errorf("vocab: sidecar %s search blob: bad header", s.scheme)
	}
	var count uint32
	var arenaLen uint64
	if err := binary.Read(r, binary.LittleEndian, &count); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &arenaLen); err != nil {
		return err
	}
	s.arena = make([]byte, arenaLen)
	if _, err := io.ReadFull(r, s.arena); err != nil {
		return err
	}
	s.ends = make([]uint32, count)
	s.docs = make([]uint32, count)
	if err := binary.Read(r, binary.LittleEndian, s.ends); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, s.docs); err != nil {
		return err
	}
	s.alt = make([]byte, (count+7)/8)
	if _, err := io.ReadFull(r, s.alt); err != nil {
		return err
	}
	return nil
}

func (s *sidecarScheme) norm(i int) string {
	start := uint32(0)
	if i > 0 {
		start = s.ends[i-1]
	}
	return string(s.arena[start:s.ends[i]])
}

func (s *sidecarScheme) isAlt(i int) bool { return s.alt[i/8]&(1<<(i%8)) != 0 }

// term materializes one doc, through the cache.
func (s *sidecarScheme) term(doc uint32) (*Term, bool) {
	s.mu.Lock()
	if t, ok := s.cache[doc]; ok {
		s.mu.Unlock()
		return t, true
	}
	s.mu.Unlock()
	data, ok, err := s.records.Get(doc)
	if err != nil || !ok {
		s.miss("record", err)
		return nil, false
	}
	t := &Term{}
	if err := json.Unmarshal(data, t); err != nil {
		s.miss("record decode", err)
		return nil, false
	}
	s.store(doc, t)
	return t, true
}

// termsMany materializes docs with one coalesced ranged read.
func (s *sidecarScheme) termsMany(docs []uint32) map[uint32]*Term {
	out := make(map[uint32]*Term, len(docs))
	var missing []uint32
	s.mu.Lock()
	for _, d := range docs {
		if t, ok := s.cache[d]; ok {
			out[d] = t
		} else {
			missing = append(missing, d)
		}
	}
	s.mu.Unlock()
	if len(missing) == 0 {
		return out
	}
	recs, err := s.records.GetMany(missing)
	if err != nil {
		s.miss("records", err)
		return out
	}
	for d, data := range recs {
		t := &Term{}
		if err := json.Unmarshal(data, t); err != nil {
			s.miss("record decode", err)
			continue
		}
		out[d] = t
		s.store(d, t)
	}
	return out
}

func (s *sidecarScheme) store(doc uint32, t *Term) {
	s.mu.Lock()
	if len(s.cache) >= sidecarTermCacheCap {
		s.cache = map[uint32]*Term{}
	}
	s.cache[doc] = t
	s.mu.Unlock()
}

func (s *sidecarScheme) miss(what string, err error) {
	if err != nil {
		slog.Warn("vocab: sidecar read failed; treating as miss", "scheme", s.scheme, "what", what, "err", err)
	}
}

// lookup is the Lookup gate: URI -> term, retired terms included.
func (s *sidecarScheme) lookup(uri string) (*Term, bool) {
	docs, err := s.uri.Lookup(uri)
	if err != nil {
		s.miss("uri lookup", err)
		return nil, false
	}
	if len(docs) == 0 {
		return nil, false
	}
	return s.term(docs[0])
}

// tierMatch resolves one identifier tier; postings are doc-ordered, so the
// first doc is the scheme's smallest URI (buildMatch's within-scheme rule).
func (s *sidecarScheme) tierMatch(tier int, key string) (*Term, bool) {
	docs, err := s.tiers[tier].Lookup(key)
	if err != nil {
		s.miss("id lookup", err)
		return nil, false
	}
	if len(docs) == 0 {
		return nil, false
	}
	return s.term(docs[0])
}

// searchRange returns the entry index range [lo, hi) whose norms match
// pred's prefix ordering: lo is the first norm >= q.
func (s *sidecarScheme) searchStart(q string) int {
	return sort.Search(len(s.ends), func(i int) bool { return s.norm(i) >= q })
}

// search implements prefix search with the map path's semantics: entries in
// norm order, deduped by term, live terms only (retired terms have no
// entries by construction).
func (s *sidecarScheme) search(q string, limit int) []*Term {
	var docs []uint32
	seen := map[uint32]bool{}
	for i := s.searchStart(q); i < len(s.ends) && strings.HasPrefix(s.norm(i), q); i++ {
		d := s.docs[i]
		if seen[d] {
			continue
		}
		seen[d] = true
		docs = append(docs, d)
		if len(docs) >= limit {
			break
		}
	}
	byDoc := s.termsMany(docs)
	var out []*Term
	for _, d := range docs {
		if t, ok := byDoc[d]; ok {
			out = append(out, t)
		}
	}
	return out
}

// matchLabel implements the exact-normalized-label gate.
func (s *sidecarScheme) matchLabel(q string) []LabelMatch {
	var docs []uint32
	var alts []bool
	seen := map[uint32]bool{}
	for i := s.searchStart(q); i < len(s.ends) && s.norm(i) == q; i++ {
		d := s.docs[i]
		if seen[d] {
			continue
		}
		seen[d] = true
		docs = append(docs, d)
		alts = append(alts, s.isAlt(i))
	}
	byDoc := s.termsMany(docs)
	var out []LabelMatch
	for i, d := range docs {
		if t, ok := byDoc[d]; ok {
			out = append(out, LabelMatch{Term: t, Alt: alts[i]})
		}
	}
	return out
}

// all streams every term in doc (URI) order -- the management listing.
func (s *sidecarScheme) all() []*Term {
	docs := make([]uint32, s.count)
	for i := range docs {
		docs[i] = uint32(i)
	}
	const chunk = 8192
	out := make([]*Term, 0, s.count)
	for start := 0; start < len(docs); start += chunk {
		end := min(start+chunk, len(docs))
		byDoc := s.termsMany(docs[start:end])
		for _, d := range docs[start:end] {
			if t, ok := byDoc[d]; ok {
				out = append(out, t)
			}
		}
	}
	return out
}
