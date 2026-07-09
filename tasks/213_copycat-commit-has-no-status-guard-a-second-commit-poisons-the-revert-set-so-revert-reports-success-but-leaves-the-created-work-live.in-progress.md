# 213 -- copycat commit has no status guard: a second commit poisons the revert set, so revert reports success but leaves the created work live

Filed from libcat-e2e on 2026-07-09 (cross-repo ask).

## Symptom

`POST /v1/copycat/batches/{id}/commit` accepts a batch that is already
`COMMITTED` and returns 200. The second commit rewrites the batch's undo set,
and because the grain now exists, it records the newly-minted work as a
*pre-existing* grain rather than a created one. A later revert then "restores"
that grain to its committed state instead of tombstoning it -- and reports
success while doing it.

A/B against the same template record, same flow, only the number of commits
differs (2026-07-09):

```
single   commits=[200]     revert=200 reverted=1 skipped=[] -> tombstoned=true  searchable=1
double   commits=[200,200] revert=200 reverted=1 skipped=[] -> tombstoned=false searchable=1
```

Both revert calls claim `reverted=1` with nothing skipped. Only the single-commit
batch actually retires its work. After the double commit the record is a normal,
live, un-tombstoned work -- `GET /v1/works/{id}/visibility` -> `{"tombstoned":false}`
-- and the batch is now `REVERTED`, so revert refuses to run again:

```
POST /v1/copycat/batches/{id}/revert -> 400 "batch ... is REVERTED; only a committed batch reverts"
```

There is no supported way to undo the import from that point. The work has to be
retired by hand with `POST /v1/works/{id}/visibility {"action":"tombstone"}`,
which is how this probe cleaned up after itself.

## Root cause

`Commit` never inspects the batch status:

- `backend/copycat/copycat.go:354` -- `func (s *Service) Commit(ctx, id, actor)`
  calls `GetBatch`, then goes straight to re-matching and writing. There is no
  `b.Status` check anywhere in it.
- Contrast `backend/copycat/revert.go:151` -- `Revert` does guard:
  `if b.Status != StatusCommitted { ... "only a committed batch reverts" }`.

The corruption happens in the undo bookkeeping:

- `backend/copycat/revert.go:105` -- `writeRevertSet` deletes the previous
  `CCREV#<batch>` rows and rebuilds them. Its doc comment even anticipates this:
  "A re-commit replaces the previous set wholesale."
- `:117-127` -- the `Created` flag is derived from `existed[p]`, a map captured
  against the blob store *as it is at commit time*. On the first commit the grain
  does not exist, so `rr.Created = true`. On the second commit the grain exists
  (the first commit made it), so `rr.Created = false` and `rr.Prior` is set to the
  **committed** bytes.
- `:183-191` -- `Revert` branches on `rr.Created`. With `Created` now false it
  takes `next := rr.Prior` and writes the committed grain back over itself. The
  `grainHash(current) != rr.PostHash` guard at `:179` does not fire, because
  `PostHash` was recomputed from those same post-commit bytes. So the Put
  succeeds, `result.Reverted++` at `:196`, and the caller is told one grain was
  reverted.

Every check in the revert path is self-consistent with the poisoned set. Nothing
is skipped, nothing errors, and the work survives.

## Why it matters

Revert is the safety net for copy cataloging -- the reason a cataloger feels free
to commit an import at all. Its own doc comment (`revert.go:141-145`) promises
"created grains are tombstoned (URL stability over deletion) ... editorial work is
never destroyed by an undo." A double-click on Commit, a retried request after a
timeout, or any client that does not disable the button silently converts that
net into a no-op that *claims to have worked*.

The failure is invisible at every layer: 200 from commit, 200 from revert,
`reverted=1`, `skipped=[]`. The only way to notice is to go look at the work
afterwards. And once the batch reads `REVERTED`, the supported undo path is
closed -- the guard that should have prevented the double commit now blocks the
recovery.

That `writeRevertSet` explicitly documents re-commit as an expected case makes
this feel like a missing guard rather than an intended flow.

## Expected

`Commit` refuses a batch that is not `STAGED`, symmetrically with `Revert`:

```go
if b.Status != StatusStaged {
    return Batch{}, fmt.Errorf("%w: batch %s is %s; only a staged batch commits", ErrValidation, id, b.Status)
}
```

`writeCopycatError` already maps `ErrValidation` to 400, so the handler needs no
change.

If re-commit is meant to be supported (the `writeRevertSet` comment suggests
someone thought so), then it must not recompute `Created` from the live blob
store -- it should preserve `Created` and `Prior` from the existing `CCREV` row
whenever one is present, so the undo set keeps pointing at the true pre-import
state.

## Repro

```
cd ~/libcat-e2e && node harness/probe_copycat.mjs   # C10 and C11 FAIL
cd ~/libcat-e2e && node harness/retest.mjs          # reports 213 STILL-BROKEN
```

`t213` stages an original, commits it twice, reverts, and asserts the work ends
up tombstoned. It tombstones the work by hand in its cleanup either way, so a run
never leaves a live sentinel record behind.

## Not bugs (verified clean this cycle)

The rest of copy cataloging is in good shape. Four templates ship
(`audiobook, book, ebook, serial`) and three SRU targets are configured
(`dnb-sru, k10plus-sru, loc-sru`). Minimum viability is enforced before anything
is staged -- an empty record gives 400 naming `LDR` and `245`, a record with no
245 gives 400 `a title is required (245 $a)` -- and a rejected staging creates no
batch. Staging does not touch the catalog (the title matches 0 works while
`STAGED`). Review sets the overlay policy. Commit creates the work and it is
immediately visible in the index, with no TTL wait. An unknown batch id gives 404
on both commit and revert. A `REVERTED` batch cannot be reverted again (400).
`DELETE` on the batch returns 204.

Also checked and **not** a bug: a tombstoned work still appears in `GET /v1/works`.
The admin index deliberately keeps retired records visible and the summary carries
the flag (`"Tombstoned": true` in the response), while the OPAC projection filters
them out at `project/project.go:541`. Cataloger-facing search showing a flagged
tombstone is the intended asymmetry.
