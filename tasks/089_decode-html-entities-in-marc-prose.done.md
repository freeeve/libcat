# Decode HTML entities and markup in MARC free-text at ingest

Vendor MARC (OverDrive MARC Express among others) embeds HTML character
references and markup in prose fields -- 520 summaries rendered literally
as "murder &#8212; but Hamlet's spiral" in the editor, and the entities
were baked verbatim into stored bf:summary literals.

`FromCodexRecords` (the chokepoint every MARC path crosses: file ingest
and copycat imports) now normalizes the crosswalk's free-text carriers --
Work.Summary (520), TableOfContents (505), and Work/Instance Notes (5xx)
-- before grains are built: character references decode to a bounded
fixpoint (vendor feeds double-escape: &amp;#8212;), markup tags strip, and
the leftover whitespace collapses. Identifier and heading fields are
untouched; the verbatim fidelity sidecar keeps the original field bytes.

Existing grains carry the old encoded text until re-ingested; the demo
corpus was cleaned by exporting all works as MARC and re-ingesting into
feed:marc (byte-stable pipeline, identities all matched, editorial
untouched).

Found along the way (tracked separately): the bulk .mrc export
XML-escapes free text -- "&#8212;" leaves as "&amp;#8212;", and
inconsistently across records. See the follow-up task on export
double-escaping.
