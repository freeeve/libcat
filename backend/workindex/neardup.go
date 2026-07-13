package workindex

import (
	"context"
	"sort"
	"strings"

	"github.com/freeeve/libcat/identity"
)

// NearDuplicateGroup is one candidate cluster the duplicate report offers
// for operator review, with the rule that produced it. Exact-key groups are
// the confident tier; the near tiers relax exactly the fields a cataloger
// is most likely to have entered differently, so false positives are cheap
// to dismiss and misses stay rare.
type NearDuplicateGroup struct {
	// Tier names the rule: "subtitle" (title-core + author + language --
	// "X" and "X: poems" cluster), "identifier" (a shared ISBN regardless
	// of title and author), "contributor" (title-core + language with
	// DIFFERING contributors -- the publisher-in-the-author-slot shape).
	Tier string
	Key  string
	IDs  []string
}

// NearDuplicateGroups returns the near-duplicate candidate tiers over the
// live works, excluding any pair the exact-key report already covers --
// each lower tier also yields only groups that add at least one new pair,
// so one underlying duplicate never repeats across tiers.
func (ix *Index) NearDuplicateGroups(ctx context.Context) ([]NearDuplicateGroup, error) {
	exact, err := ix.DuplicateGroups(ctx)
	if err != nil {
		return nil, err
	}
	sums, err := ix.Summaries(ctx)
	if err != nil {
		return nil, err
	}
	covered := map[[2]string]bool{}
	addPairs := func(ids []string) bool {
		fresh := false
		for i := 0; i < len(ids); i++ {
			for j := i + 1; j < len(ids); j++ {
				k := [2]string{ids[i], ids[j]}
				if k[0] > k[1] {
					k[0], k[1] = k[1], k[0]
				}
				if !covered[k] {
					covered[k] = true
					fresh = true
				}
			}
		}
		return fresh
	}
	for _, ids := range exact {
		addPairs(ids)
	}

	type work struct {
		id, titleCore, author, lang string
		isbns                       []string
	}
	var live []work
	for i := range sums {
		s := &sums[i]
		if s.Tombstoned || s.Suppressed || s.Title == "" {
			continue
		}
		w := work{id: s.WorkID, titleCore: identity.NormalizeKey(titleCore(s.Title))}
		if len(s.Contributors) > 0 {
			w.author = identity.NormalizeKey(s.Contributors[0])
		}
		if len(s.Languages) > 0 {
			w.lang = s.Languages[0]
		}
		for _, isbn := range s.ISBNs {
			if n := strings.ReplaceAll(strings.TrimSpace(isbn), "-", ""); n != "" {
				w.isbns = append(w.isbns, n)
			}
		}
		live = append(live, w)
	}

	var out []NearDuplicateGroup
	emit := func(tier string, byKey map[string][]string) {
		keys := make([]string, 0, len(byKey))
		for k := range keys2(byKey) {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			ids := dedupSorted(byKey[k])
			if len(ids) < 2 || !addPairs(ids) {
				continue
			}
			out = append(out, NearDuplicateGroup{Tier: tier, Key: k, IDs: ids})
		}
	}

	// Tier: subtitle -- title-core + author + language.
	sub := map[string][]string{}
	for _, w := range live {
		if w.titleCore == "" || w.author == "" {
			continue
		}
		sub[w.titleCore+"␟"+w.author+"␟"+w.lang] = append(sub[w.titleCore+"␟"+w.author+"␟"+w.lang], w.id)
	}
	emit("subtitle", sub)

	// Tier: identifier -- a shared ISBN, whatever the title says.
	byISBN := map[string][]string{}
	for _, w := range live {
		for _, n := range w.isbns {
			byISBN["isbn "+n] = append(byISBN["isbn "+n], w.id)
		}
	}
	emit("identifier", byISBN)

	// Tier: contributor mismatch -- same title-core + language, authors
	// disagree (the publisher-in-the-author-slot shape). Only groups whose
	// members span more than one author key qualify.
	byTitle := map[string][]work{}
	for _, w := range live {
		if w.titleCore == "" {
			continue
		}
		byTitle[w.titleCore+"␟"+w.lang] = append(byTitle[w.titleCore+"␟"+w.lang], w)
	}
	contrib := map[string][]string{}
	for k, ws := range byTitle {
		authors := map[string]bool{}
		ids := make([]string, 0, len(ws))
		for _, w := range ws {
			authors[w.author] = true
			ids = append(ids, w.id)
		}
		if len(authors) > 1 {
			contrib[k] = ids
		}
	}
	emit("contributor", contrib)
	return out, nil
}

// titleCore strips a trailing subtitle: the text after the first ": ".
func titleCore(title string) string {
	if i := strings.Index(title, ": "); i > 0 {
		return title[:i]
	}
	return title
}

// keys2 ranges a map's keys (kept trivial for the sorted-emit loop).
func keys2(m map[string][]string) map[string]struct{} {
	out := make(map[string]struct{}, len(m))
	for k := range m {
		out[k] = struct{}{}
	}
	return out
}

// dedupSorted returns the sorted unique ids.
func dedupSorted(ids []string) []string {
	seen := map[string]bool{}
	out := ids[:0:0]
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}
