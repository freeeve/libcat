# 081: Decode HTML entities in ingested MARC text

Work summaries in the playground corpus render literally as
`friendship&amp;#8212;a Newbery Honor Book` -- the source MARC carried
HTML-encoded (double-encoded) entities and ingest stored them verbatim, so
the editor and any public projection show `&amp;#8212;` instead of an
em-dash (spotted in the editor 2026-07-04; the text lives in the stored
`.nq` grains, e.g. `site/data/works/9d/w5h9o966r2nhos.nq`).

## Plan sketch

- Decide the decoding point: at ingest (marc provider text fields) so
  grains store clean text; existing grains need a reserialize/reproject or
  a batch cleanup.
- Decode numeric (`&#8212;` / `&#x2014;`) and named (`&amp;`, `&quot;`,
  &c.) entities, applied twice-safe for the double-encoded `&amp;#8212;`
  form; leave genuinely literal ampersands alone.
- Audit which fields are affected (summary at minimum; check title/notes).
