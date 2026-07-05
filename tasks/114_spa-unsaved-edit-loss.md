# 114 -- ui: two flows silently discard a cataloger's unsaved edits

> Filed from the 2026-07-05 full-code review.

## 1. AuthorityEditor conflict reload drops in-progress edits

On `ConflictError`, `save()` calls `load()` (screens/AuthorityEditor.svelte:123-125),
overwriting `prefRows`/`altRows`/`defRows`/`relations` from the server. A
librarian mid-relabel loses everything typed when another session touched the
term; only a terse "reloading the fresh state" message shows. The work editor
already has the right pattern -- editor.ts `reload()` keeps staged ops and
replays them after a rebase -- adopt the same here (or at minimum confirm
before discarding).

## 2. ItemsPanel bulk add drops pending manual edits

`bulk(false)` calls `load()` (components/ItemsPanel.svelte:113), which replaces
`items` from the server and clears `dirty`. Add a manual row or edit a field
(panel shows it pending), run Bulk add without saving -- the refetch silently
drops the unsaved row/edits. Fix: block bulk while dirty (prompt to save
first), or merge the refetched list with pending local rows.

## Acceptance

- An authority-save conflict preserves the user's typed changes (rebase or
  explicit confirm-discard).
- Bulk add with unsaved manual item edits either refuses until saved or keeps
  the pending edits; no silent loss.
