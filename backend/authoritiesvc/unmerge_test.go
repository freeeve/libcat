package authoritiesvc_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/freeeve/libcat/bibframe"
	"github.com/freeeve/libcat/storage/blob"

	"github.com/freeeve/libcat/backend/authoritiesvc"
	"github.com/freeeve/libcat/backend/vocab"
)

// TestUnmergeRestoresTheMerge is the round trip: merge rewrites a carrier and
// retires the loser; un-merge replays the manifest backwards -- the carrier
// references the loser again, the retirement marker is gone, the term is
// live and searchable, and the spent manifest refuses a second reversal.
func TestUnmergeRestoresTheMerge(t *testing.T) {
	svc, st, _, _ := newService(t)
	loserID, _, err := svc.Create(t.Context(), bibframe.AuthorityTerm{
		PrefLabel: map[string]string{"en": "Trans folks"},
	}, "lib@example.org")
	if err != nil {
		t.Fatal(err)
	}
	loserURI := bibframe.LocalAuthorityIRI(loserID)
	seedWork(t, st, "wunmerge0001a", nil, &bibframe.AuthoritySubject{
		URI: loserURI, Labels: map[string]string{"en": "Trans folks"},
	}, authoritiesvc.LocalScheme)

	if _, err := svc.Merge(t.Context(), loserID, vocab.TermRef{Scheme: "homosaurus", ID: homoTransPeople}, "lib@example.org"); err != nil {
		t.Fatal(err)
	}

	result, err := svc.Unmerge(t.Context(), loserID, "lib@example.org")
	if err != nil {
		t.Fatalf("Unmerge: %v", err)
	}
	if result.ManifestWorks != 1 || result.Restored != 1 || result.Skipped != 0 || !result.Complete {
		t.Fatalf("result = %+v", result)
	}

	grain, _, err := st.Get(t.Context(), bibframe.GrainPath("wunmerge0001a"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(grain), loserURI) {
		t.Fatalf("loser not restored in carrier:\n%s", grain)
	}
	if strings.Contains(string(grain), homoTransPeople) {
		t.Fatalf("winner survives in carrier after un-merge:\n%s", grain)
	}

	// The term is live again: no retirement on lookup, back in search.
	term, ok := svc.Vocab.Lookup(authoritiesvc.LocalScheme, loserURI)
	if !ok || term.MergedInto != "" {
		t.Fatalf("index after un-merge = %+v, want live", term)
	}
	if hits := svc.Vocab.Search(authoritiesvc.LocalScheme, "trans folks", 5); len(hits) == 0 {
		t.Fatal("un-merged term still out of search")
	}

	// The manifest is spent: reversal is once per recorded merge.
	if _, err := svc.Unmerge(t.Context(), loserID, "x"); !errors.Is(err, authoritiesvc.ErrValidation) {
		t.Fatalf("second un-merge err = %v, want validation", err)
	}

	// And the cycle can start over: the live term merges again.
	if _, err := svc.Merge(t.Context(), loserID, vocab.TermRef{Scheme: "homosaurus", ID: homoTransPeople}, "x"); err != nil {
		t.Fatalf("re-merge after un-merge: %v", err)
	}
}

// TestUnmergeKeepsAWinnerTheWorkAlwaysHad covers the fusion edge: a work
// carrying BOTH terms pre-merge must end the reversal with both again --
// restoring the loser must not confiscate the winner reference the work
// legitimately had.
func TestUnmergeKeepsAWinnerTheWorkAlwaysHad(t *testing.T) {
	svc, st, _, _ := newService(t)
	loserID, _, err := svc.Create(t.Context(), bibframe.AuthorityTerm{
		PrefLabel: map[string]string{"en": "Trans folks"},
	}, "lib@example.org")
	if err != nil {
		t.Fatal(err)
	}
	loserURI := bibframe.LocalAuthorityIRI(loserID)
	seedWork(t, st, "wunmerge0002a", nil, &bibframe.AuthoritySubject{
		URI: loserURI, Labels: map[string]string{"en": "Trans folks"},
	}, authoritiesvc.LocalScheme)
	// The same work also carries the winner already.
	grain, _, _ := st.Get(t.Context(), bibframe.GrainPath("wunmerge0002a"))
	grain, err = bibframe.AppendAuthoritySubject(grain, "wunmerge0002a", bibframe.AuthoritySubject{
		URI: homoTransPeople, Labels: map[string]string{"en": "Transgender people"},
	}, "homosaurus")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Put(t.Context(), bibframe.GrainPath("wunmerge0002a"), grain, blob.PutOptions{}); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.Merge(t.Context(), loserID, vocab.TermRef{Scheme: "homosaurus", ID: homoTransPeople}, "x"); err != nil {
		t.Fatal(err)
	}
	result, err := svc.Unmerge(t.Context(), loserID, "x")
	if err != nil || result.Restored != 1 {
		t.Fatalf("Unmerge = %+v, %v", result, err)
	}
	got, _, _ := st.Get(t.Context(), bibframe.GrainPath("wunmerge0002a"))
	if !strings.Contains(string(got), loserURI) || !strings.Contains(string(got), homoTransPeople) {
		t.Fatalf("work should carry BOTH terms after the reversal:\n%s", got)
	}
}

// TestUnmergeSkipsWorksALaterMergeMoved: a manifest work that no longer
// carries the winner (something moved it again) is left alone and counted --
// the reversal never guesses. The marker still clears and the term revives.
func TestUnmergeSkipsWorksALaterMergeMoved(t *testing.T) {
	svc, st, _, _ := newService(t)
	loserID, _, err := svc.Create(t.Context(), bibframe.AuthorityTerm{
		PrefLabel: map[string]string{"en": "Trans folks"},
	}, "lib@example.org")
	if err != nil {
		t.Fatal(err)
	}
	loserURI := bibframe.LocalAuthorityIRI(loserID)
	seedWork(t, st, "wunmerge0003a", nil, &bibframe.AuthoritySubject{
		URI: loserURI, Labels: map[string]string{"en": "Trans folks"},
	}, authoritiesvc.LocalScheme)
	if _, err := svc.Merge(t.Context(), loserID, vocab.TermRef{Scheme: "homosaurus", ID: homoTransPeople}, "x"); err != nil {
		t.Fatal(err)
	}
	// Something later moves the carrier off the winner entirely.
	grain, _, _ := st.Get(t.Context(), bibframe.GrainPath("wunmerge0003a"))
	grain, err = bibframe.ReplaceSubjectReference(grain, "wunmerge0003a", homoTransPeople, bibframe.AuthoritySubject{
		URI: "https://example.org/third-term", Labels: map[string]string{"en": "Elsewhere"},
	}, "local")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Put(t.Context(), bibframe.GrainPath("wunmerge0003a"), grain, blob.PutOptions{}); err != nil {
		t.Fatal(err)
	}

	result, err := svc.Unmerge(t.Context(), loserID, "x")
	if err != nil {
		t.Fatal(err)
	}
	if result.Restored != 0 || result.Skipped != 1 || !result.Complete {
		t.Fatalf("result = %+v, want 0 restored / 1 skipped / complete", result)
	}
	if term, ok := svc.Vocab.Lookup(authoritiesvc.LocalScheme, loserURI); !ok || term.MergedInto != "" {
		t.Fatalf("term after skip-heavy un-merge = %+v, want live", term)
	}
}

// TestUnmergeRevivesACarrierlessMerge: a merge that rewrote nothing still
// retired the term, and that retirement must reverse too (caught live: the
// manifest was only written when works moved, task 405).
func TestUnmergeRevivesACarrierlessMerge(t *testing.T) {
	svc, _, _, _ := newService(t)
	loserID, _, err := svc.Create(t.Context(), bibframe.AuthorityTerm{
		PrefLabel: map[string]string{"en": "Trans folks"},
	}, "lib@example.org")
	if err != nil {
		t.Fatal(err)
	}
	loserURI := bibframe.LocalAuthorityIRI(loserID)
	if _, err := svc.Merge(t.Context(), loserID, vocab.TermRef{Scheme: "homosaurus", ID: homoTransPeople}, "x"); err != nil {
		t.Fatal(err)
	}
	result, err := svc.Unmerge(t.Context(), loserID, "x")
	if err != nil || !result.Complete || result.ManifestWorks != 0 {
		t.Fatalf("Unmerge = %+v, %v; want a complete zero-work reversal", result, err)
	}
	if term, ok := svc.Vocab.Lookup(authoritiesvc.LocalScheme, loserURI); !ok || term.MergedInto != "" {
		t.Fatalf("term = %+v, want live again", term)
	}
}

// TestUnmergeNeedsAManifest: merges made before manifests existed (or a
// never-merged term) cannot be reversed, and say so.
func TestUnmergeNeedsAManifest(t *testing.T) {
	svc, _, _, _ := newService(t)
	id, _, err := svc.Create(t.Context(), bibframe.AuthorityTerm{
		PrefLabel: map[string]string{"en": "Never merged"},
	}, "lib@example.org")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Unmerge(t.Context(), id, "x"); !errors.Is(err, authoritiesvc.ErrValidation) {
		t.Fatalf("err = %v, want validation (no recorded merge)", err)
	}
}
