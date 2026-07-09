# 262 -- mark 247 deferred now that taskman has a deferred state

Filed from taskman on 2026-07-09 (cross-repo ask).

## Context

libcat asked taskman for a way to say "this task is not being worked, and that
is a decision, not a backlog position." That is now `taskman defer`, released
in taskman v0.4.0. Reinstall to pick it up:

```
go install github.com/freeeve/taskman@latest
```

`tasks/247_publish-the-lcatd-container-image-to-ghcr-from-ci.md` is the case
that prompted the ask, and it is still the shape the ask warned about: a
`pending` file whose only brake is a prose `**DEFERRED (2026-07-09,
maintainer's call).**` notice in the body. `taskman list` does not print
bodies, so an agent picking the next open task sees an ordinary pending task
asking for a workflow that publishes a container image on every version tag --
outward-facing and unrecallable once pulled. The warning is invisible at
exactly the moment it matters.

## Scope

```
taskman defer -reason "maintainer's call: outward-facing publish to GHCR, a pulled tag cannot be recalled" 247
```

That renames the file to `247_<slug>.deferred.md`, appends a dated
`## Deferred` section carrying the reason, and commits the rename with a
pathspec.

Afterwards, reduce the body's `**DEFERRED (...)**` paragraph to the operational
part that is not about status -- the `workflow_dispatch`-only first step, and
the `docs/deploy.md` consequence. The status itself now lives in the filename
and the `## Deferred` section, so the shouting paragraph is redundant; leaving
both invites them to drift.

## Notes on the shape of the feature

- Deferral is a **flag**, not a fourth status: it rides on top of pending or
  in-progress, and is orthogonal to progress. `taskman fix` therefore treats a
  deferred task's number exactly as the pending task it still is.
- `taskman list` hides deferred tasks and prints an `N deferred` count;
  `taskman list -all` shows them, labelled. This is what gets 247 out of the
  "what should I pick up next" set.
- `taskman resume 247` lifts the deferral and restores the status underneath.
  `start`/`done`/`reopen` also clear it -- acting on a task ends the hold.
- `-reason` is required. A deferral without a recorded why decays into an
  unexplained `pending` in six months.
- Note that `taskman next` prints the next free *number*, not the next task to
  work on, so deferral does not affect it. The ask's acceptance criterion
  "`taskman next` must never return one" rested on a misreading of that
  command; `list` is where the loop actually looks.

## Acceptance

- `tasks/247_*.deferred.md` exists, carrying the maintainer's reasoning in a
  `## Deferred` section.
- `taskman list` in libcat does not show 247; `taskman list -all` shows it
  marked `deferred`.
- The body no longer duplicates the status in prose.

## Outcome

Done. All three criteria hold:

    $ taskman list        -> 247 absent; footer reads "1 deferred (taskman list -all)"
    $ taskman list -all   -> 247  deferred  publish-the-lcatd-container-image-to-ghcr-from-ci

`tasks/247_...deferred.md` carries `## Deferred 2026-07-09` with the reason, and
the body's shouting `**DEFERRED (...)**` paragraph is reduced to the operational
part that is not about status: the `workflow_dispatch`-only first step and the
`docs/deploy.md` consequence.

The flag-not-a-status design is right, and the correction about `taskman next` is
accepted -- it prints the next free *number*, so my acceptance criterion in
taskman's `tasks/001` ("`taskman next` must never return one") was a misreading.
`list` is where the loop looks, and `list` is where the hold now shows.

### One snag worth knowing

`go install github.com/freeeve/taskman@latest` installed **v0.3.0**, not v0.4.0:
the tag exists locally but the taskman checkout is 5 commits ahead of `origin`,
so the module proxy never saw it. `defer` was missing and this task looked
wrong.

Built from the local checkout instead (`cd ~/taskman && go install .`), which
touched nothing in that repo. **v0.4.0 still needs pushing** before the
`go install ...@latest` line in this task's own Context section is true for
anyone else.
