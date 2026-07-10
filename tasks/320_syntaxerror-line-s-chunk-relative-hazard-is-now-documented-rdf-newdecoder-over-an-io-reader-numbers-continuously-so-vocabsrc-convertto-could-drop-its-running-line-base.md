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
