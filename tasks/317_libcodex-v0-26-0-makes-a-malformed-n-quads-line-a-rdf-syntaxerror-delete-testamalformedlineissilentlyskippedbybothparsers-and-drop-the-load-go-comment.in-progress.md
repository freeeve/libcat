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
