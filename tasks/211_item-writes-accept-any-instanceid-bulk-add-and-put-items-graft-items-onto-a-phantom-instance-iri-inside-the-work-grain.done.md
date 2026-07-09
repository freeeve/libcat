# 211 -- item writes accept any instanceId: bulk add and PUT items graft items onto a phantom instance IRI inside the work grain

Filed from libcat-e2e on 2026-07-09 (cross-repo ask).

## Symptom

Neither `POST /v1/works/{id}/items/bulk` nor `PUT /v1/works/{id}/items` checks
that `instanceId` names an Instance of `{id}`. Any string is accepted, and the
items are written into that work's grain hanging off a `#<instanceId>Instance`
IRI that does not exist.

`GET /v1/works/{id}/items` enumerates the grain's *real* instances, so the items
it just accepted are then invisible to the only endpoint that lists them.

Measured on `w0cfnsjg6micju` (`node harness/probe_items.mjs`, 2026-07-09). The
probe snapshots the grain first and restores it byte for byte:

| # | Check | Result |
|---|---|---|
| I7 | bulk add with `instanceId=izzzzzze2ephantom` (not an instance of the work) | `dryRun` → 200, **write → 200** |
| I8 | orphan quads in the grain | **10 quads**, including a `bf:hasItem` edge from the phantom IRI |
| I9 | phantom shown by `GET /v1/works/{id}/items` | **no** — lists only `["i3mlbhb415ml5q"]` |
| I10 | orphan barcodes consume the sequence | phantom took `ZZE2E0106,ZZE2E0107`; next real barcode is `ZZE2E0108` |

The realistic version of the mistake is a **valid instance id copied from the
wrong record**, and it behaves the same way:

```
(a) bulk W1 with W2's instance id -> 200 | W1 grain mentions W2 instance: 4 quads
    W1 GET items instances: [ 'i3mlbhb415ml5q' ]      <- the grafted instance is invisible
    W2 GET items (unaffected): {"ilneh9s9klnbig":[]}  <- and W2 never sees the items either
(b) PUT items, phantom instance    -> 200 | quads: 4  <- same hole on the other route
```

So `W1`'s grain now asserts `<#ilneh9s9klnbigInstance> bf:hasItem <…-item-1>`
for an Instance that belongs to `W2`. Neither record displays the item.

## Root cause

`bibframe.SetItems` (`bibframe/items.go:87`) is an unconditional writer. It
strips the editorial quads under `itemPrefix(instanceID)` and re-adds the list,
minting `inst := rdf.NewIRI(InstanceIRI(instanceID))` (`:110`) and emitting
`inst bf:hasItem <item>` (`:113`) whatever `instanceID` is. `InstanceIRI`
(`bibframe/merge.go:42`) is pure string concatenation: `"#" + id + "Instance"`.

Both callers hand it an unvalidated request field:

- `backend/httpapi/items_bulk.go:44` -- rejects only `InstanceID == ""`, then
  `:68` `bibframe.ItemsOf(grain, req.InstanceID)` returns an empty list for an
  unknown instance (no error), so the `len(existing)+req.Count > 200` guard at
  `:73` passes and `:99` calls `SetItems` with the phantom id.
- `backend/httpapi/maintenance_handlers.go:119` -- same, straight to `SetItems`
  at `:128`.

The validation already exists a few lines away and is simply not used on the
write paths: `GET /v1/works/{id}/items` (`maintenance_handlers.go:90-98`) calls
`identity.ScanGrain(grain)` and iterates `gi.Instances` -- the authoritative set
of instance ids for that work. That is exactly the membership check the writers
need, which is why the orphans are invisible to the reader.

## Why it matters

Items are the holdings: the barcode a patron scans and the call number a page
walks to. A typo in `instanceId` -- or a copy-paste from the wrong record while
working two tabs -- silently succeeds, reports `200` with a barcode list, and
puts the holdings somewhere no screen will ever show them. The cataloger's only
signal that anything is wrong is that the items they just created do not appear.

Two knock-on effects make it worse than an inert write:

1. **The barcodes are really consumed.** They land in the grain, the grain is
   indexed (`mutateWorkGrain` → `ix.Apply`), and `ix.Barcodes` feeds
   `nextBarcodes` (`items_bulk.go:82`). I10 shows the next legitimate add
   skipping straight past them. Barcode sequences are physical labels; a gap is
   permanent.
2. **The grain now carries a false assertion.** A work's grain claiming
   `bf:hasItem` on another work's Instance IRI is bad BIBFRAME, and the only way
   to remove it is to already know the phantom id, because nothing lists it.
   (`PUT items` with `items: []` and the phantom id does strip it -- that is how
   this probe cleans up -- but a cataloger has no way to discover the id.)

The 200-per-instance cap is also defeated: each phantom id gets its own fresh
200-item budget.

## Expected

Both handlers resolve the work's instances before writing and reject an
`instanceId` that is not among them:

```go
gi, err := identity.ScanGrain(grain)
// ...
if !slices.ContainsFunc(gi.Instances, func(i identity.Instance) bool { return i.InstanceID == req.InstanceID }) {
    writeError(w, http.StatusBadRequest, "no such instance on this work")
    return
}
```

`items_bulk.go` already reads the grain at `:64` for the 200-item check, so the
scan is nearly free there. `PUT items` reads it inside `mutateWorkGrain`, so the
check belongs in the mutate closure. `dryRun` should reject too -- it currently
previews barcodes for an instance that cannot receive them.

Worth considering alongside: `SetItems` could return an error when
`instanceID` is absent from the grain, so no future caller can reintroduce this.

## Repro

```
cd ~/libcat-e2e && node harness/probe_items.mjs   # I7, I8, I9, I10 FAIL
cd ~/libcat-e2e && node harness/retest.mjs        # reports 211 STILL-BROKEN
```

The probe writes only to one work, snapshots its grain first, strips every
instance it touched, and asserts the grain is restored byte for byte
(`CLEAN … identical=true`).

## Not bugs (verified clean this cycle)

Bulk add is otherwise well guarded: a missing `barcodePrefix`, `count` of 0 or
101, `barcodeWidth` of 13, a missing `instanceId` and a malformed work id all
return 400; anonymous callers get 401. `dryRun` genuinely writes nothing (grain
identical afterwards). Barcodes auto-increment past the highest existing counter
for the prefix and collide with nothing (`ZZE2E0001…0005`, all unique). The
200-items-per-instance cap holds for a legitimate instance: 105 items succeed,
the next 100 are refused. `ITEMS_BULK_ADD` and `ITEMS_EDIT` both reach the audit
log.

## Outcome

Fixed in 86d5b8d, released v0.62.0 -- both your Expected layers plus
your "worth considering":

- bibframe.SetItems refuses an instance id the grain does not describe
  (ErrNoSuchInstance; the tasks/202 subject-presence pattern), so no
  future caller can reintroduce the graft.
- The bulk handler pre-checks membership against identity.ScanGrain --
  the same authoritative set GET items reads, exactly as you pointed
  out -- and rejects on dryRun too, so a preview can't promise
  barcodes an instance cannot receive. The PUT route surfaces the
  sentinel as 400 "no such instance on this work" via
  writeMutateError.
- TestItemWritesRejectPhantomInstance pins phantom PUT, phantom bulk
  (dry and wet), byte-untouched grain after rejections, and the real
  instance still writing.

Verified with your probe: I1-I10 + CLEAN all PASS (I7-I10 were the
failures; the wrong-record variant is covered by the same membership
check).
