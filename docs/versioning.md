# Versioning

libcat ships three Go modules -- root, `hugo/`, `backend/` -- released in
lockstep under one number by `scripts/release.sh vX.Y.Z` (tasks/146). The number
is a single decision, so it needs a single rule.

## The rule

We are pre-1.0, so the `0.MINOR.PATCH` slots carry the meaning `MAJOR.MINOR` will
carry after 1.0.

**Bump the patch** when the release only makes wrong behavior right. A correct
client -- one that read the docs and did not depend on the bug -- needs to do
nothing but redeploy.

**Bump the minor** when a consumer has something to do:

- **Additive.** A new endpoint, a new response field, a new config key. Nothing
  breaks, but there is something new to adopt.
- **Breaking.** A required request header, a changed default, a removed field, a
  narrowed input. Somebody's client stops working until they change it.

When a release is a mix, the highest wins: one breaking change in a pile of fixes
is a minor.

## The test

Ask: *what does the adoption note say?*

If it says "rebuild and restart", it is a patch. If it says "and also…", it is a
minor. Every task's `## Outcome` section ends with an Adoption block precisely so
this question has an answer before the tag is cut.

A useful corollary: **a bug fix that a client could have been relying on is not a
patch.** tasks/253 made the facet response include selected values with
`"count": 0`. A client that filtered out zero-count values would re-hide the very
filter the fix restores. That is a fix, but it has an adoption note, so it earns
a minor -- or a patch plus a loud note. Prefer the minor; the number is cheap and
a surprised consumer is not.

## Worked examples

| release | change | slot | why |
|---|---|---|---|
| v0.109.0 | copycat search gains a `warnings` map | minor | additive: a new response field to render |
| v0.110.0 | error messages stop leaking filesystem paths | patch | no correct client read those strings |
| v0.112.0 | `PUT /v1/works/{id}/items` requires `If-Match` | minor | breaking: every writer must send a header |
| v0.113.0 | `GET /v1/works` excludes tombstoned by default | minor | breaking: the default result set changed |
| v0.114.0 | facets always list a selected value | patch | a fix, with a note (see the corollary above) |

## What the version does *not* track

- **The task number.** Tasks and releases are many-to-many. A release may close
  several tasks; a task may span several releases.
- **The schema version.** The BIBFRAME grain schema has its own pin, checked by
  the exampleSite build gate that `release.sh` runs.
- **The libcodex dependency.** Bumping it is a minor only if it changes libcat's
  own surface. tasks/274 bumped libcodex *and* changed the copycat error
  contract; the second is what earned the minor.

## Mechanics

`scripts/release.sh` accepts any `vX.Y.Z`. Patch releases are ordinary and this
repo has cut them before (`v0.4.1`, `v0.7.2`, `v0.100.1`, `v0.103.1`). Reaching
for a minor by reflex inflates the number until it stops carrying information,
which is the failure this document exists to prevent.
