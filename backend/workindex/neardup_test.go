package workindex

import (
	"fmt"
	"testing"

	"github.com/freeeve/libcat/storage/blob"
)

// grainWithInstance is the shared grain fixture plus the work->instance
// edge the summarizer walks for ISBNs (real grains always carry it).
func grainWithInstance(workID, title, author, isbn string) []byte {
	g := grain(workID, title, author, isbn, "")
	return fmt.Appendf(g, "<#%[1]sWork> <http://id.loc.gov/ontologies/bibframe/hasInstance> <#%[1]siInstance> <feed:overdrive> .\n", workID)
}

// TestNearDuplicateGroups pins the near-duplicate tiers (task 455) with the
// e2e repro shapes: a subtitle variant still groups (the exact key split it),
// a shared ISBN groups regardless of title, a contributor mismatch on one
// title-core is flagged, and no pair repeats across tiers -- exact groups
// stay out of the near list entirely.
func TestNearDuplicateGroups(t *testing.T) {
	ctx := t.Context()
	bs := blob.NewMem()
	// Exact twins: the confident tier owns this pair.
	seed(t, bs, "wexact0000001", grain("wexact0000001", "River of Teeth", "Gailey, Sarah", "911", ""))
	seed(t, bs, "wexact0000002", grain("wexact0000002", "River of Teeth", "Gailey, Sarah", "912", ""))
	// The subtitle split: same book, one carries ": poems".
	seed(t, bs, "wsub00000001", grain("wsub00000001", "full-metal indigiqueer", "Whitehead, Joshua", "101", ""))
	seed(t, bs, "wsub00000002", grain("wsub00000002", "full-metal indigiqueer: poems", "Whitehead, Joshua", "102", ""))
	// The contributor mismatch: publisher in the author slot.
	seed(t, bs, "wcontrib0001", grain("wcontrib0001", "full-metal indigiqueer", "Talonbooks", "103", ""))
	// The identifier tier: retitled entirely, but the ISBN agrees.
	seed(t, bs, "wisbn0000001", grainWithInstance("wisbn0000001", "Completely Different", "Somebody Else", "555"))
	seed(t, bs, "wisbn0000002", grainWithInstance("wisbn0000002", "Another Title Again", "Somebody Else", "555"))
	// A different book by the same author: must group nowhere.
	seed(t, bs, "wother000001", grain("wother000001", "Indigiqueerness", "Whitehead, Joshua", "104", ""))
	ix := New(bs, "data/works/")

	near, err := ix.NearDuplicateGroups(ctx)
	if err != nil {
		t.Fatal(err)
	}
	byTier := map[string][][]string{}
	for _, g := range near {
		byTier[g.Tier] = append(byTier[g.Tier], g.IDs)
	}

	findGroup := func(tier string, want ...string) bool {
	outer:
		for _, ids := range byTier[tier] {
			if len(ids) != len(want) {
				continue
			}
			for i := range want {
				if ids[i] != want[i] {
					continue outer
				}
			}
			return true
		}
		return false
	}
	if !findGroup("subtitle", "wsub00000001", "wsub00000002") {
		t.Fatalf("subtitle tier missing the ': poems' pair: %+v", byTier["subtitle"])
	}
	if !findGroup("identifier", "wisbn0000001", "wisbn0000002") {
		t.Fatalf("identifier tier missing the shared-ISBN pair: %+v", byTier["identifier"])
	}
	// The contributor tier holds the title-core group spanning authors --
	// and only the pairs the subtitle tier did not already cover count as
	// its contribution, but the group lists all its members for review.
	found := false
	for _, ids := range byTier["contributor"] {
		has := map[string]bool{}
		for _, id := range ids {
			has[id] = true
		}
		if has["wcontrib0001"] && has["wsub00000001"] {
			found = true
		}
	}
	if !found {
		t.Fatalf("contributor tier missing the publisher-in-author-slot group: %+v", byTier["contributor"])
	}
	// Exact pairs never re-report in the near tiers.
	for tier, groups := range byTier {
		for _, ids := range groups {
			has := map[string]bool{}
			for _, id := range ids {
				has[id] = true
			}
			if has["wexact0000001"] && has["wexact0000002"] {
				t.Fatalf("exact pair re-reported in tier %s", tier)
			}
		}
	}
	// The genuinely different book joined nothing.
	for tier, groups := range byTier {
		for _, ids := range groups {
			for _, id := range ids {
				if id == "wother000001" {
					t.Fatalf("a non-duplicate joined tier %s: %v", tier, ids)
				}
			}
		}
	}
}
