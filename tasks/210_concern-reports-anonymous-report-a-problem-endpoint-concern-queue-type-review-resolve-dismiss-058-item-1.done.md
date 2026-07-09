# 210 -- concern reports: anonymous report-a-problem endpoint, CONCERN queue type, review resolve/dismiss (058 item 1)

Opened 2026-07-09.

## Outcome

Shipped in one commit (feat tasks/210), released v0.61.0. Design
choice under the assume-and-go rule: concerns ride the existing
Suggestion machinery as Type CONCERN with the freetext in a new Note
field and a content-hash id under the 'concern' pseudo-scheme --
storage keys, the status-index queue, Review transitions, and the
queue UI all reuse, instead of a parallel record type. Trims vs the
058 sketch, noted for later: convert-to-edit is deferred (a moderator
resolves and edits manually today).

- POST /v1/concerns: challenge (3s dwell) + honeypot + shared rate
  budget; note bounds 10-2000 chars; identical resubmission no-ops.
- Review approve/reject read as resolve/dismiss with legible
  CONCERN_RESOLVE/CONCERN_DISMISS audit actions; ApprovedUnpublished
  filters TypeConcern so nothing ever publishes.
- Queue screen renders the note italic in place of the term chip with
  Resolve/Dismiss buttons (no tombstone/substitute for concerns).
- Verified live end-to-end on the playground: anonymous submit 202 ->
  queued with note and work title -> resolve 200 -> CONCERN_RESOLVE in
  the audit trail with actor. Lifecycle unit test covers bounds,
  dedupe, dismiss, and the publisher filter.
