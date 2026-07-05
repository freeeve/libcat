# 111 -- read-only demo: batch execute writes an audit row; most SPA write affordances aren't gated

> Filed from the 2026-07-05 full-code review. Two halves of the same contract:
> the demo banner promises edits are "previewed but not saved".

## Backend: /v1/batch/ops persists an audit record in read-only mode

`readOnlyAllowed` permits any path ending in `/ops` (backend/httpapi/readonly.go:55)
on the premise that the execute path is blocked at the read-only blob store.
The grains are indeed blocked, but `batch.Run` calls `s.Queue.WriteAudit(...)`
unconditionally on the non-dryRun path (backend/batch/batch.go:218-225) -- even
with zero works applied -- and `WriteAudit` Puts to the document store
(suggest/review.go:349-362), which stays writable in demo mode. A demo visitor
sending `{"dryRun":false,...}` to `/v1/batch/ops` durably writes a
`BATCH_OPS AUDIT#` row. The `/v1/works/{id}/ops` and `/marc` paths don't leak
because their WriteAudit only runs after a successful blob Put. Fix: skip (or
gate) WriteAudit when the blob store is read-only / nothing was applied.

## SPA: read-only gating covers only Save/Publish/Profiles

`isReadOnly()` gates SaveBar (components/SaveBar.svelte:33), PublishBar, and
Profiles, but these write affordances fire real mutations and surface a raw
backend error instead of the sandbox behavior: ItemsPanel save/bulk-add
(ItemsPanel.svelte:135), WorkEditor "Split selected" (WorkEditor.svelte:53),
VisibilityPanel actions, AuthorityEditor Save/Merge (AuthorityEditor.svelte:119,
150), Duplicates "Merge" (Duplicates.svelte:144), CopyCat stage/commit,
VocabSources download/upload/remove, and BatchOps Execute. No data loss (the
backend rejects), but the mode contract diverges surface to surface. Fix:
apply the same gate/preview treatment across all write affordances.

## Acceptance

- In read-only mode, `POST /v1/batch/ops` with `dryRun:false` leaves no audit
  row (and the doc store is byte-identical before/after).
- Every SPA write control in read-only mode either previews-without-saving or
  is visibly disabled -- no raw backend rejection errors.
