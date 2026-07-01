# libcatalog architecture

## 1. Framework vs. implementation

libcatalog is a **generic framework**: the reusable machinery for turning a
bibliographic graph into a fast, faceted, static discovery catalog, plus an
optional collaborative cataloging backend.

A **deployment** implements the framework -- it supplies the collection, the
controlled vocabularies, the provider feeds, the branding/theme, and any local
extensions. The reference implementation is **qllpoc** (a queer-literature
library on OverDrive/Libby, cataloged with the Homosaurus vocabulary).

The split is deliberate and load-bearing:

- Nothing library-specific lives in libcatalog. OverDrive is *one* provider;
  Homosaurus is *one* vocabulary; queer-lit branding is *one* theme.
- A deployment is a thin layer -- config + theme + local extensions -- depending
  on `github.com/freeeve/libcatalog`.
- Anything that turns out generic migrates *down* out of a deployment into the
  framework.

## 2. Design principles

- **BIBFRAME is the source of truth.** Records are an RDF graph (Work/Instance
  native), committed to git. HTML, search index, MARC/MODS exports, and JSON
  projections are all derived build artifacts.
- **No lossy intermediary.** No per-record markdown/frontmatter. HTML is
  generated from the graph (via a projection), not from a flattened copy of it.
- **Few dependencies; own the core.** Go + libcodex + roaringrange. No
  triplestore, no database for the static tier.
- **Zero paid-API baseline.** The default build needs no cloud AI. Semantic
  embeddings are opt-in (see 8).
- **The graph is the contract.** The static tier and the dynamic backend
  communicate only through the committed BIBFRAME graph, so either can exist
  without the other.

## 3. Source of truth: BIBFRAME in git

- Canonical form: **per-Work files** under `data/works/<workid>.ttl` (Turtle),
  sorted with blank nodes skolemized to stable IRIs, so git diffs are clean and
  PR-reviewable.
- A build step serializes the corpus to a bulk **N-Quads** dump (`catalog.nq`)
  for reindexing and download -- N-Quads, not N-Triples, because provenance
  rides on named graphs (see 5).
- The graph holds **stable bibliographic + authority** data only. Volatile
  circulation state is excluded (see 5).
- Scale: a mid-size collection is a few hundred thousand triples -- trivially
  in-memory. No SPARQL server.

## 4. Identity: two-tier Work / Instance

BIBFRAME-native, matching FRBR/LRM:

- **Instance (manifestation)** -- one per edition/format. Carries ISBNs,
  provider ids (e.g. OverDrive), OCLC#, and links to live availability. The
  borrowable/holdable unit.
- **Work** -- groups all instances (editions, translations, formats) of one
  intellectual creation. The discovery unit: one Work page with
  format/language facets instead of N near-duplicate cards.
- Opaque, provider-independent ids at both levels, minted once and never derived
  from a provider id.
- **Work clustering** is an enrichment step:
  1. External work ids where available -- OpenLibrary (`OL...W`, free,
     ISBN-keyed) primary; OCLC Work ids where licensed; LC name-title authority
     for the authorized heading.
  2. Computed fallback -- normalized author + normalized title (+ original
     language), i.e. the MARC 1XX+240 access-point key. The deterministic
     default when no external id matches.

## 5. Data & provenance model

- **Named graphs for provenance.** Each triple lives in a named graph
  identifying its origin: `feed:<provider>` (regenerated on ingest, never
  hand-edited) vs `editorial:` (human/authority-owned, preserved across
  ingest). Clobber-safe: re-ingesting a provider replaces only that provider's
  named graph and never touches editorial triples.
- **Availability stays out of the graph.** Available copies / holds / estimated
  wait are live and volatile; they are fetched client-side at view time
  (per-provider), never committed. The graph is diffable precisely because it
  excludes them.
- **Extend, don't fight the model.** Anything BIBFRAME 2.0 doesn't cover uses a
  framework namespace (`lcat:`); a deployment adds its own (e.g. `qll:`). RDF is
  open-world -- extensions never require forking the ontology.
- **Controlled vocabularies are linked data.** Subjects are URIs
  (`bf:subject <vocab-uri>`); a global relabel propagates for free because
  records reference authorities by id, not string. Homosaurus is one such
  vocabulary (already SKOS/JSON-LD); LCSH/LCNAF stream in via libcodex's RDF
  decoder.

## 6. Two product tiers

### Tier 1 -- static, self-serve (no backend)

Point the projector CLI at a **MARC or BIBFRAME dump** and get a faceted,
searchable, multilingual catalog site. Onboarding ramp: **MARC import via
libcodex** -- bring the MARC your existing ILS (Koha/Sierra/...) already
exports. Pure static output; no cloud infra beyond static hosting.

### Tier 2 -- dynamic, optional (collaborative cataloging)

An authenticated in-browser cataloging/review app (roles, edit history/audit)
that writes BIBFRAME back to git. Cloud infra: an API + serverless functions +
a small datastore + a git content store + OIDC auth. Distribution is a product
decision -- **self-hosted** (Terraform in the library's own cloud) or **SaaS**
(multi-tenant). Optional, because the graph is the contract.

## 7. Static tier: projector CLI + Hugo module

Hugo stays -- as the library's whole website. The catalog is a *component*
inside it, so a deployment keeps its non-catalog pages (hours, events, about) in
ordinary Hugo, themed as it likes. Hugo has no runtime plugin/exec model, so the
catalog ships as two distributable artifacts:

1. **Projector CLI** (`lcat`, Go, over libcodex + roaringrange):
   `BIBFRAME (git) -> catalog data (JSON) + search index`. Also the
   import/export front door (MARC/MODS/BIBFRAME in and out).
2. **Hugo module** (`hugo mod get github.com/freeeve/libcatalog/hugo`): catalog
   layouts, partials (facets, vocabulary picker, live-availability + search JS
   assets), and a **content adapter** (`_content.gotmpl`, Hugo >= 0.126) that
   mints a Page per Work from the projected data -- no content files, no
   per-record markdown.

Pipeline:
`BIBFRAME graph -> [projector] -> catalog JSON + search idx -> Hugo (module
content adapter + theme) -> static HTML -> S3/CloudFront`.

Downstream is tool-stable: the search reader runs over the emitted index; live
availability is client-side JS; the dynamic backend (if present) only touches
the graph.

## 8. Search: roaringrange, embeddings opt-in

- **Default: lexical.** roaringrange's BM25 / `terms` path (the Rust crate's
  `terms` feature; WASM reader in the browser), multilingual-aware. **No
  embeddings, no paid AI, no Bedrock** in the default build -- a library can
  stand up a good catalog with zero cloud-AI dependency.
- **Opt-in: semantic.** The vector/embedding arm (model2vec / provider
  embeddings) is a build flag, off by default. Enabling it requires the
  deployment to supply an embedding provider and accept its cost. The framework
  never turns it on implicitly.
- Rationale: keep the adoptable baseline free and self-contained; make the
  higher-recall arm a deliberate upgrade.

## 9. Ingest / providers

- **Provider interface.** Ingest is pluggable: a provider maps its feed into
  `bf:Instance` triples under `feed:<provider>`. OverDrive/Libby (thunder API)
  is the reference provider; MARC import (libcodex) is a first-class provider
  for existing-ILS onboarding. Physical-holdings ILSes export MARC -> same path.
- **Merge.** New feed items attach to an existing Instance by ISBN, and to a
  Work by the clustering rules in 4.

## 10. Theming / implementation model

A deployment (e.g. qllpoc):

- depends on `github.com/freeeve/libcatalog` (Go) and the Hugo module;
- supplies **config** (providers, enabled vocabularies, feature flags such as
  embeddings on/off, languages);
- supplies a **theme** (Hugo theme/overrides on top of the module's templates
  and assets);
- adds **local extensions** (its own predicates, custom facets, custom pages);
- optionally runs **Tier 2** for collaborative cataloging.

qllpoc is the reference implementation and proving ground.

## 11. Guardrails & non-goals

- Availability/circulation is **not** modeled in the graph (live, client-side).
- No triplestore, no SPARQL endpoint, no per-record database -- git + object
  storage only.
- Canonical, sorted, skolemized RDF, or the git/audit story breaks.
- **Not an ILS.** No acquisitions, no patron accounts, no lending -- borrowing is
  the provider's job (OverDrive, etc.). libcatalog is discovery + cataloging: the
  bibliographic half only.
- Embeddings / paid AI never on by default.

## 12. Component / dependency map

Sibling repos under one parent directory:

- `libcatalog/` -- this framework.
- `libcodex/` (`github.com/freeeve/libcodex`) -- MARC/MODS/DC/schema.org/BIBFRAME
  read-write-convert + RDF toolkit + streaming authority-file decoder.
- `roaringrange/{go,rust,python}` (`github.com/freeeve/roaringrange`) -- search
  index + reader. `terms` = lexical/BM25 (default); python/model2vec =
  embeddings (opt-in).
- deployments (e.g. `qllpoc/`) -- implement the framework.

## 13. Proposed repo layout

```
libcatalog/
  README.md
  docs/            ARCHITECTURE.md, ROADMAP.md
  go.mod           module github.com/freeeve/libcatalog
  cmd/lcat/        the projector / import-export CLI
  bibframe/        record <-> BIBFRAME crosswalk (over libcodex)
  identity/        two-tier Work/Instance ids + clustering
  ingest/          provider interface; overdrive + marc providers
  project/         graph -> catalog JSON + search index
  search/          roaringrange wiring (lexical default; embeddings flag)
  hugo/            the Hugo module: content adapter, layouts, partials, assets
  backend/         (Tier 2) cataloging API/app -- later
```
