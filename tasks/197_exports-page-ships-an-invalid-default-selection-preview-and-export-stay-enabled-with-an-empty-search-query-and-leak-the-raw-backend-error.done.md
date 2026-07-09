# 197 -- exports page ships an invalid default selection: preview and export stay enabled with an empty search query and leak the raw backend error

Filed from libcat-e2e on 2026-07-09 (cross-repo ask).

## Symptom

Land on `#/exports` and click **Preview** (or **Export**) without touching
anything. Both buttons are enabled; both fail:

```
400 POST /v1/batch/resolve  {"selection":{"kind":"search","query":""}}
    {"error":"batch: invalid request: search selection needs a query"}
400 POST /v1/exports        {"format":"csv","batchSelection":{"kind":"search","query":""}}
    {"error":"batch: invalid request: search selection needs a query"}
```

The page then renders the backend's internal error string verbatim:

> Projected rows: id, title, contributors, subjects, and friends. **Export
> batch: invalid request: search selection needs a query** Jobs Created …

Three problems compounded:

1. `#ex-kind` defaults to `search` while the query box is empty -- the page's
   initial state is invalid by construction.
2. **Preview** and **Export** are `disabled: false` in that invalid state, so
   the first thing a cataloger does on the page is hit an error.
3. The error surfaced is the raw service string, `batch:` package prefix and
   all. "batch: invalid request:" means nothing to a cataloger, and the word
   *batch* on the *Exports* page actively misleads.

Exports themselves are fine -- the fault is entirely the default state:

| selection | result |
|---|---|
| `search` + empty query (default) | **400** |
| `search` + `frog` | 202, `status:DONE`, `records:2` |
| `all` (Entire catalog) | 202, `status:QUEUED` |
| download via `job.downloadUrl` | 200, well-formed CSV |

## Expected

Pick one:

- default `#ex-kind` to `all` (Entire catalog) -- a valid, obvious, safe default
  that matches "Export these results…" arriving with no query; **or**
- keep `search` as the default but disable Preview/Export while the query is
  empty, with a hint ("enter a search, or choose Entire catalog").

Either way, never render a raw `batch: invalid request: …` string to a
cataloger. Map service validation errors to human copy at the UI boundary; the
`batch:` prefix in particular should never escape the package.

## Notes

- The same raw-error leak is reachable from `#/batch` (shared `writeBatchError`
  prefix), so fixing it at the UI error boundary covers both pages.
- `GET /v1/exports/{id}/download` authorizes on the `?token=` capability carried
  in `job.downloadUrl`, not the bearer -- by design
  (`backend/httpapi/export_handlers.go:145`). Noted so it is not mistaken for a
  bug: a bearer-only request correctly 403s.

## Repro

```sh
# libcat-e2e
node ui/probe_exports.mjs      # (a) default -> 400, (b) query -> 202, (c) all -> 202
node ui/capture.mjs /exports "Preview" "Export"
```

## Outcome

Fixed in 0d3fb42-equivalent (see git log: fix(ui) tasks/197), released
v0.49.0. Took BOTH of your suggested options plus the error boundary:

- Default selection is now `all` (Entire catalog) -- valid by
  construction; deep links (kind=search&q=…, ids, savedQuery) keep
  their kind.
- Preview/Export disable while the selection is incomplete (empty
  search query, no ids, no saved query picked) with your suggested
  inline hint ("enter a search, or choose Entire catalog").
- humanApiMessage() maps service strings at the UI boundary: strips
  the package prefix + "invalid request:" and capitalizes; wired into
  Exports AND BatchOps (your note that #/batch leaks the same strings).

Verified against the rebuilt playground with your probe: default state
now resolves {"kind":"all"} -> 200 matched=31 and exports -> 202 (no
400s), and Playwright confirms kind=all default, Export enabled,
Preview disabled + hint on an empty search.
