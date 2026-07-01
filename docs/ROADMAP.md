# libcatalog roadmap

Sequencing rule: **prove the model, keep qllpoc shipping, swap components last.**
Never change the source of truth, the renderer, and the backend at the same
time. Phases 0--4 are tracked as `tasks/038`--`044` in the qllpoc repo (the
reference implementation and proving ground).

## Phase 0 -- Keystone crosswalk

Record -> `codex.Record` -> `bf:Work`/`bf:Instance`, validated on all ~6,266
qllpoc records; emit canonical per-Work Turtle + a bulk N-Quads dump. Proves
BIBFRAME can represent the real corpus and yields MARC/MODS/schema.org export
immediately.
-> qllpoc `tasks/038`.

## Phase 1 -- Identity + graph-as-truth

Two-tier Work/Instance ids; cluster instances into works (OpenLibrary +
computed key). Establish the git graph with named-graph provenance; migrate
qllpoc's curated overlays (`homosaurus_subjects`, `curator_*`) into `editorial:`
triples. Availability excluded.
-> `tasks/039`, `tasks/040`.

## Phase 2 -- Tier 1 static framework

Bootstrap libcatalog: projector CLI + Hugo module (content adapter, ported
templates/assets). qllpoc renders from the graph via the module -- first as a
transitional bridge (graph -> projected data -> Hugo module), retiring the
frontmatter/markdown pipeline. roaringrange lexical search wired in, embeddings
off by default.
-> `tasks/041`, `tasks/042`.

## Phase 3 -- Providers + MARC onboarding

Provider interface; OverDrive as the reference provider; MARC import via
libcodex as the "bring your ILS" Tier-1 ramp. qllpoc's OverDrive ingest moves
behind the interface.
-> `tasks/043`.

## Phase 4 -- qllpoc as an implementation

qllpoc depends on libcatalog; QLL-specifics (Homosaurus config, OverDrive
config, branding/theme) become a thin implementation layer. The
framework/implementation split becomes real.
-> `tasks/044`.

## Phase 5 -- Tier 2 generalization (later)

Parameterize the cataloging backend (review app / API / committer / auth) for
multi-tenant or self-host. Decide self-hosted vs SaaS distribution.

## Phase 6 -- Second adopter

Onboard a non-QLL library (MARC import, own vocabulary/theme). The real test of
"generic," and the bus-factor / community payoff.
