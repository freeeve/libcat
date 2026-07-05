# 104 -- awslambda: adapter double-encodes percent-escaped path segments

> Filed from the 2026-07-05 full-code review.

## Symptom

Requests with percent-escaped path segments behave differently under Lambda than
on the container. `PUT /v1/users/eve%40example.org/roles` works on the container
but under Lambda operates on the literal key `eve%40example.org` -- role changes
and deletes silently target a non-existent user; same for any escaped `{id}`
authority route.

## Cause

`httpRequest` (backend/awslambda/httpadapter.go:44-45) builds
`url.URL{Path: ev.RawPath, RawQuery: ev.RawQueryString}` and calls `u.String()`.
`url.URL.Path` holds the *decoded* form, so a literal `%` in `ev.RawPath` is
re-escaped: `/v1/users/eve%40example.org` becomes `/v1/users/eve%2540example.org`.
After `http.NewRequestWithContext` re-parses it, `r.PathValue("email")` yields
`eve%40example.org` instead of `eve@example.org`. The standalone server sets
`URL.RawPath` correctly, so this breaks the container/Lambda parity this package
exists to provide (`httpapi/auth_handlers.go:97,108` are the concrete victims).

## Fix sketch

Preserve the raw request target instead of round-tripping through `url.URL.Path`
-- e.g. parse `ev.RawPath + "?" + ev.RawQueryString` as a request URI, or set
both `Path` (decoded) and `RawPath` on the URL.

## Acceptance

- An adapter test asserts a `%`-escaped path segment reaches `r.PathValue`
  decoded exactly once, matching the standalone server's behavior.

## Resolved

`httpRequest` now hands `http.NewRequestWithContext` the raw target string
(`ev.RawPath` + optional `?` + `ev.RawQueryString`, defaulting to `/` when
empty) instead of round-tripping through `url.URL{Path: ...}.String()`, so the
standard parser sets both `Path` (decoded) and `RawPath` exactly as the
standalone server does. `TestEscapedPathParity` routes
`PUT /v1/users/eve%40example.org/roles` through a mux and asserts
`PathValue("email") == "eve@example.org"`.
