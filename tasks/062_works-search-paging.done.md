# 062 -- Works search offset paging and matched count

## Context

Phase 4 of the admin UX overhaul. `GET /v1/works` truncated at `limit` (default 50, max
500) with no way to page, and `total` reported the whole catalog rather than the query's
hits -- past 50 matches the rest were unreachable from the UI.

## Scope

- Backend: `works_list_handler.go` accepts `offset`, counts every match before windowing,
  and responds `{works, total, matched, offset}` (`total` keeps its old meaning, so no
  client break). Covered by `works_list_handler_test.go` (windows, past-the-end, counts).
- UI: `fetchWorks(q, limit, offset)`; status line "N of M matched · T in catalog";
  Load more button + `m` key append the next window (id-deduped against the 30s summary
  cache refreshing between windows); appended pages persist in screenState across
  drill-ins.

## Known quirk

Offsets across a summary-cache refresh (30s TTL) can skip or duplicate one boundary row
when the catalog changed in between; the id dedup absorbs the duplicate case and a skip
self-heals on the next full search. Acceptable for an admin tool.

## Acceptance

- `go test ./httpapi/` green (backend module); gofmt -s clean.
- `npm run check` / `test` / `build` green; `m` appends and selection stays put.
