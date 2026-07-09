# 206 -- editing-profiles screen has no leave guard: navigating away silently discards unsaved profile JSON

Filed from libcat-e2e on 2026-07-09 (cross-repo ask).

## Symptom

`Profiles.svelte` tracks a `dirty` flag and guards **in-page profile switches**
with `confirm("Discard unsaved changes?")`, but it registers **no leave guard
and no `beforeunload`**. Clicking any nav link -- or reloading the tab -- while
the profile JSON textarea has unsaved edits discards them without a word.

Measured through the real SPA (`node ui/probe_profiles_ui.mjs`, 2026-07-09):

| Check | Result |
|---|---|
| Selecting `work-monograph` loads its JSON | 4319 chars, "shipped default" |
| Save override disabled until dirty | disabled on pristine load |
| Typing enables Save override | dirty tracked correctly |
| **Control**: switching to `fastadd` while dirty | prompts `Discard unsaved changes?`, stays put ✅ |
| **Clicking "Works" in the nav while dirty** | **0 dialogs, hash → `#/works`, editor unmounted** ❌ |
| Returning to `work-monograph` | 4319 chars again -- the 4322-char edit is gone ❌ |
| `beforeunload` while dirty | `defaultPrevented=false` ❌ |

Control run against the work editor with the identical probe confirms the
technique reads arming correctly, so the profiles result is not a probe artifact:

```
workeditor pristine armed = false
workeditor dirty    armed = true                       <-- WorkEditor arms it
nav dialogs = ["Discard unsaved changes to this record?"]  hash restored
```

versus profiles: `armed=false`, `dialogs=0`, hash changed, edit lost.

## Root cause

Task 199 added a shared leave-guard mechanism, and only the work editor adopted
it:

- `backend/ui/src/lib/router.ts:59` -- `setLeaveGuard(fn)` registers the guard;
  `confirmLeave()` (`:68`) is consulted by the shell.
- `backend/ui/src/App.svelte:119-126` -- the `hashchange` listener calls
  `confirmLeave()` and restores the previous hash when it returns false.
- `backend/ui/src/screens/WorkEditor.svelte:41-50` -- the only caller:
  registers `setLeaveGuard` **and** a `beforeunload` listener while `dirty`.
- `backend/ui/src/screens/Profiles.svelte:43-48` -- `select()` confirms, but
  nothing else does. `grep -rl registerLeaveGuard/setLeaveGuard --include=*.svelte`
  returns `WorkEditor.svelte` only.

So `dirty` already exists and is already correct on the profiles screen; it is
simply never wired to the shell's guard.

## Why it matters

The profile editor is a **raw JSON textarea over a 4.3 KB, 11-field document**
(`work-monograph`), hand-edited by an admin. This is exactly the screen where a
lost edit costs the most and is least recoverable -- unlike the work editor,
there is no background draft autosave behind it (`WorkEditor.svelte:38-39`
explicitly leans on drafts to make a discard recoverable). One stray click on
the nav bar and several minutes of careful field-definition work is gone with no
undo, no draft, and no warning.

It is also an inconsistency a cataloger will feel: the same app confirms before
discarding a one-tag staged edit on a work, and says nothing before discarding a
rewritten editing profile.

## Expected

`Profiles.svelte` mirrors `WorkEditor.svelte:41-50`: while `dirty`, register a
`setLeaveGuard(() => confirm("Discard unsaved changes to this profile?"))` and a
`beforeunload` listener, both torn down when the profile is saved, reverted, or
the screen unmounts.

Worth a look at the same time: `revert()` (`Profiles.svelte:90-106`) calls
`loadProfile(selected)` after the DELETE. For an override of a shipped default
that is right, but for a profile that has no shipped default the DELETE drops it
entirely and the reload 404s into `error = "no such profile"` while the screen
still shows it as selected.

## Repro

```
cd ~/libcat-e2e && node ui/probe_profiles_ui.mjs     # UP6, UP7, UP8 FAIL
cd ~/libcat-e2e && node harness/retest.mjs           # reports 206 STILL-BROKEN
```

The probe only dirties the textarea; it never saves, so no override persists.

## Not bugs (verified clean this cycle)

`node harness/probe_profiles.mjs` -- 19/19 pass. The API side of profiles is
solid: `profiles.Validate` runs on every save (empty fields, duplicate field
path, unknown predicate, unknown resourceType all → 400), rejected saves persist
nothing, the id pattern is enforced, a blind overwrite of an existing override
is refused (412 via `IfNoneMatch`), a stale `If-Match` is refused without
writing, librarians read but cannot `PUT`/`DELETE` (403), `PROFILE_EDIT` lands
in the audit log with the right actor, and revert restores the shipped default
(404 when there is no override).

## Outcome

Fixed as specified (fix(ui) tasks/206 commit), released v0.58.0:
Profiles.svelte mirrors WorkEditor's tasks/199 wiring while dirty --
setLeaveGuard("Discard unsaved changes to this profile?") +
beforeunload, torn down on save/revert/unmount via the $effect
cleanup. Your "worth a look" revert edge is fixed too: deleting the
override of a profile with no shipped default clears the editor with
an explanatory status instead of 404ing into a stale selection.

Verified with your own probe against the rebuilt playground:
ui/probe_profiles_ui.mjs UP1-UP9 all PASS (previously UP6/UP7/UP8
FAIL) -- nav while dirty prompts and stays, the 4322-char edit
survives, beforeunload defaultPrevented=true.
