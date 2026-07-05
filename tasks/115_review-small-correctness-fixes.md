# 115 -- small correctness fixes from the 2026-07-05 review

Independent low-severity defects, each small enough to fix without design work.
Check off per item; split out any that grows.

- [ ] **httpapi: missing work returns 409 instead of 404 on mutate routes.**
  `mutateWorkGrain` collapses every `bs.Get` failure into `errors.New("no such
  work")` (httpapi/records_handlers.go:377-385) and callers map any error to
  409 Conflict (merge :266, split :294, visibility maintenance_handlers.go:73,
  items PUT :142, items/bulk items_bulk.go:104). `POST /v1/works/merge` with a
  nonexistent target returns 409 where the read paths return 404, and a
  transient blob error is reported as a conflict. Distinguish not-found /
  conflict / internal.

- [ ] **editor: field cardinality unenforced beyond Max==1 on set.**
  `applyOne` checks `field.Max == 1 && len(op.Values) > 1` only; `add`
  (backend/editor/apply.go:108-116,133-151) never counts existing+new against
  `field.Max`, so a `Max: 2` field accepts 5 values -- batch macros call
  `ApplyOps` directly (batch.go:252,266) and can silently violate the profile.

- [ ] **batch: shared/personal partition move loses the record on a mid-move
  fault.** `UpdateMacro` Deletes from the old partition then Puts to the new
  (backend/batch/macros.go:111-118; same in itemtemplates.go:77-84). Put-fail
  after Delete-success loses the macro. Write-then-delete (reversed order)
  makes the failure mode a harmless duplicate instead.

- [ ] **project: contributors not deduped.** `contributors()`
  (project/project.go:529-567) emits one entry per bf:contribution node without
  the set-dedup that languages/classifications/subjects use, so a feed+editorial
  re-assertion (or an OverDrive Creators repeat -- bibContributions at
  ingest/overdrive/bibframe.go:163-177 doesn't dedupe, unlike hardcover's) shows
  a contributor twice in catalog.json. Display-only (facet counts dedupe).

- [ ] **bibframe: WorkID fallback hashes nil on encode error.**
  `b, _ := iso2709.Encode(rec)` (bibframe/corpus.go:30-36) discards the error;
  all no-001 records whose encode fails share the hash-of-empty grain id and
  silently overwrite each other. Phase-0 legacy path, but make it error instead
  of losing records.

- [ ] **store/dynamo: push opt.Limit down to DynamoDB.** `Query` sets no
  `Limit` on the QueryInput (store/dynamo/dynamo.go:196-204), so a Limit-2 query
  reads a full ~1MB page (ConsistentRead doubles the cost) before the
  client-side stop at :215.

- [ ] **auth/oidc: cache the discovered token endpoint.** With
  `TokenEndpoint` unset (as appdeps builds it, appdeps.go:223-227),
  `ExchangeHandler` re-runs OIDC discovery -- an upstream HTTP GET -- on every
  exchange (auth/oidc/exchange.go:56,92-100). Memoize like the JWKS cache.

- [ ] **appdeps: ephemeral signing key breaks multi-instance/Lambda sessions.**
  `signingKey` (appdeps/appdeps.go:326-329) mints a fresh Ed25519 key per Build
  and warns only about restarts; concurrent Lambda sandboxes each mint their own
  key, so tokens fail verification cross-sandbox (intermittent 401s in the
  read-only demo with LocalAuth). At minimum warn/fail loudly on the
  multi-instance path; better, require an explicit key there.
