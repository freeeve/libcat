# 113 -- ui: BatchOps Execute gate trusts a dry run that no longer matches the ops

> Filed from the 2026-07-05 full-code review.

## Symptom

Dry-run "add subject X", inspect the delta, fix a typo to "Y", click Execute:
the un-previewed op list runs across every matched work. The "always dry-run
first" safety on the danger button is defeated by any post-dry-run edit.

## Cause

`run(true)` sets `dryRunDone = true` (screens/BatchOps.svelte:134) and Execute
is enabled by `!dryRunDone` (:295), but nothing resets `dryRunDone` when the
user edits `opRows` (value/action/field), switches `macroId`/`paramValues`, or
changes the selection -- the kind select's `onchange` only resets
`matched`/`preview`.

## Fix sketch

Invalidate `dryRunDone` on any change to the inputs that feed the request --
simplest is deriving it from a snapshot: store the serialized request payload at
dry-run time and enable Execute only while the current payload matches it.

## Acceptance

- Editing any op row, macro, param, or selection after a dry run disables
  Execute until a fresh dry run.
- Component test covers the edit-after-dry-run path.

## Resolved

The boolean `dryRunDone` became `dryRunFor`: the serialized execute-relevant
payload (selection + macro/params or ops) the last dry run previewed. A
`$derived` recomputes the current payload and Execute is enabled only while
they match (`dryRunFresh`), so any edit to an op row, macro, param, or the
selection re-locks the button until a fresh dry run of those exact inputs;
executing clears the snapshot. New `batchops.test.ts` mounts the screen with
a mocked backend and covers lock -> dry-run unlock -> edit re-lock -> fresh
dry-run unlock.
