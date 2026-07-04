# Seed verified open SRU targets; per-target SRU dialect

Seed every open, anonymous SRU endpoint we verified live as a default
copycat target, not just LOC: DNB (German national bibliography) and
K10plus (GBV/SWB union catalog) join loc-sru on a fresh store.

Servers differ in dialect, so Target grows three optional SRU knobs:

- `version` -- DNB only answers SRU 1.1 (a 1.2 request fails with a
  version diagnostic).
- `schema` -- DNB names its MARC21 slim schema `MARC21-xml` and rejects
  `marcxml`.
- `indexes` -- per-access-point CQL index overrides for servers off the
  Dublin Core / Bath mapping: DNB `dnb.num` for isbn/issn, K10plus
  `pica.isb`/`pica.iss`.

The SRU search path now runs one SearchRetrieve page (the 20-record cap
fits) and decodes payloads as MARCXML directly instead of going through
the libcodex Reader, whose schema gate silently skips DNB's
`MARC21-xml`-labeled records (filed as libcodex tasks/085; revert to the
Reader once normalizeSchema folds that label).

BnF was evaluated and dropped: its SRU only serves UNIMARC/InterMARC
schemas, which the MARC21 pipeline cannot read.

Verified end-to-end against the live servers through POST
/v1/copycat/search: free-text fan-out (LOC + K10plus + DNB), fielded
ISBN through each index override, per-target failure isolation with a
deliberately misconfigured target.
