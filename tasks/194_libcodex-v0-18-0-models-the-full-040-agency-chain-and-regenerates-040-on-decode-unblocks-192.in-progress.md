# 194 -- libcodex v0.18.0 models the full 040 agency chain and regenerates 040 on decode (unblocks 192)

Filed from libcodex on 2026-07-09 (cross-repo ask).

Your ask (libcodex tasks/094) is done and released in libcodex
v0.18.0. Your tasks/192 is unblocked.

## What landed

`bibframe.AdminMetadata` gained `OrigAgency`, `DescriptionLanguage`,
`Transcriber`, `Modifiers []string` beside the existing
`DescriptionConventions`. `FromRecord` fills them from 040; `Decode`
regenerates a 040 in canonical subfield order. A BIBFRAME with no
cataloging source in its graph still emits no 040, so libcat keeps
synthesizing its own where it wants to.

## Two corrections to the ask's proposed mapping

Both settled by reading LC's XSLT (`xsl/ConvSpec-010-048.xsl` in
lcnetdev/marc2bibframe2) rather than inferring.

1. `descriptionModifier` is in the **bf:** namespace, not `bflc:`.
   (Confirmed by LC's own published RDF.)
2. 040 $a maps to **`bf:assigner`**, not `bf:source`. And 040 $c maps
   to *nothing* -- LC has no template for it at all.

Point 2 is why "$a and $c both -> bf:source" could not work: in your
own acceptance record both are DLC, so both emit the identical triple
`<admin> bf:source <organizations/dlc>`, RDF collapses it to one, and
decode cannot distinguish a $a+$c pair from a bare $a.

LC's actual answer, which libcodex now copies: the whole 040 is *also*
preserved as a `bf:Note` typed `mnotetype/internal` whose `rdfs:label`
is the field in marcKey form, alongside the semantic properties. That
note is the round-trip carrier, so $c survives without inventing any
vocabulary.

## The RDF contract libcat should read

    $a -> bf:assigner            (bf:Agent, organizations/ IRI + bf:code)
    $b -> bf:descriptionLanguage (bf:Language, languages/ IRI + bf:code)
    $c -> internal note only
    $d -> bf:descriptionModifier (bf:Agent, organizations/ IRI + bf:code)
          one node per $d, in field order
    $e -> bf:descriptionConventions        (unchanged)

    bf:note -> bf:Note, rdf:type mnotetype/internal,
               rdfs:label "040  $aDLC$beng$cDLC$dOCLCQ$erda"

Decode prefers the internal note (field-exact) and falls back to the
semantic properties (everything but $c) for a graph without one -- so a
graph libcat builds from its editorial provenance need not synthesize a
marcKey note to get a usable 040 back.

Two deliberate divergences from m2b's commented-out code, both
supersets: one `bf:descriptionModifier` per $d (m2b's dead code keeps
only the last), and a `bf:assigner` IRI for any IRI-safe agency code
(m2b's mints one only for DLC).

## Notes for 192

- Derive the modifying-agency chain from the editorial graph into
  `Modifiers` in the order you want it emitted -- libcodex preserves
  that order, one `bf:descriptionModifier` node per entry.
- The JSON-LD `@context` gained an `mnotetype` prefix. Nothing else
  changed in the serializations; RDF/XML writes extra `rdf:type`s as
  full IRIs, so its output is byte-identical apart from the new
  AdminMetadata properties.
- `docs/marc-fidelity.md`'s 040 row can flip from lost to kept. On the
  libcodex side 040 moved from `lostTags` to `coreTags` in the loss
  gate, and validation is against LC's published RDF: the three
  `bibframe/testdata/loc/*.inst.rdf` fixtures now decode back to their
  original 040s, $c included (e.g. 21263493 ->
  `040  $aDLC$beng$cDLC$dDLC$egihc`).
- Named graphs and the 040 stay orthogonal, as you framed it: the 040
  is record-level agency provenance on `bf:AdminMetadata`, untouched by
  statement-level named-graph provenance.
