# 203 -- PromoteTag skips the workindex update so an approved tag promotion stays invisible to work search for up to 30s -- the tasks/195 bug in the publish path

Filed from libcat-e2e on 2026-07-09 (cross-repo ask).

## Symptom

`POST /v1/promotions/decide {approve:true}` returns `200 {"works":1}` -- the
grain rewrite is done -- but `GET /v1/works` keeps serving the pre-promotion
state for up to `workindex.DefaultTTL` (30s): the work does not appear under the
new controlled subject, and it still matches the folk tag that was just
retracted.

Measured on the 8481 playground, one sentinel tag on one work:

```
decide -> 200 works=1 (rewrite reported complete)
index sees promoted subject after : 26728 ms
index sees tag retracted after    : 26730 ms
```

Confirmed twice. A second run sampled the transition rather than polling:

| moment | index: works matching `?tag=` | index: works matching `?subject=` |
|---|---|---|
| right after `approve` | 1 (tag still there) | work absent |
| after 35 s (past the TTL) | 0 | work present |

Both the subject arrival and the tag retraction land together at the TTL edge --
the signature of the List-diff backstop, not of a slow write.

## Root cause

This is exactly tasks/195, one package over. `batch.Service` was given an
`IndexUpdater` so its own writes stay exact; `publish.Publisher` never was.

- `backend/publish/promote.go:30` `PromoteTag` rewrites each matching grain
  through `MutateGrain` (`AppendAuthoritySubject` + retract the editorial
  `lcat:tag`) and returns the count. It never calls `Apply` / `AppendFeed` --
  the file contains no reference to the index at all.
- `backend/publish/publisher.go:35` `Publisher` has `Blob`, `Queue`, `Vocab`,
  `Trigger`, `Lease`, `Prefix`, `Summaries`. `Summaries` is a **read** source
  ("the shared maintained summary source (workindex, tasks/109) tag promotion
  scans instead of a per-run corpus walk"). There is no write-back handle.

So promotion writes fall through to `refreshLocked`'s 30s List-diff backstop,
which is meant for writes made by *other* containers.

## Why it matters

- A moderator approves a promotion, watches `{"works":N}` come back, then
  searches the new subject and finds nothing. The tag they just folded is still
  listed. Same silent-lie failure mode tasks/195 fixed for batch.
- Worse than cosmetic: within the window, `batch.Service.Resolve` resolves
  search/tag selections against the stale index. "Promote tag X, then batch-edit
  everything under subject Y" silently selects the wrong set -- the hazard
  tasks/195 called out, reachable again through this path.
- The same `Publisher` backs `POST /v1/publish` and `POST /v1/review
  {publish:true}`, so approved suggestions likely share the staleness. Worth
  checking while fixing (not yet measured).

## Expected

After `decide {approve:true}` returns, `GET /v1/works` reflects every rewritten
work -- new subject present, retracted tag gone -- with no TTL wait, matching the
batch and single-record paths.

## Suggested fix

Give `Publisher` the same narrow `IndexUpdater` seam `batch.Service` got in
7b0dccc (`Apply(grainPath, etag, grain)` + `AppendFeed(ctx, paths...)`), wired
from appdeps with the typed-nil guard. `PromoteTag`'s loop already has the grain
bytes, the path, and the new etag from `MutateGrain`; call `Apply` per rewritten
grain and one `AppendFeed` over `changed` at the end, exactly as `batch.Run`
now does. Audit the other `MutateGrain` callers in `publish/` for the same gap.

## Repro

```sh
# libcat-e2e
node harness/promote_freshness.mjs   # prints both latencies, cleans up after itself
node harness/probe_promote_tag.mjs   # samples grain vs index across the TTL edge
```

## Not bugs (checked while probing)

- The promotion workflow itself is sound: propose -> `201 PENDING`, duplicate
  proposal -> `409 promotion already proposed`, unknown term -> `400 unusable
  tag or unknown term`, a tag carrying `U+0001` -> 400, decide on an unknown tag
  -> 409, empty tag -> 400, approve -> `200 {"works":1}` and `status=APPROVED`.
- The editorial tag *is* retracted in the grain, and the controlled subject *is*
  added. Only the index disagrees, and only until the TTL lapses.

## Cleanup owed on the playground

Three sentinel promotion records persist -- `zz-e2e-promo-*`, `zz-promo2-*`,
`zz-promofresh-*` -- because there is no delete route for a decided promotion.
Their works were reverted (subject removed, tag removed); only the promotion
rows remain.

## Outcome

Fixed in 2b26dcd, released v0.53.0, along your suggested seam:
publish.Publisher gains the batch-style IndexUpdater (Apply per
rewritten work grain with its CAS etag; one AppendFeed at the end),
wired from appdeps with the typed-nil guard. Your "worth checking"
suspicion was right -- PublishApproved had the same gap, so approved
suggestions and POST /v1/publish are covered in the same commit. One
wrinkle your report couldn't see: the alias grain
(data/authorities/al/aliases.nq) rides the rebuild-trigger `changed`
list but is deliberately excluded from Apply/AppendFeed -- it is not a
work grain, and feeding it would emit a tombstone feed record.

Unit coverage: TestPromoteTagUpdatesIndex (two works applied with
etags, one feed append of the two work paths, alias excluded) and
TestPublishApprovedUpdatesIndex. Verified with your own harness
against the rebuilt playground:

    promote_freshness.mjs
    # index sees promoted subject after : 2 ms   (was 26,728)
    # index sees tag retracted after    : 3 ms   (was 26,730)

Your sentinel promotion rows (zz-e2e-promo-*, zz-promo2-*,
zz-promofresh-*) still persist -- decided promotions have no delete
route; same follow-up ask as the authorities DELETE if you want it.
