# 268 -- covers batch reports a mutated record as skipped: a failed bs.Put leaves the grain claiming a cover, uncounted in applied and unaudited, while HTTP 200 and the word skipped both say nothing happened

Filed from libcat-e2e on 2026-07-09 (cross-repo ask).

**266 is being fixed against `cover_handlers.go`. `cover_batch.go` keeps its own copy of
the error branch**, and adds a reporting bug on top: it names the outcome *"skipped"*.

Measured against **committed HEAD `6e48bea`**, not the working tree -- the probe builds
from `git archive HEAD` precisely so an in-progress edit can neither hide nor fake this.

## Symptom

One zip, three entries, on a throwaway writable clone (`:8471`, never :8481 or :8501)
with `chmod a-w` on **one** cover shard, `data/covers/<id[:2]>/`:

```
control: the batch works    a 2-entry zip -> 200, applied=2, GET /covers/<W>.png -> 200
control: grain writable     after the chmod a plain tag edit still saves (200, If-Match)

POST /v1/covers/batch   (zip of three entries)
  -> HTTP 200  {"applied": 1, "results": [...]}

  "<W>.png"                    skipped="cover store failed"   workId=<W>
  "zz-e2e-not-a-work.png"      skipped="not a work id or known isbn"
  "<CTRL>.png"                 cover="covers/<CTRL>.png"      <- applied

GET /v1/works/<W>/doc  .cover  -> "covers/<W>.png"   <- the record claims a cover
GET /covers/<W>.png            -> 404                 <- there are no bytes
audit entries written          -> 1                   <- only <CTRL>'s
```

The two `skipped` entries in that one response mean different things. The bogus name
touched nothing: it names no `workId` and changed no record. The other one **changed a
record** -- and both are reported with the same word, inside a `200` whose `applied: 1`
says exactly one work was affected. Two were.

## Root cause

`backend/httpapi/cover_batch.go:167-176`. Every other skip reason returns *before* any
store is touched. This one returns *after* the grain is already written:

```go
url := "covers/" + workID + "." + ext
if _, err := mutateWorkGrain(r, bs, ix, workID, func(g []byte) ([]byte, error) {
	return bibframe.SetCover(g, workID, url)
}); err != nil {
	res.Skipped = mutateSkipReason(err)   // nothing was written -- a true skip
	return res
}
if _, err := bs.Put(r.Context(), bibframe.CoverBlobPath(workID, ext), img, blob.PutOptions{}); err != nil {
	res.Skipped = "cover store failed"    // <- the grain statement stays
	return res
}
```

`applyBatchCover` assigns `res.Skipped` in eleven places (`:111, :125, :133, :137, :143,
:148, :154, :160, :163, :170, :174`). The first ten fire before either store is touched
-- `no extension`, `not jpg/png/webp`, `not a work id or known isbn`, `isbn matches
multiple works`, `image too large (2MB cap)`, `unreadable entry` (twice), `not a jpeg,
png, or webp image`, `image is X, not the .Y its name claims`, and `mutateSkipReason(err)`
for a grain mutation that *failed*. `cover store failed` at `:174` is the eleventh, and
the only one reached after a grain mutation **succeeded**. The field carries a promise
that one assignment breaks.

Then `:66-73` compounds it:

```go
res := applyBatchCover(r, bs, ix, byISBN, f, name)
if res.Skipped == "" {
	applied++
	if queue != nil {
		queue.WriteAudit(r.Context(), suggest.AuditEntry{
			WorkID: res.WorkID, Action: "COVER_SET", Actor: id.Email, Note: res.Cover + " (batch)",
		})
	}
}
results = append(results, res)
```

`Skipped != ""` means *not counted* and *not audited*. So the mutated work is missing
from `applied`, missing from the audit log, and the request is a `200`.

`registerCoverBatch`'s doc comment (`:31-35`) states the invariant it then misses:

> Every applied cover goes through the same grain-first SetCover path as the single
> PUT, so **a bad name never strands bytes**.

True, and beside the point: the danger of grain-first is not stranded bytes, it is a
stranded *statement*. This is `attachment_handlers.go:90-105` before `9956600`, and
`cover_handlers.go:116-127` today (**266**), in a third file.

## Why it matters

**The batch report is the only thing a librarian reads.** A single `PUT` at least
answers `500` -- 266's phantom is at least announced. Here a 500-equivalent is folded
into a per-entry note inside a `200`, next to entries that genuinely did nothing, and
summarised by an `applied` count that excludes it. Import 500 covers with a full disk or
a throttling S3 bucket, read `applied: 460, 40 skipped`, and the honest conclusion --
"40 covers didn't take, the other 460 did, nothing else changed" -- is wrong in both
directions: 40 records were changed, and they now point at images that do not exist.

**Nothing downstream can find them.** The mutated works are absent from the audit log,
so `GET /v1/audit` cannot list them. They are absent from `applied`. The zip entry name
is the only surviving evidence, in an HTTP response nobody stored. Re-running the batch
after fixing the disk does repair them -- but only if the operator knows which entries
to re-run, and the response has just told them those entries were skipped.

**The blast radius is the OPAC.** `GET /covers/{file}` is public, and the OPAC's cover
slot renders whatever the grain claims, so each phantom is a broken image on a public
page. `lcat export -covers` walks records, so it will faithfully export a manifest of
covers that do not exist.

A blob `Put` failing is ordinary: `LCATD_S3_BUCKET` is supported (`config.go:164`) and
S3 throttles, 503s, and rejects on expired credentials or a tightened bucket policy. A
directory backend fills up -- and a 64MB zip of covers is exactly when it does.

Nothing is corrupted. What is destroyed is the batch report's meaning, at the one moment
an operator depends on it.

## Expected

- **Compensate, as `9956600` did for attachments.** If `bs.Put` fails, undo the grain
  statement -- `SetCover(g, workID, "")` through `mutateWorkGrain` again -- before
  returning the entry. Then `skipped` is true again for every value it can hold, and the
  fix for **266** and this one is the same helper.
- **If the write cannot be undone, do not call it `skipped`.** Give
  `coverBatchResult` a distinct field (`"failed": "cover store failed"`, or
  `"partial": true`), count those entries separately from `applied`, and let the
  response say so: `{"applied": 460, "skipped": 39, "failed": 1}`. A librarian cannot act
  on a word that means two opposite things.
- **Audit the attempt.** `WriteAudit` is gated on `res.Skipped == ""`, so the one entry
  that changed a record is the one entry with no audit trail. Whichever way the ordering
  goes, a batch entry that mutated a grain must leave a `COVER_SET` (or
  `COVER_SET_FAILED`) entry naming the work. Compare **259**.
- Consider whether a batch that mutated nothing successfully should still be `200`.
  `applied: 0` with a non-empty `failed` list is a partial failure, and `207` exists.

## Repro

```bash
cd ~/libcat-e2e && node harness/probe_cover_batch_failure.mjs   # F3a, F3b, F3c, F3d
cd ~/libcat-e2e && node harness/retest.mjs                      # check t268
```

The probe never addresses :8481 or :8501, and it never reads `~/libcat`'s working tree:
`roinstance.buildHead()` exports committed HEAD with `git archive` into a scratch dir and
builds `cmd/lcatd` there, so this result is about released code. It clones the
playground's site (`cp -Rc`, copy-on-write), boots a writable instance on :8471, and
deletes the clone afterwards.

Its controls carry the argument. `F1` proves the batch applies covers on this instance.
`F2` proves the induced failure is *targeted* -- a plain tag edit still saves with
`If-Match` after the chmod, so the grain store is demonstrably writable and only the byte
write fails. `F5` brackets the missing audit entry with one the log does accept. And `F4`
is the point of the whole probe: in the same response, a genuinely skipped entry names no
`workId` and changes no record, so `skipped` really does mean "nothing happened" --
everywhere except the one branch.

By hand:

```bash
git -C ~/libcat archive HEAD | tar -x -C /tmp/libcat-head
cd /tmp/libcat-head/backend && go build -o /tmp/lcatd-head ./cmd/lcatd
cp -Rc ~/libcat-playground/site /tmp/site-rw
LCATD_LISTEN_ADDR=:8471 LCATD_BLOB_DIR=/tmp/site-rw LCATD_LOCAL_AUTH=1 \
  LCATD_BOOTSTRAP_ADMIN="ro@example.org:changeme123" \
  LCATD_ABUSE_SECRET=0123456789abcdef0123456789abcdef /tmp/lcatd-head &

TOK=$(curl -s -XPOST -H 'Content-Type: application/json' \
  -d '{"email":"ro@example.org","password":"changeme123"}' localhost:8471/v1/auth/login | jq -r .accessToken)
W=$(curl -s -H "Authorization: Bearer $TOK" 'localhost:8471/v1/works?limit=1' | jq -r '.works[0].WorkID')

mkdir -p /tmp/site-rw/data/covers/${W:0:2}
chmod -R a-w /tmp/site-rw/data/covers/${W:0:2}

zip -j /tmp/covers.zip $W.png            # any valid png named <workId>.png
curl -s -XPOST -H "Authorization: Bearer $TOK" -H 'Content-Type: application/zip' \
  --data-binary @/tmp/covers.zip localhost:8471/v1/covers/batch
# {"applied":0,"results":[{"file":"<W>.png","workId":"<W>","skipped":"cover store failed"}]}

curl -s -H "Authorization: Bearer $TOK" localhost:8471/v1/works/$W/doc | jq -r .cover
# covers/<W>.png     <- "skipped", but the record was changed

curl -s -o /dev/null -w '%{http_code}\n' localhost:8471/covers/$W.png
# 404

chmod -R u+w /tmp/site-rw && rm -rf /tmp/site-rw /tmp/libcat-head
```
