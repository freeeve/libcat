# 107 -- copycat: Stage and Commit each load the entire prior grain store

> Filed from the 2026-07-05 full-code review.

## Symptom

Staging or committing a single copy-cataloged record costs O(corpus): every work
grain is listed, fetched, and scanned -- twice across a stage-then-commit flow,
plus a third full listing for the pre-commit snapshot.

## Cause

`matchRecords` (backend/copycat/copycat.go:463-464) calls
`bibframe.LoadPriorStore(ctx, s.Blob, s.Prefix+"data/works/", s.feed())` and
`identity.SeedResolver(r, prior.Grains)` on every call; `Stage` calls it once and
`Commit` calls it again (copycat.go:650). `LoadPriorStore`
(bibframe/reingest_store.go:20-62) Gets and scans every `*.nq` grain including
editorial bytes. `preCommitSnapshot` (copycat/revert.go:66-74) separately lists
the whole `data/works/` tree on every Commit.

## Fix sketch

Reuse one loaded prior/resolver between Stage and Commit (cache keyed by store
generation, or persist the staged match against the batch), and share the
identity index proposed in [[106_httpapi-per-request-corpus-scans]] once it
exists -- copycat only needs provider keys and cluster keys, not editorial
bytes. Scope preCommitSnapshot's listing to the works the batch touches.

## Acceptance

- A stage-then-commit of one record against a large seeded store does not read
  every grain twice; measured blob reads drop from O(2N+) to O(N) or better
  (O(1) with the shared index).
