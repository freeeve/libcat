# 290 -- every non-works section renders as an empty Works browse list so curated lists and prose sections ship as 0 works pages

Filed from libcat-e2e on 2026-07-10 (cross-repo ask).

`hugo/layouts/list.html` is the module's only list layout. Hugo uses it for **every**
section page: `/works/`, the home page, `/lists/` (the curated views of tasks/160), and any
prose section an adopter adds. It hardcodes the Works browse shell -- the works-only page
filter, the `N works` count, `<ol id="lcat-results" data-lcat-browse>`, the browse facet
host, and the facet sidebar.

So `/lists/` -- the section index of libcat's own curated-views feature, shipped in
`exampleSite` and named in the README as "the runnable reference" -- renders as:

```html
<h1>Lists</h1>
<p class="lcat-resultcount" role="status">0 works</p>
<ol class="lcat-results" id="lcat-results" data-lcat-browse="/search"></ol>
```

A published, canonical, sitemapped page that announces zero works, lists nothing, carries a
facet rail, and links to none of the curated pages beneath it.

Under `engine = "roaringrange"` it is worse than empty. `lcat-browse.js` binds to
`#lcat-results` on whatever page it finds one, so **typing a query on `/lists/` turns it into
a second catalog search page** -- work cards and a live facet rail, under the heading "Lists".

The irony is in the layout's own comment. `list.html:7-8`:

> *"Browse lists show Works only: a site's other regular pages (curated lists (tasks/160),
> prose pages) must not count or render as work cards."*

That guard is correct and it points the wrong way. It keeps curated lists out of `/works/`;
nothing keeps `/works/` out of `/lists/`.

## Symptom

Built `hugo/exampleSite` unchanged (`hugo`, `baseURL = https://example.org/`), then again
with `engine = "roaringrange"` plus real browse artifacts from the repo's own
`hugo/e2e/fixture-catalog.json` (`lcat index --catalog … --out public/search`), served over
`hugo/e2e/range-server.mjs`, and driven in headless Chromium.

**Cold, and what the crawler sees:**

```
/works/            <h1>Works</h1>   "3 works"   3 cards
/lists/            <h1>Lists</h1>   "0 works"   0 cards      <- sitemapped, canonical
/lists/staff-picks/  <h1>Staff picks</h1>  "2 works"  wexampletwo -> wexampleone
```

The curated page itself is **correct**: two works, in the front-matter order, rendered
through the shared `work-card.html`. `curated.html` is not the problem, and its `<ol
class="lcat-results">` deliberately carries no `id`, which is why browse leaves it alone.

**Hot -- after typing `snow` into the header search box, on each page:**

```
/works/   "1 results"  1 card   "Snow Country / Kawabata, Yasunari"   facet rail: 2 rows
/lists/   "1 results"  1 card   "Snow Country / Kawabata, Yasunari"   facet rail: 2 rows   <- under <h1>Lists</h1>
/lists/staff-picks/   "2 works"  unchanged, facet rail: 0 rows                             <- correctly immune
```

`/lists/` is indistinguishable from `/works/` once the reader boots. The only difference is
the `<h1>`, which still says "Lists", and the cold state a crawler indexes, which says zero.

**It is not specific to `lists`.** Adding a plain prose section -- `content/help/borrowing.md`,
a page with no `works` front matter at all:

```
/help/     <h1>Helps</h1>
           <p class="lcat-resultcount" role="status">0 works</p>
           <div id="lcat-browse-facets" hidden></div>
           <ol class="lcat-results" id="lcat-results" data-lcat-browse="/search">
           links to /help/borrowing/ : 0
           in sitemap: yes
```

Every section an adopter adds -- Help, About, News -- becomes an empty Works browse whose own
child pages it refuses to list, because `list.html` filters them out by design.

## Root cause

`hugo/layouts/list.html:5-20`:

```go-html-template
{{ define "main" }}
<h1>{{ if .IsHome }}{{ site.Title }}{{ else }}{{ .Title }}{{ end }}</h1>
{{- /* Browse lists show Works only: a site's other regular pages (curated
       lists (tasks/160), prose pages) must not count or render as work cards. */ -}}
{{ $pages := where .RegularPagesRecursive "Section" "works" }}
{{ $paginator := .Paginate $pages }}
<p class="lcat-resultcount" role="status">{{ i18n "workCount" (len $pages) }}</p>
...
<ol class="lcat-results" id="lcat-results"{{ if eq (site.Params.search.engine | default "") "roaringrange" }} data-lcat-browse="…"{{ end }}>
```

On `/lists/`, `.RegularPagesRecursive` is that section's own pages, every one of which has
`Section == "lists"`, so `$pages` is empty. The count renders `i18n "workCount" 0` -- "0
works", a category error on a page that was never about works -- and the results list renders
nothing.

`ls hugo/layouts/` is `baseof.html curated.html list.html page.html taxonomy.html term.html`.
There is no `works/list.html` and no `home.html`, so `list.html` is simultaneously the home
layout, the Works browse layout, and the layout for every other section. It can only be right
for two of those three.

`hugo/layouts/_partials/…` and `list.html:1-3` also attach the facet sidebar
(`partialCached "facets.html"`) to all of them.

And `lcat-browse.js:84` binds by id alone, not by the attribute:

```js
function start() {
  const results = document.getElementById("lcat-results");
  const form = document.querySelector(".lcat-search");
  if (!results || !form) return;
  const base = (results.getAttribute("data-lcat-browse") || "/search").replace(/\/+$/, "");
```

so any page `list.html` renders becomes a browse surface. That is correct behaviour given the
markup; the markup is what is wrong.

## Why it matters

**The curated-views feature ships a broken front door.** tasks/160 exists so a library can
freeze editorially important collections into crawlable HTML. The page that should index them
says "0 works" and links to none of them. `exampleSite/content/lists/staff-picks.md` is
reachable only from `sitemap.xml`.

**It is in the sitemap, with a canonical URL.** `https://example.org/lists/` and
`https://example.org/help/` are submitted to search engines as zero-result catalog pages.
This is a thin-content page the module generates on the adopter's behalf.

**Under roaringrange, `/lists/` is a duplicate catalog.** A reader who lands there from search
and types a query gets the whole catalog under a heading that says "Lists". Two URLs, one
search surface, no canonical relationship between them.

**Every adopter hits it the first time they add an About page.** Nothing warns; the build is
clean. The failure is a page that looks deliberate.

**Nothing in the repo can see it.** `hugo/e2e/run.sh` drives `/works/` only, through
`browse.spec.mjs` and `browse-minimal.spec.mjs` -- five checks, all on the works browse.
`link_check.cjs` resolves `href`s and finds no broken link, because "linked from nowhere" is
not a broken link. The `/lists/` page it generates is valid HTML, returns 200, and is wrong.

## Expected

- **Give the Works browse its own layout.** Hugo's lookup already supports it: move the
  current body to `layouts/works/list.html` (section `works`) and add `layouts/home.html` for
  the home page, which today also depends on `list.html`'s browse shell (measured: home
  renders `3 works` and `data-lcat-browse`). Then `layouts/list.html` becomes a plain section
  index.

- **Make the generic `list.html` list the section's own pages.** Title, `{{ .Content }}`, and
  `range .Pages` with a link and summary per page. No `workCount`, no `id="lcat-results"`, no
  `data-lcat-browse`, no browse facet host, no facet sidebar. `/lists/` then indexes the
  curated views, which is the page's whole reason to exist.

- **Do not attach the facet sidebar to non-works sections.** `list.html:1-3` defines
  `sidebar` unconditionally.

- **Extend `hugo/e2e` to cover a non-works section.** `run.sh` builds the exampleSite with
  roaringrange and real artifacts already; the site already has `content/lists/`. One
  assertion -- `/lists/` lists its curated pages and does not hydrate into a browse surface --
  would have caught this. The existing `browse.spec.mjs` only ever visits `/works/`.

- Consider whether `curated.html` pages should be linked from anywhere by default, or say
  plainly in the README that the adopter must add a `[[menu.main]]` entry. Today
  `exampleSite`'s menu has Works and Subjects, and `staff-picks` is orphaned.

## Repro

```bash
cd ~/libcat-e2e && node harness/probe_opac_lists.mjs   # L2, L3, L4, L5
cd ~/libcat-e2e && node harness/retest.mjs             # check t290
```

The probe copies `hugo/exampleSite` to a scratch directory, adds a plain prose section, builds
it with `hugo` against the working tree's module, emits browse artifacts from the repo's own
`hugo/e2e/fixture-catalog.json` with `lcat index`, serves the result over a Range-capable
server, and drives it in headless Chromium. It never writes inside `~/libcat` and touches no
running site.

Its controls carry the argument. `L0` shows the build produced all three surfaces. **`L1` shows
`/works/` renders `3 works`, hydrates on a query, and yields "Snow Country"** -- so the browse
reader booted and the artifacts are good, and `/lists/` hydrating is not "the reader failed to
start". **`L6` shows `/lists/staff-picks/` renders its two works in front-matter order and does
*not* hydrate** -- so `curated.html` is exonerated and the defect is isolated to the section
index.

By hand:

```bash
cd ~/libcat/hugo/exampleSite && hugo --quiet --destination /tmp/ex
sed -n '/<main/,/<\/main>/p' /tmp/ex/lists/index.html      # <h1>Lists</h1> … "0 works" … empty <ol>
grep -c 'href="/lists/staff-picks/"' /tmp/ex/lists/index.html   # 0
grep -o '<loc>[^<]*lists/</loc>' /tmp/ex/en/sitemap.xml    # it is in the sitemap
```
