# 317 -- libcodex v0.26.0 makes a malformed N-Quads line a *rdf.SyntaxError: delete TestAMalformedLineIsSilentlySkippedByBothParsers and drop the load.go comment

Filed from libcodex on 2026-07-10 (cross-repo ask).

Closes libcodex 115, which you filed. Shipped in **v0.26.0**. Strict by default,
which is what you asked for -- not the deprecation-note fallback.

`TestAMalformedLineIsSilentlySkippedByBothParsers` was written to fail the day this
changed. Today is that day. Delete it (or invert it), and drop the `project/load.go`
comment pointing at the libcodex task.

## What you get

A malformed line is now a `*rdf.SyntaxError` with a 1-based `Line` field:

```
rdf: line 2: malformed N-Triples/N-Quads statement: "<http://broken"
```

from all four bulk entry points and the streaming decoder. `io.EOF` now means the
input ended, not "ended, and some of it was unreadable". Your truncated
`catalog.nq` fails the build instead of shipping a smaller catalog with exit 0.

```go
var se *rdf.SyntaxError
if errors.As(err, &se) {
    return fmt.Errorf("catalog.nq is truncated or corrupt at line %d: %w", se.Line, err)
}
```

The bulk parsers still return the quads read before the bad line, alongside the
error. That is a diagnostic ("we got 1.6M of an expected 1.7M"), not a result --
the error is what says the graph is not the document. Do not fall back to using it.

## Two things beyond what you reported

**It was not only N-Quads.** `ParseNTriples` and `ParseNTriplesShared` had the same
defect, from the same shared line parser. Your repro only exercised the quad pair.
Root cause was a single `bool`: `parseNQuadLine` returned `false` for "blank or
comment" *and* for "not parseable", and every caller skipped both. One conflation,
five entry points.

**`bibframe.Decode` and `skos.Parse` inherited it.** Both sniff the serialization
and fall back to N-Triples, so feeding either a document that is not RDF used to
yield an empty graph and a nil error. Same bug, second place, fixed by the same
change. If anything of yours feeds untrusted bytes to `bibframe.Decode` expecting
an empty result rather than an error, that is the one place to look when you bump.

## The lenient path, if you want it somewhere

`SkipMalformed` is on the Decoder only, chainable:

```go
dec := rdf.NewDecoder(r, rdf.NQuads).SkipMalformed(true)
```

Not on the bulk parsers -- `ParseNQuadsLenient` plus a `Shared` twin for each of two
formats is four functions for a case that wants streaming anyway. A caller who
really wants to skip noise across a whole document can drain a Decoder.

`lcat project` should not use it. The whole point of 115 is that a short read from
your own build step is a lie you would ship.

## Bump

`go get github.com/freeeve/libcodex@v0.26.0`, both modules together -- `go.work`
takes the max across root and backend.

## Outcome

Shipped in `5bf38aa`, released as **v0.134.0**. Bumped both modules.

Only two tests failed on the bump, both of which existed to pin the old contract.
Every other call site -- 40-odd `ParseNQuads` sites across `bibframe/`, `ingest/`,
`workindex`, `marcview`, `editor` -- parses bytes libcat serialized itself, checks
the error, and was already correct.

### `TestAMalformedLineIsSilentlySkippedByBothParsers` did its job

It was written to fail the day this changed, and it did. Inverted rather than
deleted: `TestATruncatedCatalogIsRefused` now asserts `LoadDataset` returns a
`*rdf.SyntaxError` at line 2, that the message names the file and what is wrong
with it, that `ParseNQuadsShared` agrees, and -- the control -- that the same file
without the bad line still loads. The `load.go` comment pointing here is gone,
replaced by a note saying not to reach for `SkipMalformed` on our own build output.

End-to-end: truncating the playground's 264MB `catalog.nq` to 40MB exits 1, names
line 255092, and writes no artifacts. The whole file still projects `catalog.json`,
`facets.json`, `redirects.json` and `similar.json` byte-identical to the v0.131.0
baseline.

### The vocabulary paths were the interesting half

`vocabsrc.ConvertTo` streams an **uploaded** SKOS dump, and its doc comment said
"malformed lines are skipped by the lenient parser". That was never a decision --
it was a description of whatever libcodex did. Your change made it a decision, so
it had to be measured rather than guessed.

Five real dumps on this machine, parsed strictly with v0.26.0:

```
Downloads/v5.nt                   9.6MB    69,781 quads   CLEAN
coll-support/homosaurus-v5.nt    14.5MB    98,561 quads   CLEAN
Downloads/FASTAll/FASTMeeting.nt 15.3MB   124,452 quads   CLEAN
Downloads/FASTAll/FASTTitle.nt   97.7MB   814,318 quads   CLEAN
Downloads/homosaurus-v4.nt        5.2MB    38,098 quads   SYNTAX ERROR line 38099
```

That file is exactly **5,242,880 bytes** -- 5MiB -- and its last line is cut
mid-IRI. A partial download. Under the lenient parser it converted cleanly and
would have installed a vocabulary silently missing every concept after the cut, to
label subject headings with. It is the single dump in the set that fails, and it is
the one you most want refused. So: strict, and the refusal names the line.

One bug found writing that. `ConvertTo` hands the parser one 1MB chunk at a time,
so `SyntaxError.Line` is chunk-relative. An operator chasing a bad line in a
five-million-line dump would have been sent to line 10083 instead of 30249. Fixed
with a running line base, and `TestAMalformedLineIsReportedAtItsLineInTheWholeDump`
puts the bad line past the first chunk deliberately; removing the offset makes it
fail with exactly that pair of numbers.

`lcat vocab subset` got the same treatment. Its comment claimed the "kept nothing"
guard covered a corrupt dump -- it never did, because that guard fires on the
*wrong* dump, not on the right one arriving half-written.

`subsetFromNT` (per-concept SKOS bodies over HTTP) now drops a whole concept on a
bad line instead of keeping its remaining statements. Right trade: a concept kept
without its `prefLabel` is a heading with no heading, and that loop already prints
what it skipped.

### On your two extras

`ParseNTriples`/`ParseNTriplesShared` have no call sites here, and `skos.Parse` is
never called -- `libcodex/skos` is not imported. `bibframe.Decode` has exactly one
call site (`bibframe/marcverbatim.go:153`), and it is fed our own encoder output,
so the N-Triples fallback you warned about cannot be reached with untrusted bytes.
Nothing relied on the empty-graph-and-nil-error shape.

`SkipMalformed` is used nowhere, which is the answer you predicted for `project`
and, on the evidence above, the right answer for the vocabulary paths too.
