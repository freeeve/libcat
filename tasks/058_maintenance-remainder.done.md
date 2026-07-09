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
5. **Item polish** (DONE: CSV half v0.88.0, batch item edits v0.91.0): items
   column(s) in the CSV export; batch item edits through the tasks/047 op
   machinery.
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

## Outcome

All six items shipped. The last one, batch item edits, landed in **v0.91.0**.

The design question was how a selection addresses items at all. Item ids are
minted per grain, so no op list written against a selection can name one --
which is why `bibframe.SetItems` (wholesale replace, per instance) was the wrong
primitive to reach for. The answer is that items are addressed as a **set**:

- `resource: "items"` means every `bf:Item` in the grain. It is the one
  non-work resource a batch may name, precisely because it names no node.
- An optional `where` restricts the edit to items whose current value at `path`
  is exactly that string. This is what makes a relocation safe: moving Stacks to
  Annex must leave the Reference copies alone. An item that does not assert the
  field reads as `""`, so `where: ""` is the fill-in-the-blanks case rather than
  a special one.
- An item field holds one value, so only `set` and `clear` mean anything.
  `add`/`remove` are refused rather than reinterpreted -- a cataloger who writes
  `add location` believes it is repeatable and should hear otherwise.
- **`barcode` is unreachable.** It names one physical copy: assigning it across a
  selection mints duplicates, clearing it unlinks the shelf from the catalog.
  `TestItemFieldsMatchUI` fails the build if the picker and
  `bibframe.ItemFieldNames` ever disagree, so the omission cannot erode.

`ItemEditPatch` emits a surgical patch (only the items that actually change),
not a wholesale re-mint, so an idempotent re-run reports an empty diff instead of
reporting churn as work. Because it goes through `editor.ApplyOps`, the dry-run
diff, the audit entry, the CAS write, and the index update all came for free.

One thing the work turned up: `docs/api.md` prose I had written a task earlier
named the batch route `/v1/batch/run`. The generated route table says
`/v1/batch/ops`, and it is right. Fixed. The generator earns its keep by
contradicting the prose beside it.

Verified against the playground on a real record: a guarded relocation moves the
Stacks copies and not the Reference one, preserves call numbers and barcodes,
re-runs to an empty diff, refuses `barcode`, still refuses a bare instance id in
batch, and fills only the blanks under `where: ""`.
