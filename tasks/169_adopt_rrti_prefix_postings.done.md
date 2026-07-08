# 169: Adopt roaringrange v0.30.0 RRTI prefix postings for vocab typeahead

## Shipped (2026-07-08)

roaringrange bumped to v0.30.0 (backend + root require). The sidecar's
search artifact is now `<scheme>.search.rrt` -- an RRTI whose dictionary
terms are the normalized labels and whose postings carry `doc<<1|alt`
encoded IDs (case-sensitive header, no language filters: norms are
pre-folded by normLabel on both sides). Search serves via PrefixPostings
with the map path's semantics (labels in norm order, deduped by term); a
truncated term window that underfills the limit after dedup grows 4x and
retries. MatchLabel resolves the exact label with a single-term prefix
read (the exact norm sorts first among its own prefix matches). Only the
router FST stays resident; the LCVS arena/ends/docs/alt columns are gone.

sidecarVersion bumped to 2: v1 manifests are ignored, so existing
deployments serve from maps until `lcatd vocab-index` re-runs (a rebuild
also deletes the orphaned `.search.bin`).

Also fixed (pre-existing, exposed as parity-test flake): pass 3 of
buildSnapshot could replay a shared dirty source once per scheme --
duplicating appended alt labels -- and leave a co-resident scheme
sidecar-armed after the replay populated its maps (double-serving,
map-iteration-order dependent). Sources now parse at most once per build
and a post-pass sweep drops any armed scheme whose maps got populated.

Verified against the playground's full LCSH (513,125 terms; scratch copy):
vocab-index --all 4.9s total (lcsh 4.5s, .search.rrt 41MB vs 30MB LCVS);
old-artifact fallback to maps clean; typeahead byte-identical between the
RRTI sidecar and the map path across 30 query/scheme probes (multibyte,
single-char, alt-label hits, limit=100); RSS 460MB sidecar vs 1281MB maps.

roaringrange task 078 (filed from our tasks/167) landed and is released as
v0.30.0: the Go `*TermIndex` reader now has

- `PrefixPostings(prefix string, limit int) ([]TermPosting, truncated bool, err)`
  -- prefix-matched terms in dictionary order, each with its full posting;
- `Complete(prefix string, maxTerms int) ([]string, error)` -- terms only;
- `SearchPrefix(prefix string, limit int) ([]uint32, truncated bool, err)` --
  union of matching postings, ascending doc IDs (= descending rank), heads
  read before tails, capped at 2048 terms with a truncated flag.

All three range-read only the dict blocks spanning the prefix (the resident
router walk skips blocks before it and stops at the first term past it), so the
SKOS typeahead no longer needs the whole RRTI resident -- per query it costs the
spanning dict blocks plus the matched terms' postings, on top of the ~0.9MB
resident router.

Ask: bump roaringrange to v0.30.0 and switch the LCSH typeahead path from the
resident-RRTI workaround to `PrefixPostings`/`Complete` over blob storage.
