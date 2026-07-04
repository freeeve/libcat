# 094 -- ISBN quick-add

Split out of `052_paradise-polish.md`.

## Context

Copy cataloging already supported fielded ISBN search + batch staging on the
Import (`/copycat`) screen, but only through the full advanced form. This adds a
scan/paste-one-ISBN fast path -- thin sugar over the existing copy-cataloging
flow, reusing `copycatSearch` and `stageCopycatBatch`.

## What shipped

- `ui/src/lib/isbn.ts`: `normalizeIsbn(raw)` reduces scanned/pasted/hyphenated
  input to a bare 10/13-digit token (trailing `X` allowed only as an ISBN-10
  check digit), returning `""` for non-ISBN input. No checksum enforcement --
  external targets tolerate a loose match. Unit tests in `isbn.test.ts`.
- Quick-add bar at the top of `ui/src/screens/CopyCat.svelte`: an autofocused
  ISBN input (Enter submits -- barcode scanners emit digits + Enter) and an
  "auto-stage best match" checkbox (default off).
  - default: searches the `isbn` index and drops hits into the existing results
    list (pick + stage as usual).
  - auto-stage: stages the top hit into a one-record `quick-add: <isbn>` batch
    and jumps into batch review. A `busy` re-entry guard stops a scanner's
    trailing Enter from staging duplicate batches. Zero hits shows "no match".

## Verified

UI tests + type-check pass. End-to-end on the demo playground: ISBN
`9780451524935` returned 11 hits across the seeded SRU targets; staging the top
record produced a `quick-add: <isbn>` STAGED batch (deleted after the check).
