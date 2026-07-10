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
