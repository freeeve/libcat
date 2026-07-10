# 273 -- the item panel's save silently discards a concurrent edit

Filed from libcat-e2e on 2026-07-09 (cross-repo ask).

`PUT /v1/works/{id}/items` replaces an instance's holdings **wholesale** from a list
the client built minutes ago, and reads no `If-Match`. Two catalogers with the item
panel open on the same record: the second save deletes the first one's item. Both
are told `200`, and the panel says *"saved 2 items"*.

`GET /v1/works/{id}/items` hands the client the very ETag that would have caught
it. `lib/api.ts:366` keeps it. `lib/api.ts:371` never sends it. The handler would
not read it if it did.

This is the last of the seven `mutateWorkGrain` files, and the third defect in the
family: **269** (barcodes chosen before the write), **271** (a cycle guard read
before the write), and now a whole item list read before the write -- by the
client.

Measured against **committed HEAD `9ab34de`** on a throwaway clone (`:8473`, never
:8481 or :8501).

## Symptom

```
control: the route works and hands out a token
  PUT  .../items {instanceId, [seed]}     -> 200
  GET  .../items                          -> items=[zzlu-seed-1]  etag=5053b7c0...

both catalogers open the panel and read the same list and the same etag

  A: PUT .../items [seed, zzlu-A]         -> 200
control: A's save really landed
  GET  .../items                          -> [zzlu-A, zzlu-seed-1]
control: and it MOVED the etag
                                             5053b7c0... -> 0990b4cc...

  B: PUT .../items [seed, zzlu-B]         -> 200      <- B's list predates A's write
  GET  .../items                          -> [zzlu-B, zzlu-seed-1]
                                                        ^^^^^^^ zzlu-A is gone

B, again, this time with an explicitly STALE If-Match header
  B: PUT .../items [seed, zzlu-C]
     If-Match: 5053b7c0...                -> 200      <- the header is not read

control: the same stale token, one route over, same work, same moment
  POST .../ops  If-Match: 5053b7c0...     -> 412
  POST .../ops  (no If-Match)             -> 428 {"error":"If-Match required"}

control: the write is a replacement, not a merge
  the instance holds exactly what B sent: [zzlu-B, zzlu-seed-1]
```

The controls are the argument. A's write **moved the etag** (`I1b`), so B's token
was detectably stale. The store **can** see this conflict and the repo **already
asks it to** -- one route over, on the same grain, at the same moment, the same
stale token earns a `412` (`I5`), and its absence earns a `428` (`I6`). Only this
route declines to look.

## Root cause

**The route reads no precondition.** `backend/httpapi/maintenance_handlers.go:109-147`
-- the whole of it, minus audit:

```go
mux.Handle("PUT /v1/works/{id}/items", librarian(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	var req struct {
		InstanceID string          `json:"instanceId"`
		Items      []bibframe.Item `json:"items"`
	}
	...                                         // no r.Header.Get("If-Match")
	etag, err := mutateWorkGrain(r, bs, ix, workID, func(g []byte) ([]byte, error) {
		return bibframe.SetItems(g, req.InstanceID, req.Items)   // `g` is ignored
	})
```

The closure never looks at the grain it is handed. Whatever `g` holds, the result
is `req.Items`.

**Its two siblings do read one.** `records_handlers.go:87-90` and `:175-178`:

```go
ifMatch := r.Header.Get("If-Match")
if ifMatch == "" {
	writeError(w, http.StatusPreconditionRequired, "If-Match required")
	return
}
```

with the comment above `PUT /v1/works/{id}` stating the policy outright:

> *"PUT applies an editorial patch under the client's If-Match token. No silent
> retry: a concurrent write returns 412 with the fresh state so the client can
> rebase deliberately."*

**And `mutateWorkGrain` says it is the wrong tool** (`records_handlers.go:481-483`):

> *"mutateWorkGrain CAS-updates one work's grain, retrying from fresh
> (server-initiated edits like merge/split/batch own their concurrency, **unlike
> the client-token PUT**), ..."*

`PUT .../items` *is* the client-token PUT. Its retry-from-fresh loop re-reads the
grain and re-applies a list computed against a grain that no longer exists, so the
compare-and-swap cannot see the conflict it exists to catch. That is **269**'s
sentence with `req.Items` in place of `generated`.

**`SetItems` is a replacement, and it knows it** (`bibframe/items.go:88-91`):

> *"SetItems replaces an Instance's holdings wholesale: every editorial item
> statement under the Instance's item namespace is dropped and the given items
> re-asserted on freshly numbered skolem nodes."*

and `bibframe/itemops.go:12-16` blesses that shape for exactly this caller:

> *"SetItems replaces an Instance's holdings wholesale, which is **right for the
> item panel** and wrong for a selection."*

It is right for the item panel -- a panel that reads a list, lets a human edit it,
and writes it back. That is the read-modify-write cycle, and it is the one thing
that must not run against a stale read.

**The sibling that appends is safe; the one that clobbers is not.** `items_bulk.go`
(after **269**) re-reads inside the closure and appends:

```go
etag, mErr = mutateWorkGrain(r, bs, ix, workID, func(g []byte) ([]byte, error) {
	current, err := bibframe.ItemsOf(g, req.InstanceID)   // <- from the grain being written
	...
	return bibframe.SetItems(g, req.InstanceID, append(current, generated...))
})
```

so a CAS retry appends to the winner. It also runs under `ix.AllocateBarcodes`.
Neither protection is on the route that overwrites the list outright.

**The UI notices nothing.** `ItemsPanel.svelte:136-152`:

```ts
await putItems(workId, instanceId, cleaned);
status = `saved ${cleaned.length} item${cleaned.length === 1 ? "" : "s"}`;
await load();
```

No `ConflictError` branch, no `etag`. `MarcPanel` handles `ConflictError` and
reloads with a notice; the item panel has never had a token to be wrong about. B
reads *"saved 2 items"*, the reload shows B's two items, and neither cataloger sees
that a third ever existed.

**The same file's `POST /v1/works/{id}/visibility` also takes no `If-Match`.**
*Read, not measured.* Its four actions set or clear a flag rather than replace a
collection, so a stale request re-asserts the state it meant to assert; the
`redirectTo` payload on `tombstone` is the one value it can clobber. Worth a look
in the same pass, not the same severity.

## Why it matters

**It deletes holdings, silently.** A barcode names one physical copy on one shelf.
`bibframe/itemops.go:18-20` refuses to let a *batch* edit touch barcodes at all,
and says why:

> *"Barcode is deliberately absent. A barcode names one physical copy, so assigning
> one across a selection would mint duplicates; clearing one across a selection
> would silently unlink the shelf from the catalog."*

Batch edit will not go near a barcode for fear of unlinking a shelf. The item panel
unlinks shelves whenever two people have it open, and reports success. The harm the
codebase names as unacceptable is reachable by the ordinary path.

**Two people on one record is the normal case for holdings.** Items are added when
copies arrive, and copies arrive in a truck. One cataloger barcoding the new
paperbacks while another fixes a call number on the same instance is a Tuesday, not
a race condition somebody had to engineer. Neither needs to be quick about it: the
window is however long the panel stays open, because the stale list is whatever was
loaded when it opened.

**Nothing anywhere records the loss.** The audit entry says `ITEMS_EDIT` by B with
B's etag, which is true and useless: it names the write that happened, not the one
it erased. There is no tombstone for an item, no feed entry, and the freshly
numbered skolem nodes mean even the grain history shows a renumbered list rather
than a deletion. The first sign is a book that will not check out.

**It is the one route in the family with no defence.** The other two record-editing
routes require the token. `items/bulk` re-reads and appends under a lock. The store
detects the conflict when asked. Every piece needed to fix this is already built and
already used.

## Expected

- **Require `If-Match` and honour it.** The route is a client-token PUT; make it one.
  Reject a missing token with `428` and a stale one with `412` carrying the fresh
  state, exactly as `PUT /v1/works/{id}` documents. `mutateWorkGrain`'s retry loop is
  the wrong helper here -- its whole purpose is to retry *past* the conflict this
  route needs to report. The grain store already refuses a `Put` whose `IfMatch`
  does not hold; the handler just has to pass the client's token instead of the one
  it read a microsecond ago.
- **Send the token from the panel.** `lib/api.ts:371` `putItems` drops the `etag`
  that `fetchItems` (`:366`) already returns; thread it through and give
  `ItemsPanel.save()` the `ConflictError` branch `MarcPanel` has -- reload, show the
  other edit, let the cataloger merge deliberately. "Reload and retype" is a poor
  experience; it is a far better one than losing a copy.
- **Give the invariant a home, or say plainly that it has none.** If `SetItems`
  stays the panel's write, then the panel owes the token. The alternative worth
  considering is an additive/surgical item route (`ItemEdit` already exists for
  batch), which would make the common case -- *add one copy* -- not a whole-list
  replacement in the first place. Then a lost update needs two people editing the
  same item, not the same instance.
- **Check `POST /v1/works/{id}/visibility` in the same pass**, at least for
  `redirectTo`. Same file, same missing precondition, smaller blast radius.
- Consider whether `mutateWorkGrain`'s doc comment should say *and refuse* what it
  says: a helper whose comment names a caller it must not have is a helper that will
  acquire one. It has now acquired three (269, 271, 273).

## Repro

```bash
cd ~/libcat-e2e && node harness/probe_items_lost_update.mjs   # I2, I3, I4
cd ~/libcat-e2e && node harness/retest.mjs                    # check t273
```

The probe never addresses :8481 or :8501 and never reads `~/libcat`'s working tree:
`roinstance.buildHead()` exports committed HEAD with `git archive` into a scratch
dir and builds `cmd/lcatd` there. It clones the playground's site (`cp -Rc`,
copy-on-write), boots a writable instance on :8473, and deletes the clone, so the
items it writes can never reach the playground.

Its controls carry the argument. `I0` shows the route works and hands out an etag.
`I1` shows A's save really landed, so B is overwriting a real edit rather than
writing into a gap. `I1b` shows A's write **moved the etag**, so B's token was
detectably stale -- without it, "no 412" would be equally consistent with an etag
that never changes. `I5` and `I6` show the same stale token earning `412`, and its
absence earning `428`, on `POST .../ops` against the same work at the same moment:
the store can see this conflict and the repo already asks it to. `I7` shows the
instance ends holding *exactly* what B sent, so this is a replacement rather than a
failed append.

`I4` is the check that closes the "the client should have sent it" reading: B sends
an explicitly stale `If-Match` and still gets `200`. The header is not merely
unsent. It is unread.

By hand:

```bash
TOK=...; W=...; I=...        # a work and one of its instance ids

curl -s -H "Authorization: Bearer $TOK" localhost:8473/v1/works/$W/items
# {"workId":"...","etag":"5053b7c0...","items":{"<I>":[{"barcode":"zzlu-seed-1",...}]}}

# A saves
curl -s -XPUT -H "Authorization: Bearer $TOK" -H 'Content-Type: application/json' \
  -d '{"instanceId":"'$I'","items":[{"barcode":"zzlu-seed-1"},{"barcode":"zzlu-A"}]}' \
  localhost:8473/v1/works/$W/items                                     # 200

# B saves the list it read before A -- with A's own etag, for good measure
curl -s -XPUT -H "Authorization: Bearer $TOK" -H 'Content-Type: application/json' \
  -H 'If-Match: 5053b7c0...' \
  -d '{"instanceId":"'$I'","items":[{"barcode":"zzlu-seed-1"},{"barcode":"zzlu-B"}]}' \
  localhost:8473/v1/works/$W/items                                     # 200

curl -s -H "Authorization: Bearer $TOK" localhost:8473/v1/works/$W/items
# items: zzlu-seed-1, zzlu-B        <- zzlu-A is gone

# the same stale token, one route over:
curl -s -XPOST -H "Authorization: Bearer $TOK" -H 'Content-Type: application/json' \
  -H 'If-Match: 5053b7c0...' -d '{"ops":[...]}' localhost:8473/v1/works/$W/ops
# 412
```
