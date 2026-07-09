# 190 -- nquads mapped contributor names are final sort-form labels: no lastFirst re-inversion, no yearLed false positive (coll-support 030)

Filed from coll-support on 2026-07-08 (cross-repo ask).

Root cause of queerbooks' remaining 5-grain parity residue (their tasks/034
audit at v0.41.0; coll-support tasks/030). The coll-feed contract's
dcterms:contributor literals are FINAL label forms: the exporter already
lastFirsts transcribed person names and passes OverDrive creator sortNames
verbatim (which is also how corporate/direct-form names ride). Two provider
behaviors break that:

1. **ingest/nquads/record.go:156 re-applies lastFirst(name) to the mapped
   contributor name.** Comma-bearing person names pass through (no-op), but
   comma-less direct forms invert wrongly: "Barefoot Books" -> "Books,
   Barefoot", "Twin Cities GLBT Oral History Project" -> "Project, Twin
   Cities GLBT Oral History", "Christo Casas" -> "Casas, Christo" (grains
   wsiciscndbibeq, wrl2u3t3mrppba, wtemuhlsu6cucq, w1dkou17c25uj0; the old
   qbd pipeline carried the OD sortNames verbatim). Fix: use the mapped
   name as the Label unchanged. (The 069 creator-FALLBACK lastFirst is
   fine -- creators are raw access points, not sort forms.)

2. **The 186 junk gate's yearLed test runs on the sort-form name.**
   "5000, Alaska Thunderfuck (narrator)" -- a real narrator credit (drag
   artist Alaska 5000, OD sortName form) -- matches ^\d{4}\b and drops,
   and the creator fallback then fabricates role "author" (grain for
   coll:6994). A comma-bearing "NNNN, Rest" is an inverted name, not a
   bare copyright-year line; exempt comma-bearing names from yearLed (the
   heuristic targets raw debris like "1999" / "2011 EMI Records Ltd.").
   qbd ran the junk gate on the RAW pre-inversion name, so it never hit
   this.

Repro: ingest coll-support's catalog.coll.nq (22:28 export or later --
its literals are verifiably direct-form: grep
'<urn:coll:work:85424> .*contributor' gives "Barefoot Books (author)")
and diff the five grains against queerbooks works-qbd-pre030flip; all
five should converge, closing their parity residue to zero.

## Outcome

Fixed in 6e17103, released v0.43.0. Both behaviors exactly as
diagnosed:

1. The mapped-contributor path no longer applies lastFirst -- the name
   rides into the Label unchanged (final sort-form per the coll-feed
   contract). The creator fallback keeps lastFirst; its literals are
   raw access points.
2. isJunkContributor's year-led test exempts comma-bearing names, so
   "5000, Alaska Thunderfuck (narrator)" survives with its real role
   and the fallback no longer fabricates an author for coll:6994.
   Comma-less debris ("1999 EMI Records Ltd.") still drops.

Repro against the 22:28 export: all FIVE residue grains converge --
coll:85424, 85431, 87641, 71881 (name forms) and coll:6994 (Alaska) --
zero non-prefLabel diff lines vs works-qbd-pre030flip after id
normalization. Note for the audit: coll:20081 and coll:26282 carry
"Books, Barefoot (author)" IN the export itself (coll-support's own
author-fallback inverted them; no OD sortName on those records) and
qbd's baseline has the same form, so they diff clean -- verbatim
passthrough preserves them by design; if the inversion bothers anyone
it's exporter-side (coll-support), not provider-side.
