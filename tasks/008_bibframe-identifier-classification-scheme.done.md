# 008 -- Scheme fidelity for BIBFRAME identifiers and classification

## Context

The direct OverDrive provider (`ingest/overdrive/bibframe.go`) maps the Thunder
JSON straight to BIBFRAME and now retains data the MARC detour dropped: BISAC
classification (16,246 `bf:Classification` quads over the QLL corpus), the
OverDrive title id, and the Reserve ID -- all as `bf:identifiedBy` /
`bf:classification` nodes.

But libcodex's `bibframe.Identifier` and `bibframe.Classification` structs carry
only `{Class, Value}` -- **no source/scheme**. So today:

- BISAC codes render as a bare `bf:Classification` with a `bf:classificationPortion`,
  with **no `bf:source "bisacsh"`** to say what scheme the code belongs to.
- The OverDrive title id and the Reserve ID both render as plain `bf:Identifier`
  with a value and nothing distinguishing them. The availability adapter
  (`tasks/004`) keys on the **Reserve ID** and needs to find it unambiguously.

## Needed

Depends on a libcodex change (`libcodex/tasks/037`): add a `Source` field to
`Identifier` and `Classification`, rendered as `bf:source` across all four
serializations (RDF/XML, JSON-LD, N-Triples/N-Quads, Turtle) with
`TestEncodersIsomorphic` kept green.

Then, in this repo:

- Set `Source: "bisacsh"` on BISAC classifications.
- Tag the OverDrive identifiers so the Reserve ID is recoverable -- e.g.
  `Source: "overdrive"` on the title id and a distinct source/class on the
  Reserve ID (the Thunder availability key). Revisit whether the Reserve ID
  belongs on the Instance as an identifier or is better modeled as an
  availability-adapter key held outside the feed grain (see `tasks/004`).

## Acceptance

- [x] libcodex `Identifier`/`Classification` carry a source; `bf:source` emitted.
      (libcodex v0.6.0, `tasks/037`; renders across all four serializations.)
- [x] BISAC nodes carry `bf:source "bisacsh"`.
- [x] Reserve ID is unambiguously recoverable from a grain: it carries
      `bf:source "overdrive-reserve"` (the title id carries `"overdrive"`). Kept
      in the feed grain -- the Reserve ID is a stable per-edition key, not
      volatile availability data, so §5 keeps it in the graph; only the live
      copies/holds/wait stay out (fetched client-side by `tasks/004`).
- [x] Grain golden test updated (`ingest/overdrive/bibframe_test.go`: asserts the
      BISAC source, both identifier sources, and the `bf:source` scheme labels in
      the serialized N-Quads).

## Done

`ingest/overdrive/bibframe.go` sets `Source` on the BISAC classification and the
two OverDrive identifiers, via exported constants `SourceBISAC` /
`SourceOverDrive` / `SourceReserveID` (so downstream consumers select nodes by
scheme). Corpus: 27073 `bf:source` quads (14567 bisacsh + 6253 title + 6253
reserve); clustering unchanged (5659 works, zero id churn), re-ingest
byte-identical -- scheme is additive rendering, and identity keys are recovered
by identifier type+value, not source.

Handoff to `tasks/004`: the projector still flattens all non-ISBN identifiers into
`Instance.ProviderIDs` (a `[]string`), so catalog.json does not yet distinguish the
Reserve ID by scheme. Surfacing it there is a schema decision the availability
adapter should own (what shape it needs), so it is deferred to `tasks/004` rather
than bumping the catalog schema here.
