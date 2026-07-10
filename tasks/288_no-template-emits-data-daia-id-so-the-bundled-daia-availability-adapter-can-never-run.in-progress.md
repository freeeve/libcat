# 288 -- no template emits data-daia-id so the bundled DAIA availability adapter can never run

Filed from libcat-e2e on 2026-07-10 (cross-repo ask).

`lcat-availability.js` registers two adapters (`:276-288`):

```js
registerAdapter({ providerKey: "overdrive", domAttr: "data-overdrive-reserve", … });
registerAdapter({ providerKey: "daia",      domAttr: "data-daia-id",           … });
```

`collect()` (`:480`) finds each adapter's editions with
`doc.querySelectorAll("[" + adapter.domAttr + "]")` (`:484`), and `init()` (`:499`) only
creates a job for a provider that `collect()` found elements for.

```
$ grep -rn 'daia' hugo/layouts/ hugo/content/
$        # nothing
```

**No libcat template emits `data-daia-id`.** The DAIA adapter -- ~120 lines,
`daiaItemStatus` / `daiaItemLocation` / `normalizeDaia` / `daiaRequest` / `fetchDaiaBatch`,
six unit tests, a README section, and a row in `docs/availability-providers.md` -- cannot
fire on any page libcat builds.

Meanwhile `hugo/README.md:355` says, as a statement of fact about this module:

> *"Editions carry `data-daia-id` (the DAIA document id)."*

They do not.

## Symptom

Built `hugo/exampleSite` with `[params.availability]` enabling both adapters, added a DAIA
identifier to one Work's instance, served it, and stubbed the DAIA endpoint with a Playwright
route:

```
catalog:  wexampletwo.instances[0].providerIds += { "source": "daia", "value": "ppn:e2e-daia-1" }
config:   [params.availability.daia]  baseUrl = "http://127.0.0.1:8477/daia"

A7  the Work page renders 1 edition(s), 0 carrying data-daia-id,
    and the DAIA adapter issued 0 request(s)
```

Zero requests. `collect()` found nothing to ask about, so `init()` created no DAIA job. The
stub endpoint was up the whole time and was never contacted.

The same run shows the machinery is otherwise healthy, which is what makes the zero
attributable:

```
A2  overdrive, direct transport, stubbed Thunder
      data-status="available"  text="Available now"   (1 Thunder request)
```

Same page, same `collect()`, same `renderInto()`. The only difference is the attribute.

## Root cause

`hugo/layouts/page.html:86` is the only place an edition's identifiers reach the DOM:

```go-html-template
<li class="lcat-edition" data-instance="{{ .id }}"
    {{- with .format }} data-format="{{ . }}"{{ end }}
    {{- range .providerIds }}{{ if eq .source "overdrive-reserve" }} data-overdrive-reserve="{{ .value }}"{{ end }}{{ end }}>
```

One hardcoded scheme, one hardcoded attribute. A `providerIds` entry with any other
`source` is dropped on the floor.

The projector is not the blocker. `instances()` (`project.go:1087`) passes **every**
non-ISBN identifier through into `providerIds` with its `bf:source` scheme:

```go
pids = append(pids, ProviderID{Source: p.identifierSource(id), Value: v})
```

`availabilitySources` (`:1067`, `{"overdrive-reserve": true}`) gates only the `held` flag
(`:1116`, tasks/078), not the projection. And `ProviderID`'s own doc comment (`:261-263`)
states the intent plainly:

> *"so a client-side availability adapter selects its key by scheme (e.g. OverDrive's
> `overdrive-reserve` Reserve ID vs the `overdrive` title id) rather than guessing from a flat
> list"*

The data model is right, the reader is right, and the template in between knows about
exactly one provider.

**There is also no defined `bf:source` scheme for a DAIA document id.** Nothing in libcat
names one, so an adopter with a Koha or GBV endpoint has no way to say "this identifier is
the DAIA id" even if the template were fixed. That is the second half of the fix, and it is
why this is not a one-line change.

## Why it matters

**It is a shipped, documented, tested feature that cannot be switched on.** `README.md:340-364`
sells DAIA as the proof of the digital/physical superset -- *"it populates `locations[]`
(per-branch shelf location, call number, status, and due date) that the digital adapters
leave empty"* -- and `docs/availability-providers.md:51` calls it the adapter that "proves the
superset". A library with physical holdings configures it exactly as documented, deploys, and
gets nothing: no request, no error, no rendered status.

**Physical holdings are most libraries.** The one bundled adapter that works, OverDrive, is
digital-only. The catalog's answer to "is this book on the shelf?" is code that runs in no
browser.

**The unit tests certify the unreachable half.** Six of the 23 tests in
`availability_test.cjs` exercise DAIA -- `daiaItemStatus`, `normalizeDaia`, `daiaRequest`,
`resolve(daia)` twice, and `statusText: physical holding shows shelf location and due date`.
All pass. All call the pure core directly. None can observe that `collect()` will never hand
it an id, because `init()` and `collect()` are called by no test in the repo, and no
`hugo/e2e/*.spec.mjs` mentions availability.

## Expected

- **Define the `bf:source` scheme for a DAIA document id** (e.g. `daia`, or `daia-ppn`), and
  say so where `ProviderID` documents scheme selection (`project.go:261-263`) and in
  `hugo/README.md`.

- **Emit the attribute from the scheme, generically.** `page.html:86` should map schemes to
  adapter attributes rather than hardcoding one, so registering an adapter and projecting its
  identifier is enough:

  ```go-html-template
  {{- range .providerIds }}
    {{- if eq .source "overdrive-reserve" }} data-overdrive-reserve="{{ .value }}"
    {{- else if eq .source "daia" }} data-daia-id="{{ .value }}"{{ end }}
  {{- end }}
  ```

  A table in `site.Params` or a module `data/` file would let a third adapter arrive without
  touching the layout at all -- which is what `registerAdapter` already promises on the JS
  side.

- **Fix `README.md:355`.** *"Editions carry `data-daia-id`"* is false today. Either make it
  true or mark the adapter as requiring a theme override -- because an adopter *can* reach it
  by shadowing `page.html`, and nothing says so.

- **Give `exampleSite` a DAIA identifier**, the way tasks/285 concluded the cover slot needed
  a cover. Its catalog has three works, all OverDrive, all digital. The physical path has no
  fixture anywhere in the module.

- Note that even once the attribute is emitted, DAIA still cannot fetch: `daiaRequest`
  requires `cfg.baseUrl` (direct) or `cfg.proxyUrl` (proxied), and Hugo lowercases both out
  of existence (**tasks/287**). The two must land together for DAIA to work at all.

## Repro

```bash
cd ~/libcat-e2e && node harness/probe_opac_availability.mjs   # A7 (A3, A4, A5 = tasks/287)
cd ~/libcat-e2e && node harness/retest.mjs                    # check t288
```

The probe builds `hugo/exampleSite` in a scratch directory with a DAIA identifier injected
and both adapters configured, serves it over http, and routes the DAIA endpoint so a request
would be recorded if one were made. It never writes inside `~/libcat` and touches no running
site.

`A2` is the control that gives `A7` its meaning: on the same build, through the same
`collect()` and `renderInto()`, the OverDrive edition renders `data-status="available"` from
a stubbed Thunder answer. The DOM wiring works. Only DAIA is unreachable.

By hand:

```bash
grep -rn 'daia' ~/libcat/hugo/layouts/    # no output
sed -n '355p' ~/libcat/hugo/README.md     # "Editions carry `data-daia-id` (the DAIA document id)."
sed -n '283,288p' ~/libcat/hugo/assets/lcat-availability.js   # domAttr: "data-daia-id"
```
