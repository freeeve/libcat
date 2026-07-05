# 116 -- quality refactors from the 2026-07-05 review

Duplication and structure cleanups; no behavior change intended. Check off per
item; each should land with tests proving equivalence.

- [x] **suggest: one CAS retry helper.** Five near-identical
  get/unmarshal/mutate/marshal/Put(CondIfVersion)/retry loops: `mutateFolk`
  (backend/suggest/service.go:173), `bumpAggregate` (:213), `transition` (:313),
  `mutateSuggestion` (:318), `mutatePromotion` (promotion.go:155). Factor a
  generic helper (mutate closure + casRetries/casBackoff); watch the two that
  also maintain the status index inside the loop.

- [x] **httpapi: shared work-grain read helper.** `readGrain` closures are
  byte-identical in records_handlers.go:43-59 and marc_handlers.go:27-43, and
  the same validate-id/Get/404-500 sequence is re-inlined in
  maintenance_handlers.go (:30, :97), subject_lookup.go (:59, :142), and
  items_bulk.go (:66). One helper centralizes the read/error convention
  (pairs with the 404/409 fix in [[115_review-small-correctness-fixes]]).

- [x] **shared label-language fallback.** The "prefer `en`, then untagged,
  then any" precedence is implemented five times: `bestLang`
  (backend/export/authority.go:247), `Term.Label` (vocab/vocab.go:59),
  `bestLabel` (authoritiesvc/service.go:321), inline in export/run.go:277-284
  and enrich/enrich.go:97-103.

- [x] **bibframe: unify LoadPrior / LoadPriorStore.** The per-grain sequence
  (ScanGrain, preservedQuads, Editorial accumulation, ScanMerges, ScanPins) is
  duplicated between reingest.go:30-74 (filesystem) and reingest_store.go:20-62
  (blob store), and the catalog.nq skip rule already differs subtly. Share a
  per-grain core; keep only enumeration source-specific.

- [x] **batch: generic owned/shared CRUD.** macros.go and itemtemplates.go
  carry parallel Create/Update/Delete/Get/List/put/scope with the same
  partition and ownership flow (itemtemplates.go:44-155); factor a generic
  store keyed by prefix+type.

- [x] **copycat: split copycat.go (744 lines).** Three separable concerns:
  target CRUD (PutTarget/DeleteTarget/Targets/SeedDefaultTargets), search
  fan-out + protocol/query assembly (SearchAll/protocolSearch/sruSearch/
  sruQuery/z3950Query/readUpTo), and the staging/commit lifecycle -- siblings
  already live in their own files (profiles.go, revert.go, templates.go).

## Status (2026-07-05 session)

Done -- all six items, existing suites passing throughout as the equivalence
proof (plus behavior notes below where the convention deliberately tightened).

- **suggest CAS helper.** `Service.casUpdate` (service.go): raw-bytes apply
  closure, allowCreate for bumpAggregate's get-or-create, per-call conflict
  message; the status-index side effects moved after the loop (apply
  captures the winning attempt's state), preserving the old
  write-after-winning-put ordering.
- **httpapi readWorkGrain** (grain_read.go): validate/Get/error mapping in
  one place, aligned with writeMutateError's 400/404/500 convention.
  Behavior note: the maintenance, items-bulk, and subject-lookup sites used
  to map *any* read failure to 404; store faults now surface as 500.
- **vocab.PickLabel**: one fallback (preferred tags, en, untagged, then
  lexicographically-first non-empty -- deterministic where map order was
  not); Term.Label, export's bestLang, authoritiesvc's bestLabel, and the
  enrich/export inline copies all ride it, keeping their own final
  fallbacks (term URI / subject ID).
- **bibframe Prior.accumulateGrain**: the shared per-grain core of
  LoadPrior/LoadPriorStore, off a single parse (was four parses per grain:
  ScanGrain, preservedQuads, ScanMerges, ScanPins -- a real win for
  RunStore's per-commit load); `isWorkGrainName` unifies the catalog.nq
  skip rule.
- **batch owned/shared CRUD** (owned.go): generic `ownedKind[T]` +
  create/update/delete/get/list engine; Macro and ItemTemplate embed the
  new `OwnedMeta` (same JSON keys; construction literals updated).
- **copycat split**: targets.go (Target type + CRUD + seeding) and
  search.go (fan-out + protocol/query assembly) out of copycat.go, which
  keeps the staging/commit lifecycle (744 -> 473 lines).
