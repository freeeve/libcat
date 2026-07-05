# 116 -- quality refactors from the 2026-07-05 review

Duplication and structure cleanups; no behavior change intended. Check off per
item; each should land with tests proving equivalence.

- [ ] **suggest: one CAS retry helper.** Five near-identical
  get/unmarshal/mutate/marshal/Put(CondIfVersion)/retry loops: `mutateFolk`
  (backend/suggest/service.go:173), `bumpAggregate` (:213), `transition` (:313),
  `mutateSuggestion` (:318), `mutatePromotion` (promotion.go:155). Factor a
  generic helper (mutate closure + casRetries/casBackoff); watch the two that
  also maintain the status index inside the loop.

- [ ] **httpapi: shared work-grain read helper.** `readGrain` closures are
  byte-identical in records_handlers.go:43-59 and marc_handlers.go:27-43, and
  the same validate-id/Get/404-500 sequence is re-inlined in
  maintenance_handlers.go (:30, :97), subject_lookup.go (:59, :142), and
  items_bulk.go (:66). One helper centralizes the read/error convention
  (pairs with the 404/409 fix in [[115_review-small-correctness-fixes]]).

- [ ] **shared label-language fallback.** The "prefer `en`, then untagged,
  then any" precedence is implemented five times: `bestLang`
  (backend/export/authority.go:247), `Term.Label` (vocab/vocab.go:59),
  `bestLabel` (authoritiesvc/service.go:321), inline in export/run.go:277-284
  and enrich/enrich.go:97-103.

- [ ] **bibframe: unify LoadPrior / LoadPriorStore.** The per-grain sequence
  (ScanGrain, preservedQuads, Editorial accumulation, ScanMerges, ScanPins) is
  duplicated between reingest.go:30-74 (filesystem) and reingest_store.go:20-62
  (blob store), and the catalog.nq skip rule already differs subtly. Share a
  per-grain core; keep only enumeration source-specific.

- [ ] **batch: generic owned/shared CRUD.** macros.go and itemtemplates.go
  carry parallel Create/Update/Delete/Get/List/put/scope with the same
  partition and ownership flow (itemtemplates.go:44-155); factor a generic
  store keyed by prefix+type.

- [ ] **copycat: split copycat.go (744 lines).** Three separable concerns:
  target CRUD (PutTarget/DeleteTarget/Targets/SeedDefaultTargets), search
  fan-out + protocol/query assembly (SearchAll/protocolSearch/sruSearch/
  sruQuery/z3950Query/readUpTo), and the staging/commit lifecycle -- siblings
  already live in their own files (profiles.go, revert.go, templates.go).

## Status (2026-07-05 session)

Not started; no item begun. Context from the fixes that landed meanwhile:
tasks/115 added `writeMutateError` in httpapi/records_handlers.go -- the
shared readGrain helper here should fold into that same error-mapping
convention (404/500/409). The suggest CAS loops, label-fallback copies,
LoadPrior duplication, macros/itemtemplates parallel CRUD, and copycat.go
split are all untouched.
