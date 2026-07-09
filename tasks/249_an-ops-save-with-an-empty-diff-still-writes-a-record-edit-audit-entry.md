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
