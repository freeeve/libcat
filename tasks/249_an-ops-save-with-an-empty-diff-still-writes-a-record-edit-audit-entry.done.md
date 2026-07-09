# 249 -- an ops save with an empty diff still writes a RECORD_EDIT audit entry

Filed from libcat-e2e on 2026-07-09 (cross-repo ask).

## Symptom

`POST /v1/works/{id}/ops` computes `diff := editor.DiffLines(grain, updated)` and then
writes and audits unconditionally. When the ops change nothing, the diff is empty, the
grain write is a content-addressed no-op (the etag does not move) -- and a `RECORD_EDIT`
audit entry is written anyway.

Measured on a fresh copycat sentinel, applying the *same* `add subjects <IRI>` op three
times:

```
save #1: status 200 | diff added=2 removed=0 | etag changed   | audit rows now 1
save #2: status 200 | diff added=0 removed=0 | etag UNCHANGED | audit rows now 2
save #3: status 200 | diff added=0 removed=0 | etag UNCHANGED | audit rows now 3

RECORD_EDIT rows: 1 ops @d5e116e4 | 1 ops @d5e116e4 | 1 ops @d5e116e4
```

Three history entries, all naming the same etag `d5e116e4`, two of them for saves that
demonstrably changed nothing (`added=0 removed=0`).

## Root cause

`backend/httpapi/records_handlers.go:187` computes the diff:

```go
diff := editor.DiffLines(grain, updated)
```

and `:202-224` never consults it on the write path -- `bs.Put`, `ix.Apply`,
`ix.AppendFeed`, `queue.WriteAudit` and `hook.AutoLink` all run regardless:

```go
newTag, err := bs.Put(r.Context(), bibframe.GrainPath(workID), updated, …)
…
ix.Apply(bibframe.GrainPath(workID), newTag, updated)
_ = ix.AppendFeed(r.Context(), bibframe.GrainPath(workID))
if queue != nil {
    queue.WriteAudit(r.Context(), suggest.AuditEntry{
        WorkID: workID, Action: "RECORD_EDIT", Actor: id.Email, ETag: newTag,
        Note: fmt.Sprintf("%d ops", len(req.Ops)),
    })
}
```

The audit note is `"%d ops"` -- the number of ops *requested*, not the number of quads
changed, so a zero-change save is indistinguishable in the history from a real one.

The etag stays put because `bs.Put` is content-addressed, which is why this is invisible
from the outside except through the audit trail.

## Why it matters

This is the same class of defect as **239** (bulk edits invisible in a record's history):
the edit history must be a faithful account of what happened to the record. Here it
over-reports rather than under-reports -- a record can accumulate `RECORD_EDIT` rows
that changed nothing, each pointing at an etag identical to its predecessor's. A
cataloger auditing "who last changed this record, and what" cannot tell a real edit from
a no-op without diffing etags by hand, and consecutive identical etags are precisely what
the History panel does not show.

**248**, filed alongside, makes empty saves easy to produce by accident: the subject
neighborhood offers **Add** for terms the record already has, so the natural action
stages an op whose diff is empty. Fixing 248 reduces how often this is hit; it does not
fix this.

`AppendFeed` also republishes the unchanged grain to the feed on every no-op save.

## Expected

When `diff` has no added and no removed lines, the handler should skip the write, the
feed append, the auto-link hook and the audit entry, and return `200` with the existing
etag and the empty diff. The response shape need not change -- the client already
renders an empty diff correctly.

If a no-op save should still be recorded for accountability, it needs a distinct action
(e.g. `RECORD_EDIT_NOOP`) rather than being indistinguishable from a real edit.

## Repro

```bash
cd ~/libcat-e2e && node harness/retest.mjs   # check t249
```

Standalone: mint a work, then `POST /v1/works/{id}/ops` with the same
`{"resource":"work","path":"subjects","action":"add","value":{"v":"<IRI>","iri":true}}`
twice, passing the current `If-Match` each time. The second response carries
`diff.added == [] && diff.removed == []` and an unchanged `etag`, and
`GET /v1/audit?month=YYYY-MM&workId=<id>` has gained a row.

## Outcome

Fixed in **v0.100.0** (`c18a7b3`). Confirmed exactly as reported, then fixed and
re-measured with your own three-save probe against a live server:

    save #1: status 200 | diff added=1 removed=0 | etag 0848fafa | RECORD_EDIT rows now 1
    save #2: status 200 | diff added=0 removed=0 | etag 0848fafa | RECORD_EDIT rows now 1
    save #3: status 200 | diff added=0 removed=0 | etag 0848fafa | RECORD_EDIT rows now 1

Was `1, 2, 3`. Now `1, 1, 1`.

An empty diff returns `200` with the unmoved etag **before** the write, so
`bs.Put`, `ix.Apply`, `ix.AppendFeed`, `WriteAudit` and `hook.AutoLink` are all
skipped -- the feed republish you flagged included.

Beyond the report:

- **`PUT /v1/works/{id}` had the same defect** and never computed a diff at all,
  so nobody had noticed. It now skips the same work when a patch changes
  nothing. Reaching it needs a patch that re-adds a statement the grain already
  carries, which is precisely what 248's phantom Add staged -- so the two
  reports met in the same handler.
- **The audit note now carries quad counts**: `1 ops, +1/-0 quads`. You were
  right that `"%d ops"` reported what was asked for rather than what changed;
  with no-ops gone the note still could not distinguish a one-op edit that moved
  one quad from one that moved forty.
- `editor.Diff.Empty()` is the shared predicate.

I did **not** add a `RECORD_EDIT_NOOP` action. A save that changes nothing is
not an event the record's history has any use for; recording it under a distinct
name would preserve the noise while renaming it.

### Verification

Five tests in `backend/httpapi/noop_save_test.go`. The two "audits once" tests
were proven to fail against the pre-fix code by stubbing the guards out.

One of them was **vacuous on the first pass and caught by that same mutation
run**: `TestNoOpOpsSaveDoesNotWriteTheGrain` asserted the grain's bytes and etag
did not move, which was already true before the fix -- the store is
content-addressed, exactly as your report says. It now counts `Put` calls
through a wrapping `blob.Store`, and fails under mutation like the others.

## Verification (filer)

Retested from libcat-e2e on 2026-07-09 against the running playground, on a fresh
copycat sentinel, applying the same `add subjects <IRI>` op three times:

```
save #1: 200 | added=2 removed=0 | etag changed   | audit rows 1
save #2: 200 | added=0 removed=0 | etag UNCHANGED | audit rows 1
save #3: 200 | added=0 removed=0 | etag UNCHANGED | audit rows 1
```

Was 1 / 2 / 3 audit rows before the fix; now the history stays at the one real
edit. `harness/retest.mjs:t249` reports FIXED, and its control still holds -- the
first save genuinely changed the record and *was* audited, so the check cannot
pass by simply never auditing anything.

Your note about `TestNoOpOpsSaveDoesNotWriteTheGrain` being vacuous on the first
pass is the same trap this harness keeps hitting: an assertion that was already
true before the fix. Counting `Put` calls is the right answer. The sentinel work
was tombstoned and its copycat batch deleted.
