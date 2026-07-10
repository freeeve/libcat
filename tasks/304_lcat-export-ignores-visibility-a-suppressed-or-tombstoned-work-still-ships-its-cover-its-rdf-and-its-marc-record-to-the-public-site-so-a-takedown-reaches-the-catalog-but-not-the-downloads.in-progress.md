# 304 -- lcat export ignores visibility: a suppressed or tombstoned work still ships its cover, its RDF and its MARC record to the public site, so a takedown reaches the catalog but not the downloads

Filed from libcat-e2e on 2026-07-10 (cross-repo ask).

The **projector** honours visibility everywhere. `project/project.go:578-582` drops both
stances before a Work reaches `cat.Works`:

```go
if len(p.view.Objects(w, bibframe.PredTombstoned)) > 0 {
	continue
}
if lit, ok := p.view.Literal(w, bibframe.PredSuppressed); ok && lit == "true" {
	continue
}
```

Every downstream artifact is derived from `cat.Works` -- `cat.Facets()`, `cat.Similar()`, the
term sideband, the search index (built from `catalog.json`), and `resolveRelations`, which drops
`hasPart`/`partOf` links whose target left the projection. `Relations` has exactly those two
fields (`project.go:153-156`), so that filter is complete. **Nothing about the projected catalog
leaks a hidden work.**

The **exporter** honours nothing. `export/export.go:95-105`:

```go
nq, err := copyGzip(filepath.Join(opts.In, "catalog.nq"), filepath.Join(opts.Out, "catalog.nq.gz"), opts.PublicSources, opts.Log)  // :95
…
if err := copyCovers(opts.In, opts.CoversOut); err != nil {                        // :102
…
mrc, xml, err := emitMARC(grains, opts.Out, opts.Log, opts.OrgCode)                // :105
```

- `copyGzip` is a line copier. Its only filter is the **provenance** allowlist
  (`--public-sources`), which strips `lcat:extra/sources` quads. It never looks at a work.
- `copyCovers` (`:335-356`) walks the blob tree and writes every file it finds:

  ```go
  root := filepath.Join(in, "data", "covers")
  return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
  	…
  	return os.WriteFile(filepath.Join(out, filepath.Base(path)), data, 0o644)
  })
  ```

  It never opens `catalog.json`, never reads a grain, never asks what is visible. The flattened
  name is `<workID>.<ext>` -- exactly the `covers/<workID>.<ext>` string the projector would
  have emitted as `extra.cover` had the work been published.
- `emitMARC(grains, …)` is handed **every grain**, one ISO 2709 record each.

`grep -rn 'Tombstoned\|Suppressed' export/` returns nothing.

`runExport`'s own doc comment (`cmd/lcat/export.go:13-17`) shows this is not a general
inattention to the download path -- somebody thought carefully about what to strip from it:

> `--public-sources` applies the same provenance allowlist to the nq download that
> `lcat project` applies to `catalog.json`; **the on-disk graph of record stays complete.**

Provenance was carried across from the projector to the exporter. Visibility was not.

## Symptom

Measured on a throwaway writable clone of the playground, pinned to committed HEAD, driving the
real CLI (`lcat serialize` → `lcat project` → `lcat export --covers-out`) exactly as the
playground's `run.sh` and `lcat build` do. Three sentinel works were given cover images through
`PUT /v1/works/{id}/cover`; one was then suppressed, one tombstoned, one left visible.

Controls first. The visible work projects and its cover is published, so covers are copied at
all (`H1`). The projector really does hide the other two -- neither id appears in `catalog.json`
(`H2`). And nothing in `catalog.json` references either hidden work's cover (`H3`), so the files
below are **unreferenced**: no page renders them, no crawler reaches them by following a link.

Then:

```
covers/w1dh6vtir43o8i.png    the SUPPRESSED work's cover -- published
covers/w41iq8jmgsm1po.png    the TOMBSTONED work's cover -- published
covers/w0cfnsjg6micju.png    a stale-format orphan, unreferenced (its work points at .jpg) -- published

catalog.nq.gz   266 quads naming the suppressed work, 9 naming the tombstoned one
catalog.mrc.gz  3274 MARC records, one per grain, while catalog.json carries 30 works
```

`redirects.json` **publishes the tombstoned work's id** (`from: "w41iq8jmgsm1po"`, the
no-successor entry the host serves as gone, tasks/051). So `covers/w41iq8jmgsm1po.png` is not
guessed. It is *derived* from an artifact the site serves.

### The scale, on the playground's own store

```
$ cd site/data/works
$ find . -name '*.nq' | wc -l                             3274   grains
$ grep -rl 'tombstoned' . --include=*.nq | wc -l          3236   tombstoned
$ grep -rl 'suppressed> "true"' . --include=*.nq | wc -l   327   suppressed
$ jq '.works | length' ../../catalog.json                    31   works in the public catalog
```

**The public catalog shows 31 works. `lcat export` publishes the complete RDF and a full MARC
record for 3274 -- 3236 of them tombstoned.** The downloads page and the catalog describe
different collections, and the difference is precisely the set of records somebody decided the
public should not see.

### `lcat covers --reap` cannot clean this up

The obvious objection, and it does not hold. `findOrphanCovers` (`cmd/lcat/covers.go:150-183`)
asks three questions of each blob:

```go
switch {
case !c.hasGrain:                        … reasonNoWork
case c.cover == "":                      … reasonNoCover
case c.cover != coverURLOf(entry.Path):  … reasonStaleFormat
}
```

Suppression and tombstoning are editorial statements, not deletions -- `bibframe/visibility.go:
8-12` says a suppressed Work *"merely hides from projection with no redirect and is fully
restorable"*, and `SetTombstone` (`:83-86`) *"returns the re-canonicalized grain"*. A hidden work
still has a grain and still claims its cover, so it is an orphan by none of the three reasons and
`--reap` never touches it. Measured: the reaper scanned 5 blobs, found 1 orphan, and named
neither hidden work.

(Relatedly, `reasonNoWork`'s comment reads *"a missing work is a tombstone or a hand-deleted
grain"*. A tombstone leaves the grain in place; only the hand-deleted case reaches that branch.)

## Root cause

`export/export.go:95-105` and `export/export.go:335-356`. `export.Run` publishes from the
**store**; `lcat project` publishes from the **graph view**, with the visibility filter applied.
The two run from the same directory in the same build -- `cmd/lcat/build.go:241-252` calls
`export.Run(opts)` with `[export] covers-out` -- and disagree about what the collection is.

## Why it matters

**`lcat covers`'s own doc comment states the threat model, one directory over**
(`cmd/lcat/covers.go:46-52`):

> those images are still served from a public, unauthenticated, guessable URL, and nothing in the
> catalog points at them, so nothing would ever collect them. **A takedown that looks done is not
> done.**

That command exists to collect exactly this residue from the *store*. Meanwhile the exporter
copies whatever is in the store onto the *public site*, hidden or not. Reaping a blob after it
has been published to a CDN does not unpublish it.

**Suppression is the takedown button.** It is what a librarian presses for a record that must
come off the public catalog now -- a privacy complaint, a legal demand, a patron name left in a
note field, a cover image with a rights problem. The record leaves `catalog.json`, the facets,
the search index, and the "more like this" rail. Its cover stays at a derivable URL, its full RDF
stays in `catalog.nq.gz`, and its MARC record stays in `catalog.mrc.gz` and `catalog.xml.gz`.
**The one action whose entire purpose is to remove a record from public view removes it from four
artifacts and leaves it in four others.**

**The downloads are the worst place for it to leak.** A cover is one image. `catalog.nq.gz` is
the complete graph of record -- every note, every editorial quad, every statement the cataloguer
thought was internal. It is a single unauthenticated file, and harvesters mirror it.

**Nothing in the product can see this.** `catalog.json` is correct, so the OPAC looks correct.
The manifest reports `Records: len(grains)` (`export.go:99`) with no counterpart from the catalog
to compare against. A librarian who suppresses a record and reloads the site sees it gone.

**Tombstones make it worse, not better.** A tombstoned work's id is *published* in
`redirects.json` by design. The site names the id of every record it retired, and the exporter
leaves each one's cover at `covers/<that id>.<ext>`.

## Expected

- **Filter the exporter by the stance the projector already uses.** `export.Run` reads the grains
  anyway; `bibframe.Visibility(grain, workID)` is one call. Skip a hidden work in `emitMARC`, and
  skip quads about it in `copyGzip`.

  The tombstoned case needs one judgement the suppressed case does not: `redirects.json`
  deliberately names the id, so the id is public. Dropping the *record* from the downloads is
  still right -- the redirect says "this id is gone", not "here is what it was".

- **Drive `copyCovers` from the catalog, not the blob tree.** The set of covers the public site
  needs is exactly `{w.Extra.Cover : w ∈ cat.Works}`. That also collects the tasks/243
  stale-format residue for free, since a work references one cover and the projector emits that
  one. Today `--reap` is the only thing between an orphan blob and the public site, and it runs
  against the store, after the fact, if it runs at all.

- **Give the manifest a number that can be wrong.** `nq.Records = len(grains)` cannot disagree
  with anything. If the exporter recorded that count beside the projector's work count, a build
  that published 3274 records for a 31-work catalog would be visible in the build log.

- **Decide what `--public-sources` means.** It is described as "the same provenance allowlist
  `lcat project` applies to `catalog.json`". If the exporter is meant to mirror the projector's
  public face, visibility belongs in the same sentence. If it is meant to be a complete archival
  dump, it should not be written into the site's public directory, and *"the on-disk graph of
  record stays complete"* should say which of the two it means.

## Repro

```bash
cd ~/libcat-e2e && node harness/probe_hidden_cover_published.mjs   # H4-H7, H9, H10
cd ~/libcat-e2e && node harness/retest.mjs                         # check t304
```

**Touches neither `:8481` nor `:8501`.** It boots its own writable clone of the playground
(`cp -Rc`, APFS copy-on-write) pinned to committed HEAD, uploads three sentinel covers through
the real `PUT /v1/works/{id}/cover`, hides two works through the real
`POST /v1/works/{id}/visibility`, runs the real `lcat serialize` / `project` / `export` built
from that same HEAD, exports into a scratch directory, and deletes both wholesale.

By hand, against any grain root:

```bash
lcat serialize --dir site
lcat project   --catalog site/catalog.nq --out site --provider marc
lcat export    --in site --out site/exports --covers-out site/covers

jq '.works | length' site/catalog.json                          # what the public sees
gzip -dc site/exports/catalog.mrc.gz | tr -cd '\035' | wc -c    # what the public can download
ls site/covers | wc -l
```

An earlier version of this probe read `catalog.nq.gz` through `spawnSync`'s 256MB `maxBuffer`.
The file is 306MB, so stdout was silently truncated -- and because `SerializeGrains` emits grains
in sorted id order, the suppressed sentinel (`w1d…`) fell before the cut and the tombstoned one
(`w41…`) after it. The probe reported that the nq download carries suppressed works but not
tombstoned ones: a clean, plausible, entirely invented asymmetry, and one I nearly filed. It now
streams through `grep` and counts matches. **A truncated read does not report an error; it
reports an absence.**
