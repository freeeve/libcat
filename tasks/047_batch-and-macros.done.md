# 047 -- Batch operations, macros, command palette

## Context

One op-list machinery serves Koha's batch record modification, MARC
modification templates, and advanced-editor macros. The shared Selection model
also feeds exports (tasks/048).

## Scope

1. `backend/batch/`: `Selection = {kind: search|savedQuery|ids|importBatch|all}`
   + resolver; batch executor (per-grain apply with per-op results, dry-run
   mode returning aggregate diff); saved queries in the datastore.
2. Macros: `{id, label, keys?, ops[], params[]}` recorded in the editor
   (capture ops as you edit), replayed against the current doc, stored per-user
   + library-shared; parameter prompts; keyboard-assignable. Shared macros run
   in batch context = MARC modification templates.
3. SPA: BatchOps screen (selection bar on search results, dry-run diff,
   execute, results), Macros manage/record/replay screen, CommandPalette
   (Ctrl+K: fuzzy actions, run macro, jump to work).

## Acceptance

- [x] Batch dry-run shows exact quad deltas; execute applies with per-record
  success/failure reporting and audit entries.
- [x] A recorded macro replays on another record; a shared macro runs over a
  selection.

## Delivered (2026-07-02)

- **`backend/batch`**: `Selection{ids|search|savedQuery|all}` + resolver
  (importBatch fails closed with a pointer to tasks/050); the search kinds
  filter `ScanSummaries` through the new shared `WorkSummary.Matches` (the
  works-list matcher, moved to ingest so a saved query means the same thing
  everywhere). The executor runs an `editor.Op` list per grain -- the same
  op shape the editor stages, per apply.go's everything-is-an-op-list rule --
  via `publish.MutateGrain` CAS; dry-run reads and diffs without writing.
  `RunResult` carries aggregate +added/-removed quad counts, per-record
  etag/error/diff (diffs truncated past 50 with an explicit flag, counts
  never), one audit entry and one grains-changed event per execute. Runs cap
  at 2000 works and refuse instance-targeted ops (meaningless across a
  selection). Saved queries are per-user datastore records.
- **Macros**: `{id, label, keys?, ops[], params[], shared}` stored per-user
  or in the shared partition (flipping `shared` moves the record; only the
  owner updates/deletes, everyone runs shared ones). `ApplyParams`
  substitutes `${name}` from caller values falling back to declared
  defaults, failing closed on an unresolved reference so a template never
  writes placeholder text. The identical substitution ships client-side
  (`ui/src/lib/macros.ts`) so editor replay and batch runs agree.
- **HTTP**: `POST /v1/batch/resolve` (selection preview), `POST
  /v1/batch/ops` (ops or macroId+params; dryRun flag), macro CRUD
  (`/v1/macros`), saved-query CRUD (`/v1/queries`), and `GET /v1/profiles`
  (the op builder's field definitions). All librarian-gated.
- **SPA**: BatchOps screen (selection builder with saved-query save/pick,
  preview, hand-built op rows off the work profile or a macro with param
  prompts, dry-run-before-execute gating, aggregate + per-work result list
  with expandable diffs); Macros screen (list own+shared, edit
  label/key/sharing/params/ops, run-over-selection deep link); MacroBar in
  the WorkEditor ("Save staged edits as macro…" records the session's op
  list; "Apply macro…" replays into the staging pipeline with param prompts;
  single-character macro keys bind into the editor scope);
  CommandPalette on Ctrl/Cmd+K (navigation, run-macro, jump-to-work with
  live search). Axe a11y tests cover all three new surfaces.
- **Tests**: resolver kinds/caps, dry-run vs execute, per-record failures,
  macro CRUD/sharing/param substitution, template-over-selection
  (backend/batch); full HTTP flows incl. fail-closed missing params
  (httpapi); param substitution unit tests + 3 axe screens (ui). Verified
  end-to-end against a live lcatd: macro created, selection resolved, dry
  run returned exact deltas without writing, execute stamped the grain and
  reported the missing work per-record.
