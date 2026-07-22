package identity

import (
	"fmt"
	"sort"
	"strings"
)

// Record is the identity-relevant projection of one incoming provider record:
// the keys it can be resolved by and the fields of its computed clustering key.
type Record struct {
	// ProviderKeys are resolution keys in priority order, each namespaced so keys
	// from different schemes never collide, e.g. "overdrive:3389970" then
	// "isbn:9781682308233". The first key that already maps wins; ISBN is the
	// cross-provider merge key (ARCHITECTURE §9).
	ProviderKeys []string
	Author       string
	Title        string
	// Langs is the work's full language set. The cluster key sorts and dedupes
	// it, so translations stay distinct Works (one Work per language) while a
	// flattened multi-language node keys on a stable set, not an arbitrary first
	// language.
	Langs []string
	// MatchKey is the un-namespaced cross-feed clustering key a feed that
	// namespaces its intra-feed key (so its own records never re-merge) sets to
	// opt this record into the cross-feed dedup: the raw author+title+language-set
	// key, computed with WorkKeySet. Empty for a feed whose cluster key is
	// already un-namespaced -- it clusters cross-feed on the cluster key directly.
	MatchKey string
	// Anchors are the record's work-level anchor keys (namespaced, e.g.
	// "oclcwork:12345"), a stable work identifier a provider supplies. When one
	// resolves to a committed Work, it clusters this record onto it AHEAD of the
	// fuzzy author+title+language key -- so editions and cross-feed duplicates a
	// shared work id names merge even where their access points differ. The
	// anchor is language-scoped at resolution, so translations stay distinct.
	Anchors []string
}

// Assignment is the resolved identity for a record: its stable Instance and Work
// ids, and whether either was freshly minted this ingest (vs resolved to an
// already-committed id).
type Assignment struct {
	InstanceID     string
	WorkID         string
	MintedInstance bool
	MintedWork     bool
}

// Merge is an editorial merge decision recovered from the grains (ARCHITECTURE
// §4): every reference to From resolves to To. It is the under-merge fix -- two
// records that should be one Work but clustered apart -- and the computed key
// cannot undo it. Chains collapse to a single canonical id.
type Merge struct {
	From string
	To   string
}

// Pin is an editorial split decision recovered from the grains (ARCHITECTURE §4):
// Instance is assigned to Work regardless of the computed clustering key. It is the
// over-merge fix -- records the key wrongly clustered together are pinned apart --
// and makes the split reproducible across re-ingest.
type Pin struct {
	Instance string
	Work     string
}

// Resolver assigns stable Work/Instance ids across ingests. Seed it with the
// identity already committed (from the grains), then Resolve each incoming record
// to an existing id or a freshly minted one. It is the mint-or-resolve core of
// ARCHITECTURE §4 / an unchanged record keeps its previously minted
// ids, so its grain -- and its public URL -- do not churn.
//
// A Resolver is not safe for concurrent use; ingest is single-threaded per
// corpus.
type Resolver struct {
	instByProvider map[string]string // provider key -> instance id
	workByInst     map[string]string // instance id -> work id
	workByKey      map[string]string // computed cluster key -> work id
	workByAnchor   map[string]string // language-scoped work anchor -> work id
	mergedInto     map[string]string // work id -> canonical work id (editorial overlay)
	pinByInst      map[string]string // instance id -> pinned work id (editorial split overlay)
	usedInst       map[string]bool
	usedWork       map[string]bool
	// feed is the provenance feed of the run currently resolving, so the
	// cross-feed dedup never bridges a namespaced feed's record onto its own
	// prior work (which its intra-feed key deliberately kept distinct).
	feed string
	// seedKeyWorks maps a committed Work's (un-namespaced) cluster key to the set
	// of prior work ids under it, and seedWorkFeeds the feeds each prior work
	// came from -- the cross-feed dedup bridges an un-namespaced match key to a
	// single prior work from a different feed.
	seedKeyWorks  map[string]map[string]bool
	seedWorkFeeds map[string]map[string]bool
	// conflicts records provider keys seen mapped to more than one instance
	//: surfaced rather than silently remapped.
	conflicts []string
}

// NewResolver returns an empty resolver. Seed it from the committed identity
// before resolving new records.
func NewResolver() *Resolver {
	return &Resolver{
		instByProvider: map[string]string{},
		workByInst:     map[string]string{},
		workByKey:      map[string]string{},
		workByAnchor:   map[string]string{},
		mergedInto:     map[string]string{},
		pinByInst:      map[string]string{},
		usedInst:       map[string]bool{},
		usedWork:       map[string]bool{},
		seedKeyWorks:   map[string]map[string]bool{},
		seedWorkFeeds:  map[string]map[string]bool{},
	}
}

// SeedInstance records a previously minted Instance: its id, the provider keys it
// answers to, and the Work it belongs to. Called once per committed Instance
// before ingest so re-ingest resolves rather than re-mints.
func (r *Resolver) SeedInstance(instanceID, workID string, providerKeys []string) {
	r.usedInst[instanceID] = true
	r.usedWork[workID] = true
	r.workByInst[instanceID] = workID
	for _, k := range providerKeys {
		r.instByProvider[k] = instanceID
	}
}

// SeedWorkKey records the computed clustering key of a committed Work, so a new
// record with the same key clusters onto it. The caller recomputes the key from
// the Work's data with WorkKeySet. The key->works multiplicity is tracked so the
// cross-feed dedup can tell an unambiguous bridge target from a key several
// distinct prior works share.
func (r *Resolver) SeedWorkKey(clusterKey, workID string) {
	r.usedWork[workID] = true
	if clusterKey != "" {
		r.workByKey[clusterKey] = workID
		if r.seedKeyWorks[clusterKey] == nil {
			r.seedKeyWorks[clusterKey] = map[string]bool{}
		}
		r.seedKeyWorks[clusterKey][workID] = true
	}
}

// SeedWorkAnchor records a committed Work's language-scoped work anchor, so a
// new record carrying the same anchor clusters onto it ahead of the fuzzy key.
// The key is already language-scoped by the scanner (anchorKey), matching the
// form Resolve computes from a record's Anchors and Langs. First seeded wins: a
// re-seed of the same anchor to a different Work is ignored, so a re-ingest
// cannot silently re-home an anchored Work.
func (r *Resolver) SeedWorkAnchor(anchorKey, workID string) {
	if anchorKey == "" {
		return
	}
	r.usedWork[workID] = true
	if _, ok := r.workByAnchor[anchorKey]; !ok {
		r.workByAnchor[anchorKey] = workID
	}
}

// SeedWorkFeed tags a committed Work with a feed it was recovered from (empty
// feeds are ignored). A multi-feed Work is tagged once per feed, so the
// cross-feed dedup can exclude bridge targets a namespaced feed already
// contributes to.
func (r *Resolver) SeedWorkFeed(workID, feed string) {
	if feed == "" {
		return
	}
	if r.seedWorkFeeds[workID] == nil {
		r.seedWorkFeeds[workID] = map[string]bool{}
	}
	r.seedWorkFeeds[workID][feed] = true
}

// SetFeed records the feed of the run currently resolving, so CrossFeedMerges
// never bridges one of this feed's records onto a prior Work from the same feed.
func (r *Resolver) SetFeed(feed string) { r.feed = feed }

// SeedMerge records an editorial merge: every reference to from
// resolves to to. Merges override the computed key, so a re-ingest cannot undo a
// human decision. Chains collapse to a single canonical id.
func (r *Resolver) SeedMerge(from, to string) {
	r.mergedInto[from] = to
}

// SeedPin records an editorial split pin: instanceID is assigned to
// workID regardless of the computed clustering key, so an over-merge the key would
// otherwise recreate stays split across re-ingest. The pinned Work id is reserved
// so it is never minted for anything else.
//
// Two pins for one instance is a data-integrity event (a split written twice before
// made the endpoint idempotent, or a hand-edited grain). The first pin seen
// wins deterministically -- pins arrive in canonical quad order, so the choice no
// longer depends on which random Work id sorts higher -- and the second is reported
// rather than silently dropped. The discarded id is not reserved: it denotes nothing,
// and reserving it would burn an id out of the space for good.
func (r *Resolver) SeedPin(instanceID, workID string) {
	if prev, ok := r.pinByInst[instanceID]; ok && prev != workID {
		r.conflicts = append(r.conflicts,
			fmt.Sprintf("instance %s is pinned to both %s and %s; keeping %s", instanceID, prev, workID, prev))
		return
	}
	r.pinByInst[instanceID] = workID
	r.usedWork[workID] = true
}

// Resolve returns the stable identity for a record, minting only what is genuinely
// new. An Instance resolves by its first already-known provider key. Its Work is
// an editorial pin if one exists (an over-merge split, applied first), else the
// existing instance->work link, else the computed cluster key, else a freshly
// minted Work. Editorial merges are applied last, so a pinned or clustered Work
// still follows a later merge to its survivor.
func (r *Resolver) Resolve(rec Record) Assignment {
	instanceID, mintedInst := r.resolveInstance(rec.ProviderKeys)

	mintedWork := false
	workID, pinned := r.pinByInst[instanceID]
	if !pinned {
		var ok bool
		workID, ok = r.workByInst[instanceID]
		if !ok {
			key := WorkKeySet(rec.Author, rec.Title, rec.Langs)
			anchored := r.anchorTarget(rec.Anchors, rec.Langs)
			switch {
			case anchored != "":
				// A stable work-level anchor (OCLC work id / LCCN) resolves ahead
				// of the fuzzy key, so a shared work id clusters editions and
				// cross-feed duplicates even where their access points differ.
				workID = anchored
			case key != "" && r.workByKey[key] != "":
				workID = r.workByKey[key]
			default:
				workID = r.mint(WorkPrefix, r.usedWork)
				if key != "" {
					r.workByKey[key] = workID
				}
				mintedWork = true
			}
			// Bind this record's anchors onto the resolved Work so a later
			// record (any feed) carrying one resolves here. First binding wins,
			// so a fuzzy-clustered anchor cannot later be re-homed.
			r.bindAnchors(rec.Anchors, rec.Langs, workID)
		}
	}
	r.workByInst[instanceID] = workID

	return Assignment{
		InstanceID:     instanceID,
		WorkID:         r.canonical(workID),
		MintedInstance: mintedInst,
		MintedWork:     mintedWork,
	}
}

// anchorTarget returns the committed Work a record's work-level anchors resolve
// to -- the first anchor (in the record's priority order), language-scoped, that
// is already bound -- following the merge overlay to the survivor. Empty when no
// anchor is known, so the caller falls back to the fuzzy cluster key.
func (r *Resolver) anchorTarget(anchors, langs []string) string {
	for _, a := range anchors {
		if wid, ok := r.workByAnchor[anchorKey(a, langs)]; ok {
			return r.canonical(wid)
		}
	}
	return ""
}

// bindAnchors records each of a record's anchors (language-scoped) against the
// Work it resolved to, so a later record carrying the same anchor clusters here.
// First binding wins, matching SeedWorkAnchor: a Work first reached by the fuzzy
// key still claims its anchors, and a re-ingest cannot re-home them.
func (r *Resolver) bindAnchors(anchors, langs []string, workID string) {
	for _, a := range anchors {
		k := anchorKey(a, langs)
		if _, ok := r.workByAnchor[k]; !ok {
			r.workByAnchor[k] = workID
		}
	}
}

// resolveInstance finds the instance for a record's keys, minting a new one when
// none is known. It binds every key to the resolved instance, recording a
// conflict when a key was already bound to a different instance rather than
// silently remapping it.
func (r *Resolver) resolveInstance(keys []string) (string, bool) {
	instanceID := ""
	for _, k := range keys {
		if id, ok := r.instByProvider[k]; ok {
			instanceID = id
			break
		}
	}
	minted := false
	if instanceID == "" {
		instanceID = r.mint(InstancePrefix, r.usedInst)
		minted = true
	}
	for _, k := range keys {
		if prev, ok := r.instByProvider[k]; ok && prev != instanceID {
			r.conflicts = append(r.conflicts,
				fmt.Sprintf("provider key %q maps to both %s and %s", k, prev, instanceID))
			continue
		}
		r.instByProvider[k] = instanceID
	}
	return instanceID, minted
}

// WorkForProviderKey returns the Work id a provider key resolves to when the
// resolver already knows it (seeded from a prior grain), following merges -- and
// nothing (false) otherwise. It has no side effects, unlike Resolve. Used to
// translate a feed's cluster-merge statement, which names records by provider id,
// into the Work-id space SeedMerge operates on.
//
// When the exact key is not indexed, it falls back to the cluster's format
// buckets: a single-format cluster's grain indexes only the
// format-suffixed instance key (e.g. "id:coll:51812:ebook"), never the bare
// cluster key ("id:coll:51812") a feed merge names, so the exact lookup misses
// and the merge is skipped. Every instance of a cluster resolves to the same
// Work, so the bare key is resolved through any indexed key that extends it with
// a ":<suffix>". The trailing colon keeps "id:coll:5181" from matching
// "id:coll:51812:..."; agreement across all matching buckets is required, so a
// cluster already split across Works resolves to nothing rather than guessing.
func (r *Resolver) WorkForProviderKey(key string) (string, bool) {
	if inst, ok := r.instByProvider[key]; ok {
		work, ok := r.workByInst[inst]
		if !ok {
			return "", false
		}
		return r.canonical(work), true
	}
	prefix := key + ":"
	found := ""
	for k, inst := range r.instByProvider {
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		work, ok := r.workByInst[inst]
		if !ok {
			continue
		}
		switch c := r.canonical(work); {
		case found == "":
			found = c
		case found != c:
			return "", false // cluster split across Works: ambiguous, do not guess
		}
	}
	if found == "" {
		return "", false
	}
	return found, true
}

// Merges returns every merge the resolver is applying -- editorial merges seeded
// from prior grains and feed cluster-merges alike -- as From->To
// pairs in deterministic order. The retirement pass diffs on this so a Work folded
// away by a feed's isReplacedBy has its stale grain removed, not just its records
// re-homed: without it the retired cluster's grain lingers and keeps duplicating
// the survivor's identifiers.
func (r *Resolver) Merges() []Merge {
	out := make([]Merge, 0, len(r.mergedInto))
	for from, to := range r.mergedInto {
		out = append(out, Merge{From: from, To: to})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].From != out[j].From {
			return out[i].From < out[j].From
		}
		return out[i].To < out[j].To
	})
	return out
}

// CrossFeedMerges computes the cross-feed dedup: each of this run's freshly
// minted Works whose un-namespaced match key resolves to exactly one prior Work
// from a DIFFERENT feed is merged onto that prior Work. This lets a feed that
// namespaces its intra-feed key (so its own records never re-merge) still
// deduplicate against the other providers -- the same title arriving from
// OverDrive and coll clusters into one Work. It refuses to bridge an ambiguous
// key rather than guess: two of this run's Works claiming one match key (the
// namespaced feed's own genuine collision), or two distinct prior Works from
// other feeds answering to it. workMatch maps each candidate Work id to its
// match key; a "" match key is skipped.
func (r *Resolver) CrossFeedMerges(workMatch map[string]string) []Merge {
	perMatch := map[string]int{}
	for _, m := range workMatch {
		if m != "" {
			perMatch[m]++
		}
	}
	ids := make([]string, 0, len(workMatch))
	for id := range workMatch {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var out []Merge
	for _, wid := range ids {
		m := workMatch[wid]
		if m == "" || perMatch[m] >= 2 {
			continue
		}
		target, ok := r.singleOtherFeedTarget(m)
		if !ok || target == r.canonical(wid) {
			continue
		}
		out = append(out, Merge{From: wid, To: target})
	}
	return out
}

// singleOtherFeedTarget returns the one prior Work under match key that belongs
// to a feed other than the current run's -- the unambiguous cross-feed bridge
// target -- or false when there is none or several distinct ones (a same-key
// collision the dedup must not guess through).
func (r *Resolver) singleOtherFeedTarget(matchKey string) (string, bool) {
	found := ""
	for wid := range r.seedKeyWorks[matchKey] {
		if r.seedWorkFeeds[wid][r.feed] {
			continue // this feed's own prior work: never a bridge target
		}
		switch c := r.canonical(wid); {
		case found == "":
			found = c
		case found != c:
			return "", false // two distinct prior works: ambiguous
		}
	}
	if found == "" {
		return "", false
	}
	return found, true
}

// Canonical returns the surviving Work id for a (possibly merged-away) Work,
// following the merge overlay -- the id an ingest run must key a bridged Work's
// grain and presence entry by.
func (r *Resolver) Canonical(workID string) string { return r.canonical(workID) }

// TranslationTargets computes the bf:translationOf links for this run's Works.
// Works that share an author+title base but carry different language sets are
// language siblings (BIBFRAME one-Work-per-language: a translation is a distinct
// Work). Each such sibling links to its group's representative -- the Work under
// the lexicographically smallest language set (tie-broken by id), so the whole
// language cluster points at one primary expression (English sorts ahead of most
// translations). workKey maps each of this run's Work ids to its full cluster
// key (author+title+language-set); prior Works from the committed grains are
// folded in as sibling candidates. A Work that is its group's representative, is
// language-less, or has no differing-language sibling gets no entry.
//
// The links are recomputed each ingest, so as a feed re-ingests they converge;
// an edge can lag until then when a later ingest introduces a new representative.
func (r *Resolver) TranslationTargets(workKey map[string]string) map[string]string {
	type member struct{ workID, langs string }
	base := map[string][]member{}
	langsInBase := map[string]map[string]bool{}
	add := func(fullKey, workID string) {
		b, langs, ok := splitBaseLangs(fullKey)
		if !ok || langs == "" {
			return
		}
		cid := r.canonical(workID)
		base[b] = append(base[b], member{cid, langs})
		if langsInBase[b] == nil {
			langsInBase[b] = map[string]bool{}
		}
		langsInBase[b][langs] = true
	}
	for fk, ids := range r.seedKeyWorks {
		for id := range ids {
			add(fk, id)
		}
	}
	for id, fk := range workKey {
		add(fk, id)
	}

	rep := map[string]member{}
	for b, ms := range base {
		best := member{}
		for _, m := range ms {
			if best.workID == "" || m.langs < best.langs || (m.langs == best.langs && m.workID < best.workID) {
				best = m
			}
		}
		rep[b] = best
	}

	out := map[string]string{}
	for id, fk := range workKey {
		b, langs, ok := splitBaseLangs(fk)
		if !ok || langs == "" || len(langsInBase[b]) < 2 {
			continue
		}
		if cid := r.canonical(id); cid != rep[b].workID {
			out[id] = rep[b].workID
		}
	}
	return out
}

// canonical follows the editorial merge chain to the surviving Work id.
func (r *Resolver) canonical(workID string) string {
	seen := map[string]bool{}
	for {
		to, ok := r.mergedInto[workID]
		if !ok || seen[workID] {
			return workID
		}
		seen[workID] = true
		workID = to
	}
}

// Conflicts returns provider-key collisions seen during resolution (
// §4), for the caller to surface. Nil when there were none.
func (r *Resolver) Conflicts() []string { return r.conflicts }

// mint draws unused ids so a (vanishingly unlikely) crypto/rand collision can
// never alias two records to one id.
func (r *Resolver) mint(prefix string, used map[string]bool) string {
	for {
		id := Mint(prefix)
		if !used[id] {
			used[id] = true
			return id
		}
	}
}
