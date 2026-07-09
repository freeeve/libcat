# 058 -- Maintenance remainder (concerns, covers, relationships, clone)

## Context

The tasks/051 scope items outside its acceptance criteria, split out when the
acceptance surfaces (visibility, duplicates+merge UI, bf:Item) shipped. Each
is a UI/plumbing layer over machinery that exists.

## Scope

1. **Concerns** (DONE -> tasks/210, v0.61.0; convert-to-edit deferred): `CONCERN` queue item type (freetext + workId + reporter),
   anonymous report-a-problem endpoint sharing the suggestion anti-abuse
   challenge, review-screen actions resolve / dismiss / convert-to-edit.
2. **Covers/attachments** (DONE: covers -> tasks/215 v0.65.0, zip batch -> tasks/220 v0.70.0, attachments -> tasks/229 v0.79.0): upload to the blob store + `lcat:coverImage`
   editorial quad (attachments same shape under `lcat:attachment`); batch
   zip upload keyed by workId/ISBN; projector surfaces the cover URL.
3. **RelationshipsPanel** (DONE -> tasks/221, v0.71.0; OPAC surfacing -> tasks/222): `bf:hasPart`/`bf:partOf`/series + enumeration in
   the editor (the write shapes exist via the editorial patch machinery).
4. **Clone** (DONE -> tasks/217, v0.67.0): copy doc, strip provider keys, mint fresh work/instance ids,
   open as a draft -- needs a create-work path (grain built from an
   editorial-only doc), which none of the current surfaces have.
5. **Item polish** (CSV half DONE, v0.88.0): items column(s) in the CSV export;
   batch item edits through the tasks/047 op machinery -- REMAINING.
6. **Merge chooser polish** (DONE, v0.90.0): per-field adopt-left/adopt-right
   staging ops on the survivor before the merge (the tasks/051 compare view is
   read-only).

## Acceptance

- An anonymous concern lands in the queue, is reviewable, and resolves or
  converts to an edit with audit.
- A cover uploads, projects into catalog.json, and renders in the Hugo module.
- Clone produces an editable new work whose ids are fresh and whose provider
  keys are gone.

## Progress

**Item 5, CSV half -- v0.88.0.** `backend/export/run.go` grew
`workHoldings`, so the CSV carries `itemCount,callNumbers,locations,barcodes`.
The same pass taught the exporter to resolve a subject's label through the
vocabulary when the grain does not carry one (`subjectLabel`), rather than
emitting a bare IRI. Covered by `backend/export/csv_subjects_test.go`.

**Item 6 -- v0.90.0.** `backend/ui/src/lib/mergeadopt.ts` holds the adoption
logic as pure functions (`adoptionChanges`, `adoptionValues`, `adoptionOps`);
`Duplicates.svelte` renders an adopt button on every field where the losing
record would actually change the survivor, and `merge()` posts the staged ops
under the survivor's ETag before writing the merge markers.

Two shapes to remember, both learned the hard way:

- The ops contract takes `values` only on `set`. An `add` carrying an array is
  refused server-side with `add needs a value` (`backend/editor/apply.go`), so a
  repeatable field adopts as one `add` per new value.
- Because of that, op count is not field count. The staged-count message counts
  distinct paths, or a three-tag adoption would claim three fields.

Adoption is a union: a value the survivor already holds offers no button and
stages no op, so re-running it writes nothing and the audit trail never carries
an empty diff. Changing the survivor clears staging, since the ops were computed
against the old one.

## Remaining

Item 5's second half: batch item edits through the tasks/047 op machinery. The
batch screen edits work-level fields; items are a separate resource with their
own paths, so this needs a design pass on how a macro addresses "every item on
the matched works" before any code.
