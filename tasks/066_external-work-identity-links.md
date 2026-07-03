# 066 -- External work-identity links (enrichment pass)

## Context

libcatalog mints opaque, provider-independent Work ids (`identity/`, ARCHITECTURE §4) --
which matches orthodox BIBFRAME practice: there is no global work-id registry, every
implementor mints local URIs (LoC included) and links outward. The shared-identity
concepts that do exist are linking hubs, not primary keys: LC's `bf:Hub`
(id.loc.gov/resources/hubs), OCLC Work IDs/Entities (membership+network gated -- wrong
dependency for offline-first ingest), Wikidata, OpenLibrary work ids, BookBrainz. ISTC,
the ISO work identifier, is defunct. Minted `w…` ids stay the primary identity; external
identities become attached links.

## Scope

- An enrichment pass (fits the existing enrichment arm and `--prov-enrichment` graph)
  that matches works against external hubs and writes what sticks into the enrichment
  graph as `bf:identifiedBy` (typed identifier nodes) and/or `owl:sameAs`:
  - LC Hubs (creator+title access point match against id.loc.gov, batch-downloadable),
  - Wikidata (QID via ISBN/OL crosswalks or title+author),
  - OpenLibrary work ids (ISBN -> edition -> work walk; dumps available for offline runs).
- Match conservatively: exact normalized creator+title (reuse `identity.NormalizeKey` /
  `WorkKey` discipline) plus a corroborating signal (ISBN in the cluster, language);
  ambiguous candidates are skipped, never guessed. Offline-friendly: run from downloaded
  dumps/snapshots, no per-record live API on the ingest path.
- Links ride merges: `lcat:mergedInto` survivors keep the union of external identities;
  duplicate/conflicting links surface in the maintenance view rather than silently
  winning.
- Surfacing: identifiers appear in the work editor's passthrough/identifier area with the
  enrichment provenance rail; exports carry them (MARC 024/758 per the libcodex
  crosswalk's capabilities -- check the fidelity table, note gaps rather than force).

## Non-goals

- Replacing or re-deriving minted `w…` ids from any external registry.
- OCLC coupling.
- Live network lookups during ingest.

## Acceptance

- A work with a confidently matched external identity carries the link in its grain's
  enrichment graph, visible in the editor and present in N-Quads/JSON-LD exports.
- Re-running the pass is idempotent; unmatched works are untouched.
- Match precision demonstrated on a qllpoc sample (spot-check list in the task notes
  before enabling by default).
