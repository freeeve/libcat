# 239 -- bulk edits are invisible in a record's own edit history: BATCH_OPS writes one audit entry with no workId, so the History tab shows nothing for every record a bulk op rewrote

Filed from libcat-e2e on 2026-07-09 (cross-repo ask).

## Symptom

Run a bulk op over three works through the Bulk Ops screen's endpoint
(`POST /v1/batch/ops`, what `runBatch` calls). All three grains are rewritten --
new etags, `+3/-0 quads`. Then open any of them in the editor and press `3` for
the History tab:

```
0 entries in 2026-07
```

Measured on the 8481 playground against copycat-minted sentinels
(`ui/probe_history.mjs`):

```
PASS H1  CONTROL: a per-record edit lands in that work's history
         POST /v1/works/w4t5pi98ovm734/ops -> 200; history 0 -> 1; actions ["RECORD_EDIT"]
PASS H2  the bulk edit applies              POST /v1/batch/ops -> 200; matched=3 applied=3 failed=0
PASS H3  the bulk edit really rewrote the records     3/3 works have a new etag
FAIL H4  a bulk-edited work records the edit in its own history
         3/3 works were rewritten with no new history entry (e.g. wfkipo7r8frcjo)
FAIL H5  the batch audit entry names the works it changed
         BATCH_OPS workId="", note="ids selection: 3 matched, 3 applied, 0 failed, +3/-0 quads" -- names 0/3 of them
PASS H8  the panel shows the per-record edit          w4t5pi98ovm734: "1 entry in 2026-07", 1 row(s)
FAIL H9  the panel shows the bulk edit on a work it rewrote
         wfkipo7r8frcjo was rewritten twice by bulk ops; its panel reads "0 entries in 2026-07" with 0 row(s)
```

**H1 and H8 are the controls.** They establish that the per-work filter works,
that a record which is edited normally *is* auditable, and that the panel
renders entries when they exist. So H4/H9's empty history is the bulk path
failing to record, not the history surface failing to read.

## Root cause

`backend/batch/batch.go:287` -- one audit entry per *run*, not per record, and
it carries no `WorkID`:

```go
if s.Queue != nil {
    s.Queue.WriteAudit(ctx, suggest.AuditEntry{
        Action: "BATCH_OPS", Actor: actor,
        Note: fmt.Sprintf("%s selection: %d matched, %d applied, %d failed, +%d/-%d quads",
            sel.Kind, result.Matched, result.Applied, result.Failed, result.Added, result.Removed),
    })
}
```

`AuditEntry.WorkID` is what the per-work read filters on --
`backend/httpapi/review_handlers.go:181-189`:

```go
if workID := r.URL.Query().Get("workId"); workID != "" {
    filtered := entries[:0]
    for _, e := range entries {
        if e.WorkID == workID { filtered = append(filtered, e) }
    }
    entries = filtered
}
```

An entry with `WorkID == ""` matches no work, so `HistoryPanel.svelte:30`
(`fetchAudit(month, workId)`) always renders zero rows for it.

Every other write path sets `WorkID` and is therefore attributable:
`RECORD_EDIT` (`records_handlers.go:129,212`), `MARC_EDIT`
(`marc_handlers.go:154`), `ITEMS_EDIT` (`maintenance_handlers.go:137`),
`ITEMS_BULK_ADD` (`items_bulk.go:123`), `WORK_CLONE` (`clone_handlers.go:54`),
`ATTACHMENT_ADD`/`_REMOVE` (`attachment_handlers.go:107,152`),
`COVER_SET`/`_REMOVE` (`cover_handlers.go:71,96`), `VISIBILITY_*` and
`WITHDRAWN_*` (`maintenance_handlers.go:74,202`). The two batch routes are the
only exceptions.

The information is in hand at the write site. `batch.go` already collects
`changed` -- the grain paths it rewrote -- and passes it to
`s.Index.AppendFeed(ctx, changed...)` two lines above. It is simply not
recorded.

The sibling route `POST /v1/batch` (`records_handlers.go:368`) has the same
missing `WorkID`, and its note is `batchNote(results)` -- `json.Marshal` of the
results slice cut at 512 **bytes** (`records_handlers.go:436-442`):

```go
func batchNote(results any) string {
    b, _ := json.Marshal(results)
    if len(b) > 512 { b = b[:512] }
    return string(b)
}
```

Past ~7 works that truncation lands mid-token, so even the ids it does carry
stop being parseable, and a slice at a byte boundary can split a UTF-8 rune.

## Why it matters

The History tab is the only per-record answer to "who changed this, and when".
Bulk ops are precisely the edits a cataloger most needs that answer for: they
are applied at a distance, by someone else, to hundreds of records at once, and
they are the easiest way to damage a catalog. Today a record can be silently
rewritten by a macro run and its own history still reads `0 entries` -- there is
nothing to notice, nothing to attribute, and nothing to undo from.

The aggregate entry does not close the gap. `"ids selection: 3 matched, 3
applied"` cannot be joined back to a record: it names no work, and for the
`search` and `savedQuery` selection kinds the set of matched works is not even
reconstructible after the fact, because the query's results move as the catalog
changes. A month later there is no way to answer "was this record touched by
that run?"

It also silently skews `/v1/stats`, whose `works` count is derived from audit
entries: a run over 400 records contributes one entry naming no work.

## Expected

Write one audit entry per changed record, as every other write path does:

```go
for _, path := range changed {
    s.Queue.WriteAudit(ctx, suggest.AuditEntry{
        WorkID: workIDFromGrainPath(path), Action: "BATCH_OPS", Actor: actor,
        ETag: etagFor(path), Note: fmt.Sprintf("%s selection, %d ops", sel.Kind, len(ops)),
    })
}
```

`result.Results` already carries the per-work id and etag, so the loop needs no
new plumbing. Keep the aggregate entry too if the Audit screen wants a
run-level row -- the two are not in tension, and a `runId` on both would let the
per-record rows link back to the run that made them.

Then apply the same to `POST /v1/batch` (`records_handlers.go:368`), and either
drop `batchNote`'s truncation or make it truncate the *list* rather than the
serialized bytes, so the note stays valid JSON.

If a per-record entry per bulk run is considered too much audit volume, say so
in the panel: an empty history must not be shown when the record has in fact
been edited.

## Repro

```
cd ~/libcat-e2e && node ui/probe_history.mjs
```

Expect `H4`, `H5` and `H9` to flip to PASS, with `H1`, `H2`, `H3` and `H8`
staying green -- in particular `H1`/`H8` (the controls: a per-record edit still
lands in history and still renders) must not regress. The probe mints its own
copycat sentinels and tombstones every one of them. `harness/retest.mjs` carries
the same check as `t239`.

`H6` and `H7` in that probe belong to tasks/240, not here.

## Outcome

Fixed in **v0.93.0**. `probe_history.mjs` is 11/11; `retest.mjs` reports 239
FIXED.

Both bulk routes (`POST /v1/batch/ops` and `POST /v1/batch`) now write one audit
entry per rewritten record plus one aggregate entry for the run, tied together by
a new `AuditEntry.RunID`.

Three decisions the report did not have to make:

- **Only records that actually changed get an entry.** The suggested loop runs
  over `changed`, which holds every selected work the run did not fail on --
  including the ones whose diff was empty. A relocation over 400 works that moves
  3 would have put "edited" in 400 histories. The aggregate entry still records
  that the run selected them.
- **The rewritten set is captured before `maxItemDiffs`.** That cap nils the
  per-record diff of every result past the 50th, so reading "did this change?"
  back off `result.Results` would have silently dropped record 51's audit entry.
- **The aggregate note is JSON; per-record notes are prose.** A per-record note
  is read by a cataloger in the History tab. The aggregate appears only on the
  Audit screen and exists to answer "what did that run touch?", so it names its
  records. `suggest.RunNote` truncates the *list* at 100 ids and reports how many
  it dropped, replacing `json.Marshal(results)[:512]`.

## The trail was never newest-first

Writing four entries inside one microsecond exposed a defect underneath this one.
The audit sort key was `time.RFC3339Nano`, which **trims trailing zeros** from
the fractional second. An entry at `.167790000` keys as `".16779Z"`; one at
`.167792000` keys as `".167792Z"`. A descending lexicographic scan ranks `'Z'`
(0x5A) above `'2'` (0x32), so the **older** entry came back first.

Live, before the fix:

```
.16779Z   BATCH_EDIT  workId=wp8ljrjke4qsdm   <- 167790ns, returned first
.167792Z  BATCH_EDIT  workId=''               <- 167792ns, the newest, returned second
.167788Z  BATCH_EDIT  workId=wilv1l65a723g0
.167777Z  BATCH_EDIT  workId=wg1t52gdovldjm
```

This is why `H6` kept failing after the note was fixed: the probe takes the first
`BATCH_EDIT` row, and the first row was not the newest one.

Roughly one entry in ten ends in a zero digit, so the trail has been subtly
out of order for every month it has existed. It only became visible when a single
action started writing more than one entry.

Fixed on both sides: `auditSKLayout` is RFC3339 with a fixed-width nanosecond
field, and `Audit` sorts on the timestamp, so the keys already written come back
in order too.

## One check in the filer's harness encodes the option I did not take

`retest.mjs` `t240` still reports STILL-BROKEN, with "B's grain is clean, but the
dry-run still reports 1 addition(s) to B for a patch about A". Its pass condition
is `added.length === 0` for B.

That is tasks/240's **option 2** (refuse the request). I took **option 1**
(rebind the subject per work), which the report lists first. Under a rebind, a
patch dry-run against B correctly reports one addition **to B**, because the
statement now describes B:

```
work A -> added: ['<#AWork> <bf:subject> <.../zzprobe> <editorial:> .']
work B -> added: ['<#BWork> <bf:subject> <.../zzprobe> <editorial:> .']
```

The preview equals the write, which is what the report asked for. The harness
predicate should be "no quad in B's previewed diff names A", not "B's diff is
empty". Raised with the filer in their tasks/026.

## Verification (filer)

Fixed. Confirmed 2026-07-09 by `harness/retest.mjs` (`t239` FIXED, no errors in
the suite) and by `ui/probe_history.mjs`, now **10/10**:

```
PASS H4  a bulk-edited work records the edit in its own history   every changed work gained an entry
PASS H5  the batch audit entry names the works it changed         names 3/3 of them
PASS H9  the panel shows the bulk edit on a work it rewrote       "2 entries in 2026-07", 2 row(s)
```

`H9` closes the report: a work rewritten twice by bulk ops now shows both edits
in its own History tab. Controls `H1`/`H8` (a per-record edit still lands, and
still renders) stayed green.

Two corrections the fix taught me, both about my own checks:

- **The trail was never newest-first, and no assertion of mine could have caught
  it.** `H4` and `H9` count entries. A history in arbitrary order passes a count.
  Verified the ordering separately after reading the outcome -- `VISIBILITY_tombstone`
  at `17:16:03` ahead of `COPYCAT_COMMIT` at `17:15:51`, monotonically
  descending. Counting is the assertion you reach for when you have not decided
  what the data should look like.
- Keeping the aggregate `BATCH_OPS` row at `workId: ""` is right, and I was wrong
  to suggest per-record entries might replace it. A run-level row belongs to no
  single record; giving it one would duplicate it into that record's history.
  The structured note naming every touched work is what makes the aggregate row
  useful, and `H6` confirms it is valid JSON end to end rather than cut at 512
  bytes.
