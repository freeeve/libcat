# 217 -- clone-work-editorial-draft (058 item 4)

Opened 2026-07-09. Split from tasks/058 scope item 4.

Clone copies an existing work into a brand-new one: fresh work/instance
ids, provider keys stripped, every statement editorial (no feed graphs),
opened as a draft in the editor. Requires the create-work path (a grain
built from an editorial-only doc) that no current surface has.

058 acceptance: "Clone produces an editable new work whose ids are fresh
and whose provider keys are gone."

## Outcome

Shipped in v0.67.0 (commit 25da7a6). `bibframe.CloneGrain` +
`POST /v1/works/{id}/clone` (librarian) + a Clone button in the editor
header -- the first eager create-work path; every other surface births
works through the ingest pipeline.

Semantics: fresh work/instance ids in every fragment IRI; every kept
statement re-graphed editorial; born suppressed (the draft state).
Stays with the source: provider identifiers (identifier-duplicate
resolution would trip otherwise), admin metadata (the clone's 040
derives from its own graph facts per 192), holdings, lcat curation
markers, and uncontrolled blank-node subject/genreForm headings.
Controlled subject IRIs carry over.

Two design finds along the way:

1. Blank structure nodes in the editorial graph are unpatchable (the
   editorial patch machinery refuses blank nodes), so a naive re-graph
   made the clone's title uneditable. CloneGrain skolemizes blanks to
   grain-local fragment IRIs (`#<id>n<k>`).
2. Blanket skolemization forged controlled terms: an IRI object of
   bf:subject reads as a controlled subject in summaries/projection.
   Hence uncontrolled headings stay behind. (Noted, not fixed here: the
   editor's own `-ed-` skolem shape for subjectLabels adds has the same
   IRI-as-controlled-term leak on any work.)

Verified live on the playground: clone -> fresh ids, editorial-only,
no identifiers, suppressed, clean summary; retitle + tag add via /ops
succeed; index seam opens the clone in the editor immediately. Unit
lifecycle tests in bibframe and httpapi.
