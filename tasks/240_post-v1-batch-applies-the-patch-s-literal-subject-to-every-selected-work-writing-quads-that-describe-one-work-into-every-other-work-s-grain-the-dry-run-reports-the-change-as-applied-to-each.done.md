# 240 -- POST /v1/batch applies the patch's literal subject to every selected work, writing quads that describe one work into every other work's grain; the dry-run reports the change as applied to each

Filed from libcat-e2e on 2026-07-09 (cross-repo ask).

## Symptom

`POST /v1/batch` is documented as "one patch applied to many works"
(`records_handlers.go:312`). A patch names its subject as an absolute IRI. The
route never rebinds that subject per work, so every work in the selection gets a
statement **about the first work**.

Two fresh copycat sentinels, `A = ws8fad6e8eje4i` and `B = w6nlabe89d27n2`, and
one patch adding a subject to `#<A>Work`:

```
dry-run results: [{"w":"ws8fad6e8eje4i","added":1},{"w":"w6nlabe89d27n2","added":1}]

B = w6nlabe89d27n2 | quads in B's grain naming A as subject: 1
    <#ws8fad6e8eje4iWork> <http://id.loc.gov/ontologies/bibframe/subject> <https://homosaurus.org/v4/zzprobe> <editorial:> .

B doc subjects: null
```

Three things are wrong at once:

1. **B's grain now contains a statement describing A.** It is permanent.
2. **B's editor doc shows nothing** -- the doc reads statements whose subject is
   B's own work IRI, so the foreign quad is invisible. It exports, though:
   `nquads` and `jsonld` carry it.
3. **The dry-run says `added: 1` for B.** The caller is told the subject was
   added to B. It was not; a quad about A was added to B's grain.

The only other option is to omit the subject, and that is refused:

```
POST /v1/batch with s:"" -> 400 "editor: statement with empty term"
```

So there is no way to use this route correctly across more than one work.

Measured by `ui/probe_history.mjs`:

```
FAIL H7  a batch patch does not write another work's subject into a grain
         2 grain(s) now contain a statement about #wfkipo7r8frcjoWork
```

## Root cause

`backend/httpapi/records_handlers.go:357-359` applies the same
`req.Patch` to each work's grain unchanged:

```go
etag, err := mutateWorkGrain(r, bs, ix, workID, func(grain []byte) ([]byte, error) {
    return bibframe.ApplyEditorialPatch(grain, req.Patch.ToBibframe())
})
```

`ApplyEditorialPatch` → `ApplyPatch` (`bibframe/editorial.go:105-111`) writes
the patch's terms verbatim into the named graph. Nothing rewrites `S`, and
`editor.Patch` has no placeholder convention for "this work" -- `Validate`
rejects an empty term outright (`editor/patch.go`).

The dry-run diverges from the write for the same reason and in the same place:
`editor.ComputeDiff(grain, req.Patch)` (line 351) counts the patch's statements
as additions to the grain it was handed, without checking that their subject is
that grain's work.

`TestRecordEditFlow` (`records_handlers_test.go:228-253`) does exercise the
route with two work ids -- but the second is `"wmissing000000"`, which errors
before any write, and the patch's subject is `WorkIRI(editWorkID)`, the first
one. So the only case the test covers is the one where the literal subject
happens to be correct. Two *real* works have never been batched.

The sibling route `POST /v1/batch/ops` does not have this bug: it takes
`[]editor.Op`, which addresses `resource: "work"` symbolically and resolves it
per grain (`batch.go:runOne`). That is the model `/v1/batch` is missing.

## Why it matters

This is silent, invisible, permanent corruption of every record in a batch
except one. The grain is the source of truth. A statement describing work A now
lives in work B's grain, where:

- the editor doc will never show it, so a cataloger cannot see or remove it;
- `nquads` and `jsonld` exports carry it, so it escapes into whatever consumes
  them, asserting a fact about A from a document that is supposed to be B;
- `CloneGrain` copies it (its subject is not the work or an instance, so it is
  neither dropped nor re-minted -- see tasks/238), propagating it to clones;
- nothing will ever clean it up, because nothing knows it is there.

And the dry-run -- the one affordance the route offers for checking a batch
before committing it -- actively reports success. A cataloger who does the
responsible thing and dry-runs first is told the edit will land on all N works.

The route is librarian-gated and documented in `docs/api.md:219`. The SPA does
not call it (`runBatch` posts to `/v1/batch/ops`), so the blast radius today is
API consumers only. That is what keeps this from being an emergency; it is not
what makes it correct.

## Expected

Pick one, and make the dry-run agree with it:

- **Rebind the subject per work.** Give `editor.Statement` a way to say "this
  work" -- a sentinel subject like `"#Work"`, or an empty `S` that `Validate`
  accepts only in a batch context -- and substitute `bibframe.WorkIRI(workID)`
  per grain. This is what `/v1/batch/ops` already does with
  `resource: "work"`, and it makes the route mean what its comment says.
- **Or refuse the request.** If `S` must stay absolute, reject any batch whose
  patch names a work IRI other than the single work it is applied to. With more
  than one work id, that means rejecting every work-subject patch -- which is
  an honest way of saying the route only ever worked for one work.

Either way, `ComputeDiff` must not report an addition to a grain whose work the
statement does not describe.

Worth deciding separately: whether `ApplyEditorialPatch` should refuse a
statement whose subject is a `#<id>Work` / `#<id>Instance` fragment belonging to
a *different* grain. Every caller today would still pass, and it would have
turned this into a 400 rather than silent corruption.

## Repro

```
cd ~/libcat-e2e && node ui/probe_history.mjs
```

Expect `H7` to flip to PASS. `H6` (the truncated `batchNote`, tasks/239) shares
the probe. The probe mints its own copycat sentinels and tombstones all of them;
it never touches a pre-existing record. `harness/retest.mjs` carries the same
check as `t240`.

To see the dry-run disagreement directly, the standalone measurement above is:
patch `{s: "#<A>Work", p: bf:subject, o: <iri>}`, `workIds: [A, B]`,
`dryRun: true` → both report `added: 1`; then execute and read B's grain.

## Outcome

Fixed in **v0.92.0**. Took the first of the two options: **rebind the subject
per work**, and diff the rebound patch so the dry run previews the write.

- `bibframe.WorkIDFromIRI` recognizes exactly the node `WorkIRI` mints, and
  nothing that merely starts like it (`#<id>Work-ed-title` is a title node).
  Unit-tested and fuzzed for round-trip and ambiguity.
- `editor.Patch.RebindWork(workID)` rewrites Work-node subjects;
  `Patch.Rebindable()` refuses what cannot be rebound -- an Instance node or a
  skolem child names a node in one grain and nothing at all in another, and a
  grain-local object has the same problem.
- `POST /v1/batch` calls `Rebindable` once (400 on the whole request: the patch
  is malformed, not the selection), then applies `RebindWork(workID)` for both
  the dry-run diff and the write.

Also closed the front door the report did not name: **`PUT /v1/works/{id}` takes
the same raw patch shape**, so a subject naming another work wrote a quad about
that work into this work's grain, one record at a time. A single-record patch
cannot be rebound (the caller named the work in the URL), so a Work-node subject
there must name that work; `POST /v1/works/{id}/validate` now refuses exactly
what the write refuses. Both guards were verified to fail before they existed.

## A correction to the "worth deciding separately" aside

The report suggests pushing the check into `bibframe.ApplyEditorialPatch`, on the
grounds that "every caller today would still pass." That is not so, and it is why
the guard sits at the route instead.

`AddMergeMarker` (`bibframe/merge.go:96`) writes

```
<#wFROMWork> <lcat:mergedInto> <#wTOWork> <editorial:> .
```

into the **survivor's** grain. Its subject is a Work node belonging to a
different grain, and its object is a grain-local IRI -- exactly what a
grain-level refusal would reject. A grain describing another Work node is not
universally wrong; for merge provenance it is the point, and the identity
resolver reads it.

The consequence, stated plainly: `PUT /v1/works/{id}` can no longer hand-write a
merge marker. `POST /v1/works/merge` is the audited path for that, and always
was.

Verified with the filer's own probe on the rebuilt playground: `H7` **PASS**.
`H4`, `H5`, and `H9` still fail -- those are tasks/239, taken next.

## Verification (filer)

Fixed. Confirmed 2026-07-09 by `harness/retest.mjs` (`t240` FIXED) and by
`ui/probe_history.mjs` (`H7`). Measured directly against two fresh sentinels,
with a patch whose subject names A, applied to `[A, B]`:

```
dry: [{"w":"wlsbrbrhgrpr6m","added":1},{"w":"wt158qifc8a1qq","added":1}]
B names A as subject:                0
B has the term bound to ITSELF:      1
```

Rebinding the subject per work is a better fix than the "refuse the request"
alternative I offered: it makes the route mean what its comment always said, and
it brings `/v1/batch` into line with `/v1/batch/ops`, which resolves
`resource: "work"` per grain.

**I had to rewrite my own check to see it.** `t240` originally asserted that a
correct implementation would report **no** additions to B in the dry run --
which encoded the *old*, broken semantics, where the only truthful preview of a
patch about A applied to B was "nothing happens to B". Under rebinding, B
genuinely gains a statement about itself, so `added: 1` is the right answer and
my check was reporting STILL-BROKEN against a working fix. It now asserts the
invariant that actually matters, and that holds under either design:

- B's grain contains no statement whose subject is `#<A>Work`;
- B gains exactly one statement bound to `#<B>Work`;
- **the dry run's predicted addition count equals what execute performed.**

That last line is the one worth keeping. It is the property the report was
really about, it cannot pass vacuously, and it would have caught the original
bug from either end. `H7` in `ui/probe_history.mjs` was widened the same way: it
had only checked for the *absence* of a foreign subject, which would have passed
even if the patch had written nothing at all.
