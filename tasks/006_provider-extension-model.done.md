# 006 -- Provider extension model: compile-in registry

## Decision
Providers plug in at **compile time** via a Go interface + registry (Option A),
not dynamic loading. Rationale in ARCHITECTURE §9a: the provider set is finite,
known, and first-party / deployment-authored, and a deployment already builds its
own `lcat` (§10). Subprocess (gRPC) and WASM transports are deferred until non-Go
authors or untrusted third-party providers are real (roadmap Phase 5).

## Scope
1. **`Provider` interface** (in `ingest/` or `provider/`): `Name()`, `Role()`
   (ingest -> `feed:<name>` / enrich -> editorial|enrichment), and `Run(ctx, cfg)
   (rdf.Dataset, error)` or a streaming emitter. Define it so a future
   subprocess/WASM transport can implement it unchanged (the transport boundary).
2. **Registry + composition.** `Register` / `lcat.Run(providers...)`. The default
   `cmd/lcat` registers the first-party set (OverDrive, MARC, OPDS). Explicit
   registration in `main`, not `init()`.
3. **Config unification.** One `lcat` config lists enabled build-time providers
   and enabled runtime (JS) availability adapters; the projector emits the
   runtime adapter-enable list into the Hugo build. Configure once.
4. **Two-half contract.** A provider's ingest half (Go) and availability half
   (JS adapter, `tasks/004`) share one id contract (§9): ingest emits the id the
   adapter keys on.

## Acceptance
- [x] A first-party provider (OverDrive) and a stub custom provider both register
  and run through the registry, each emitting into the correct named graph.
  (`ingest` package tests: registry composition, graph routing, the real OverDrive
  provider end-to-end, plus a corpus run.)
- [x] A deployment adds a provider with only its own Go package + a small `main`,
  no libcatalog fork. (The stub provider is defined in an external `ingest_test`
  package using only exported API, proving external implementability;
  `cmd/lcat/providers.go` is the one-call composition pattern to copy.)
- [x] Enabling/disabling a **provider** is a config change, not a code change
  (`lcat ingest --provider <name>` selects at runtime from the registry). The
  runtime **availability-adapter** enable-list is out of scope here -- it rides
  with `tasks/004` (client-side, in the Hugo/deployment repo per the boundary).

## Done (commit pending)

Compile-time registry model (Option A) delivered as a new `ingest` package:

- `ingest.Provider` (`Name`/`Role`/`Records(ctx)`) + `ingest.Record`
  (`Identity`/`Work`/`Instance`). `overdrive.Item` already had those three methods,
  so it satisfies `Record` with **zero changes** -- the pipeline is genuinely
  provider-agnostic. `Role` (`RoleIngest`/`RoleEnrich`) fixes the provenance graph.
- `ingest.Registry` (`Register`/`New`/`Names`) maps a type key to a `Factory`;
  `Config{Feed,Source,Params}` carries build-time config; the key defaults the
  feed graph. Registration is explicit in `cmd/lcat/providers.go` (not `init()`).
- `ingest.Run(prov, out)` is the extracted, provider-agnostic pipeline (seed prior
  -> resolve -> cluster -> `bibframe.BuildWorks` -> drop retired), returning a
  `Result` instead of printing (the CLI owns presentation). It executes only
  ingest-role providers; enrichment execution is future work.
- `ingest/overdrive/provider.go`: `overdrive.New` factory + `ProviderName`.
- CLI: new generic `lcat ingest --provider <name> --source <in> --out <dir>
  [--feed <name>]`; `lcat overdrive` is now a thin alias (keeps the MARC-fixture
  export) that routes its grain build through the registry.

Validated: `ingest` unit tests (compose/route/reingest-stable/reject-enrich/real-
OverDrive); on the 6,253-record corpus the generic `ingest` subcommand re-run into
the `overdrive`-built tree minted **0** and left it **byte-identical** -- one shared
deterministic pipeline. `--feed hoopla` routes to `feed:hoopla`; unknown provider and
missing `--out` error cleanly.

## Remaining (deferred, tracked)

- **MARC as a `Provider`.** `lcat build` still uses the record-based
  `bibframe.BuildMARC` path (libcodex `FromRecord`), not the direct-BIBFRAME
  resolve/cluster pipeline; making MARC yield `Work()`/`Instance()` records is a
  follow-on tied to `tasks/003`/`tasks/007`.
- **Full config unification (scope items 3-4).** A single `lcat` config file
  listing enabled build-time providers **and** enabled runtime JS availability
  adapters, with the projector emitting the adapter-enable list into the Hugo
  build. The adapter half depends on `tasks/004` (client-side); revisit together.
- **Multi-provider into one graph.** `LoadPrior(dir, feed)` recovers one feed's
  identity and treats other graphs as preserved editorial; aggregating several
  ingest feeds into one catalog dir needs a deliberate merge story (not needed by
  the single-feed OverDrive deployment today).
- **`RoleEnrich` execution.** The role is defined and `Run` refuses it; an
  enrichment runner (authorities, embeddings) is future work.

## Related
- Availability adapters: `tasks/004`.
- Enrichment providers (clustering, authorities, embeddings): §4, §5, §8.
