# 205 -- whitespace-only batch search query bypasses the empty-query guard and silently selects the entire catalog

Filed from libcat-e2e on 2026-07-09 (cross-repo ask).

## Symptom

`batch.Resolve` refuses an empty search selection. It happily accepts one
containing a single space, and then targets **every work in the catalog**.

Measured on the 8481 playground (31 works), all via `dryRun` / `resolve`, so
nothing was mutated:

```
POST /v1/batch/resolve   {"selection":{"kind":"search","query":<q>}}
  q = ""     -> 400  batch: invalid request: search selection needs a query
  q = " "    -> 200  matched=31   <== ENTIRE CATALOG
  q = "  "   -> 200  matched=31   <== ENTIRE CATALOG
  q = "\t"   -> 200  matched=31   <== ENTIRE CATALOG
  q = "\n"   -> 200  matched=31   <== ENTIRE CATALOG

POST /v1/batch/ops       {"selection":{"kind":"search","query":" "}, ops:[…], "dryRun":true}
  -> 200  matched=31  applied=31
```

The same hole is reachable through a **saved query**, which persists it:

```
POST /v1/queries  {"label":"zz-e2e-ws","query":" "}     -> 201  (accepted)
POST /v1/batch/ops {"selection":{"kind":"savedQuery","savedQueryId":"…"}, dryRun:true}
  -> 200  matched=31  <== ENTIRE CATALOG
```

## Root cause

The guard tests the **raw** string; the scan **normalizes** it.

`backend/batch/batch.go:153`

```go
case KindSearch:
	if sel.Query == "" {                     // raw -- " " passes
		return nil, fmt.Errorf("%w: search selection needs a query", ErrValidation)
	}
	return s.scan(ctx, sel.Query)
```

`backend/batch/batch.go:178`

```go
func (s *Service) scan(ctx context.Context, query string) ([]Target, error) {
	...
	q := normQuery(query)                    // -> ""
	for _, summary := range summaries {
		if q != "" && !summary.Matches(q) {  // q == "" -> no filter at all
			continue
		}
		targets = append(targets, ...)       // every work
	}
```

`backend/batch/batch.go:328`

```go
// normQuery matches the works listing's treatment: lowercase, trimmed.
func normQuery(q string) string { return strings.ToLower(strings.TrimSpace(q)) }
```

So `" "` survives the guard, then trims to `""`, and `scan` treats `""` exactly
as `KindAll` does (`batch.go:167` -> `s.scan(ctx, "")`).

`CreateQuery` has the identical mismatch -- `backend/batch/macros.go:179`:

```go
if label == "" || query == "" {   // raw again
	return SavedQuery{}, fmt.Errorf("%w: a saved query needs a label and a query", ErrValidation)
}
```

so a whitespace query is stored, and every later use of that saved query silently
means "entire catalog".

## Why it matters

- A cataloger scopes a destructive batch ("remove tag X from these records"),
  the query field ends up holding a stray space -- pasted, autocompleted,
  trimmed-to-nothing -- and Execute rewrites **all 31 works** (all 62,602 on a
  real catalog). `dryRun` confirms `applied=31`.
- The guard exists precisely to stop unscoped search selections. `KindAll` is the
  supported way to say "everything", and the Exports page was recently changed
  (tasks/197) to make that choice explicit. This path re-opens it implicitly.
- A saved query freezes the hole: it looks like a scoped, named selection in the
  UI and in `GET /v1/queries`, but resolves to the whole catalog forever.
- The blast radius is the same class as the tasks/195 stale-selection hazard,
  but here the selection is wrong at the source rather than stale.

## Expected

Normalize once, then validate. Both call sites should reject a query that is
empty *after* `normQuery`:

```go
case KindSearch:
	q := normQuery(sel.Query)
	if q == "" {
		return nil, fmt.Errorf("%w: search selection needs a query", ErrValidation)
	}
	return s.scan(ctx, q)
```

and in `CreateQuery`:

```go
label, query = strings.TrimSpace(label), normQuery(query)
if label == "" || query == "" { ... }
```

Consider having `scan` refuse `""` outright and giving `KindAll` its own
unfiltered path, so "no query" can never silently mean "everything" again.

Guard with a table test over `"", " ", "\t", "\n", " \t\n "` for both
`KindSearch` and `CreateQuery`, asserting `ErrValidation`.

## Repro

```sh
# libcat-e2e -- non-mutating, uses resolve + dryRun
node harness/probe_queries.mjs
```

## Note

`GET /v1/works?q=%20` returning the full list is fine -- that is a browse. The
defect is that a *batch selection* inherits the same permissiveness.
