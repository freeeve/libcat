# 077 -- Original cataloging: create new records from scratch

## Context

Tasks 074-076 cover finding records elsewhere (fielded copycat search), editing
them faster (keyboard ops, text mode), but not making one from nothing --
074 explicitly defers "new record from nothing". Everything in the corpus today
originates from a feed (OverDrive) or a copycat import; a cataloger cannot host
a work the library holds that exists in neither. The plumbing already exists:
copycat commit (tasks/050) runs staged records through `ingest.RunStore`, whose
clustering pipeline mints new Work ids when nothing matches -- so an original
record is "a staged record that came from the editor instead of a Z39.50
target". tasks/066 external identity links apply to such works the same as any
other; no new identity scheme is needed.

## Scope

1. **Blank-record templates**: a small set of MARC skeletons per material type
   (book, ebook, audiobook, serial to start) -- LDR/008 prefilled via the
   fixed-field defaults, required tags (245) present but empty. Shipped as
   data, deployment-overridable like editing profiles.
2. **New-record entry point**: "New record" action opens the MARC editor
   surface (grid, or tasks/076 text mode once it lands) on a chosen template,
   backed by a draft that is not yet in the grain tree.
3. **Stage-from-editor endpoint**: saving the draft stages it into a copycat
   batch with source `original` (no target round-trip), so it flows through
   the existing review banner ("would merge with Work w..." -- catches the
   case where the "new" record already exists) and the same commit/revert/
   CAS machinery. No parallel commit path.
4. **Validation gate**: staging refuses records failing minimum viability
   (valid LDR, 245 with $a, parseable 008), with field-anchored errors in the
   editor.

## Out of scope

- Authority record creation (bib records only).
- A full Koha-style framework editor for templates -- templates are static
  files in this pass.

## Acceptance

- "New record" > book template > fill in 245/100/020 > stage > review shows
  it as source `original`; commit mints a new Work visible in `/v1/works`
  and (after rebuild) public search.
- Staging a record whose ISBN matches an existing work shows the merge banner
  before commit, same as a copycat import.
- Staging with an empty 245$a is refused with an error anchored to the field.

## Outcome

- `backend/copycat/templates.go`: four embedded skeletons (book, ebook,
  audiobook, serial -- LDR + 40-char 008, ebook adds 006/007, common data
  fields present but empty), `LoadTemplatesDir` for deployment overrides
  (profiles convention); `ValidateOriginal` (24-char LDR, non-empty 245$a,
  40-char 008s, 3-char tags) returns field-anchored `FieldError`s;
  `pruneEmptyFields` drops untouched skeleton rows at staging;
  `StageOriginal` prunes, validates, and stages one record as a source
  "original" batch through the ordinary Stage path (match banner, review,
  commit, revert all inherited).
- Endpoints: `GET /v1/copycat/templates`, `POST /v1/copycat/original` (400
  carries `{error, fields:[{tag,message}]}`).
- UI: `NewRecord` screen at `#/copycat/new` -- template picker (repick
  restarts), grid/text surfaces reused with the field clipboard, mod+s or
  button stages; `FieldedApiError` surfaces the anchored refusals; success
  navigates to `#/copycat?batch=<id>` and CopyCat's new `batchId` prop opens
  the batch for review. "New record…" entry on the CopyCat screen.
- The draft lives in screenState only; nothing touches the grain tree until
  the batch commits (per scope).
