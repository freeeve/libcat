package project

import (
	"sort"
	"strings"
)

// Merge unions per-feed projections by work id (tasks/172). Project views one
// provenance graph at a time (feed:<provider> + editorial:), so a multi-feed
// corpus projects each feed separately and merges here: earlier catalogs win a
// shared id -- list the primary feed first, its records are richer than a
// sidecar's -- and the result is sorted by id like Project's output. The input
// catalog.nq must cover every feed's works; after a multi-feed ingest, `lcat
// serialize` regenerates it, since each ingest run rewrites catalog.nq with
// only its own run's works.
func Merge(cats []*Catalog) *Catalog {
	merged := &Catalog{Version: SchemaVersion}
	seen := map[string]bool{}
	for _, c := range cats {
		for _, w := range c.Works {
			if seen[w.ID] {
				continue
			}
			seen[w.ID] = true
			merged.Works = append(merged.Works, w)
		}
	}
	sort.Slice(merged.Works, func(i, j int) bool { return merged.Works[i].ID < merged.Works[j].ID })
	return merged
}

// SanitizeSources rewrites every Work's extra "sources" attribution list to
// the public allowlist, dropping the key when nothing public remains, and
// returns how many attributions were stripped (tasks/172). Provenance under
// lcat:extra/sources may name sources whose attribution must not leak on a
// public surface; the same allowlist governs the nq download (export
// package). Values are comma-joined by convention and compared trimmed; kept
// values are re-joined ", " and keep their order.
func SanitizeSources(cat *Catalog, public map[string]bool) int {
	stripped := 0
	for i := range cat.Works {
		e := cat.Works[i].Extra
		raw, ok := e["sources"]
		if !ok {
			continue
		}
		var kept []string
		for s := range strings.SplitSeq(raw, ",") {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			if public[s] {
				kept = append(kept, s)
			} else {
				stripped++
			}
		}
		if len(kept) == 0 {
			delete(e, "sources")
		} else {
			e["sources"] = strings.Join(kept, ", ")
		}
	}
	return stripped
}

// SourceSet parses a comma-separated source-name list into the allowlist set
// SanitizeSources and the export nq filter consume. Names are trimmed; empty
// entries are ignored, so "" yields an empty (strip-everything) set.
func SourceSet(csv string) map[string]bool {
	set := map[string]bool{}
	for s := range strings.SplitSeq(csv, ",") {
		if s = strings.TrimSpace(s); s != "" {
			set[s] = true
		}
	}
	return set
}
