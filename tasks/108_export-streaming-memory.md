# 108 -- export: full-corpus exports materialize the whole output (CSV: three copies) in RAM

> Filed from the 2026-07-05 full-code review. Memory is the known system wall
> (~57KB/work RSS, tasks/085).

## Symptom

A `Selection.All` export job holds the entire serialized catalog in memory before
writing -- many GB at the stated millions-of-works scale. CSV is worst: the
merged corpus N-Quads, the fully projected `Catalog`, and the CSV buffer coexist.

## Cause

- `emitNQuads`/`emitMARC`/`emitJSONLD`/`emitCSV` (backend/export/run.go:173-321)
  each return one complete `[]byte`/`bytes.Buffer`, and `Run` (run.go:44-49)
  passes it whole to `s.blob.Put`.
- `emitCSV` (run.go:245-266) first concatenates every grain's quads into one
  `merged bytes.Buffer`, then `project.Project(merged.Bytes(), ...)`
  (project/project.go:382) builds a `*Catalog` holding every Work, then renders
  the CSV -- three full-corpus copies at peak.

## Fix sketch

Give `blob.Store` (or the export path specifically) a streaming write --
temp-file spool or a `Put(io.Reader)` variant (S3 supports multipart) -- and
emit per-grain incrementally. For CSV, project incrementally per grain instead
of via one whole-corpus `Project` call.

## Acceptance

- A full-corpus export's peak RSS is bounded (per-grain working set, not
  output-sized), demonstrated against a large seeded store.
- CSV export no longer builds the merged corpus buffer and the full Catalog
  simultaneously.
