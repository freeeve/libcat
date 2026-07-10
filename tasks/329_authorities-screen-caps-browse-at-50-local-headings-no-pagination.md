# 329 -- the Authorities screen browses at most 50 local headings, with a bare count and no pagination

Filed from libcat-e2e on 2026-07-10 (cross-repo ask).

The sibling of 328 on the Authorities screen: a bare "N terms" count that is really
a fetch limit, on the one surface that manages the deployment's own headings. Latent
on the shipped corpora (queerbooks has 0 local authorities, the playground 9) but a
real defect for any library that mints more than 50 local headings.

## Symptom

`Authorities.svelte` is "the one place the deployment manages what it owns" -- the
local-scheme headings a library creates. On an empty query (the default landing
state) it fetches once and renders a bare count:

```
const page = await fetchAuthorities(query);     // Authorities.svelte:61
{st.terms.length} term{s}                        // Authorities.svelte:116
```

`fetchAuthorities(q, limit = 50)` (`api.ts:628`) sends `limit=50`, and the screen
has no cursor state and no "Load more". `GET /v1/authorities` with an empty `q`
lists every local term, drops labelless debris, then truncates:

```go
if q == "" && len(terms) > limit { terms = terms[:limit] }   // authorities_handlers.go:72-73
writeJSON(w, ..., map[string]any{"terms": terms})            // no total, no cursor
```

So a deployment with more than 50 local headings sees exactly 50, the screen reads
**"50 terms"** (a wrong count -- it is the fetch limit, not the total), and headings
51+ are unreachable by browsing: the only way to reach them is to already know a
label and search for it.

Measured on a throwaway `fromHead` clone (`node harness/probe_authorities_list.mjs`,
2026-07-10):

| step | result |
|---|---|
| S0 control | the clone ships **9** local authority headings |
| mint | created **51** more (`POST /v1/authorities`, librarian) → 60 total |
| S1 control | `GET /v1/authorities?q=&limit=200` returns **60** (all stored) |
| **P1** | `GET /v1/authorities?q=&limit=50` (the screen's exact call) returns **50 of 60**, body has **no `total` and no `cursor`** |

## Root cause

Two layers, neither of which supports listing a large local scheme:

1. **The endpoint has no pagination.** `authorities_handlers.go:47-79` truncates the
   empty-query result to `limit` (`terms[:limit]`) and returns `{terms}` with no
   total and no cursor. The `limit` param is honoured but hard-capped at 200
   (`:52`, `n <= 200`), so there is no way to page past 200 either, and no signal
   that anything was dropped. (Confirmed against :8501: the response body's only key
   is `terms`.)

2. **The client asks for 50 and shows the result length as the count.**
   `fetchAuthorities` defaults `limit=50`; `Authorities.svelte` passes no override,
   keeps no cursor, offers no "Load more", and prints `st.terms.length`.

Contrast the Review Queue screen (`Queue.svelte`), which reads the *same* kind of
capped endpoint correctly: it threads `st.cursor`, appends pages on a "Load more"
button (`:329-330`), and labels the count `"{n} suggestions (more available)"`
(`:264`). The Authorities screen does none of this.

## Why it matters

Local authorities are exactly the headings a library does original work to create,
and this screen is the only place to see them. A cataloger who has built 60 local
subject or name headings is shown 50, told there are "50 terms", and given no way to
reach the other 10 except by guessing a label to search. The displayed count is also
simply wrong. It is the same shape as 328 (a page size presented as a total) and as
115 / 261 / 313 (a number the UI presents as authoritative is bounded by an
implementation detail the reader cannot see).

Latency, stated honestly: no shipped instance triggers it today (queerbooks 0 local
headings, playground 9), so this is a correctness/scalability defect waiting for a
deployment that uses local authorities heavily, not a live incident.

## Expected

The Authorities browse should either page or report a true total, like the queue:

- give `GET /v1/authorities` a cursor (and/or a `total`) for the empty-query list,
  and have `Authorities.svelte` thread it with a "Load more" and a
  "N terms (more available)" label; or
- at minimum, when the returned page is full, show "50+ terms" rather than a bare
  "50 terms", so the count never claims to be exact when it is not.

## Repro

```
node harness/probe_authorities_list.mjs   # 3/4: mints 51 on a clone, shows limit=50 returns 50 of 60
node harness/retest.mjs                    # check t329 (STILL-BROKEN)
```

Both run on a throwaway `fromHead` clone: create 51 local authority headings
(`POST /v1/authorities {prefLabel:{en}}`, a librarian CRUD write -- no challenge, no
rate limit), confirm `limit=200` returns them all, then issue the screen's
`limit=50` call and confirm it returns exactly 50 with no total or cursor. The clone
is discarded; nothing touches :8481.
