# 243 -- replacing a cover with a different image format leaves the old image stored and publicly served forever; cover uploads are also unvalidated, uncacheable-safely, and content-type case-sensitive

Filed from libcat-e2e on 2026-07-09 (cross-repo ask).

Four findings in the cover pipeline, ordered by harm. The first is the one that
matters; the rest are hardening in the same two handlers.

## 1. A replaced cover of a different format stays public

`PUT` a JPEG, then `PUT` a PNG. The grain repoints to the PNG. The JPEG is never
deleted and keeps serving from its public, unauthenticated URL.

```
PASS V1  a JPEG cover uploads and is recorded in the grain
         grain says "covers/w8u0agcntalobe.jpg"
PASS V3  replacing with another format repoints the grain
         grain says "covers/w8u0agcntalobe.png"
FAIL V4  the replaced cover stops being served
         after replacing the jpg with a png, GET /covers/w8u0agcntalobe.jpg -> 200
         (the old image is still public, and nothing references it)
```

Nothing in the catalog points at the old blob any more, so nothing will ever
clean it up. `DELETE /v1/works/{id}/cover` does sweep all three extensions
(`V13` passes), so the orphan is collected if and when the cover is *removed* --
but a replaced cover is never removed, which is the whole point of replacing it.

### Root cause

`backend/httpapi/cover_handlers.go:56-67` -- PUT writes the new blob and nothing
else:

```go
url := "covers/" + workID + "." + ext
etag, err := mutateWorkGrain(...SetCover(g, workID, url)...)
if _, err := bs.Put(r.Context(), bibframe.CoverBlobPath(workID, ext), data, blob.PutOptions{}); err != nil {
```

DELETE already knows the right shape (`cover_handlers.go:90-92`):

```go
for ext := range map[string]bool{"jpg": true, "png": true, "webp": true} {
    _ = bs.Delete(r.Context(), bibframe.CoverBlobPath(workID, ext))
}
```

The same sweep, for the extensions other than the one just written, is missing
from PUT. `applyBatchCover` (`cover_batch.go:163`) has the identical gap.

### Why it matters

Covers are served publicly and unauthenticated -- `GET /covers/{file}` is
deliberately open, and the handler's own comment says so. The reason a cataloger
replaces a cover is usually that the old one was *wrong*: the wrong edition, a
rights complaint, or an image that should not have been published. In every one
of those cases the old image stays fetchable at a stable, guessable URL
(`/covers/<workId>.jpg`) after being replaced, and the editor shows no trace of
it. A takedown that looks done is not done.

Secondarily the blobs accumulate: each work can hold three, and nothing counts
or reaps them.

### Expected

On a successful `SetCover`, delete the blobs for the extensions that are no
longer referenced -- the loop DELETE already runs, skipping the extension just
written. Do it after the `Put` succeeds, so a failed store never destroys the
old cover. `applyBatchCover` needs the same.

## 2. Any bytes are accepted, and served as an image

The upload's content type comes from the request header and is never checked
against the body:

```
PUT html-as-png -> 200
GET /covers/<id>.png -> 200
  content-type: image/png
  x-content-type-options: null
  body: "<html><script>alert(1)</script></html>"
```

`cover_handlers.go:41` trusts the `Content-Type` header; nothing sniffs or
decodes the bytes.

**This is not stored XSS.** A browser given `Content-Type: image/png` will not
render the body as HTML -- per the MIME-sniffing rules an `image/*` response is
treated as an image, and this one merely fails to decode. The concrete outcome
is a broken cover and a blob whose declared type is a lie, both for the OPAC and
for `lcat export -covers`, which copies these bytes into a static site.

Worth fixing cheaply anyway: `image.DecodeConfig` on the leading bytes rejects a
non-image and confirms the format matches the declared type (today a JPEG
uploaded as `image/png` is stored at `.png`). Adding
`X-Content-Type-Options: nosniff` to `GET /covers/{file}` costs one line and
retires the question.

## 3. A cover response cannot be revalidated

```
FAIL V5  a cover response is revalidatable
         cache-control="public, max-age=3600" etag=null last-modified=null
```

`cover_handlers.go:127-128` sets a one-hour public cache lifetime and no
validator, discarding the etag `bs.Get` returns (`data, _, err :=`).

A same-format replacement keeps the URL identical, so for up to an hour after a
correction every cache between the blob store and the reader -- the browser, any
CDN -- keeps serving the old image. The server is right (`V6`: it returns the
new bytes); the readers are not.

`CoverPanel.svelte:19` works around this with `?v=${bump}`, but `bump` is
component state initialised to `0`, so it lives exactly as long as the page
does. Reload, and the panel requests `?v=0` again -- the URL whose old bytes the
browser cached an hour ago. Today that is masked by tasks/242, which leaves the
panel rendering no image at all after a reload; fixing 242 without this will
surface it.

Set the blob's etag on the response so conditional requests work. A shorter
`max-age` with `must-revalidate` would also do.

## 4. The content type is matched case-sensitively

```
FAIL V7  the content type is matched case-insensitively
         PUT with Content-Type "Image/PNG" -> 415
```

`cover_handlers.go:41` looks the trimmed header up in `coverTypes` verbatim.
RFC 9110 §8.3.1 makes type and subtype case-insensitive, so `Image/PNG` is a
valid spelling of `image/png`. One `strings.ToLower` fixes it. Low impact -- the
SPA sends lowercase -- but it is a real refusal of a correct request, and the
415 body ("cover must be image/jpeg, image/png, or image/webp") does not tell
the caller what was wrong with what they sent.

## Repro

```
cd ~/libcat-e2e && node ui/probe_cover.mjs
```

Expect `V4`, `V5`, `V7` and `V8` to flip to PASS. `V20` (the stale-cache render
after a reload) is gated behind tasks/242 and reports ERROR until the panel
shows an existing cover at all.

These must not regress: `V2` (a cover serves with the right content type), `V6`
(a same-format replacement returns the new bytes from the server), `V9`--`V12`
(empty body 400, unsupported type 415, anonymous PUT 401, unknown work 404) and
`V13` (remove clears the statement and every stored format).

The probe mints its own copycat sentinel work, removes its cover, and tombstones
the work. `harness/retest.mjs` carries the same check as `t243`.

## Outcome

All four findings fixed in **v0.95.0**. `ui/probe_cover.mjs` is 22/22 and
`retest.mjs` reports 243 FIXED. The listed non-regressions (`V2`, `V6`,
`V9`--`V13`) all stayed green.

1. **The replaced cover is swept.** `sweepStaleCovers` deletes every format
   except the one just written, and runs *after* the `Put` succeeds, so a failed
   store never destroys the image it was replacing. `DELETE` now shares that
   loop rather than keeping its own copy of the extension list, and
   `applyBatchCover` calls it too.
2. **The bytes must be the format the header claims.** `http.DetectContentType`
   sniffs the magic bytes and they have to agree with the declared type -- which
   also stops a JPEG being stored at a `.png` path. That is stdlib, so it
   covers webp without pulling in `golang.org/x/image`. The zip batch path
   checks the same thing against the entry's extension.
3. **The public response revalidates.** It carries the blob's ETag,
   `must-revalidate`, and `nosniff`, and answers a matching `If-None-Match`
   with 304. The etag changes when the bytes do, so a stale conditional request
   gets the new image instead of a 304.
4. **Content-Type is matched case-insensitively**, and the 415 quotes what it
   was sent.

## The byte check earned its keep before it shipped

Its first run failed `TestCoverBatch`: the fixture zip stored PNG bytes under
`978-1-250-31319-5.jpg`. Exactly the mismatch the check exists to catch, sitting
in the test suite for the feature.

## On the XSS question

The report is right that this was not stored XSS, and right to fix it anyway. An
`image/*` response is not sniffed into HTML, so the concrete harm was a blob
whose declared type lied to the OPAC and to `lcat export -covers`. `nosniff` is
now set regardless, because blobs written before this release can still be
anything and nothing rewrites them.

## Not repaired

Orphaned cover blobs from replacements made **before** this release are still in
the store and still served. Their URLs are guessable (`/covers/<workId>.jpg`).
Anything a rights complaint touched between tasks/215 and now needs a sweep. The
one-line fix for an operator is `DELETE /v1/works/{id}/cover` followed by a
re-upload, which clears all three formats -- and that is worth saying out loud in
the reply, because "we fixed the leak" and "we plugged the hole" are different
sentences.
