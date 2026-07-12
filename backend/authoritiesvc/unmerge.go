package authoritiesvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/freeeve/libcat/bibframe"
	"github.com/freeeve/libcat/ingest"
	"github.com/freeeve/libcat/storage/blob"

	"github.com/freeeve/libcat/backend/publish"
	"github.com/freeeve/libcat/backend/suggest"
	"github.com/freeeve/libcat/backend/trigger"
	"github.com/freeeve/libcat/backend/vocab"
)

// mergeManifest records what one merge actually rewrote -- the reversal set.
// By un-merge time the store cannot distinguish works the merge moved from
// works that always carried the winner, so the merge writes this down while
// it still knows.
type mergeManifest struct {
	Loser  string        `json:"loser"`
	Winner vocab.TermRef `json:"winner"`
	Actor  string        `json:"actor"`
	At     time.Time     `json:"at"`
	Works  []mergedWork  `json:"works"`
}

// mergedWork is one rewritten carrier. AlsoWinner marks a work that carried
// BOTH terms before the merge fused them: un-merge must restore the loser
// WITHOUT removing the winner reference the work always had.
type mergedWork struct {
	ID         string `json:"id"`
	AlsoWinner bool   `json:"alsoWinner,omitempty"`
}

// manifestPath is where a loser's reversal set persists. One live merge per
// loser (Merge refuses a second winner while the marker stands), so the
// path keys by loser id.
func (s *Service) manifestPath(loserID string) string {
	return s.Prefix + "data/authorities/merges/" + loserID + ".json"
}

// writeMergeManifest persists the union of a prior same-winner manifest (a
// resumed merge) and this run's rewrites. A stale manifest from a different
// winner -- possible after an un-merge followed by a re-merge elsewhere --
// is replaced, not unioned.
func (s *Service) writeMergeManifest(ctx context.Context, loserID string, m mergeManifest) error {
	if prior, err := s.readMergeManifest(ctx, loserID); err == nil && prior.Winner.ID == m.Winner.ID {
		seen := map[string]bool{}
		for _, w := range m.Works {
			seen[w.ID] = true
		}
		for _, w := range prior.Works {
			if !seen[w.ID] {
				m.Works = append(m.Works, w)
			}
		}
	}
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	_, err = s.Blob.Put(ctx, s.manifestPath(loserID), data, blob.PutOptions{ContentType: "application/json"})
	return err
}

func (s *Service) readMergeManifest(ctx context.Context, loserID string) (mergeManifest, error) {
	data, _, err := s.Blob.Get(ctx, s.manifestPath(loserID))
	if err != nil {
		return mergeManifest{}, err
	}
	var m mergeManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return mergeManifest{}, err
	}
	return m, nil
}

// UnmergeResult summarizes one reversal.
type UnmergeResult struct {
	Loser  string `json:"loser"`
	Winner string `json:"winner"`
	// ManifestWorks is the recorded rewrite set's size; Restored is how many
	// were repointed back to the loser this run.
	ManifestWorks int `json:"manifestWorks"`
	Restored      int `json:"restored"`
	// Skipped counts manifest works left alone: no longer carrying the
	// winner (a later merge moved them again, or the work is gone). They
	// are reported, never guessed at.
	Skipped int `json:"skipped"`
	// Complete: every eligible work restored, the marker removed, and the
	// manifest retired -- the loser is live again.
	Complete bool `json:"complete"`
}

// Unmerge reverses a recorded merge: every work the merge's manifest names
// that still carries the winner is repointed back to the loser (a work that
// carried both terms keeps its winner reference and regains the loser), the
// loser's retirement marker is removed, and the manifest is retired. Works a
// LATER merge moved off the winner are skipped and counted -- reversal never
// guesses. Merges made before manifests existed cannot be reversed.
func (s *Service) Unmerge(ctx context.Context, loserID, actor string) (UnmergeResult, error) {
	loserURI := bibframe.LocalAuthorityIRI(loserID)
	manifest, err := s.readMergeManifest(ctx, loserID)
	if errors.Is(err, blob.ErrNotFound) {
		return UnmergeResult{}, fmt.Errorf("%w: no recorded merge for %s -- only merges that wrote a reversal manifest can be un-merged", ErrValidation, loserID)
	}
	if err != nil {
		return UnmergeResult{}, err
	}
	result := UnmergeResult{Loser: loserURI, Winner: manifest.Winner.ID, ManifestWorks: len(manifest.Works)}

	loserPath := s.grainPath(loserID)
	loserGrain, _, err := s.Blob.Get(ctx, loserPath)
	if err != nil {
		return result, err
	}
	term, err := bibframe.ParseAuthorityGrain(loserGrain, loserURI, LocalScheme)
	if err != nil {
		return result, err
	}
	// A loser somehow retired into a DIFFERENT term than the manifest
	// records is a state conflict; reversing against the manifest would
	// fight whatever produced it.
	if term.MergedInto != "" && term.MergedInto != manifest.Winner.ID {
		return result, fmt.Errorf("%w: %s is merged into %s but the manifest records %s", ErrValidation, loserID, term.MergedInto, manifest.Winner.ID)
	}
	loserSubject := bibframe.AuthoritySubject{URI: loserURI, Labels: term.PrefLabel, Broader: term.Broader}

	summaries, paths, err := ingest.SummariesOf(ctx, s.Summaries, s.Blob, s.Prefix+"data/works/")
	if err != nil {
		return result, err
	}
	byID := map[string]*ingest.WorkSummary{}
	for i := range summaries {
		byID[summaries[i].WorkID] = &summaries[i]
	}

	// Same execute-then-stamp shape as Merge: restore works, THEN clear the
	// marker, THEN retire the manifest -- a failure partway stays resumable
	// by re-issuing the un-merge (already-restored works no longer carry
	// the winner and skip).
	var changed []string
	finish := func(err error) (UnmergeResult, error) {
		note := fmt.Sprintf("%d of %d works restored (%d skipped)", result.Restored, result.ManifestWorks, result.Skipped)
		if err != nil && !result.Complete {
			note = fmt.Sprintf("partial: restored %d of %d works, then failed; retry to finish", result.Restored, result.ManifestWorks)
		}
		s.audit(ctx, suggest.AuditEntry{
			Action: "AUTHORITY_UNMERGE", Actor: actor,
			Terms: []string{LocalScheme + ":" + loserURI, manifest.Winner.Scheme + ":" + manifest.Winner.ID},
			Note:  note,
		})
		if s.Trigger != nil && len(changed) > 0 {
			_ = s.Trigger.Notify(ctx, trigger.Event{Kind: "grains-changed", Paths: changed, At: time.Now().UTC()})
		}
		rerr := s.Reload(ctx)
		if err != nil {
			return result, err
		}
		return result, rerr
	}

	for _, mw := range manifest.Works {
		summary := byID[mw.ID]
		if summary == nil || !slices.Contains(summary.Subjects, manifest.Winner.ID) {
			result.Skipped++
			continue
		}
		path := paths[mw.ID]
		workID := mw.ID
		mutate := func(old []byte) ([]byte, error) {
			if mw.AlsoWinner {
				// The work carried both terms; the merge fused them. Give
				// the loser back, leave the winner standing.
				return bibframe.AppendAuthoritySubject(old, workID, loserSubject, LocalScheme)
			}
			return bibframe.ReplaceSubjectReference(old, workID, manifest.Winner.ID, loserSubject, LocalScheme)
		}
		if _, err := publish.MutateGrain(ctx, s.Blob, path, mutate); err != nil {
			return finish(fmt.Errorf("restore %s: %w", workID, err))
		}
		result.Restored++
		changed = append(changed, path)
	}

	if _, err := publish.MutateGrain(ctx, s.Blob, loserPath, func(old []byte) ([]byte, error) {
		return bibframe.RemoveAuthorityMergeMarker(old, loserURI, manifest.Winner.ID, LocalScheme)
	}); err != nil {
		return finish(err)
	}
	changed = append(changed, loserPath)
	if err := s.Blob.Delete(ctx, s.manifestPath(loserID)); err != nil && !errors.Is(err, blob.ErrNotFound) {
		return finish(err)
	}
	result.Complete = true
	return finish(nil)
}
