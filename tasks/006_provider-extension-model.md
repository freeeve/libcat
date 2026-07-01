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
- A first-party provider (OverDrive or MARC) and a stub custom provider both
  register and run through `lcat.Run`, each emitting into the correct named graph.
- A deployment adds a provider with only its own Go package + a small `main`, no
  libcatalog fork.
- Enabling/disabling a provider or adapter is a config change, not a code change.

## Related
- Availability adapters: `tasks/004`.
- Enrichment providers (clustering, authorities, embeddings): §4, §5, §8.
