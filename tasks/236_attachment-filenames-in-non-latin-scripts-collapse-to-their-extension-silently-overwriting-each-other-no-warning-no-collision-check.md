# 236 -- attachment filenames in non-Latin scripts collapse to their extension, silently overwriting each other; no warning, no collision check

Filed from libcat on 2026-07-09 (cross-repo ask).

## Symptom

Attach `文書.pdf` to a work, then attach `資料.pdf` -- a different document with a
different name. The work ends up with **one** attachment, called `pdf`, holding
the second document. The first is gone. No error, no warning, no trace in the
panel.

Measured on the 8481 playground through the real file input
(`ui/probe_attachments.mjs`), against a copycat-minted sentinel work:

```
FAIL T1  distinct non-Latin filenames stay distinct
         文書.pdf 資料.pdf 報告書.pdf Тест.pdf all fold to "pdf"
FAIL T4  two distinct uploads produce two attachments
         after 文書.pdf: ["pdf"];  after 資料.pdf: ["pdf"]
FAIL T5  the first upload is not overwritten by the second
         only "pdf" exists and it holds "SECOND-DOCUMENT" -- 文書.pdf's bytes are gone
FAIL T6  the panel tells the cataloger the name was changed
         panel reads "Attachments … pdf ×  scan.pdf ×" -- no notice that 文書.pdf became "pdf"
FAIL T7  POST to an existing name is refused or versioned
         re-POST scan.pdf -> 201; the stored bytes are now "REPLACED"
```

Everything else in the surface is sound: the traversal name `../../grain` is
refused (400), an empty body is refused (400), anonymous access is 401, an ASCII
name round-trips, and remove clears both the statement and the bytes.

## Root cause

Three things line up, and each is individually defensible.

**1. The sanitizer discards whole scripts.** `backend/ui/src/lib/api.ts:787-794`:

```js
export function safeAttachmentName(name: string): string {
  const folded = name
    .normalize("NFKD")
    .replace(/[^A-Za-z0-9._-]+/g, "-")   // every CJK/Cyrillic/Arabic run -> "-"
    .replace(/^[^A-Za-z0-9]+/, "")       // ... which is then stripped from the front
    .slice(0, 100);
  return /^[A-Za-z0-9]/.test(folded) ? folded : "";
}
```

For `文書.pdf` the fold gives `-.pdf`, the leading-strip gives `pdf`. Any filename
whose stem is entirely non-Latin collapses to its bare extension, so every `.pdf`
in a Japanese, Russian, or Arabic filename becomes the same name:

```
"文書.pdf"   -> "pdf"        "Тест.pdf"  -> "pdf"
"資料.pdf"   -> "pdf"        "صورة.jpg"  -> "jpg"
"報告書.pdf" -> "pdf"        "日本語"     -> ""     (this one the panel does catch)
```

**2. The panel only reports the total loss, never the partial one.**
`AttachmentsPanel.svelte:29-33` errors when the folded name is empty and is
otherwise silent -- a name that was mangled beyond recognition uploads without
comment:

```js
const name = safeAttachmentName(file.name);
if (!name) { error = "that filename has no usable characters -- rename and retry"; return; }
```

**3. The server has no collision check.**
`backend/httpapi/attachment_handlers.go:51-91` validates the name, writes the
`lcat:attachment` statement (idempotent), and calls `bs.Put` -- which overwrites.
A second POST to a live name returns 201 and replaces the bytes.

The server is not wrong to be idempotent, and `ValidAttachmentName`
(`bibframe/attachment.go:23`) is right to be strict: its leading-alphanumeric
anchor is what keeps `..` and dotfiles out. The defect is that the client's way
of *satisfying* that strictness destroys information, and nothing downstream
notices that two different documents now claim one name.

## Why it matters

This is silent, unrecoverable loss of a cataloger's file. They attached a scan,
the panel showed a row, and the earlier scan is simply not there -- discoverable
only by opening the one row and finding the wrong document. The attachments
surface exists to hold "scans, correspondence, acquisition paperwork"
(`attachment_handlers.go:20`), which is exactly the material that arrives named
in whatever script the institution works in.

The failure is worse for being invisible: nothing in the panel, the audit log
(`ATTACHMENT_ADD` records the *folded* name), or the grain records that `文書.pdf`
ever existed. A librarian looking for the missing scan has nothing to search for.

And it is not an exotic input. Any library cataloging material in Japanese,
Chinese, Korean, Russian, Greek, Hebrew, or Arabic hits it on the second upload.

## Expected

Any one of these closes the data loss; the first two together are the honest fix.

- **Preserve the name.** Transliterate rather than delete: `文書` → something
  stable and distinct, or percent-encode the original into the stored segment and
  keep the display name in the `lcat:attachment` statement. The stored blob
  segment must stay within `ValidAttachmentName`; the *displayed* name need not.
- **Never silently rename.** When `safeAttachmentName(file.name) !== file.name`,
  tell the cataloger what it will be stored as and let them confirm or retype.
- **Refuse to clobber.** `POST /v1/works/{id}/attachments` should 409 when the
  name is already attached, or version it (`pdf`, `pdf-2`). Re-uploading the same
  file deliberately is then an explicit DELETE-then-POST, or a `?replace=true`.
- If a name cannot be made unique, the upload must fail loudly rather than
  succeed onto someone else's bytes.

Worth deciding: whether the display name and the blob segment should be separate
concepts. They are conflated today, which is what forces the sanitizer to be
lossy in the first place.

## Repro

```
cd ~/libcat-e2e && node ui/probe_attachments.mjs
```

Expect `T1`, `T4`, `T5`, `T6` and `T7` to flip to PASS, with `T2`, `T3`, `T8`-`T11`
staying green (an ordinary filename must keep folding to a sane name, and the
traversal / empty-body / authz guards must not regress). The probe mints its own
sentinel work, removes every attachment it creates, and tombstones the work.
`harness/retest.mjs` carries the same check as `t236`.
