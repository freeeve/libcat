# Bulk .mrc export XML-escapes free text

Found during tasks/089's demo-corpus re-ingest: `POST /v1/exports` with
format `marc` writes ISO 2709 whose free-text is XML-escaped -- a grain
storing `&#8212;` (itself vendor-encoded) exported as `&amp;#8212;`, and
inconsistently: in one 31-record export, 8 works' 520s left
single-escaped while others gained the extra `&amp;` layer. ISO 2709 is
not XML; text must be raw bytes.

Contrast: the single-work `GET /v1/works/{id}/marc` JSON view shows the
text without the extra layer, so the escaping is on the bulk binary
path. Suspects: the grain -> MARC round-trip in libcodex
(reader_crosswalk / marcxml encode feeding the iso2709 writer) or
libcatalog's backend/export encode step. If the bug is libcodex-side,
file a task there instead and bump.

Since tasks/089, ingest decodes entities to a fixpoint, so a
re-imported double-escaped export self-heals -- but a library handing
the .mrc to another ILS ships mangled text, so the export itself must
be fixed.

Repro: export all works on the playground, then
`python3 -c 'd=open("out.mrc","rb").read().decode(); print(d.count("&amp;#"))'`.

## Resolution -- not an export bug (stored-data artifact)

Investigated and could not reproduce on current code: the bulk export does
**not** XML-escape. The export path (`backend/export.emitMARC`) has always been
`bibframe.DecodeGrainMARC` -> `iso2709.Writer.Write`, and neither escapes:
`DecodeGrainMARC` runs `codexbf.Decode` (BIBFRAME N-Quads -> `codex.Record`,
no XML intermediate), and the iso2709 writer emits raw bytes (binary format).
Git history confirms it never routed through marcxml. The single-work
`GET /v1/works/{id}/marc` view uses the *same* `DecodeGrainMARC`, which is why
it looked correct -- the `codex.Record` was already clean.

Reproduced the encode over real grains (QLL + a LAPL slice) carrying literal
`&`, em-dashes, and markup: every one emits **raw** (`&amp;`=0, `&lt;`=0). The
`&amp;#8212;` originally observed was the *grain's stored content* (pre-089,
double-encoded) emitted faithfully -- not a layer added by export. The
"inconsistency" (8 of 31 single-escaped) was mixed grain vintages, not export
nondeterminism. It self-healed once tasks/089 (decode-to-fixpoint) landed and
the corpus was re-ingested; the playground now exports clean.

Locked in with a regression test:
`backend/export.TestExportMARCEmitsRawFreeText` ingests a record with `&`,
`&amp;`, `&#8212;`, and `<b>` markup, exports MARC, and asserts the bytes carry
raw `&`/em-dash and none of `&amp;`/`&lt;`/`<b>`.

This investigation also surfaced the genuine live bug behind the mangled text:
the OverDrive/Thunder provider decoded nothing, so entity-laden *titles* were
stored verbatim -- fixed under tasks/081.
