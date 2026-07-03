# 073 -- ISBN subject lookup from external targets; identifier type badges

## Context

Works often carry subjects at LC/OCLC that the local record lacks; the
copycat targets (Z39.50/SRU) can find those records by ISBN and their 6XX
headings are exactly what a cataloger would crosswalk by hand. Fan-out is
seconds-slow, so this is a button, not an automatic fetch. Separately, the
editor's Identifiers field shows bare values with no hint whether one is an
ISBN, ISSN, or a provider id -- the grain's bf:identifiedBy nodes carry the
type.

## Scope

1. **Subject lookup**: `POST /v1/works/{id}/subjects/lookup` searches the
   copycat targets by the work's ISBNs, extracts 6XX headings (600/610/650/
   651/655 etc. with their source: LCSH/MeSH/$2), dedupes across targets
   with occurrence counts, reconciles each heading against the local index
   (whole-heading match -> controlled TermRef), and skips headings the work
   already carries.
2. **Suck-these-in UI**: a "Look up subjects at targets…" affordance by the
   Subjects field runs the lookup and lists candidates; reconciled headings
   add as controlled subjects, unreconciled ones add as tags -- each a
   normal staged editorial op (provenance: editorial + the save audit).
3. **Identifier kinds**: `GET /v1/works/{id}/identifiers` maps each
   identifier value to its BIBFRAME type (ISBN/ISSN/other) from the grain;
   the editor badges each Identifiers value with its kind.

## Acceptance

- A work with an ISBN held at a configured target lists that target's
  headings with counts; clicking Add stages an op (controlled when the
  heading matches a loaded vocabulary, tag otherwise).
- Headings already on the work do not reappear as candidates.
- The Identifiers field shows ISBN/ISSN/ID badges per value.
