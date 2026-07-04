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
