# 110 -- vocabsrc: snapshot install buffers the whole converted dump; fetch has no size ceiling

> Filed from the 2026-07-05 full-code review.

## Symptom

Installing a vocabulary snapshot (e.g. the builtin LCSH subjects dump, ~450k
concepts) spikes RSS by roughly the full converted N-Quads size -- hundreds of
MB -- and a hostile or misconfigured snapshot endpoint can OOM the process.

## Cause

- `Convert` (backend/vocabsrc/download.go:333-352) grows a single
  `out []byte` via `enc.AppendQuad` across every kept quad before one
  `Blob.Put`; only the 1MB read `chunk` is bounded. The `concepts` map adds one
  entry per prefLabel subject URI on top.
- `fetchSnapshot` (download.go:267-286) returns `resp.Body` with no
  `io.LimitReader` (the suggest client caps at 4MB, suggest.go:115-122); gzip is
  transparently expanded, and a dump with no newlines makes
  `br.ReadBytes('\n')` grow an unbounded line buffer. SnapshotURL is admin-set
  (validateSource only checks the scheme prefix, vocabsrc.go:240-244), so this
  is trust-bounded but has no defensive ceiling.

## Fix sketch

Stream the conversion (spool to a temp file or chunked blob write -- pairs with
the streaming Put from [[108_export-streaming-memory]]), and add a configurable
max snapshot size plus a max line length on the fetch path.

## Acceptance

- Installing the builtin LCSH snapshot holds peak RSS near the chunk size, not
  the dump size.
- An over-limit or newline-less response fails cleanly with a size error rather
  than growing without bound.
