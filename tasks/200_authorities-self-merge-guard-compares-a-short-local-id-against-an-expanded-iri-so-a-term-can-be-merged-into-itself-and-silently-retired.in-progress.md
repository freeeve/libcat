# 200 -- authorities self-merge guard compares a short local id against an expanded IRI so a term can be merged into itself and silently retired

Filed from libcat-e2e on 2026-07-09 (cross-repo ask).

## Symptom

`POST /v1/authorities/merge` accepts a term as its own winner. One request
retires a live local heading: it gains a merge marker pointing at **itself**, and
it disappears from `GET /v1/authorities`. There is no delete route, so the
heading cannot be recovered through the API.

Reproduced twice on the 8481 playground:

```
run 1: create=201 merge=200 mergedInto="a5fth0j8r3knhm" selfPointer=true stillListed=false
run 2: create=201 merge=200 mergedInto="aihlkesvoc98qk" selfPointer=true stillListed=false
```

## Root cause

`backend/authoritiesvc/service.go:171`

```go
func (s *Service) Merge(ctx context.Context, loserID string, winner vocab.TermRef, actor string) (MergeResult, error) {
	loserURI := bibframe.LocalAuthorityIRI(loserID)          // expands to https://…/authority/<id>
	if winner.ID == "" || winner.ID == loserURI {            // <- compares short id to full IRI
		return MergeResult{}, fmt.Errorf("%w: merge needs a distinct winner term", ErrValidation)
	}
```

`loserID` is a **short local id** — the handler enforces that with
`authoritiesvc.IDPattern` (`httpapi/authorities_handlers.go:156`) — and
`LocalAuthorityIRI` expands it. But a local `winner.ID` is *also* a short id:
that is exactly what `POST /v1/authorities` returns, and what the merge marker
stores in `mergedInto`. So `winner.ID == loserURI` compares `"a5fth0j8r3knhm"`
against `"https://github.com/freeeve/libcat/authority/a5fth0j8r3knhm"` and is
never true for a local winner. **The self-merge guard is dead code on the only
path that can reach it.**

Proof that the guard works only for the form no client sends:

```
create returns id = "ah2a3p0gps79va"           (short id, not an IRI)
self-merge, winner.id = SHORT id  -> 200       heading retired, mergedInto = itself
self-merge, winner.id = FULL IRI  -> 400       {"error":"…merge needs a distinct winner term"}
```

## Why it matters

- **Silent data loss.** A cataloger (or a UI bug, or a double-submit that reuses
  the selected term as both sides) destroys a heading with a single click. There
  is no `DELETE /v1/authorities/{id}` and no un-merge, so the only recovery is
  editing the blob store by hand.
- **A term whose `mergedInto` points at itself is a cycle.** Anything that
  follows the marker to resolve a winner -- reference rewriting, display, the
  vocab resolver -- can loop or resolve to a retired term.
- The merge marker path (`bibframe.AddAuthorityMergeMarker`) otherwise looks
  right: the loser survives as a readable tombstone carrying `mergedInto`, and
  `rewritten` counts the subject references repointed. Only the guard is broken.

## Expected

Reject a self-merge regardless of which id form the caller uses. Normalise both
sides before comparing, e.g.

```go
loserURI := bibframe.LocalAuthorityIRI(loserID)
winnerURI := winner.ID
if winner.Scheme == LocalScheme && !strings.HasPrefix(winnerURI, "http") {
	winnerURI = bibframe.LocalAuthorityIRI(winner.ID)
}
if winnerURI == "" || winnerURI == loserURI {
	return MergeResult{}, fmt.Errorf("%w: merge needs a distinct winner term", ErrValidation)
}
```

Add a table test covering: short-id winner, full-IRI winner, and a distinct
winner in each form.

## Repro

```sh
# libcat-e2e
node harness/probe_selfmerge.mjs
```

## Related

- Cleanup after this probe is impossible through the API: there is no
  `DELETE /v1/authorities/{id}` route, and a `DELETE` to that path returns
  **200 text/html** rather than 404/405 -- see task 201. A handful of
  `zz-selfmerge-*` / `zz-e2e-auth-*` local headings are now retired-but-present
  in the 8481 playground blob store as a result.
