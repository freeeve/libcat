# 229 -- work-attachments (058 item 2 completion)

Opened 2026-07-09. The last piece of tasks/058 scope item 2 (covers
shipped in 215, zip batch in 220).

Generalize the covers machinery to arbitrary work attachments under
`lcat:attachment` editorial statements:

- Blob path `data/attachments/<shard>/<workId>/<filename>` (sanitized
  basename, [A-Za-z0-9._-], <=100 chars); 20MB cap; any content type.
- POST /v1/works/{id}/attachments?name=<filename> (librarian, raw
  body, grain-first with the describes-guard), DELETE .../{name}, GET
  list. Download serves application/octet-stream with
  Content-Disposition: attachment -- no inline render, so an uploaded
  HTML file is not an XSS surface.
- Staff-only: attachments are cataloging working material (scans,
  correspondence); NOT projected to the OPAC. Public surfacing, if
  ever wanted, is a later opt-in with its own review.
- Editor: AttachmentsPanel beside CoverPanel (list, upload, remove).
- Audit ATTACHMENT_ADD / ATTACHMENT_REMOVE. Clones do not carry
  attachments (the lcat:* drop in CloneGrain already covers the
  statements; bytes stay with the source).

## Outcome

Shipped in v0.79.0 (commit b19868e), exactly per the scope above, plus
a client-side name folder (`safeAttachmentName`) so real-world
filenames ("My Scan (1).pdf") land on the server's safe shape instead
of 400ing. Downloads ride fetch with the bearer (an anchor can't carry
the header) and object-URL to disk.

Verified live on the playground: upload -> statement in the grain +
201; list; authorized download with attachment/nosniff headers; anon
download 401; delete removes statement, bytes, and the listing. Unit
lifecycle in bibframe (including clone-drop and hostile-name table)
and httpapi.

058 item 2 is now complete end to end; the 058 remainder is items 5
(item polish) and 6 (merge chooser).
