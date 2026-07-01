package bibframe

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	codex "github.com/freeeve/libcodex"
	codexbf "github.com/freeeve/libcodex/bibframe"
	"github.com/freeeve/libcodex/iso2709"
	"github.com/freeeve/libcodex/rdf"
)

// BuildStats reports what a corpus build produced.
type BuildStats struct {
	Records int // records read from the source
	Grains  int // per-Work grain files written
}

// WorkID returns a stable, filesystem-safe id for a record's grain, taken from
// the control number (MARC 001) and falling back to a hash of the record when
// absent. Phase 0 only: ARCHITECTURE §4's identity model replaces this with a
// minted, provider-independent id in identity/ (Phase 1), which also changes the
// grain's subject IRIs and filename.
func WorkID(rec *codex.Record) string {
	if id := strings.TrimSpace(rec.ControlField("001")); id != "" {
		return sanitize(id)
	}
	b, _ := iso2709.Encode(rec)
	return "x" + hashID(b)[:16]
}

// GrainPath is the sharded on-disk path for a work id under root:
// <root>/data/works/<xx>/<id>.nq, sharded by a hash prefix so no directory
// holds an unbounded number of files (ARCHITECTURE §3). The shard width is
// tunable; two hex chars (256 buckets) suits a mid-size collection.
func GrainPath(root, id string) string {
	shard := hashID([]byte(id))[:2]
	return filepath.Join(root, "data", "works", shard, id+".nq")
}

// BuildCorpus writes one canonical N-Quads grain per record under root (sharded
// by WorkID) in the provider's feed graph, plus a bulk catalog.nq. The grains
// are the RDFC-1.0 canonical, diffable source of truth; catalog.nq is a derived
// bulk serialization for reindexing/download.
//
// catalog.nq is not a byte-concatenation of the grain files: each grain
// canonicalizes its blank nodes to _:c14nN independently, so concatenating them
// would merge distinct blanks that happen to share a label. It is instead
// re-serialized from the records through one shared encoder, keeping blank
// labels unique across the corpus. All records are held in memory for the sorted
// bulk write; at large scale (ARCHITECTURE §3, >10M records) that becomes an
// out-of-core concern.
func BuildCorpus(root string, records []*codex.Record, provider string) (BuildStats, error) {
	feed := FeedGraph(provider)
	stats := BuildStats{Records: len(records)}

	type entry struct {
		id  string
		rec *codex.Record
	}
	entries := make([]entry, 0, len(records))
	for _, rec := range records {
		id := WorkID(rec)
		grain, err := Grain(rec, feed)
		if err != nil {
			return stats, fmt.Errorf("grain %s: %w", id, err)
		}
		if err := writeFile(GrainPath(root, id), grain); err != nil {
			return stats, err
		}
		stats.Grains++
		entries = append(entries, entry{id, rec})
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].id < entries[j].id })
	sorted := make([]*codex.Record, len(entries))
	for i, e := range entries {
		sorted[i] = e.rec
	}
	if err := codexbf.WriteNQuadsFile(filepath.Join(root, "catalog.nq"), sorted,
		func(*codex.Record) rdf.Term { return feed }); err != nil {
		return stats, fmt.Errorf("write catalog.nq: %w", err)
	}
	return stats, nil
}

// BuildFromMARC reads an ISO 2709 (.mrc) MARC file -- e.g. an OverDrive
// Marketplace MARC Express export -- and builds the corpus under root.
func BuildFromMARC(root, marcPath, provider string) (BuildStats, error) {
	recs, err := iso2709.ReadFile(marcPath)
	if err != nil {
		return BuildStats{}, fmt.Errorf("read marc %s: %w", marcPath, err)
	}
	return BuildCorpus(root, recs, provider)
}

// writeFile creates the parent shard directory and writes data.
func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// hashID is the hex SHA-256 of b, used for shard prefixes and id fallbacks.
func hashID(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// sanitize maps a raw id to a filesystem-safe token, replacing anything outside
// [A-Za-z0-9._-] with an underscore.
func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '.', r == '_', r == '-':
			return r
		default:
			return '_'
		}
	}, s)
}
