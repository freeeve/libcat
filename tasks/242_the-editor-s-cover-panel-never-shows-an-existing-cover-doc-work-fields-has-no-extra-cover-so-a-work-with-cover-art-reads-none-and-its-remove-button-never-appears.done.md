# 242 -- the editor's Cover panel never shows an existing cover: doc.work.fields has no extra/cover, so a work with cover art reads 'none' and its Remove button never appears

Filed from libcat-e2e on 2026-07-09 (cross-repo ask).

## Symptom

Give a work a cover, reload the editor, and the Cover panel says the work has
none. The image is there -- it is in the grain, it is served publicly, and the
OPAC renders it. Only the editor cannot see it.

Measured on the 8481 playground against a copycat-minted sentinel
(`ui/probe_cover.mjs`):

```
PASS V15  the panel uploads and renders the cover        img src="/covers/w5of6k5nn9el7s.png?v=1"
PASS V16  CONTROL: the panel renders the uploaded image  rgba=[255,0,0,255] (an opaque red 1x1)
PASS V17  the replacement shows immediately              rgba=[0,0,255,255] (blue, after replacing)
FAIL V18  a work that has a cover still shows one after a reload
          the work's grain carries lcat:extra/cover, but the reloaded panel has 0 <img>
          and reads "COVER none Upload…"
FAIL V19  the existing cover can be removed from the editor
          Remove buttons: 0; the upload control reads "Upload…"
          (it says "Replace…" only when the panel knows a cover exists)
```

V15--V17 are the controls: the panel *can* render a cover, and does, for as long
as it is the thing that uploaded it. The state is entirely in-memory.

Directly against the API, the same work:

```
PUT cover -> 200
grain cover quads:
   <#wss0b4dcsqtnsoWork> <https://github.com/freeeve/libcat/ns#extra/cover> "covers/wss0b4dcsqtnso.png" <editorial:> .
doc work field names: [ 'content', 'language', 'title' ]
doc extra/cover: null
```

The consequences, in order of how much they hurt:

1. **The cover cannot be removed through the UI.** `Remove` renders only inside
   `{#if current}` (`CoverPanel.svelte:64`). On a fresh load `current` is `""`,
   so a cataloger who wants to take down a wrong or rights-encumbered cover has
   no control to click. The only route is `DELETE /v1/works/{id}/cover` by hand.
2. **A cataloger cannot see what cover a record has.** The panel shows `none`,
   so there is nothing to check against the item in hand.
3. **"Upload…" instead of "Replace…"** invites an upload onto a record that
   already has a cover, with no warning that something is being overwritten.

## Root cause

`WorkEditor.svelte:182` sources the prop from a profile field:

```svelte
<CoverPanel {workId} cover={$session.doc?.work.fields["extra/cover"]?.[0]?.v ?? ""} />
```

`backend/editor/doc.go:183` builds `work.fields` by walking the **profile's**
declared fields and nothing else:

```go
for _, field := range profile.Fields {
```

No profile declares `extra/cover`. `backend/profiles/defaults/work-monograph.json`
has exactly eleven paths -- `title, subtitle, contributors, summary, language,
subjects, subjectLabels, tags, genreForm, content, classification` -- and
`grep -rn '"extra/' backend/profiles/` finds nothing. So
`fields["extra/cover"]` is `undefined` for every work under every shipped
profile, and the `?? ""` swallows it.

The statement itself is fine. `bibframe.SetCover` (`bibframe/cover.go:25`)
writes `lcat:extra/cover` into the editorial graph, `cover_handlers.go` reads
and serves it, `export/export.go:53` exports it, and the OPAC's cover slot
reads it (tasks/022/025/215). Because no profile claims the predicate, the
quad falls into the doc's **passthrough** (`doc.go:62` -- "the unclaimed
statements as raw N-Quads lines"), which is exactly where an unclaimed
statement belongs. Nothing is corrupt; the client is reading the wrong place.

`grep -rn 'extra/cover'` across the repo returns four hits, and
`WorkEditor.svelte:182` is the only one that treats it as a profile field.
Every other reader goes to the grain.

## Why it matters

Cover art is one of the few pieces of a record that is *only* editable through
this panel -- there is no MARC field for it, no ops path, no bulk route in the
SPA. The panel is the whole surface, and two thirds of it (see, remove) has
never worked on a reloaded page.

Removal is the part that actually bites. Covers are the most likely part of a
catalog record to attract a rights complaint or a "that's the wrong edition"
correction, and they are served **publicly and unauthenticated** by
`GET /covers/{file}`. Today the cataloger who needs to take one down cannot,
from the UI, at all. They will conclude the record has no cover, because that is
what the panel says.

The bug hides well: it is invisible in the session that uploads the cover, which
is exactly the session in which anyone would test the feature.

## Expected

The panel needs the work's current cover at load time. Either:

- **Declare the field.** Add an `extra/cover` path to the work profiles, so
  `doc.work.fields["extra/cover"]` populates and `WorkEditor.svelte:182` starts
  being true. Cheapest, and it makes the existing client code correct as
  written -- but it also exposes cover URLs to the generic field editor, which
  is probably not wanted.
- **Or read it where it lives.** Have `GET /v1/works/{id}/doc` surface the
  cover explicitly (a `cover` key beside `workId`/`profileId`), or have
  `CoverPanel` fetch it -- there is already a public `GET /covers/{id}.{ext}`,
  though probing three extensions is worse than being told.

The second is the honest one: the cover is not a profile field, and pretending
it is would put a blob URL in front of catalogers as editable text.

Whichever way, `Remove` must render whenever the work has a cover, and the
upload control must read "Replace…" so an overwrite is never silent.

## Repro

```
cd ~/libcat-e2e && node ui/probe_cover.mjs
```

Expect `V18` and `V19` to flip to PASS, and `V20` to stop reporting ERROR (it is
skipped while no cover renders after a reload -- it belongs to tasks/243). The
controls `V15`, `V16` and `V17` must stay green: the panel must keep rendering
the cover it just uploaded. The probe mints its own sentinel work, removes the
cover, and tombstones the work.

`harness/retest.mjs` carries the same check as `t242`.

## Outcome

Fixed in **v0.95.0**. `ui/probe_cover.mjs` is 22/22; `retest.mjs` reports 242
FIXED. `V15`--`V17` stayed green, so the panel still renders the cover it just
uploaded.

Took the report's second option, and for its stated reason: the cover is not a
profile field, and declaring it would put a blob URL in front of catalogers as
editable text.

- `bibframe.CoverOf(grain, workID)` reads the effective cover -- an editorial
  statement overlays a feed-carried one, the same precedence `SetCover` writes,
  which is why it cannot simply read the editorial graph.
- `GET /v1/works/{id}/doc` returns it as `cover`, beside `doc` and `etag`.
- The editor session carries `cover`, and `WorkEditor` passes it to the panel.
  `CoverPanel` already had the prop and the `{#if current}` branches; nothing
  ever filled it.

`V20` needed both this and tasks/243: the panel's `?v=${bump}` cache-buster is
component state initialised to `0`, so after a reload it requests the same URL
the browser cached an hour earlier. With the cover response now carrying an ETag
and `must-revalidate`, that request revalidates and the replacement appears. The
report predicted this exactly -- "fixing 242 without this will surface it" -- and
it would have, since `V20` reported ERROR only because nothing rendered at all.

## What made this hard to see

The report's own line: "it is invisible in the session that uploads the cover,
which is exactly the session in which anyone would test the feature." The panel
holds `current` in local state, so the person who just uploaded a cover sees it,
the Remove button, and the "Replace…" label. Everything works, once, for the one
person who cannot be harmed by it not working.

## Verification (filer)

Fixed. Confirmed 2026-07-09 by `harness/retest.mjs` (`t242` FIXED) and by
`ui/probe_cover.mjs`, now **21/21**:

```
PASS V18  a work that has a cover still shows one after a reload
          the reloaded panel has 1 <img> and reads "COVER Replace… Remove"
PASS V19  the existing cover can be removed from the editor
          Remove buttons: 1; the upload control reads "Replace…"
PASS V20  after a reload the panel shows the replacement, not the cached original
          src="/covers/whhq5b30j8si0i.png?v=0" (cache-buster back to v=0: true); rgba=[0,0,255,255]
```

Controls held: `V15`, `V16` and `V17` stayed green, so the panel still renders
the cover it just uploaded and the in-session cache-buster still works.

Returning `cover` at the top of the doc response, beside `doc` and `etag`, is the
better of the two options I offered, and the code comment says why more precisely
than my report did: it is not a profile field, so putting it in `doc.work.fields`
would have handed a blob URL to the generic field editor as editable text.

`V20` passes only because tasks/243 shipped the etag in the same batch. On a
reload the panel requests `?v=0` again -- the exact URL the browser cached an
hour earlier -- and gets the new image because the response now revalidates. Had
this landed alone, fixing the panel would have *surfaced* the stale-cache bug
rather than hidden it. The two were more coupled than either report said.

**I had to correct my own check to see the fix.** `t242` looked for the cover at
`doc.work.fields["extra/cover"]` or `doc.cover` -- inside the doc -- because those
were the two shapes my report proposed. The fix put it at `response.cover`,
satisfying the panel and neither of my guesses, so the check reported
STILL-BROKEN against working code. It now accepts any of the three. A retest that
recognises only the fix its filer imagined is worse than no retest: it argues
confidently for the wrong conclusion.
