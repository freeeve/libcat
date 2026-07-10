# 252 -- RemoveSnapshot leaves the sidecar index artifacts on disk, contrary to its own doc comment

Filed from libcat-e2e on 2026-07-09 (cross-repo ask).

## Symptom

`DELETE /v1/vocabsources/{name}/snapshot` removes the snapshot `.nq` and its `.json`
sidecar-meta, reloads the index, and the scheme's terms correctly stop resolving. It
does **not** remove the eight roaringrange artifacts `BuildSidecar` wrote. They stay on
disk forever.

Installed a sentinel source, uploaded a two-concept dump, removed the snapshot, then
deleted the source entirely:

```
upload -> 200 {"installed":true,"terms":2}
remove snapshot -> 200 {"removed":true}
term after removal -> 404
source deleted

$ ls site/data/authorities/sidecar | grep zzsidecar
zzsidecar.id1.rril  zzsidecar.id2.rril  zzsidecar.id3.rril
zzsidecar.manifest.json
zzsidecar.rrsr.bin  zzsidecar.rrsr.idx
zzsidecar.search.rrt  zzsidecar.uri.rril
```

The snapshot the manifest names is gone, so the manifest dangles:

```json
{"version":2,"scheme":"zzsidecar",
 "source":"data/authorities/vocab/zzsidecar.nq",
 "sourceETag":"2f12c35b70…","sourceSchemes":["zzsidecar"],"terms":2,"live":2}

$ ls site/data/authorities/vocab/zzsidecar.nq
No such file
```

This is not hypothetical accumulation. The playground was carrying an orphan from an
**earlier** harness cycle, days old and surviving every restart since:

```
$ cat site/data/authorities/sidecar/zze2e.manifest.json
{"version":2,"scheme":"zze2e","source":"data/authorities/vocab/zz-e2e-snap-4ryz.nq",…}
```

`zze2e` was not a registered source, had no snapshot, and served no terms
(`GET /v1/terms?scheme=zze2e` -> `{"terms":[]}`) -- eight dead files, indefinitely.

Sizes make it matter. Per-scheme sidecar footprint on the playground:

```
lcsh:       8 files, 169M
lcshac:     8 files, 6.1M
homosaurus: 8 files, 3.5M
lcgft:      8 files, 1.4M
```

An operator who removes `lcsh` to reclaim space reclaims the `.nq` and leaves **169MB**
of artifacts for a scheme that no longer exists.

## Root cause

`backend/vocabsrc/vocabsrc.go:338-353`. The doc comment states the contract, and the
body does not implement it:

```go
// RemoveSnapshot deletes an installed snapshot and its sidecar, then reloads
// the index so the scheme's terms drop out.
func (s *Service) RemoveSnapshot(ctx context.Context, name string) error {
	if _, _, err := s.Blob.Get(ctx, s.metaPath(name)); errors.Is(err, blob.ErrNotFound) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	if err := s.Blob.Delete(ctx, s.snapshotPath(name)); err != nil && !errors.Is(err, blob.ErrNotFound) {
		return err
	}
	if err := s.Blob.Delete(ctx, s.metaPath(name)); err != nil {
		return err
	}
	return s.Reload(ctx)
}
```

Two deletes, neither for `<prefix>sidecar/<scheme>.*`. `DeleteSource`
(`vocabsrc.go:225`) does not remove them either.

There is no helper to call: `sidecarPath` is unexported in
`backend/vocab/sidecar_build.go:58`, and nothing in `backend/vocab` exports a removal.
The build path already deletes one stale artifact by hand
(`sidecar_build.go:184`, the legacy `.search.bin`), so the precedent for cleanup exists
in exactly one place.

`backend/vocabsrc/vocabsrc_test.go:351` is why this was never caught:

```go
	// Remove: snapshot and sidecar go, terms drop out of the index.
	if err := s.RemoveSnapshot(ctx, "lcgft"); err != nil { … }
	if got := ix.Search("lcgft", "zin", 5); len(got) != 0 { … }
```

The comment asserts the sidecar goes; the code only asserts the index forgot the terms.
Nothing looks at the blob store. A test comment is not a test.

## Why it matters, and what it is *not*

Correctness is safe: I checked before filing. `RemoveSnapshot` calls `Reload`, which is
the same `buildSnapshot` the server runs at startup, so the orphan behaves identically
across restarts. Pass 3 (`backend/vocab/vocab.go:262-287`) arms a manifest only if its
scheme is present in `deferred`, which is built from the `.nq` inventory in pass 2. The
source `.nq` is deleted, so the scheme is never deferred, `!ok` holds, it logs
`vocab: sidecar stale; serving scheme from maps`, and the scheme never arms. **Removed
terms do not come back.** I verified the 404 survives the reload.

So this is a resource leak and a broken contract, not data resurrection:

- dead bytes proportional to the vocabulary removed (169MB for `lcsh`), unbounded
  across distinct scheme names over a deployment's life;
- an object store bill for artifacts nothing can read;
- a `sidecar/` directory whose contents no longer describe what is installed, which is
  the first place anyone debugging vocabulary loading will look. The dangling manifest
  actively lies: it says `terms: 2, live: 2` for a scheme that serves nothing.

The blast radius is small precisely because pass 3 is defensive. That defensiveness is
what turned a possible "deleted vocabulary reappears after restart" bug into a leak, and
it is worth keeping.

## Expected

`RemoveSnapshot` should delete the scheme's sidecar artifacts along with the snapshot
and meta, honouring its comment. That needs an exported helper in `backend/vocab` --
something like `RemoveSidecar(ctx, st, prefix, scheme) error` deleting the manifest
**first** (its presence is what arms the scheme, so removing it first is the safe order
if the process dies mid-delete), then the six data artifacts and the legacy
`.search.bin`.

`DeleteSource` should be considered too: deleting a source whose snapshot is still
installed leaves an "orphan install" that `Views` deliberately synthesizes so it stays
removable, so that path is probably correct as-is -- but it should be a decision, not an
accident.

The test at `vocabsrc_test.go:351` should assert what its comment claims: that
`sidecar/lcgft.manifest.json` is absent from the blob store afterwards. Given the
comment already says so, a one-line `Get` that expects `ErrNotFound` would have caught
this.

## Repro

```bash
cd ~/libcat-e2e && node harness/probe_vocabsource.mjs   # check S20
cd ~/libcat-e2e && node harness/retest.mjs              # check t252
```

By hand, against the playground:

```bash
TOK=…   # admin
curl -XPOST  -H "Authorization: Bearer $TOK" -H 'Content-Type: application/json' \
     -d '{"name":"zzleak","scheme":"zzleak"}' localhost:8481/v1/vocabsources
printf '<http://example.org/z/1> <http://www.w3.org/2004/02/skos/core#prefLabel> "Z"@en .\n' \
  | curl -XPUT -H "Authorization: Bearer $TOK" --data-binary @- \
         localhost:8481/v1/vocabsources/zzleak/snapshot
ls ~/libcat-playground/site/data/authorities/sidecar | grep zzleak   # 8 files
curl -XDELETE -H "Authorization: Bearer $TOK" localhost:8481/v1/vocabsources/zzleak/snapshot
curl -XDELETE -H "Authorization: Bearer $TOK" localhost:8481/v1/vocabsources/zzleak
ls ~/libcat-playground/site/data/authorities/sidecar | grep zzleak   # still 8 files
```

The harness cleaned up the orphans it had left on the playground (`zze2e.*`,
`zz-e2e-vsrc*.*`, `zzsidecar.*`); `sidecar/` is back to the four real schemes.
