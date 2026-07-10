# 309 -- libcodex v0.25.0 moves 490 series to a bf:Relation/bf:Series on the Work: project.go:1097,1106 and ingest/enrich.go:458 must move off the flat Instance literals

Filed from libcodex on 2026-07-10 (cross-repo ask).

This closes libcodex 110, which your 111 report helped scope. **Breaking change to
the emitted RDF.** There is a compatibility path, so nothing breaks the moment you
bump -- but the flat shape is now legacy and will be removed.

## What changed

490 no longer emits flat literals on the Instance. It follows marc2bibframe2's
`ConvSpec-Process6-Series.xsl`: one `bf:relation` per 490, on the **Work**.

```
<Work> bf:relation _:rel .
_:rel   bf:relationship <http://id.loc.gov/vocabulary/relationship/series> ;
        bf:associatedResource _:series ;
        bf:seriesEnumeration "bk. 2" .        # on the RELATION, not the series
_:series a bf:Series ;
        bf:status <http://id.loc.gov/vocabulary/mstatus/t> ;    # transcribed, always
        bf:status <http://id.loc.gov/vocabulary/mstatus/tr> ;   # traced, when ind1=1
        bf:title [ a bf:Title ; bf:mainTitle "Firebrand fiction" ] ;
        bf:identifiedBy [ a bf:Issn ; rdf:value "0075-2118" ] .
```

Why: the flat shape paired statement to enumeration by *list position*. RDF graphs
are sets, so two 490s sharing a `$v` emitted one triple and the pairing died at any
conformant consumer. The field round-tripped through libcodex and was lossy through
rdflib or Jena. One relation subject per 490 fixes it.

`Work.Series []Series` replaces `Instance.SeriesStatements` /
`Instance.SeriesEnumerations` in the Go model, if you construct BIBFRAME values
directly (I don't think you do).

## Your three read sites

From reading your v0.116.1 source:

- `project/project.go:1097` -- `p.view.Objects(inst, pSeriesStatement)` -> `i.Series`
- `project/project.go:1106` -- `p.view.Objects(inst, pSeriesEnum)`, first non-empty wins
- `ingest/enrich.go:458` -- `merged.Objects(inst, bfNS+"seriesStatement")` -> `WorkSummary.Series`

All three read the Instance. Nothing on the Instance carries series data any more,
so on a graph produced by v0.25.0 they return empty. Sketch of the new read:

```go
const (
    pRelation           = bfNS + "relation"
    pRelationship       = bfNS + "relationship"
    pAssociatedResource = bfNS + "associatedResource"
    seriesIRI           = "http://id.loc.gov/vocabulary/relationship/series"
)

// Series now hang off the Work, not the Instance.
for _, rel := range g.Objects(work, pRelation) {
    if r, ok := g.Object(rel, pRelationship); !ok || r.Value != seriesIRI {
        continue // a 76x-78x linking entry, not a series
    }
    res, ok := g.Object(rel, pAssociatedResource)
    if !ok {
        continue
    }
    title := /* bf:title -> bf:Title -> bf:mainTitle on res */
    enum, _ := g.Literal(rel, bfNS+"seriesEnumeration")  // on the relation
    ...
}
```

Two things follow for you specifically.

**Your enumeration gets better, not just different.** You currently take the first
non-empty `bf:seriesEnumeration` on the Instance and discard the rest, because the
flat shape gave you no way to know which series a `$v` belonged to. Now each
enumeration is a property of its own relation, so `i.SeriesEnumeration` can become
per-series instead of max-1. That is the actual payoff -- a record with two 490s
was never giving you the right answer.

**Series moved Work-ward, which may suit `WorkSummary.Series` better.** You noted
in 286 that `ingest.WorkSummary.Series` is collected across several Instances of
one Work and needs `sortedUnique` because real records transcribe the same 490 on
each printing. Series are now Work-level in the graph, so that cross-instance
dedupe may become unnecessary -- worth checking rather than assuming, since your
grains merge several source graphs.

## Compatibility window

`Decode` still reads the flat Instance literals when a Work carries **no** series
relation, so archived graphs and anything libcodex wrote before v0.25.0 keep
decoding to 490s. That path inherits the defect it cannot fix: in those graphs two
490s sharing a `$v` were already one triple.

The window exists so you can migrate on your own schedule. It closes in a later
release; when it does, `bf:seriesStatement` stops being read at all.

## Also recovered

Reading LC's XSLT surfaced two subfields the flat mapping silently dropped, both
now carried: `490 $x` (series ISSN) -> `bf:Issn` on the series, and `ind1=1`
(traced) -> an `mstatus/tr` status. Still not carried: `$n`/`$p`, `$l`, `$3`, and
the 880 parallel grouping.

## Bump

`go get github.com/freeeve/libcodex@v0.25.0`, both modules together -- `go.work`
takes the max across root and backend, so a one-module bump proves nothing.

## Outcome

Shipped in `fdeb664`, released as **v0.129.0**. Both modules bumped together.

### The breakage was real, silent, and invisible to the test suite

After bumping libcodex and before touching any code: `go build ./...` clean,
`go test ./...` **all green**. A record with two 490s projected `series=[] enum=""`.

The suite could not see it. Every series test was a hand-written nquads fixture
carrying the flat Instance literals -- so the fixture agreed with the reader, and
neither agreed with libcodex. The same shape as the tasks/308 cover regression.
The new tests drive real `codex.Record`s through `ingest.Run` into `Project`, so
libcodex is in the loop and can disagree.

### What shipped

`project.Work.Series []{title, enumeration, issn, traced}` replaces
`Instance.Series []string` + `Instance.SeriesEnumeration string`. **Schema v12.**

Series moved Work-ward in the projection because they moved Work-ward in the
graph, and because they were never per-edition facts: a 490 is transcribed on
every printing, and a Work's membership in "Firebrand fiction, bk. 2" does not
change with the carrier you borrow. That also answers the question in the ask --
`ingest.WorkSummary.Series` no longer needs `sortedUnique` across Instances,
because the graph stops repeating the 490 per printing.

Your predicted payoff is real. Two 490s now project as:

```
{Firebrand fiction  bk. 2  0075-2118  traced}
{Second series      v. 7}
```

`490$x` and `ind1=1` both carried, per your "also recovered" note.

### The compatibility window is honoured on our side too

Your `Decode` keeps reading flat literals; so do both of our readers, when a Work
carries no series relation. An adopter who bumps without re-ingesting keeps their
series rather than watching them vanish. That path inherits the defect it cannot
fix -- every legacy series gets the Instance's first non-empty enumeration, which
is what the old projector gave all of them -- and says so in a comment. Pinned by
`TestRelationsAndSeries`, whose fixture is deliberately still the flat shape.

### Mutation-tested

Five guards, each stubbed out and the suite re-run:

- accept any `bf:relation` regardless of `bf:relationship`: 1 fail
- read the enumeration off the series instead of the relation: 2 fail
- drop the legacy fallback: 1 fail
- any `bf:status` means traced: 1 fail
- take any identifier as the ISSN: 1 fail

Three findings worth recording, because two of my tests were wrong first:

**The 76x control was vacuous.** I wrote `TestANonSeriesRelationIsNotProjectedAsA
Series` with a 765 field, and it passed with the `bf:relationship` check deleted.
libcodex v0.25.0 emits **no** `bf:relation` for 765 or 830 -- I checked the graph
rather than assuming -- so a MARC-driven test cannot produce a competing relation.
Rewritten as an nquads fixture with a `relationship/translationOf` relation, which
is what the guard actually defends: nquads ingest, editorial writes, and whatever
you map next.

**The ISSN control was vacuous twice.** First because a MARC series node carries
exactly one identifier, so the `bf:Issn` type check could never fire. Then, after
I added a two-identifier fixture, *because the reader took the last match* and my
fixture happened to list the ISSN last. Fixed both: the reader takes the first
`bf:Issn` and stops, so the result does not depend on `Objects()` order, and the
fixture lists an LCCN first so a type-blind reader grabs the wrong one.

**The agreement test only proved the legacy paths agreed.** `TestBothConverters
AgreeOnTheSameGraph` used the flat fixture, so both new readers were unexercised
by the one test whose job is catching them drifting. It now runs both converters
over both shapes, and asserts the series is actually named rather than equally
empty on both sides.

### Not carried

`$n`/`$p`, `$l`, `$3`, the 880 parallel grouping -- as you noted. The default Hugo
layout renders `title, enumeration` and passes `issn`/`traced` through to adopters
without rendering them: an ISSN is a serials-control number nobody is looking for
on a novel's page, and "traced" is a cataloging fact about added entries.
