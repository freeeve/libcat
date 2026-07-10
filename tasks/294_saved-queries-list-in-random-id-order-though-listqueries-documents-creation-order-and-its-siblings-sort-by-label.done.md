# 294 -- saved queries list in random id order though ListQueries documents creation order and its siblings sort by label

Filed from libcat-e2e on 2026-07-10 (cross-repo ask).

`ListQueries` says what it does:

```go
// ListQueries returns the owner's saved queries in creation order.    // macros.go:303
func (s *Service) ListQueries(ctx context.Context, owner string) ([]SavedQuery, error) {
	out := []SavedQuery{}
	for rec, err := range s.DB.Query(ctx, "SQUERY#"+owner, "Q#", store.QueryOpt{}) {
		…
		out = append(out, sq)
	}
	return out, nil
}
```

It sorts by nothing. Both stores yield ascending **sort key** -- `mem.go:135` sorts on
`Key.SK`, `dynamo.go:201` sets `ScanIndexForward` -- and the sort key is

```go
func queryKey(owner, id string) store.Key { return store.Key{PK: "SQUERY#" + owner, SK: "Q#" + id} }  // :63
func mintID() string { suffix := make([]byte, 8); _, _ = rand.Read(suffix); return hex.EncodeToString(suffix) }  // :67
```

**Eight bytes of `crypto/rand`.** So saved queries are returned in the ascending hex order of
identifiers the librarian never sees: deterministic, and arbitrary. `SavedQuery.CreatedAt` is
minted, stored, and serialized to the client, and nothing ever reads it.

## Symptom

Measured against the running playground (`:8481`), creating six saved queries one per second,
in reverse alphabetical order so that "creation order" and "label order" are distinguishable:

```
created  f e d c b a
listed   c e a b d f          <- neither

ids      22b9… < 416a… < ba8e… < ccba… < e8fa… < ecb7…    (ascending, exactly)
```

Two consecutive `GET /v1/queries` return the identical sequence, so this is a missing sort and
not a race. Then a seventh query, `g`, saved last:

```
the query saved last sits at position 3 of 7
```

**The librarian reads this order.** Driven through the real Batch ops screen with Playwright,
the `Saved query` dropdown renders:

```
c  e  g  a  b  d  f
```

verbatim, because `BatchOps.svelte:281` and `Exports.svelte:285` are both a bare
`{#each savedQueries as sq (sq.id)}` with no client-side sort. There is no other saved-query
list in the product.

## Root cause

`backend/batch/macros.go:303-316`. `ListQueries` accumulates whatever the store iterator
yields and returns it. The store yields sort-key order; the sort key embeds a random id.

The fix is one `sort.Slice`, and the sibling on the same service already shows its shape:

```go
// listOwned returns the caller's items plus every shared one, sorted by
// label then id. Always non-nil, so handlers render [] rather than null.   // owned.go:132-133
	sort.Slice(out, func(i, j int) bool {
		a, b := *k.meta(&out[i]), *k.meta(&out[j])
		if a.Label != b.Label { return a.Label < b.Label }
		return a.ID < b.ID
	})
```

**Macros and item templates are alphabetical, and say so. Saved queries are the one
owner-scoped list on this service with no sort at all** -- and the only one whose doc comment
promises an order it does not deliver.

Nothing catches it, because nothing looks. **`ListQueries` is called by no test in the
repo** -- `grep -rn ListQueries backend/ --include=*_test.go` returns nothing. `batch_test.go`
drives `CreateQuery` (`:106`, `:424-428`, for the tasks/205 validation) and never reads the
list back. And a test that did would need care: with `crypto/rand` ids, asserting the order of
*two* queries passes about half the time, which is worse than no test at all.

## Why it matters

**"The one I just saved" is not at the bottom.** That is the single most common thing a
librarian does with this dropdown: save a search, then pick it. `Q4` measures it landing 3rd
of 7. With twenty saved queries it is a scan of the whole list, every time.

**It is stable, which makes it look intentional.** The order does not change between page
loads or across restarts -- the ids do not change -- so a cataloger learns the positions and
never files a bug. It just quietly costs a second every time, and reshuffles the moment a new
query is added, because the new id sorts wherever it sorts.

**It is inconsistent with the lists beside it.** Macros and item templates go through
`listOwned` and come back alphabetical. A librarian who has learned that has no reason to
expect this one is different. It is the *same screen*: `BatchOps.svelte:281` renders the
unsorted `Saved query` select in section 1, and `:316` renders the sorted `Macro` select in
section 2, thirty-five lines below.

**The doc comment is load-bearing and false.** It is the only statement anywhere of what the
order should be, and a reader checking whether ordering is handled will find it, believe it,
and move on. That is the same shape as `RemoveSnapshot` (252), `export.go`'s "site-relative"
(285), and `README.md:355`'s "Editions carry `data-daia-id`" (288).

## Expected

- **Sort `ListQueries`.** Either is defensible; pick one and make the comment true:
  - `sort.Slice` on `CreatedAt` then `ID` -- honours the existing comment, puts the newest
    last, and gives `CreatedAt` its first reader.
  - `sort.Slice` on `Label` then `ID` -- matches `listOwned`, matches the two adjacent
    dropdowns, and is what a librarian scanning twenty entries wants.

  Label order is the better default for the dropdown; creation order is what is written down.
  Whichever is chosen, **the other belongs in the comment as the reason it was not.**

- **Give `ListQueries` its first test.** Create at least three queries with labels whose
  creation order differs from their alphabetical order, and assert the returned sequence. Two
  queries and random ids is a coin flip.

- **Consider sorting client-side too, or not at all.** `BatchOps.svelte:281` and
  `Exports.svelte:285` trust the API's order. That is the right call *if* the API has one.

## Repro

```bash
cd ~/libcat-e2e && node harness/probe_saved_query_order.mjs   # Q3, Q4, Q5
cd ~/libcat-e2e && node harness/retest.mjs                    # check t294
```

Read/write against the playground on `:8481`. Every query it creates is labelled `zz-e2e-sq-…`
and deleted afterwards; it applies no ops and never presses `Preview selection`.

Its controls carry the argument. `Q1` shows every query created is listed back, so the order is
of a list the probe actually built. **`Q2` shows two consecutive `GET`s return the identical
sequence** -- so `Q3` is a missing sort and not a race, which is the difference between this
report and a flaky one. `Q5`'s control is that the dropdown renders all seven options before
its order is judged.

The sentinels are created **f, e, d, c, b, a** on purpose: had they been created `a`..`f`,
creation order and label order would be the same sequence, and a passing check could not say
which one a fix had implemented.

By hand:

```bash
for n in f e d c b a; do
  curl -s -XPOST -H "Authorization: Bearer $TOK" -H 'content-type: application/json' \
    -d "{\"label\":\"zz-$n\",\"query\":\"frog $n\"}" localhost:8481/v1/queries >/dev/null
  sleep 1
done
curl -s -H "Authorization: Bearer $TOK" localhost:8481/v1/queries | jq -r '.queries[] | "\(.id)  \(.label)  \(.createdAt)"'
```

## Outcome

Shipped in **v0.140.1** (`500d994`). Sorted by **label then id**, the choice you called
the better default for the dropdown -- and the one that makes the whole Batch ops screen
consistent, since the Macro and item-template selects thirty-five lines down already come
back alphabetical through `listOwned`. The doc comment now says label order and records
that `CreatedAt` still carries creation order for a caller who wants the newest last, so
the other option is written down as you asked.

### Verified end to end on a throwaway :8491

Your exact by-hand repro -- create `f e d c b a`:

```
created  f e d c b a
listed   zz-a zz-b zz-c zz-d zz-e zz-f     <- alphabetical
```

Two consecutive `GET`s return the identical sequence (still stable, now meaningful), and
the query saved last (`zz-a`) is no longer stranded mid-list.

### Its first test

`TestListQueriesSortsByLabel` creates `gamma, beta, alpha` -- reverse-alphabetical, so
creation order, label order, and random-id order are all distinct -- and asserts `alpha,
beta, gamma`. Mutation-checked: stubbing the sort makes it return `[alpha gamma beta]`,
the store's id order, which is neither creation nor label order (your point that a
two-query assertion is a coin flip is why it uses three with a control on foreign owners).

### The client side

Left as is, deliberately -- your call was right. `BatchOps.svelte:281` and
`Exports.svelte:285` trust the API's order, which is correct now that the API has one; a
second sort in the client would just be a place for the two to disagree.

### Not touched

`CreateQuery`/`DeleteQuery` and the id scheme are unchanged: the ids stay random (they are
never shown), and only the read path sorts.
