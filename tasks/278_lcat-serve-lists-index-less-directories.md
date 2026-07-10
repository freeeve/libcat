# 278 -- lcat serve lists index-less directories

Filed from libcat-e2e on 2026-07-09 (cross-repo ask).

`serveHandler` is `http.FileServer(http.Dir(dir))`, and Go's FileServer auto-generates
a directory listing for any directory with no `index.html`. No static host does this.
So `lcat serve` disagrees with production in exactly the place a published catalog is
most likely to be wrong -- and it is not a preview toy: **tasks/181 specifies it as the
queerbooks OPAC's server** (*"`--addr :8502` exposes it ... queerbooks can drop
`scripts/opac-server.go` on adoption"*), which is how :8502 is running right now.

Three consequences, all measured on the live catalogs:

1. **`/page/` is a listing on every paginated libcat site**, and it is one URL
   truncation away from a link the home page itself emits (`/page/2/`). This one needs
   no other bug and no guessing.
2. It **hides** the class of bug it should surface. `/classifications/813/` renders a
   listing instead of the 403/404 a static host gives. That is why **tasks/276** -- a
   taxonomy term minting a nested path -- looked fine in preview.
3. It **enumerates** the search-artifact directory. `GET /search/` on :8502 lists
   **125 files, ~33 MB**, including a 9.9 MB record store and a 6.2 MB trigram index.
   Neither catalog serves a `robots.txt`.

Measured read-only against :8482 and :8502. Nothing was mutated anywhere.

## Symptom

The pagination directory, on the **playground** (:8482, 31 works, a quiet build):

```
GET /              -> links to /page/2/, /page/3/, /page/4/    (Hugo's pager)
GET /page/2/       -> 200, a styled catalog page
GET /page/         -> 200, and the body is Go's FileServer listing:

    <!doctype html>
    <meta name="viewport" content="width=device-width">
    <pre>
    <a href="1/">1/</a>
    <a href="2/">2/</a>
    <a href="3/">3/</a>
    <a href="4/">4/</a>
```

Hugo emits `/page/N/index.html` and never `/page/index.html`, so **every paginated
libcat site has this directory and none of them have an index for it.** A reader who
deletes `2/` off the end of a URL, or a crawler that walks up from a link it followed,
lands on the listing.

The artifact directories, on **queerbooks** (:8502):

```
GET /search/  -> 200, listing of 125 entries:

    <a href="browse-docs.json">browse-docs.json</a>
    <a href="browse-facets.rrsf">browse-facets.rrsf</a>
    <a href="browse-index.rrs">browse-index.rrs</a>
    <a href="browse-records.bin">browse-records.bin</a>
    ... 121 more ...

    browse-records.bin   10,424,189 bytes   (9.9 MB)
    browse-index.rrs      6,473,978 bytes   (6.2 MB)
    browse-facets.rrsf                      (5.6 MB)
    browse-docs.json                        (1.2 MB)
    ------------------------------------------------
    /search/             ~33 MB across 125 files

GET /lcat/                     -> 200, listing (the WASM reader)
GET /search/browse-index.rrs   -> Cache-Control: no-store, Accept-Ranges: bytes
```

and tasks/276's accidental parent, on the playground:

```
GET /classifications/813/      -> 200, listing (one entry, ".6/")
```

Neither catalog serves a `robots.txt` (404 on both).

```
control: directories that DO carry an index.html render the catalog normally
:8502  GET /            -> 200, 1 stylesheet, 0 <pre> blocks
:8502  GET /works/      -> 200, 1 stylesheet, 0 <pre> blocks
:8502  GET /downloads/  -> 200, 1 stylesheet, 0 <pre> blocks
        (137 index-bearing directories across the two catalogs, all styled)
```

The control is the argument: the same server renders styled catalog pages wherever an
`index.html` exists. A listing is the server choosing to list, not the build failing to
render.

`/page/`, `/search/` and `/lcat/` will never carry an `index.html` -- Hugo does not emit
one for a pager, and the other two are asset directories. Under `lcat serve` they are
listings **permanently**. No rebuild fixes them.

**A measurement caveat, recorded because it nearly went into this report as a fact.**
Queerbooks' `/page/` listed 2094 entries on one fetch and 2175 three fetches later,
growing by one link per request. Nothing pathological: a `lcat build --only hugo` was
running in `~/queerbooks-demo` while I measured, and `lcat serve` was listing a directory
mid-rebuild. The counts above are taken from the playground, which was quiet. That
`lcat serve` will happily serve and enumerate a half-written directory is worth a
thought of its own, and is not this task's claim.

## Root cause

`cmd/lcat/serve.go:38-43`, the whole handler:

```go
func serveHandler(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		files.ServeHTTP(w, r)
	})
}
```

`http.FileServer` calls its `dirList` when a request resolves to a directory and
`index.html` is absent. There is no flag for it. The standard idiom is to wrap the
`http.FileSystem` so `Open` on such a directory returns `fs.ErrNotExist`, which
FileServer renders as its own 404.

**The doc comment understates the command's role, and that is part of how this
survived.** `serve.go:10-16` scopes it to *"local previews"* and *"the whole local
loop"*. tasks/181, which specified it, does not:

> *"Binds localhost by default; `--addr :8502` exposes it. ... queerbooks can drop
> `scripts/opac-server.go` on adoption."*

A command whose doc comment says "preview" and whose task says "production OPAC server"
gets reviewed as the first and deployed as the second. Both demo catalogs run it, and
:8502 uses the exact `--addr` form 181 names.

## Why it matters

**Any reader can reach it, on any libcat site.** `/page/2/` is on the home page of every
paginated catalog. Deleting the last segment is what people do to a URL, and walking up
from a followed link is what crawlers do. There is no obscurity to rely on here and no
prerequisite bug: pagination is the default.

**A preview that disagrees with production is worse than no preview.** The one job of
`lcat build && lcat serve` is to show what the published site will be. Every static host
libcat could plausibly publish to -- S3 website endpoints, CloudFront, nginx (whose
`autoindex` defaults to off), GitHub Pages, Netlify -- answers an index-less directory
with 403 or 404. `lcat serve` answers 200 and a listing. A URL that will be dead on the
host looks alive in preview, and the developer sees a page.

That is not hypothetical. **tasks/276 is the demonstration.** A Dewey number `813/.6`
mints `/classifications/813/.6/`, leaving `/classifications/813/` with no index. On a
real host that is a broken ancestor sitting in the catalog's own URL space. Under `lcat
serve` it is a cheerful 200. The bug reached a published catalog and sat there, and the
local loop reported success the whole time. Fixing 278 makes 276-shaped defects visible
the moment they are built -- which is worth more than fixing 276 itself.

**What a patron sees.** A blank white page: no header, no stylesheet, no search box, no
way back -- a `<pre>` block of blue links in the top-left corner. It does not look like
a broken page. It looks like a hacked one. (`shots/opac-dirlist.png` in libcat-e2e.)

**The artifact directory is enumerated, not secret.** Everything under `/search/` is
fetched by `lcat-browse.js` anyway, so this leaks nothing and I am **not** calling it a
vulnerability. The harm is operational: 32.8 MB of binary artifacts, each a live
`<a href>`, on a catalog with no `robots.txt`. A crawler that finds `/search/` walks all
125 links and pulls the lot -- repeatedly, because the handler sets `Cache-Control:
no-store` on *everything*, which is right for a rebuild loop and wrong for a 9.9 MB
record store on a public site. Nothing in `sitemap.xml` points at `/search/`; the
listing is the only thing that puts it in front of a crawler.

**It is a few lines to fix**, and the fix is the standard Go idiom.

## Expected

- **Do not list directories.** Wrap the filesystem so a directory without `index.html`
  is `fs.ErrNotExist`, and let FileServer render its own 404. Range support, `no-store`
  and the instant start are untouched:

  ```go
  // noDirList makes an index-less directory a 404, as every static host does.
  // A preview that lists them hides exactly the URL-shape bugs it exists to
  // surface (tasks/276).
  type noDirList struct{ fs http.FileSystem }

  func (n noDirList) Open(name string) (http.File, error) {
  	f, err := n.fs.Open(name)
  	if err != nil {
  		return nil, err
  	}
  	info, err := f.Stat()
  	if err != nil {
  		f.Close()
  		return nil, err
  	}
  	if info.IsDir() {
  		index, err := n.fs.Open(path.Join(name, "index.html"))
  		if err != nil {
  			f.Close()
  			return nil, fs.ErrNotExist
  		}
  		index.Close()
  	}
  	return f, nil
  }
  ```

  The unit test 181 already has (206 + exact bytes + `Content-Range` + `no-store` +
  plain-GET 200) extends naturally: a directory with an `index.html` serves 200; one
  without answers 404 and no listing. Assert the *absence of a listing*, not just the
  status -- a 404 body that still enumerates would pass a status-only check.

- **Fix `serve.go`'s doc comment to say what 181 says it is.** It is the OPAC's server,
  not only a preview. That one sentence is why a directory listing read as acceptable.

- **Reconsider `Cache-Control: no-store` for a served catalog**, or put it behind a
  flag. It is exactly right for `lcat build && lcat serve` and exactly wrong for a
  public site re-fetching a 9.9 MB record store on every navigation. A `--dev` flag
  that enables `no-store`, with a cacheable default, lets one binary do both jobs
  honestly. *Read, not measured*: I have not profiled a real browse session, and the
  WASM reader's Range requests may make this cheaper than it looks.

- **Separately, consider `enableRobotsTXT` in the Hugo module**, or a `robots.txt` in
  `static/`. Both catalogs 404 it today. Not this task's bug -- a note, because crawler
  policy is the other half of the artifact-enumeration story and it lives in the build,
  not the server.

## Repro

```bash
cd ~/libcat-e2e && node harness/probe_opac_dirlist.mjs   # D2
cd ~/libcat-e2e && node harness/retest.mjs               # check t278
```

Read-only against the published catalogs. The probe never writes anywhere and never
touches :8481 or :8501.

Its controls carry the argument. `D0` shows the target is the published site and not
`lcatd` (`/v1/healthz` → 404). `D1` is the decisive one: directories that **do** carry
an `index.html` -- `/`, `/works/`, `/downloads/` -- render styled catalog pages with a
stylesheet and no `<pre>` block, so a listing elsewhere is the server's choice and not a
broken build. `D2` walks the catalog's directories and flags every one answering with
Go's `dirList` shape. `D3` records that `robots.txt` is absent, which is what turns an
enumerable directory into a crawled one.

`D2` asserts a *property* -- no reader-reachable path renders a `<pre>`-of-links with no
stylesheet -- rather than naming `/search/`, so it will not pass vacuously when the
artifact directory is renamed. It is also why one check covers `/page/`, `/lcat/`,
`/search/` and 276's accidental parent across both catalogs: they are one defect seen
four times. It sweeps by walking every ancestor of every directory the home page links
to, so `/page/` is found the way a crawler finds it.

Run it against the **playground** for stable counts. Queerbooks' `/page/` grows while a
`lcat build` is running.

By hand:

```bash
# the one any reader reaches, on the quiet catalog
curl -s localhost:8482/ | grep -o 'href="/page/2/"'      # the home page links it
curl -s localhost:8482/page/ | head -4
# <!doctype html>
# <meta name="viewport" content="width=device-width">
# <pre>
# <a href="1/">1/</a>

curl -s localhost:8502/search/ | grep -c 'href='                             # 125
curl -sI localhost:8502/search/browse-records.bin | grep -i content-length    # 10424189
curl -sI localhost:8502/search/browse-index.rrs   | grep -i cache-control     # no-store
curl -s -o /dev/null -w '%{http_code}\n' localhost:8502/robots.txt            # 404

# the control: an index-bearing directory is a real page
curl -s localhost:8502/works/ | grep -c 'rel="stylesheet"'                    # 1
curl -s localhost:8502/works/ | grep -c '<pre>'                              # 0
```
