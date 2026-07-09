# 189 -- work editor issuance IRI renders raw (id.loc.gov>mono) instead of the LOC issuance label

Opened 2026-07-08.

## Outcome

Fixed in 4a5a06c, released v0.42.0. The MARC crosswalk emits
bf:issuance as id.loc.gov/vocabulary/issuance IRIs, but rdaterms.ts
only shipped the content/media/carrier closed lists, so the editor's
read-only Issuance field fell through to the generic IRI display
("id.loc.gov > mono"). The four-term LOC issuance vocabulary (single
unit, multipart monograph, serial, integrating resource) now ships as
data alongside the 33X lists and resolves through the same iriTerm
path (ProfileForm and the Duplicates value renderer both pick it up).
Verified in the playground editor: the Frog and Toad audio collection
instance renders Issuance "single unit" with the mono code chip and
marc provenance badge.
