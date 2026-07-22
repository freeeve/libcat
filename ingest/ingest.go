package ingest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/freeeve/libcat/bibframe"
	"github.com/freeeve/libcat/identity"
	"github.com/freeeve/libcat/storage"
)

// Result is what one ingest run produced: the grain/instance counts, how many ids
// were freshly minted at each tier, how many merge-retired grains were dropped, and
// any resolver conflicts to surface. The pipeline returns this rather than printing
// so the CLI (or a cloud handler) owns presentation.
type Result struct {
	Stats           bibframe.BuildStats
	MintedWorks     int
	MintedInstances int
	Retired         int
	Conflicts       []string
	// WorkIDs are the Works this run's records resolved to -- the presence
	// set the feed reconciliation pass diffs the corpus against.
	WorkIDs []string
}

// Run ingests a provider's records into canonical grains under out, the shared
// direct-BIBFRAME pipeline every ingest provider uses (ARCHITECTURE §4/§9). It
// seeds the resolver from any grains already under out (so re-ingest reuses ids and
// clusters editions), applies committed editorial merges/pins, resolves each record
// to stable two-tier ids, groups records into Works (first record wins shared
// metadata; a duplicate Instance -- e.g. a shared ISBN -- is emitted once), carries
// each Work's preserved editorial statements across the feed rewrite, writes one
// grain per Work plus catalog.nq, and drops the grains of merge-retired Works. The
// run is deterministic in record order. Only ingest-role providers are executed.
func Run(prov Provider, out string) (Result, error) {
	if prov.Role() != RoleIngest {
		return Result{}, fmt.Errorf("ingest: provider %q has role %s, not ingest", prov.Name(), prov.Role())
	}
	feed := prov.Name()

	recs, err := prov.Records(context.Background())
	if err != nil {
		return Result{}, fmt.Errorf("provider %q records: %w", feed, err)
	}

	prior, err := bibframe.LoadPrior(out, feed)
	if err != nil {
		return Result{}, fmt.Errorf("load prior grains: %w", err)
	}
	var seeds []MergeSeed
	if ms, ok := prov.(MergeSeeder); ok {
		seeds = ms.MergeSeeds()
	}
	works, res, r := cluster(recs, prior, seeds, feed)

	stats, err := bibframe.BuildWorks(storage.Dir(out), works, feed)
	if err != nil {
		return res, err
	}
	res.Stats = stats

	retired, err := removeRetiredGrains(out, r.Merges())
	if err != nil {
		return res, err
	}
	res.Retired = retired
	res.Conflicts = r.Conflicts()
	return res, nil
}

// cluster is the provider-independent middle of an ingest run: it seeds the
// resolver from the prior grains (identity map, editorial merges, split
// pins), resolves every record to its stable two-tier ids, groups records
// into WorkGroups (first record wins shared metadata and per-record
// capabilities; duplicate Instances emit once), and carries each Work's
// preserved editorial statements. Shared by the directory Run and the
// store-backed RunStore.
func cluster(recs []Record, prior bibframe.Prior, mergeSeeds []MergeSeed, feed string) ([]bibframe.WorkGroup, Result, *identity.Resolver) {
	r := identity.NewResolver()
	r.SetFeed(feed)
	identity.SeedResolver(r, prior.Grains)
	// Seed editorial merges and split pins: a merge resolves a retired
	// Work's Instances onto the survivor; a pin forces an over-merged Instance onto
	// its split-off Work. Neither can be undone by the computed key.
	for _, m := range prior.Merges {
		r.SeedMerge(m.From, m.To)
	}
	for _, p := range prior.Pins {
		r.SeedPin(p.Instance, p.Work)
	}
	// Seed feed cluster-merges: a source that folded one cluster into
	// another names the pair by provider id; translate each to Work ids through the
	// now-seeded resolver and merge the retired Work onto the survivor, so a
	// re-clustered record resolves to the survivor's prior grain instead of orphaning
	// one. A merge whose ids the resolver does not know (no prior grain) is skipped.
	for _, m := range mergeSeeds {
		from, okF := r.WorkForProviderKey(m.FromKey)
		to, okT := r.WorkForProviderKey(m.ToKey)
		if okF && okT && from != to {
			r.SeedMerge(from, to)
		}
	}

	byWork := map[string]*bibframe.WorkGroup{}
	seenInstance := map[string]bool{}
	// workMatch records each freshly minted Work's un-namespaced cross-feed match
	// key, so the cross-feed dedup pass can bridge it onto an unambiguous prior
	// Work from another provider. firstKey records every Work's un-namespaced
	// full cluster key (author+title+language-set) for the bf:translationOf pass,
	// first record winning.
	workMatch := map[string]string{}
	firstKey := map[string]string{}
	var res Result
	for _, rec := range recs {
		im := rec.Identity()
		a := r.Resolve(im)
		if a.MintedWork {
			res.MintedWorks++
			workMatch[a.WorkID] = crossFeedMatchKey(im)
		}
		if a.MintedInstance {
			res.MintedInstances++
		}
		wg, ok := byWork[a.WorkID]
		if !ok {
			firstKey[a.WorkID] = crossFeedMatchKey(im)
			wg = &bibframe.WorkGroup{WorkID: a.WorkID, Work: rec.Work()}
			// The first record of a clustered Work also supplies its non-BIBFRAME
			// display extras (cover/rating/dateRead), carried through to catalog.json's
			// `extra` object via the feed provenance graph.
			if ep, ok := rec.(ExtraProvider); ok {
				wg.Extras = ep.Extras()
			}
			// The first record likewise supplies the Work's work-level anchors
			// (OCLC work id / LCCN), emitted onto the Work node so a re-ingest
			// and other feeds cluster onto it ahead of the fuzzy key.
			if wa, ok := rec.(WorkAnchorer); ok {
				wg.Anchors = wa.WorkAnchors()
			}
			// The first record likewise contributes the Work's controlled subjects
			// (authority URIs + labels + broader), emitted into the feed graph.
			if se, ok := rec.(SubjectEnricher); ok {
				wg.Subjects = se.ControlledSubjects()
			}
			// And its standalone term descriptions (ancestor chains): labels +
			// hierarchy only, no subject link.
			if td, ok := rec.(TermDescriber); ok {
				wg.Terms = td.DescribedTerms()
			}
			// And its contributors' authority identities: the extra
			// owl:sameAs statements on IRI agent nodes.
			if ai, ok := rec.(AgentIdentifier); ok {
				for _, a := range ai.AgentIdentities() {
					wg.Agents = append(wg.Agents, bibframe.AgentIdentity(a))
				}
			}
			byWork[a.WorkID] = wg
		}
		if seenInstance[a.InstanceID] {
			continue
		}
		seenInstance[a.InstanceID] = true
		gi := bibframe.GroupInstance{
			InstanceID: a.InstanceID,
			Instance:   rec.Instance(),
		}
		// A MARC-sourced record's crosswalk-lossy fields ride along verbatim
		//, so the loss table stays a rendering concern, not a
		// data loss.
		if vp, ok := rec.(VerbatimProvider); ok {
			gi.Verbatim = vp.Verbatim()
		}
		wg.Instances = append(wg.Instances, gi)
	}

	// Cross-feed dedup: fold each freshly minted Work onto an unambiguous prior
	// Work from another feed sharing its un-namespaced match key, so a namespaced
	// feed (coll) deduplicates against the other providers. Applied as merges, so
	// the folded records land in the surviving Work's grain (multi-feed) and a
	// later re-ingest resolves them by instance id with no re-bridging.
	for _, m := range r.CrossFeedMerges(workMatch) {
		r.SeedMerge(m.From, m.To)
	}

	// Fold the run's Work groups through the merge overlay: a bridged group's
	// instances move onto the surviving (canonical) Work id, first source id
	// winning the shared Work metadata for determinism.
	byCanon := map[string]*bibframe.WorkGroup{}
	canonKey := map[string]string{}
	canonOrder := make([]string, 0, len(byWork))
	srcIDs := make([]string, 0, len(byWork))
	for id := range byWork {
		srcIDs = append(srcIDs, id)
	}
	sort.Strings(srcIDs)
	for _, id := range srcIDs {
		wg := byWork[id]
		canon := r.Canonical(id)
		wg.WorkID = canon
		if existing, ok := byCanon[canon]; ok {
			existing.Instances = append(existing.Instances, wg.Instances...)
			continue
		}
		byCanon[canon] = wg
		canonKey[canon] = firstKey[id]
		canonOrder = append(canonOrder, canon)
	}
	sort.Strings(canonOrder)
	res.WorkIDs = canonOrder

	// Link each Work to its language-sibling primary expression (bf:translationOf):
	// same author+title, different language set -- distinct Works under
	// one-Work-per-language, related rather than merged.
	translations := r.TranslationTargets(canonKey)

	works := make([]bibframe.WorkGroup, 0, len(canonOrder))
	for _, id := range canonOrder {
		wg := byCanon[id]
		// Carry the Work's committed editorial statements across the re-ingest so the
		// feed rewrite does not clobber them (ARCHITECTURE §5). For a bridged Work the
		// canonical id keys the other feed's preserved grain, so its statements ride
		// along and the grain stays multi-feed.
		wg.Editorial = prior.Editorial[id]
		if target := translations[id]; target != "" {
			wg.TranslationOf = []string{target}
		}
		works = append(works, *wg)
	}
	return works, res, r
}

// crossFeedMatchKey is a record's un-namespaced cross-feed clustering key: the
// explicit MatchKey a namespacing feed sets, else its cluster key (already
// un-namespaced for a feed that does not namespace). It is what the cross-feed
// dedup bridges on.
func crossFeedMatchKey(im identity.Record) string {
	if im.MatchKey != "" {
		return im.MatchKey
	}
	return identity.WorkKeySet(im.Author, im.Title, im.Langs)
}

// removeRetiredGrains deletes the per-Work grain file of every Work retired by a
// merge and returns how many were removed. A retired grain that is already gone (a
// re-ingest after the first merge-aware run) is not an error.
func removeRetiredGrains(dir string, merges []identity.Merge) (int, error) {
	n := 0
	for _, id := range bibframe.RetiredWorks(merges) {
		path := filepath.Join(dir, filepath.FromSlash(bibframe.GrainPath(id)))
		if err := os.Remove(path); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return n, fmt.Errorf("remove retired grain %s: %w", id, err)
		}
		n++
	}
	return n, nil
}
