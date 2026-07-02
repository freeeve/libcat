# 002 -- Identity: minted-id persistence across re-ingest

## Problem
ARCHITECTURE.md §4 mints opaque Work/Instance ids **once**, "never derived from
a provider id." ARCHITECTURE.md §5 fully **regenerates** `feed:<provider>` on
every ingest. So each re-ingest must re-attach freshly generated feed statements
to the *previously minted* Instance/Work id -- which requires a persisted
`provider-id -> minted-id` map that ingest reads and writes. This mapping is
itself durable state; it is not currently specified.

## Scope
1. **Where the map lives.** Options: an `editorial:`-adjacent mapping graph
   committed to git (diffable, auditable) vs a sidecar index. Prefer the
   committed graph so identity is versioned with the data.
2. **Mint-or-resolve on ingest.** For each incoming provider record: resolve
   (ISBN / provider id) to an existing Instance, else mint a new id and record
   the mapping. Never re-mint for an id already mapped.
3. **Interaction with clustering.** Work-id assignment (`tasks/001`) reads the
   Instance map; merges/splits rewrite Work ids and must update the map
   atomically with the overlay.
4. **Collision + reassignment.** Handle a provider id that moves between
   Instances (rare, e.g. a provider re-issues an identifier) explicitly rather
   than silently.

## Acceptance
- [x] Re-ingesting the same feed twice produces byte-identical grains (no id churn).
- [x] Ingesting a changed feed preserves ids for unchanged records and mints only
  for genuinely new ones.
- [x] The map round-trips through git with clean diffs (RDFC-1.0 canonical).

## Done (commit pending)

Satisfied by the **derive-from-grains** model (Decision A): the committed grains *are*
the durable `provider-id -> minted-id` map -- there is no side-channel index. Resolved
across the identity + ingest work this session and earlier:

1. **Where the map lives** -- in the grains themselves (scope §1's "committed graph"
   choice). `bibframe.LoadPrior` walks the per-Work grains under the build dir and
   `identity.SeedResolver` seeds the resolver from them (Instance keys from `bf:Isbn`
   etc., Work cluster keys recomputed from the grain), so identity is versioned with
   the data and diffs cleanly (RDFC-1.0 canonical grains). No separate map file.
2. **Mint-or-resolve** -- `identity.Resolver.Resolve` resolves an Instance by provider
   key / ISBN to a committed id, else mints; a Work by the Instance's existing link,
   else the computed cluster key, else mints. Never re-mints a mapped id.
3. **Clustering interaction** -- `tasks/001` merges/splits (`lcat:mergedInto` /
   `lcat:workAssignment` in `editorial:`) are seeded via `SeedMerge`/`SeedPin` *before*
   the computed key and survive re-ingest; a merge's retired grain is dropped so it
   can't re-seed as a live cluster.
4. **Reassignment** -- because resolution keys off the ISBN/provider id (not the
   title-derived cluster key), a record whose content changes keeps its id; an id that
   genuinely moves surfaces via `Resolver.Conflicts()` rather than churning silently.

Tests (`ingest` package, this session): `TestRunReingestStable` (same feed twice ->
0 minted, byte-identical), **`TestRunAddedRecordsMintOnlyNew`** (add a record -> only
it mints; the unchanged grains persist byte-identical), **`TestRunChangedRecordKeepsId`**
(a record's title changes -> 0 minted, same grain file, content-only update). Also
proven on the real corpus (OverDrive 6,253 instances, MARC Express sample) and for the
MARC provider (`tasks/006`).

## Known non-goal

A record *removed* from a feed keeps its grain (and id) -- intentional: a transient
feed absence must not churn ids if the record returns. Permanent removal / stale-grain
GC is a separate concern (a deployment/display filter, cf. qllpoc `015`), not id
persistence.
