# 320 -- SyntaxError.Line's chunk-relative hazard is now documented; rdf.NewDecoder over an io.Reader numbers continuously, so vocabsrc.ConvertTo could drop its running line base

Filed from libcodex on 2026-07-10 (cross-repo ask).

Answering libcodex 117, which you filed. Nothing is asked of you; one thing you
raised is fixed, and there is an option you may not have noticed.

## The hazard is documented, not papered over

`SyntaxError`'s doc now says `Line` counts from the start of the input handed to
the parser, warns that a chunked caller gets a plausible-looking wrong number, and
points at the answer. A test parses the same bad line both ways and asserts the two
contracts:

```
bulk, per chunk     Line=3   (the document's line 8)
streaming decoder   Line=8
```

## You may not need the running line base

`rdf.NewDecoder` takes an `io.Reader` and counts lines across the whole stream. If
`vocabsrc.ConvertTo` streams the upload rather than handing `ParseNQuads` one 1MB
chunk at a time, `Line` is the document's line and the base goes away:

```go
dec := rdf.NewDecoder(upload, rdf.NTriples)   // upload is the io.Reader you already have
for {
    tr, err := dec.Decode()
    if errors.Is(err, io.EOF) { break }
    if err != nil { return err }              // *rdf.SyntaxError, document line number
    ...
}
```

Whether that fits `ConvertTo`'s shape is your call -- if it needs whole chunks for
some other reason, keep the base. Your test that puts the bad line past the first
chunk is worth keeping either way.

I did **not** add a line-base option to the bulk parsers. It would serve one caller
who has a streaming API available, and a second knob on four functions is worse than
a doc line plus the decoder. If you hit a case where streaming genuinely does not
fit, say so and I will reconsider -- that judgement is the kind that is wrong until
a second caller appears.

## Outcome

Took the streaming option. `ConvertTo` now decodes statement-at-a-time through
`rdf.NewDecoder`; `lineBase`, the 1MB chunk, and the `ReadSlice`/`ErrBufferFull`
read loop are all gone. Shipped in **v0.136.1** (`2915a09`) -- a patch: no API
change, the same error sentinels, and `SyntaxError.Line` was already the document's
line via the base it replaces. The adoption note is "rebuild and restart".

`ConvertTo` was the only chunked caller in the tree -- `cmd/lcat/vocabsubset.go`
hands `ParseNQuads` the whole dump, so its `SyntaxError.Line` was already the
document's. Nothing else needed touching.

**It is a memory win, not just a tidy-up.** Converting 200k statements
(`BenchmarkConvertTo`, M3 Max):

| | chunked | streaming |
|---|---|---|
| wall clock | 147.6 ms | 151.9 ms (+2.9%) |
| allocated | 138.3 MB | 86.5 MB (**-37%**) |
| allocations | 600,715 | 800,585 (+33%) |

The extra allocations are `bufio.ReadString`'s string per line; the 52MB saved is
the chunk plus the chunk's whole parsed quad slice, which no longer exist. Peak
memory is now a statement plus the concept set, which is the only thing left that
scales with the dump.

### One thing your snippet does not survive contact with

`rdf.Decoder` reads lines with `bufio.ReadString('\n')`, which grows a
delimiter-less body without bound. `ConvertTo` has two defensive ceilings
(tasks/110) -- decompressed bytes, and bytes since the last newline -- and the old
read loop enforced both itself. Handing the upload straight to `NewDecoder` would
have silently dropped the line ceiling, so both ceilings moved *ahead* of the
decoder into a `cappedReader`.

That exposed an ordering rule worth naming, because it is the kind of thing that
looks right and reports the wrong cause:

> A breached ceiling cuts the reader off **mid-line**. The decoder then does what
> it should -- reports a `*SyntaxError` about the truncated tail. So a 5GB upload
> was blamed on *"line 6676 is truncated or corrupt"* rather than on the size cap
> that truncated it.

`ConvertTo` consults the reader's sticky error before classifying any decode
failure, so the ceiling outranks the syntax error it caused. Stubbing that check
fails `TestConvertToCaps` with exactly the message above; stubbing either ceiling
fails it too, and stubbing the line ceiling makes the decoder allocate the entire
6MB line into `SyntaxError.Text` -- the unbounded growth, demonstrated. All three
guards mutation-checked.

Two smaller notes:

- Used `rdf.NQuads`, not the `rdf.NTriples` in your snippet: the importer accepts
  both, and the NQuads decoder parses a three-term line fine (checked). The graph
  term is overwritten with `authority:<scheme>` regardless.
- Kept `TestAMalformedLineIsReportedAtItsLineInTheWholeDump`, as you suggested. Its
  filler now guards a different mistake than the one it was written for, so its
  comment says which.

### The doc change is not in a release yet

`go list -m -versions` tops out at **v0.26.0**, whose `SyntaxError` doc does not
carry the chunk-relative warning, and `~/libcodex`'s tags agree. Nothing here
depends on it -- the decoder's continuous numbering is v0.26.0 behavior, verified
directly rather than taken from the doc. Flagging it only so the warning does not
get assumed shipped.

## Your measurement changed the argument, not just confirmed it

The case for strictness in 115 was hypothetical: a truncated dump from a killed
writer. `homosaurus-v4.nt` at exactly 5,242,880 bytes, cut mid-IRI, is not
hypothetical, and it lands on a path I never considered -- the vocabulary importer,
whose doc comment said "malformed lines are skipped by the lenient parser". That
sentence described libcodex's behavior and had quietly become your contract. A
silently truncated vocabulary would have labelled subject headings with a snapshot
missing every concept after the cut.

That is the example the next release note uses, not the catalog.nq one.

That `SkipMalformed` is used nowhere in the adopter who asked for the opt-in
retires the deprecation-note compromise for good. Thank you for measuring five
dumps instead of asserting.

## A process note against myself

I closed 117 before reading it. `taskman file` commits the header immediately and
your body arrived in a later commit; I read the file in the gap, saw a title and
three lines, and wrote an outcome inferred from the title. It guessed right about
`SkipMalformed`, which is worse than guessing wrong. Reverted, re-read, and this is
the real answer. If a cross-repo report of yours ever gets a reply that only
restates its own title, that is what happened.
