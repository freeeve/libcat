# 048 -- Export UX (selection, formats, exports screen)

## Context

Cataloger-facing half of export jobs (tasks/038): subset selection shares the
batch Selection model; formats are MARC .mrc, BIBFRAME N-Quads, JSON-LD, and
CSV of projected rows; the dialog is honest about MARC lossiness.

## Scope

1. `backend/exportsvc/` (or fold into export/): selection -> format emitters
   wiring over the 038 job runner; CSV columns from `project.Work` (+ item
   columns when items land).
2. SPA Exports screen: new-export dialog (format, selection summary, lossiness
   note linking the rendered marc-fidelity table), job list with status/record
   count/expiry, download links.
3. SelectionBar integration: "Export selection" from search results and batch
   screens.

## Acceptance

- [x] Export of a search-selection produces exactly those works.
- [x] MARC option shows the fidelity note; `lcat:marcVerbatim` re-emission noted
  once 049 lands (the note links docs/marc-fidelity.md, which will cover it).
- [x] Job list reflects live status transitions; expired jobs show as expired.

## Delivered (2026-07-02)

The tasks/038 runner already had formats, job lifecycle, and download links;
this added the cataloger-facing half:

- **Selection bridge**: `POST /v1/exports` accepts a `batchSelection`
  (`search`/`savedQuery`/`ids`/`all` from tasks/047) that compiles to the
  export's frozen work-id list at create time via `batch.Service.Resolve`
  ("all" passes through -- the runner scans the tree itself). An empty
  selection fails closed. No new backend service needed -- the batch
  machinery was the missing piece, exactly as this task predicted.
- **SPA Exports screen**: new-export form (selection kinds incl. saved
  queries, live preview count, format picker with per-format notes; the MARC
  option shows the lossiness warning linking the marc-fidelity table); job
  table with status badges, record counts, download links, expiry countdown,
  EXPIRED state (computed from `expiresAt`), and a 4s poll while any job is
  QUEUED/RUNNING.
- **Selection integration**: "Export these results…" on the work search and
  "Export selection…" on the batch screen deep-link to
  `#/exports?kind=…&q=…/ids=…/sq=…` with the form prefilled and the count
  pre-resolved; Exports joined the nav and the command palette.
- **Tests**: httpapi acceptance test (search-selection CSV export contains
  exactly the matched works, token download, list, empty-selection 400);
  axe a11y over the screen with the MARC note, an expired job, and an active
  job visible. Verified live against lcatd: search selection -> CSV with
  exactly the selected work's projected row -> token download; MARC export of
  the same selection; both in the job list.
