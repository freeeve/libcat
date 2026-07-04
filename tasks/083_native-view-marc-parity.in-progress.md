# 083: Native view renders the full MARC-derived record

The native editor shows only the 9 shipped profile fields; for a typical
OverDrive record ~100 quads (contributor, subjects-with-labels, genre/form,
links, notes, extent, duration, content type, edition, publication,
digital characteristics, classification) hide in the raw passthrough
disclosure. The MARC tab shows everything, so the native tab reads as
missing data (Koha parity gap).

## Plan

Backend:
- profiles: allow 3-predicate chains; add a `readOnly` field flag
  (3-chains must be readOnly -- the op layer cannot build nested typed
  structures yet).
- editor.ToDoc: claim chained fields through N intermediate hops.
- editor.ApplyOps: reject ops against readOnly fields.
- work-monograph: + contributors (100/700, readOnly), subjectLabels (650,
  readOnly), genreForm (655, readOnly), content (336, editable entity),
  classification (072/084, readOnly).
- instance-ebook: + links (856, editable entity), edition (250), duration
  (306), responsibility (245$c) editable literals; extent (300), notes
  (5xx), publisher/place/date (260), digital format (347), issuance
  (readOnly).

UI:
- rdaterms: RDA content types (336) list for the picker + labeled chips.
- ProfileForm: field specs for the new paths; `readonly` kind (values +
  provenance, no edit affordances); secondary fields grouped under a
  default-collapsed "Additional details" disclosure; generic http(s) IRI
  values render as links (856 covers/samples).

Follow-up (not this task): editable structured fields (notes,
publication, contributors) need typed skolem structures in valueQuads so
the MARC re-encode recognizes them.
