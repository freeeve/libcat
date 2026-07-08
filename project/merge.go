package project

import (
	"maps"
	"slices"
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
//
// The vocabulary sideband (Catalog.Terms, tasks/178) merges by term id,
// field-wise: labels fill per language (earlier catalogs win a language, like
// works win an id), broader edges union, the first non-empty scheme sticks --
// per-feed projections describe the same authority term from different
// coverage, so the union is at least as rich as any one input (tasks/180).
func Merge(cats []*Catalog) *Catalog {
	merged := &Catalog{Version: SchemaVersion}
	seen := map[string]bool{}
	terms := map[string]*Term{}
	for _, c := range cats {
		for _, w := range c.Works {
			if seen[w.ID] {
				continue
			}
			seen[w.ID] = true
			merged.Works = append(merged.Works, w)
		}
		for _, t := range c.Terms {
			cur := terms[t.ID]
			if cur == nil {
				// Private copies: later collisions fill labels/broader in
				// place, and the inputs must stay untouched.
				terms[t.ID] = &Term{ID: t.ID, Scheme: t.Scheme, Labels: maps.Clone(t.Labels), Broader: slices.Clone(t.Broader)}
				continue
			}
			for lang, label := range t.Labels {
				if _, ok := cur.Labels[lang]; !ok {
					if cur.Labels == nil {
						cur.Labels = map[string]string{}
					}
					cur.Labels[lang] = label
				}
			}
			for _, parent := range t.Broader {
				if !slices.Contains(cur.Broader, parent) {
					cur.Broader = append(cur.Broader, parent)
				}
			}
			if cur.Scheme == "" {
				cur.Scheme = t.Scheme
			}
		}
	}
	sort.Slice(merged.Works, func(i, j int) bool { return merged.Works[i].ID < merged.Works[j].ID })
	for _, id := range slices.Sorted(maps.Keys(terms)) {
		t := *terms[id]
		sort.Strings(t.Broader)
		merged.Terms = append(merged.Terms, t)
	}
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
