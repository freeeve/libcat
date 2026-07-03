# 049 -- MARC view (dual-view editor, fidelity sidecar)

## Context

Copy catalogers get a real MARC editing grid over the same grain: materialized
via libcodex `bibframe.Decode`, written back via `FromRecord` as a *diff* so
untouched fields are no-ops and MARC edits get identical override/audit
semantics. Crosswalk-lossy fields are preserved verbatim rather than silently
dropped.

## Scope

1. `bibframe/fidelity.go`: promote `knownLostFields`/`coreFields` out of
   roundtrip_test.go as the consumable loss table.
2. `lcat:marcVerbatim` sidecar: on MARC ingest, crosswalk-dropped fields stored
   verbatim as quads (stable tag+indicators+subfields serialization) in the
   record's graph; re-emitted on MARC export; visible read-only in the editor.
3. `backend/marcview/`: grain -> field-array JSON (`[{tag, ind1, ind2,
   subfields}]` + leader + verbatim + lossy annotations); edited array ->
   re-encode -> diff vs old encoding -> editorial ops.
4. SPA MARC tab: `MarcGrid.svelte` (keyboard-first: tab/enter navigation,
   duplicate-field key), `FixedFieldGrid.svelte` positional builders driven by
   JSON definitions (leader, 008 by material type, 006, 007), non-blocking
   lossy-tag warnings linking docs/marc-fidelity.md.

## Acceptance

- [x] Open a vendored MARC Express record, edit one field, save: only that field's
  delta lands as editorial quads. (marcview + httpapi tests run exactly this on
  the vendored od-sample-ebook.mrc: the 520 edit produces editorial statements
  touching only the summary group; the view then shows the edited value once
  and the feed original survives shadowed for revert.)
- [x] Editing a lossy tag warns; the value round-trips to MARC export via
  lcat:marcVerbatim. (037 edit -> editorial verbatim + override claim ->
  DecodeGrainMARC/export reproduces the edited field.)
- [x] Untouched-field MARC save is a no-op (grain byte-identical). (Save returns
  the input bytes when the diff is empty; re-saves are idempotent because
  skolem names are content-hashed.)

## Delivered (2026-07-02, commits b3d0891 + SPA)

- **`bibframe/fidelity.go`**: `CoreFields`/`KnownLoss` (tag -> reason) +
  `LossyTag` promoted out of roundtrip_test.go; the CI gates consume the
  exported table.
- **`lcat:marcVerbatim` sidecar**: MARC ingest serializes each record's
  known-loss fields field-exact (tag + indicators + subfield runs) onto its
  Instance node in the feed graph (`VerbatimProvider` capability ->
  `GroupInstance.Verbatim` -> `BuildWorks`); `DecodeGrainMARC` honors
  editorial `lcat:overrides` shadows and re-attaches verbatim fields with the
  record-to-node mapping mirrored from the libcodex decoder; MARC/JSON-LD
  export use it, so the original forms round-trip. Documented in
  docs/marc-fidelity.md.
- **`backend/marcview`**: View (field arrays, lossy annotations) and Save as
  a diff: the edited record re-crosswalks onto the grain's node IRIs, changed
  (subject, predicate) groups land as override claim + editorial re-assertion
  with content-hash-skolemized structures (stale skolems retracted), lossy
  tags flow through the sidecar instead. Feed statements are never touched.
- **HTTP**: GET/POST `/v1/works/{id}/marc` (If-Match, dryRun quad delta,
  MARC_EDIT audit, knownLoss served for warnings).
- **SPA**: MARC tab in the work editor -- MarcPanel (load/preview/save/
  conflict-reload), MarcGrid (tag/indicator/"$a …" line rows, Enter inserts,
  Alt+D duplicates, control-vs-data switching, non-blocking lossy warnings
  linking the fidelity table), FixedFieldGrid (positional builders for
  leader/006/007/008 from JSON slot definitions, raw line always visible).
  Subfield-line syntax helpers unit-tested; axe a11y over the grid with a
  lossy field and an expanded builder.
