# 245 -- lcat covers -- find and reap cover blobs no grain references

Opened 2026-07-09.

## Context

tasks/243 stopped `PUT /v1/works/{id}/cover` from leaving the previous format's
blob behind. It did not clean up behind itself: every cover replaced with a
different format between tasks/215 and v0.95.0 left an image in the blob store,
still served from `GET /covers/<workId>.<ext>` -- public, unauthenticated, and
guessable.

Nothing in the catalog references those blobs, so nothing will ever collect
them, and nothing can even tell an operator they exist. From the doneness note
filed to libcat-e2e:

> We plugged the hole; we did not clean up behind it.

A cataloger replaces a cover precisely when the old image must stop being
published -- wrong edition, rights complaint, an image that should not have gone
out. A takedown that looks done is not done, and after v0.95.0 it *looks* done
for every future replacement while the historical ones stay up.

## Scope

`lcat covers --store <blob-root>`:

- Walks `data/covers/`, resolves each blob back to its Work's grain, and reports
  every blob the grain does not reference.
- Reasons are distinguished, because they mean different things to an operator:
  a blob for a work that no longer exists, a blob for a work whose cover is a
  different format, a blob for a work with no cover at all, and a blob whose
  path does not parse as a cover.
- Read-only by default. `--reap` deletes, and says what it deleted.
- `--json` for scripting.

A work's cover is `bibframe.CoverOf` -- an editorial statement overlays a
feed-carried one -- so a work whose cover statement points at an external
provider URL has no local blob it references, and all of its blobs are orphans.

## Non-goals

- Reaping anything but covers.
- Touching grains. This command never writes a grain; the cover statement is the
  authority and it is already correct.

## Acceptance

- On a store with a replaced-format cover, the command names the stale blob and
  `--reap` removes it, leaving the referenced one.
- A work with no cover statement but a stored blob is reported.
- A blob whose work's grain is missing is reported.
- The referenced cover is never reported and never deleted, including when the
  work carries a feed cover that an editorial one overlays.
- Re-running after `--reap` reports nothing.

## Outcome

Shipped in **v0.97.0** (`4637ce2`), as `lcat covers --store <blob-root>
[--reap] [--json]`.

- `cmd/lcat/covers.go` -- `findOrphanCovers` walks `data/covers/`, resolves each
  blob to its Work through `bibframe.GrainPath`, and compares it against
  `bibframe.CoverOf`. Grains are read once per work, not once per blob, because
  a work with several stored formats is the case this command exists for.
- Reasons stay distinguished: `the work's cover is a different format` (the 243
  residue), `the work has no cover`, `no grain for this work`, `not a cover
  path`. "No grain" and "no cover" are carried on a `claim` struct rather than
  inferred from an empty string, which is what an earlier draft got wrong.
- Report-only is the default -- the command deletes public images. The guard was
  verified by mutation: forcing the delete branch on makes
  `TestCoversDefaultsToReportOnly` fail.
- `cmd/lcat/covers_test.go` -- 7 tests, including the editorial-overlay case and
  a work whose cover is a provider URL (every local blob it stored is an orphan,
  which a naive "does this work have a cover?" check would miss).

`coverWorkOf` rejects a path whose shard disagrees with its id. Covers shard by
`workID[:2]` (`CoverBlobPath`) while grains shard by a hash prefix
(`GrainPath`) -- the command reads grains through `GrainPath` rather than
reimplementing either.

### Found a real orphan

Run read-only against the demo playground store, the command found exactly the
blob it was written for:

    data/covers/w0/w0cfnsjg6micju.png
      (the work's cover is a different format, references covers/w0cfnsjg6micju.jpg)

Both were live: `GET /covers/w0cfnsjg6micju.png` returned 200 from the running
server while the work's doc reported `cover: covers/w0cfnsjg6micju.jpg`. That is
the exposure, reproduced on a real store.

`--reap` was **not** run against the playground: it is an irreversible delete in
a persistent store outside this repo, and no one asked for that blob to be
destroyed. The reap was verified instead against a copy of the playground's own
bytes -- it deleted the `.png`, spared the referenced `.jpg`, left the grain
byte-identical, and a second pass reported clean. **The orphan is still present
on 8481** and is Eve's call to reap.
