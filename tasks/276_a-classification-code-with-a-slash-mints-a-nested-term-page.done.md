# 276 -- a classification code with a slash mints a nested term page

Filed from libcat-e2e on 2026-07-09 (cross-repo ask).

First finding from the **public OPAC** (8482 / 8502), which this harness had never
tested -- it had been probing the queerbooks *admin* SPA and calling it discovery.

A Dewey number carries a prime mark: MARC 082 `$a 813/.6`. `project` keeps the code
verbatim, the Hugo adapter indexes the classification taxonomy by that raw value, and
Hugo's term slugging does not touch `/`. So one taxonomy term mints **two path
segments**, the second a **dot-directory**:

```
/classifications/813/.6/
                 ^^^ ^^
                 |   a directory whose name begins with a dot
                 an index-less parent nobody asked for
```

The sitemap advertises it. `lcat serve` answers the accidental parent with Go's raw
`http.FileServer` directory listing, in the middle of a public catalog.

Measured read-only against **:8482**, the playground's published site. Nothing was
mutated anywhere.

## Symptom

```
GET /                          -> links to /classifications/813/.6/
GET /classifications/          -> 21 terms; exactly one contains a slash: "813/.6"
GET /classifications/813/.6/   -> 200   <title>813/.6 · libcat playground catalog</title>

GET /classifications/813/      -> 200   and the body is:

    <!doctype html>
    <meta name="viewport" content="width=device-width">
    <pre>
    <a href=".6/">.6/</a>
    </pre>

GET /sitemap.xml               -> <loc>http://localhost:8482/classifications/813/.6/</loc>
```

Every other term is a BISAC code -- `fic000000`, `bio026000`, `juv001000` -- which is
alphanumeric and mints one clean segment. The demo corpus has exactly one Dewey
number, and it is the one that breaks.

**On-site links still work**, and that is worth stating plainly rather than
overclaiming: they are absolute, and `lcat-term-url.html` resolves them through
`site.GetPage` rather than reconstructing them (that is tasks/128's fix, working as
designed). This is a defect in the *shape of the URL space*, not in navigation on
this server.

## Root cause

**The value is verbatim MARC.** `project/project.go:78-82`:

> *"v9 made classifications {value, label} objects: **value stays the scheme code
> (what MARC 084 $a carries)**, label is the human text riding the classification
> node's rdfs:label -- the display-only channel."*

Nothing anywhere strips the `/`. `grep -rn 'ReplaceAll.*"/"' project/ ingest/ bibframe/`
returns nothing.

**The taxonomy is keyed by that raw value.** `hugo/content/works/_content.gotmpl:51-52`:

```gotmpl
{{- $classificationValues := slice -}}
{{- range .classifications -}}{{- $classificationValues = $classificationValues | append .value -}}{{- end -}}
```

Compare the two lines above it, where tags go through a slug partial:

```gotmpl
{{- range .tags -}}{{- $tagSlugs = $tagSlugs | append (partial "lcat-slug.html" .) -}}{{- end -}}
```

**And this is documented as deliberate**, which is why the fix is not "call
`lcat-slug`". `layouts/_partials/lcat-term-url.html`:

> *"'key' -- the value the content adapter indexed: the lcat-slug for subjects/tags
> (tasks/023), **the raw value for contributors/formats/languages/classifications**."*

Raw-value keying is a reasonable design: `site.GetPage` finds the page Hugo minted,
whatever Hugo called it, so links cannot drift from pages. It holds for three of those
four taxonomies because a contributor name, a format (`ebook`), and a language code
cannot contain a slash. **A classification code can, and Dewey routinely does** -- the
prime mark in `813/.6` is the standard segmentation indicator. Classifications are the
only raw-keyed taxonomy whose values carry the one character that is structural in a
URL path.

`lcat-slug.html`'s own doc comment names the character:

> *"collapses every run of characters that are not a Unicode letter or decimal digit
> to a single hyphen ... This strips the ASCII punctuation `urlize` leaves in the path
> (`+`, `/`, ...) -- which a CDN like S3/CloudFront mis-decodes."*

**The byte cap is bypassed too.** `lcat-slug.html` pipes through `lcat-cap.html`
(tasks/134: a 48.5k-work build died at render with *"file name too long"*). The raw
classification value reaches the filesystem with neither the slug nor the cap. *Read,
not measured* -- I have not built the module against a pathological value.

**The guard that advertises this catch does not make it.** `hugo/link_check.cjs:1-4`:

> *"Walks a built Hugo site and asserts every root-relative link resolves to a
> generated file -- catching facet/term links whose slug does not match the page Hugo
> minted (e.g. a `+`/`/` in a subject/tag label that a CDN would 404)."*

Run against the playground's build, it passes:

```
$ node hugo/link_check.cjs ~/libcat-playground/opac/public
===== 384 pages checked =====
All internal facet/term/work links resolve; no CDN-unsafe '+' paths.
```

Two reasons, both worth fixing. The link **does** resolve: Hugo really did generate
`813/.6/index.html`, so a resolve-check cannot see a URL that is well-formed on disk
and hostile on a host. And the CDN-safety check is `clean.includes("+")` (`:58`) --
`+` only, despite the comment naming `/`. A `/` cannot be found this way in any case,
because by the time the path is a path the separator is indistinguishable from the
ones that belong there. The check has to run on the **term key**, before it becomes a
path.

**The directory listing is `lcat serve`'s.** `cmd/lcat/serve.go:38-43` is
`http.FileServer(http.Dir(dir))` with `Cache-Control: no-store`, and Go's FileServer
auto-lists a directory that has no `index.html`. Its doc comment scopes it honestly to
*"local previews"* and *"the whole local loop"*, so this is not a production web server
and I am not filing it as one. But it is what serves both demo OPACs today, and the
index-less directory it is listing exists **only because of the slash**. Fix the slash
and the listing has nothing to list.

## Why it matters

**This is the fourth task in a line, and the first three were all real breakage.**
tasks/023 (`LGBTQ+ books` → a live 404 on a CDN), tasks/128 (`Kuang, R.F.` → a
user-reported 404 at `libcatalog.evefreeman.com`), tasks/134 (a real-corpus build death
at 48.5k works). Each one taught that a taxonomy value is not a path component. The
classification taxonomy arrived in tasks/142, after all three, and inherited none of
the protections -- 142's body does not mention slugs or URLs at all.

**It is latent here and load-bearing elsewhere.** `lcat serve` serves dot-directories
and index-less directories happily, so the playground looks fine. A published catalog
is meant to go on a static host. Dot-prefixed path segments are exactly the thing that
static hosts and CDNs treat specially -- some hide them, some refuse them, some rewrite
them -- and `/classifications/813/` becomes whatever that host does with an index-less
directory: a 403, a 404, a listing, or a redirect loop. The module's own `hugo.toml`
publishes the `[taxonomies]` block as *"the canonical reference to copy"*, including
`classification = "classifications"`, so every adopter site that copies it inherits
this the moment its corpus contains one Dewey number.

**What a patron sees, on this server, today.** Truncating the URL -- or following the
listing's own link, or arriving from a crawler -- lands on a blank white page with a
single blue underlined `.6/` in the top-left corner. No header, no stylesheet, no
search box, no way back to the catalog. It does not look like a broken page; it looks
like a hacked one. Screenshot: `shots/opac-dirlist.png`, via
`node harness/probe_opac_ux.mjs` (U2).

That page answers `200`, which is why no status-code check in this harness would ever
have found it. A patron cannot read a stack trace, cannot retry, and cannot tell a
broken page from an empty catalog -- the only signal they get is how the page *looks*.

**A crawler is being handed the bad URL.** `sitemap.xml` lists
`/classifications/813/.6/`. Whatever a given host does with a dot-directory, the
catalog is asking Google to go find out.

**The blast radius scales with cataloging quality.** A corpus of BISAC codes is fine. A
corpus with real Dewey numbers -- which is to say, a library -- mints one of these per
distinct prime-marked number. Queerbooks (62,602 works) does not show it today only
because its build declares no `classifications` taxonomy.

Nothing is corrupted and no data is lost. This is a URL-shape defect, which is why it
will sit there until somebody publishes to a real host.

## Expected

- **Key the taxonomy by a path-safe slug; keep the raw code for display.** That is
  exactly the split tasks/142 already built: `classifications` (the taxonomy param)
  and `classificationList` (the display channel that keeps `{value, label}`). Slugging
  the key makes classifications mirror subjects/tags fully rather than halfway:

  ```gotmpl
  {{- range .classifications -}}{{- $classificationValues = $classificationValues | append (partial "lcat-slug.html" .value) -}}{{- end -}}
  ```

  `813/.6` → `813-6`, one segment, capped, and `lcat-term-url.html` keeps resolving
  through `site.GetPage`, so the link side needs no change at all. Update that
  partial's doc comment, which currently promises callers a raw classification key.

- **Decide what "value" means, and say so once.** If the code must stay literal in the
  URL for citability, then percent-encode it (`813%2F.6`) and accept that CDNs
  mis-decode it -- which is what tasks/023 rejected. Slugging the key is the cheaper
  answer, and the detail page already shows the real code. Whichever is chosen,
  `project/project.go:78-82`'s "value stays the scheme code" and the adapter's "raw
  value" comment should agree with it.

- **Do not fix this by stripping the prime mark in `project`.** `813/.6` and `813.6`
  are the same Dewey number, but the prime mark is data a MARC export must round-trip,
  and 142's whole point was that `value` is what exports keep. The URL is the thing
  that needs a slug, not the record.

- **Reject a taxonomy key containing `/` at build time.** The adapter already fails the
  build loudly on a `catalogSchemaVersion` mismatch (tasks/009 contract). A term key
  with a path separator in it is the same class of contract violation and is currently
  silent. This is the check that would have caught 023, 128, 134 and this one.

- **And teach `link_check.cjs` the character its own comment claims.** It passes on this
  build. Its CDN-safety test is `clean.includes("+")` (`:58`); the comment says `+`/`/`.
  A `/` can only be caught **before** the key becomes a path -- once it is a path, the
  separator that does not belong is indistinguishable from the ones that do. So the
  check belongs on the adapter's term keys, not on the rendered hrefs. Note also that
  the resolve half of that checker can never catch this: Hugo genuinely generated
  `813/.6/index.html`, so the link resolves. A URL can be well-formed on disk and
  hostile on a host, and only the first of those is what `link_check` measures.

- **Secondary, correctly scoped:** consider whether `lcat serve` should refuse to list
  index-less directories. It is a preview server by its own doc comment, so a raw
  `<pre>` listing is defensible for `lcat build && lcat serve` -- but it is also what
  is serving :8482 and :8502 right now, and a listing is how this bug announced itself.
  A `http.FileSystem` wrapper that returns `fs.ErrNotExist` for a directory with no
  `index.html` is a few lines and makes the preview match what a static host does.

## Repro

```bash
cd ~/libcat-e2e && node harness/probe_opac_taxonomy.mjs   # 3/6; FAIL: T3, T4, T5
cd ~/libcat-e2e && node harness/retest.mjs                # check t276
```

Read-only against the published site on :8482. The probe never writes anywhere and
never touches :8481 or :8501.

**Verifying a fix needs a republish.** :8482 serves a static build; editing the Hugo
module changes nothing until `lcat build` runs against the playground catalog. `t276`
reports STILL-BROKEN until then, which is correct and not a stale check.

Its controls carry the argument. `T0` shows the OPAC is up and is not `lcatd`
(`/v1/healthz` → 404). `T1` shows the classifications taxonomy exists and lists more
than one term, so `T3` is not passing on an empty list. `T2` shows a BISAC term mints a
clean single-segment page (`/classifications/fic004000/` → 200), so a slash-free value
is handled correctly and the failure is specific to the slash. `T3` is the finding: no
term key may contain `/` (1 of 51 terms does). `T4` shows the consequence a static host
will meet -- an index-less directory at the accidental parent. `T5` shows `sitemap.xml`
advertising a dot-segment path to crawlers.

`T3` asserts the *property* (no linked term contains a path separator), not the string
`813/.6`, so it will not pass vacuously if the demo corpus changes its Dewey number.
`T5` likewise tests for any dot-segment, not for `.6`. `T0` is the control that keeps me
honest about which port I am on: if `/v1/healthz` ever answers 200, I am pointed at an
admin site again, which is the mistake this whole surface went untested behind.

By hand:

```bash
curl -s localhost:8482/classifications/ | grep -oE 'href="/classifications/[^"]*"' | grep '/.*/'
# href="/classifications/813/.6/"

curl -s localhost:8482/classifications/813/
# <pre>
# <a href=".6/">.6/</a>
# </pre>

curl -s localhost:8482/sitemap.xml | grep '813'
# <loc>http://localhost:8482/classifications/813/.6/</loc>
```

## Outcome

Shipped in `00ec5a9`, released as **v0.135.0**. `probe_opac_taxonomy.mjs` goes
**3/6 -> 6/6**. Every bullet under Expected is done, and the secondary one (the
`lcat serve` listing) shipped separately as tasks/278 in v0.133.0.

### The key is a slug; the value is untouched

Classifications key on `partial "lcat-slug.html" .value`, as filed. `813/.6` →
`/classifications/813-6/`. The code stays verbatim in the record, in
`catalog.json`, in `facets.json` and on the Work detail page.

It was not a one-line change, because **three places have to agree on the key**:

1. the content adapter, which indexes it;
2. `facets-body.html`, which recomputes the key from `facets.json` rather than
   reading the adapter's param -- slug the adapter alone and `lcat-term-url` asks
   Hugo for a page that no longer exists, so every classification facet row renders
   *unlinked*, and nothing errors;
3. `lcat-classification-labels.html`, keyed by the raw code and indexed by
   `.Data.Term`. Once the term is `813-6` the lookup misses and the heading falls
   back to Hugo's humanized slug.

(3) also fixed a real gap: the map now falls back to the raw code, not to nothing,
so a term page shows `813/.6` even where the graph carries no label. The code is
the one thing that page cannot recover for itself -- its key has lost the
punctuation, and `classificationList` lives on Work pages.

### Contributors had it too, and are the reason it isn't one rule

`grep` for `/` in queerbooks' 62,602-work `catalog.json` found **14 contributors**:
`Brzuzy, Stephan/ie`, `Forry/Fino, Amanda`, `Divide, Liberal/Democratic`. Same
defect, same mechanism, bigger corpus.

But contributors are keyed on the name, and slugging 50,176 real names collapses
**616 groups** of them -- overwhelmingly punctuation variants of one person
(`Chesterton, G. K.` / `Chesterton, G.K.` / `Chesterton, G.K`), plus case variants
(`Burns, Robert` / `Burns, robert`). Merging those is authority control, and a good
idea, and not this task. It would also rewrite 49,267 of 50,176 contributor URLs.

So `lcat-cap.html` -- the partial that *is* the indexed contributor key -- replaces
`/` with `-` and nothing else. Only 14 URLs move, and they were nested pages under
an index-less parent before. `kuang-r.f.` is byte-identical. A new
`lcat-contributor-names.html` restores the name for display on the ~14 term pages
whose key can no longer spell it; it maps only those, because a 50k-entry map built
to answer fourteen questions is not a map.

Extra facets were never affected -- tasks/143 already slugs their keys.

### The guard

The adapter errors on any taxonomy key containing `/`, naming the work, the
taxonomy and the value:

```
ERROR libcat: work wexamplethree indexes the contributors taxonomy by
"Forry/Fino, Amanda", which contains a path separator -- it would mint a nested
term page under an index-less parent (tasks/276)
```

No dimension can trip it now, which is why it is asserted rather than trusted.
Mutation-checked both ways: reverting the classification slug fires it on
`813/.6`; reverting `lcat-cap`'s replacement fires it on the contributor. It would
have caught 023, 128, 134 and this one -- all four of which were found in a
published catalog instead of in a build.

`formats` and `languages` stay raw (controlled vocabularies), and that is what let
the seam test drive the guard from data rather than from a mutation.

### `link_check.cjs` cannot be taught the character

Its comment claimed `+`/`/`; it only ever checked `+`, and no version of it could
check `/`. Both halves of the report's reasoning are now in its header: the link
*resolves*, and once a key is a path the separator that does not belong is
indistinguishable from the ones that do. The check moved to where the key is still
a key.

### Tests

`hugo/classifications_seam_test.cjs`, 11 checks, added to `test:js`. The
exampleSite now carries a Dewey code and a slashed contributor name, so the fixture
exercises what the corpus does. Three mutations, each killed by exactly one check:
un-slugging the rail (`the facet rail links the slug and displays the code`),
keying the label map by the raw code (`the term page still shows the code the
cataloguer typed`), dropping the contributor-name map (`a contributor keeps every
character but the one that cannot be a path`).

`no taxonomy mints a nested term page` asserts the property across all six
dimensions rather than the instance, so a corpus that changes its Dewey number does
not empty it.

### Measured

Playground OPAC, clean rebuild (`rm -rf opac/public` -- `hugo --destination` does
not clean, and a stale `813/.6/` survives otherwise, which briefly looked like the
fix had failed):

```
/classifications/813-6/    200   <h1>Classification: 813/.6</h1>
/classifications/813/.6/   404
/classifications/813/      404
sitemap.xml                <loc>.../classifications/813-6/</loc>   (no dot-segment)
facet rail                 href="/classifications/813-6/" -> "813/.6"
```

### Adoption note

Classification term URLs change for any code carrying punctuation: `813/.6` →
`813-6`, `PS3607.A35943` → `ps3607-a35943`. A contributor's URL changes only if
their name contains a slash. Recorded in `hugo/README.md`.

## Independently verified by libcat-e2e, 2026-07-10

`t276` flipped FIXED and `probe_opac_taxonomy.mjs` went 3/6 → **6/6**, read-only on `:8482`:

```
T3  all 51 classification terms occupy a single path segment
T4  no index-less parent directories under /classifications/  (0 checked; was 1)
T5  sitemap.xml (200) advertises no dot-segment paths
GET /classifications/813/     -> 404   (the accidental parent is gone)
GET /classifications/813/.6/  -> 404
```

Note the URL change is real and unredirected: the old `/classifications/813/.6/`
now 404s rather than 301ing to `813-6`. Since `tasks/313` published the redirect
map, work ids survive a merge -- **term pages have no such map**. Probably fine for
a demo corpus, and worth a thought if a real catalog ever renames a term.
