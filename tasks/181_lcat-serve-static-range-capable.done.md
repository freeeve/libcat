# 181: lcat serve -- Range-capable static file server for built sites

## Done (2026-07-08)

`lcat serve [--dir public] [--addr 127.0.0.1:8500]` -- net/http FileServer
(Range/206 native) with Cache-Control: no-store so a rebuild is visible on
reload instead of the browser replaying yesterday's index artifacts. Binds
localhost by default; `--addr :8502` exposes it. No SPA fallback, as
specified. Serves an already-built public/ instantly -- no render step.
README's Range note now points at it, and queerbooks can drop
scripts/opac-server.go on adoption.

Verified: unit test pins 206 + exact bytes + Content-Range + no-store and
the plain-GET 200; live check against a built exampleSite returned 206 with
exactly the requested bytes.

---

Left by the queerbooks-demo session 2026-07-08 (uncommitted cross-repo ask).

Adopters preview the hugo output locally, and the roaringrange WASM reader
fetches index artifacts with HTTP Range requests -- python http.server
(everyone's default) ignores Range and silently breaks client browse, which
cost us a debugging session once already. queerbooks-demo carries a 60-line
scripts/opac-server.go for exactly this; Eve now wants zero Go in adopter
repos, and every lcat-index adopter will hit the same wall.

Ask: `lcat serve [--dir site/public] [--addr :8502]` -- net/http
FileServer/ServeContent already does Range; it is a tiny subcommand and it
makes `lcat build && lcat serve` the whole local loop. A --spa fallback flag
is NOT needed (the hugo output is plain static pages).

UPDATE (same day, tested): `hugo server` DOES serve correct 206 partial
content -- but it always re-renders on start (~18 min at queerbooks' 125k
pages; a catalog.json touch invalidates everything), and Eve confirmed
that delay rules it out as the daily preview. So the ask STANDS: serving
an already-built public/ instantly is the case hugo cannot cover. hugo
server remains fine for theme-iteration workflows that rebuild anyway.
