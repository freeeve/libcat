# 291 -- lcat export emits sequential _:bN blank-node labels, so catalog.nq.gz and its published sha256 change on every release even when the catalog is identical -- grains already use canonical _:c14nN

Filed from queerbooks-demo on 2026-07-10 (cross-repo ask).

## What we saw

Adopting v0.116.0 over v0.114.0, with the corpus untouched:

- all 62,602 grains re-ingested **byte-identical**
- `catalog.json` re-projected **byte-identical**
- `catalog.mrc.gz` and `catalog.xml.gz` **byte-identical**
- `catalog.nq.gz` **changed**

The nq change is a pure blank-node relabeling. Both dumps have 5,458,350
statements and 1,093,632 blank nodes; erasing `_:b\d+` makes the sorted files
hash-identical, and the multiset of per-node signatures (sorted predicate+object
per blank subject) and the degree multiset both match exactly. Same graph, new
names.

`lcat export` run twice at a fixed version is deterministic, so this is not
map-iteration flakiness within a build -- the label assignment just follows a
traversal order that any code change can perturb.

## Why it matters

Grains already carry canonical labels:

    data/out/data/works/61/wod3d74r058vra.nq   ->  _:c14n0, _:c14n1, …
    site/static/downloads/catalog.nq.gz        ->  _:b1, _:b10, _:b100, …

The export drops that canonicalization. `downloads.json` publishes a sha256 per
file:

    {"name":"catalog.nq.gz","bytes":60826172,"sha256":"b4cd816a…","records":62602}

So every release republishes a 60MB dump with a new checksum for a catalog that
did not change. Anyone mirroring the download, diffing it, or pinning the hash
sees a spurious change; an S3 sync re-uploads it. The mrc and xml dumps, which
have no blank nodes to label, are already stable across the same bump -- so the
churn is specifically the nq serializer's.

## Ask

Emit canonical blank-node labels from `lcat export` -- the same `_:c14nN`
canonicalization the grain writer already uses -- so an unchanged catalog
exports byte-identically across releases, and the manifest's sha256 changes only
when the data does.

If full canonicalization over 1.1M blank nodes is too costly at export time, a
cheaper fix that still holds: assign labels in a deterministic order derived from
the grain (work id, then the grain's own `_:c14nN`), rather than from the
traversal.

## Outcome

Fixed in **v0.120.0** (root, hugo, backend in lockstep), commit `8d39354`. The
fallback the ask offers is what shipped, and it is the better fix: a grain's
labels are already canonical, so nothing needs re-canonicalizing.

### Diagnosis

The label churn was not in the export alone. Both N-Quads merges -- `lcat
export`'s dump and `bibframe.SerializeGrains`' `catalog.nq` -- pushed every
grain back through one `rdf.Encoder`. That encoder renames blank nodes `_:b1,
_:b2, …` in first-encounter order across the whole document, so a label encoded
*how many blank nodes the traversal had already seen*. Nothing about the catalog
had to change for every label to move; a change in the serializer's own
traversal order was enough, and one was.

### Fix

Parse each grain to validate it, then emit the grain's **own canonical bytes**,
rewriting only its blank-node labels to `_:<grainId>_c14nN`. Labels are scoped to
the document, so a grain's `_:c14n0` cannot be used bare -- two grains would
merge it into one node. The grain id namespaces them and is unique per grain.

New shared code, so the two writers cannot drift apart again:
`bibframe.GrainBlankPrefix(path)` and `bibframe.RelabelGrainBlanks(grain,
prefix)`. The relabeler *scans* rather than reparses -- IRIs and literals keep
exactly the escaping the grain writer produced, since re-serializing is a second
chance to differ. `_:c14n0` inside a literal is text, not a node, and stays text.

The load-bearing property, now a test: **a grain contributes the same lines
whether it is merged alone or beside sixty thousand others.** The merged document
changes only when a grain does.

### A second, quieter bug found on the way

`rdf.Encoder.AppendNQuads` opens a fresh blank-node scope **per graph**, and both
writers called it once per graph. So a blank node a grain states in two graphs --
*one node*, by dataset semantics -- came out of the merge as two. On the
playground corpus exactly one grain of 2,994 does this (`w1dh6vtir43o8i.nq`), and
its `_:c14n16` was being emitted as two distinct document nodes. Distinct blank
nodes across the corpus: **23,244 -> 23,215**, a delta of exactly 29, which is
the count that grain contributes. Taking the label from the grain rather than
from the traversal makes the split unrepresentable.

### Verification

- Old vs new `catalog.nq` over the same grain tree: same **1,769,694**
  statements, and the same graph modulo blank labels (both canonicalize to
  sha256 `44865a9a…`). So the RDF is unchanged; only the labels are.
- `catalog.json`, `facets.json`, `redirects.json`, `similar.json`: **byte-identical**.
- A grain's lines are identical merged alone or among all 3,013 grains.
- Two exports of one store are byte-equal. (The first draft of that test rebuilt
  the fixture per call and failed by 3 bytes -- ingest stamps timestamps into
  `adminMetadata`, so a second ingest is a *different corpus*. The test was
  wrong, not the export.)

### Mutation testing

Every guard was stubbed out and proven to fail: literal skipping, IRI skipping, a
constant prefix instead of a grain-derived one, and the duplicate-id guard all
bit. Two findings from doing it honestly:

- A dot-terminator guard in `RelabelGrainBlanks` **could not** be made to change
  the output -- label bytes are copied verbatim, so the terminator lands where it
  went in. It was unfalsifiable dead code and is deleted.
- `TestNQuadsExportLabelsAreNamespacedByGrain` first asserted only that a label
  *contained* `_`, which a constant prefix `g_` satisfies. It now checks the
  prefix is the grain's actual id, and that grains do not share one.

### Behavior change

`SerializeGrains` now **errors** on two grains sharing an id (`"two grains share
the id %q … their blank nodes would merge"`) rather than silently merging their
blank nodes into one wrong graph. Ids are file basenames and unique in practice.
