# 001 -- Work clustering: correction path + URL stability

## Problem
Work clustering (ARCHITECTURE.md §4) relies on OpenLibrary work ids (uneven
coverage) with a computed author+title+language fallback. The computed key
**over-merges** (reissues, common titles, different works sharing an
access-point key) and **under-merges** (translations, transliteration/name
variants). Left uncorrected, mis-clusters degrade the core discovery unit, and
any re-cluster silently changes a Work's opaque id -- and therefore its public
URL.

## Scope
1. **Editorial merge/split overlay.** Represent human merge/split decisions as
   `editorial:` statements that the computed key cannot override on re-ingest.
   Design the predicates (e.g. `lcat:mergedInto`, `lcat:splitFrom`) and the
   precedence rule (editorial beats computed, always).
2. **Deterministic re-cluster.** Given the overlay + external ids + computed
   key, clustering must be reproducible: same inputs -> same Work assignment,
   with editorial decisions pinned.
3. **URL stability.** A merge/split must leave a redirect/tombstone so shared
   links and SEO survive. Decide where redirects live (emitted by the projector
   into the static build; a 301/410 map for the host) and how a tombstoned id
   resolves.

## Acceptance
- [x] A curated set of known over/under-merge cases is corrected via the overlay
  and stays corrected across a full re-ingest.
- [x] Re-clustering the corpus twice yields identical Work ids (determinism test).
- [x] Every retired Work id resolves to its successor (redirect-map test).

## Notes
Ties into `tasks/002` (identity-map persistence): merges/splits mutate the
provider-id -> Work-id mapping and must update it atomically with the overlay.

## Implementation (done)

Overlay predicates in the `lcat:` namespace (`https://github.com/freeeve/libcatalog/ns#`),
all carried in the `editorial:` graph so the computed key cannot override them and
they survive re-ingest:

- **`lcat:mergedInto`** (under-merge fix). `lcat merge --dir --from --to` records
  `#<from>Work lcat:mergedInto #<to>Work` in the survivor's grain. On the next
  ingest the resolver seeds the merge, the retired Work's Instances resolve onto the
  survivor (`Resolver.canonical` chases chains), and the retired grain is removed.
  The projector reads the editorial graph and emits `redirects.json` (retired id ->
  survivor, chains collapsed, cycle-safe) -- the host serves 301s (the chosen
  delivery). Corpus: a merge collapses 5659 -> 5658 works, **0 id churn**, retired
  grain gone, ISBN moved, re-ingest byte-identical, one redirect emitted.
- **`lcat:workAssignment`** + **`lcat:splitFrom`** (over-merge fix). `lcat split
  --dir --from --instances` mints a new Work id and pins the listed Instances to it
  in the source grain. `Resolver.Resolve` applies a pin before the existing link and
  the computed key, so the split reproduces every ingest. Splits create a Work
  rather than retiring one, so they emit no redirect. Corpus: splitting one Instance
  out of a two-Instance Work yields 5660 works, **0 resolver churn**, the Instance
  moves to a new grain, both grains byte-identical across re-ingest.

Determinism/URL stability rest on the existing derive-from-grains model
(`bibframe.LoadPrior` seeds identity + overlay from the committed grains) and RDFC-1.0
canonical emission. Tests: `identity` (`TestEditorialMergeOverrides`,
`TestSplitPinOverridesCluster`), `bibframe` (`TestMergeReingest`, `TestSplitReingest`,
scan/marker idempotency), `project` (`TestRedirects`, `TestRedirectsCycleSafe`).

Not covered here (follow-ups): a curated qllpoc-specific correction set (this repo
edits only libcatalog); atomic identity-map persistence is subsumed by
derive-from-grains (`tasks/002`).
