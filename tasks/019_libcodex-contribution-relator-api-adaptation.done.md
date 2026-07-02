# 019 -- Adapt OverDrive ingest to libcodex's relator/agent Contribution API (+ bump)

## Context

The sibling `libcodex` is mid-refactor -- "relator IRIs + agent fidelity for
contributions" (commit `fa40d61` plus uncommitted work across `bibframe/bibframe.go`,
`reader.go`, `reader_crosswalk.go`, `shape.go`, `vocab.go` as of 2026-07-02). That change
reworks `bibframe.Contribution`: the plain `Role` string field libcatalog sets today is
being replaced by richer relator IRIs / structured agent fidelity.

Because libcatalog builds against a local `replace => ../libcodex`, that in-flight edit
currently breaks libcatalog's Go build:

```
ingest/overdrive/bibframe.go:168:53: unknown field Role in struct literal of type
  "github.com/freeeve/libcodex/bibframe".Contribution
ingest/overdrive/bibframe.go:173:53: ... (narrator contribution)
```

This is an **external concurrent edit**, not a libcatalog regression. Do not fix it by
editing libcodex or by chasing its unsettled API -- wait until libcodex commits and tags
the change, then adapt libcatalog's call sites to the released shape.

## Blocked on

- libcodex landing + tagging the contribution/relator refactor (its own task). Until
  then the target API is unstable.

## Discovered API deltas (from the in-flight libcodex tree)

The refactor is broader than the 057 read-side; these `bibframe` struct changes affect
libcatalog's `ingest/overdrive/bibframe.go`:

1. `Contribution.Role string` -> `Contribution.Roles []Role`, where
   `Role{IRI, Term}` (relator IRI + label term). **Adapted (staged).**
2. `Instance.Provision *Provision` -> `Instance.Provisions []Provision`, and a
   `Provision` now carries a `Class` ("Publication"/...). **Adapted (staged).**
3. `Instance.Media`/`Instance.Carrier`: `string` -> `[]RDATerm` (seen mid-edit;
   `rdaCode`/`rdaTerm`/`content06` still undefined in the tree). **Pending** -- update
   `rdaMedia`/`Carrier` assignment in `bibframe.go` and the `Media`/`Carrier` assertions
   in `bibframe_test.go` once the `RDATerm` shape is final.

## Status (2026-07-02) -- DONE

libcodex **v0.8.0** is released (clean, green). All three deltas adapted in
`ingest/overdrive/bibframe.go`:
- Contributions build `Roles []Role{IRI, Term}` from the relator map (author ->
  `.../relators/aut`, narrator -> `nrt`); `Term` = the role label.
- Instance provision -> `Provisions []Provision` with `Class: "Publication"`.
- Instance media/carrier -> `[]RDATerm`: media `{Code, Label}` = `{s, audio}` (audiobook)
  / `{c, computer}` (ebook); carrier `{cr, online resource}`.

`go.mod` bumped to `libcodex v0.8.0` (local `replace` kept for co-dev). Tests updated:
`bibframe_test.go` asserts the new Roles/Provisions/RDATerm shapes and that the relator
IRIs reach the serialized n-quads; `ingest_test.go` stub updated. **Verified:** `go build
./...` clean; `go test ./...` all green -- incl. `./search` (the tasks/010 gate) and
`./project` (catalog.json's contributor role label + format facet still project
correctly); `gofmt -s` clean.

## Scope (once libcodex releases)

1. **Bump** the `github.com/freeeve/libcodex` require in `go.mod` to the released version
   (mirrors `tasks/013`'s 0.7.0 bump); keep or drop the local `replace` per dev setup.
2. **Adapt `ingest/overdrive/bibframe.go`** -- construct contributions with the new
   relator-IRI / agent-fidelity API instead of the old `Contribution{Role: ...}` string.
   OverDrive gives author + narrator roles; map them to the correct relator IRIs
   (e.g. `aut`, `nrt`).
3. **Re-verify** the round-trip and ingest paths: `go build ./...`, `go test ./...`
   (incl. `./search`, which only fails today because of the libcodex breakage), and the
   MARC/BIBFRAME round-trip fidelity checks (`tasks/003`).
4. **Check** whether the projector's `catalog.json` contributor shape changes (it emits
   `{name, role}` -- if relators become IRIs, decide whether `role` stays a human label
   or gains the IRI; keep the Hugo module's `contributorList` rendering working).

## Acceptance

- libcatalog builds and all tests pass against the **released** libcodex.
- OverDrive contributions carry the new relator/agent fidelity; `catalog.json` still
  renders contributors correctly in the Hugo module (no template break).
- `tasks/010`'s `go test ./search` is green again (it is only red today due to the
  external libcodex mid-edit).

## Refs

- libcodex `bibframe.Contribution` refactor (commit `fa40d61` + follow-on). libcatalog
  `tasks/013` (prior libcodex bump), `tasks/003` (round-trip fidelity), `tasks/006`
  (overdrive provider), `tasks/010` (search build, currently red through the replace).
