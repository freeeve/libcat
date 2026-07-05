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

## Status (2026-07-05 session)

Done.

- **Streaming write primitive** (shared with [[108_export-streaming-memory]]):
  `blob.StreamPutter` optional capability + `blob.PutStream` helper (buffered
  fallback for stores without it). DirStore streams into its temp file
  (hashing as it copies -- ETag identical to a buffered Put); blobs3 spools
  to a local temp file for a seekable upload body. Conformance-tested for
  content/ETag equivalence and precondition carry-through.
- **Streamed conversion.** `Convert` became a buffered wrapper over the new
  `ConvertTo(w, r, scheme, maxBytes)`, which emits per-chunk instead of
  growing one output slice; `installFrom` pipes ConvertTo straight into
  PutStream, so peak memory is the 1MB chunk plus the concept-count set.
  A conversion failure (bad bytes, over-cap, zero concepts) aborts the pipe
  before the store commits, so a previously installed snapshot survives a
  failed refresh (the old buffered flow's behavior, kept; pinned by
  TestInstallUploadKeepsOldSnapshotOnBadDump). Bounded memory pinned by
  TestConvertToStreamsWithBoundedMemory (~60MB synthetic dump, heap growth
  bounded well under output size).
- **Defensive ceilings.** Decompressed-size cap (default 4GB; configurable
  via `Service.MaxSnapshotMB` / `LCATD_VOCAB_SNAPSHOT_CAP_MB`) counts as the
  stream expands, so gzip bombs and huge plain bodies hit it alike on both
  download and upload paths; a 4MB line cap catches newline-less responses
  -- including inside bufio, where `ReadBytes` used to accumulate the whole
  delimiter-less body before any caller check ran (switched to bounded
  `ReadSlice`). Pinned by TestConvertToCaps.
