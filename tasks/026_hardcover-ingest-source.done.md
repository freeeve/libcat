# 026 -- Hardcover ingest source (first-party provider)

## Context

The `libcatalog-demo` adopter (Eve's Library) currently builds its catalog from a
Hardcover "Read" shelf with three Node ESM scripts
(`scripts/fetch-hardcover.mjs`, `map-subjects.mjs`, `gen-facets.mjs`, driven by
`npm run data:refresh`). That direct-map approach was the documented fallback in the
demo's tasks/001: it re-implements clustering, format collapsing, subject control, and
facet counting outside the framework, and `gen-facets.mjs` hand-mirrors the projector's
ordering rather than using it.

The ecosystem already has the right shape for this: a compile-time `ingest.Provider`
(ARCHITECTURE §9a, tasks/006) whose records flow through the shared identity + clustering
pipeline into `feed:<name>`, then `lcat project` emits `catalog.json` + `facets.json`
from the real projector. `ingest/overdrive/` is the reference. A first-party
`ingest/hardcover/` provider lets any adopter pull a Hardcover shelf through the genuine
BIBFRAME -> project path -- which is exactly what the demo's fetch script aspired to
(its tasks/001 §3 note) -- and lets the demo drop its Node scripts and the `npx` build
step.

Reference implementation to port (in the sibling adopter repo, not here):
`libcatalog-demo/scripts/{fetch-hardcover,map-subjects,gen-facets}.mjs` and
`libcatalog-demo/data/subject-map.json`. They encode the live GraphQL shape and every
mapping decision below.

## Scope

1. **`ingest/hardcover/` provider**, mirroring `ingest/overdrive/`:
   - `ProviderName = "hardcover"`; `New(ingest.Config) (ingest.Provider, error)`;
     `Name()` -> feed (default `feed:hardcover`); `Role()` -> `RoleIngest`;
     `Records(ctx)` fetches the shelf and returns `[]ingest.Record`.
   - A `hardcover.Record` per `user_book` implementing `Identity() identity.Record`,
     `Work() codexbf.Work`, `Instance() codexbf.Instance` -- so the shared `ingest.Run`
     clustering and two-tier ids apply unchanged (no bespoke `workId`/dedupe).
   - Register in `cmd/lcat/providers.go`
     (`must(reg.Register(hardcover.ProviderName, hardcover.New))`), plus a
     `cmd/lcat/hardcover.go` if a dedicated config/subcommand is warranted.

2. **Fetch (port `fetch-hardcover.mjs`).** Endpoint
   `https://api.hardcover.app/v1/graphql`; `Authorization: Bearer <token>`. Token from
   `HARDCOVER_API_TOKEN` (via `Config.Params`/env), trimmed and with any existing
   `Bearer ` prefix stripped; **never committed**. Resolve `me { id }` -> user_id, then
   paginate `user_books where { user_id _eq, status_id _eq 3 }` `order_by id asc` with
   `limit`/`offset`. `ctx` must cancel in-flight requests. Preserve an `--introspect`
   affordance (dump a GraphQL type's fields) because Hardcover's schema drifts.

3. **Map `user_book.book` -> BIBFRAME Work/Instance:**
   - Title, subtitle.
   - `contributions[]` -> contributors, name normalized to `Last, First Middle`
     (comma-bearing and single-token names pass through); role from `contribution`
     (default `author`); dedupe by `name|role`.
   - `cached_tags.Genre` ({tag,count}[]) -> genres, most-voted first, distinct, capped
     at 8. These are uncontrolled tags unless mapped in step 4.
   - `editions[]` collapsed to one Instance per format, keyed on `reading_format_id`
     (1 physical, 2 audiobook, 4 ebook) with a fallback to `edition_format` /
     `physical_format` text; carry ISBN-13/10 (prefer an edition that has one), and set
     a `hardcover` `ProviderID{Source,Value}` on the Instance (mirroring OverDrive's
     source-tagged identifiers) for provenance/back-links.
   - `languages` default `["eng"]` (Hardcover rarely exposes language).
   - **Adopter-extra display fields**, none of which exist in upstream schema v5
     (`project.Work` is a fixed field set): `cover` (`book.image.url`, fallback first
     edition image), `book.description`, `user_book.rating`, and
     `last_read_date`/`first_read_date` -> `dateRead`. The projector drops unknown
     fields today, so these must be carried through an extras mechanism -- this
     **depends on / drives tasks/022** (adapter-forward-extra-params). Land 022 first (or
     together), otherwise the demo regresses on covers/ratings/read-dates.

4. **Controlled subjects in-graph (fold in `map-subjects.mjs` + `subject-map.json`).**
   Rather than a post-projection JSON rewrite, the provider (or a companion `RoleEnrich`
   provider) should emit controlled subjects into the graph: genre string -> authority
   URI (LCSH / Homosaurus) with localized `labels` and optional `skos:broader`, sourced
   from an authorities table. Then `project` carries subjects (with labels + hierarchy)
   and counts them in `facets.json` -- no bespoke subject step, and subjects become
   first-class (relates to tasks/012, tasks/015). Keep the genre->authority table
   data-driven (ship the demo's `subject-map.json` shape or an equivalent), not
   hardcoded per work.

5. **Facets from the real projector.** With ingest + project doing the work,
   `facets.json` comes from `cat.Facets()` (count-desc then label-asc) -- retire the
   demo's `gen-facets.mjs`. Confirm `project.SchemaVersion` is 5 (what the demo's Hugo
   module consumes).

6. **Adopter usage + migration.** Document the two-step refresh:
   `lcat ingest --provider hardcover ...` -> `catalog.nq`, then
   `lcat project --provider hardcover --catalog catalog.nq --out assets/` ->
   `catalog.json` + `facets.json` (+ `redirects.json`). The demo's `data:refresh`
   becomes a thin wrapper over `lcat` (or is dropped), removing Node/`npx` from its build
   and deploy (`.github/workflows/deploy.yml`).

7. **Tests.** Golden test in the `ingest/overdrive` style (`overdrive_test.go` /
   `bibframe_test.go`): a recorded Hardcover GraphQL response fixture under `testdata/`,
   asserting the mapped Work/Instance/subjects -- no live API in CI.

## Acceptance

- `ingest/hardcover/` implements `ingest.Provider` and is registered in `lcat`; a
  recorded fixture drives a golden test (no network in CI).
- `lcat ingest --provider hardcover` + `lcat project` produce `catalog.json` +
  `facets.json` for a Read shelf, schema v5, with the projector's facet ordering.
- Output is equivalent to the demo's current pipeline (same works, formats, contributors,
  controlled subjects with labels + broader, and the cover/rating/dateRead extras), so
  `libcatalog-demo` can delete `scripts/*.mjs`, the `data:*` npm scripts, and the `npx`
  build step.
- Hardcover token via `HARDCOVER_API_TOKEN` only; never written to disk or committed.

## Notes

- Filed from the `libcatalog-demo` session per the workspace convention (cross-repo work
  proposed as an uncommitted task). Corresponds to that repo's tasks/006 §3, which chose
  the upstream-ingest-source option over an adopter-local `cmd/data`.
- Live availability (OverDrive-style Tier-2 sidecar) is out of scope; this is the
  bibliographic ingest half only.

## Implementation plan (from recon of the demo scripts + the `ingest.Provider` contract)

Grounded in `ingest/overdrive/` (the reference provider) and the demo's
`scripts/{fetch-hardcover,map-subjects,gen-facets}.mjs` + `data/subject-map.json` (the
port source of truth). The provider Record contract is:

```go
type Provider interface { Name() string; Role() Role; Records(ctx context.Context) ([]Record, error) }
type Record   interface { Identity() identity.Record; Work() codexbf.Work; Instance() codexbf.Instance }
```

`ingest.Run(prov, out)` clusters purely on `Identity()` (`ProviderKeys` + Author/Title/Lang),
then crosswalks each Record's `Work()`/`Instance()` into the graph. Registration is one line
in `cmd/lcat/providers.go`; the generic `lcat ingest --provider hardcover` path then works
with no new subcommand.

### File-by-file

1. **`ingest/hardcover/hardcover.go`** -- source model + fetch (port of `fetch-hardcover.mjs`).
   - Endpoint `https://api.hardcover.app/v1/graphql`; POST `{query,variables}`;
     `Authorization: Bearer <token>`. Token from `Config.Params["token"]` else env
     `HARDCOVER_API_TOKEN` (fallback `HARDCOVER_TOKEN`), `strings.TrimSpace` + strip a
     leading case-insensitive `Bearer `. Never logged/written.
   - `me { id }` (response may be a single-element array -- handle both) -> userID.
   - Paginate `user_books(where:{user_id:{_eq:$u}, status_id:{_eq:3}} order_by:{id:asc}
     limit:$l offset:$o)` selecting `id,rating,last_read_date,first_read_date,book{...}`
     (title, subtitle, description, image.url, slug, contributions{contribution,
     author{name}}, cached_tags, editions{isbn_13,isbn_10,reading_format_id,edition_format,
     physical_format,image.url}). `status_id = 3` is a **literal constant** in the query,
     not a bound var. Loop `offset += limit` (default 100) until a short page; de-dup on
     `book.id` via a seen-set. `ctx` cancels in-flight requests (`http.NewRequestWithContext`).
   - Keep an `--introspect <type>` affordance (schema drifts) on the subcommand.
   - Structs: `userBook{ID, Rating, LastReadDate, FirstReadDate, Book book}`, `book{...}`,
     `edition{...}`; helpers `formatOf(edition) string` (reading_format_id {1:physical,
     2:audiobook,4:ebook}; else text fallback on edition_format/physical_format:
     audio|audible->audiobook, ebook|e-book|kindle->ebook, other non-empty->physical),
     `lastFirst(name)` (trim; pass through if empty/has-comma/<2 tokens; else `last, rest`),
     `genres(cachedTags)` (cached_tags may be a JSON **string or object**; read `Genre`/`genre`
     `[{tag,count}]`; sort count-desc, distinct, cap 8), `safeJSON`.

2. **`ingest/hardcover/provider.go`** -- the `ingest.Provider` glue, mirroring overdrive:
   `ProviderName = "hardcover"`, `Provider{feed, token, limit, ...}`, `New(cfg)`, `Name()`,
   `Role() -> RoleIngest`, `Records(ctx)` (fetch shelf; return one Record per book).

3. **`ingest/hardcover/bibframe.go`** -- crosswalk (`userBook`/`book` implements `Record`):
   - `Work() codexbf.Work`: `Class:"Text"`; `Titles:[{MainTitle:title, Subtitle:subtitle}]`;
     `Contributions` from `book.contributions` -- `{Class:"Person", Label:lastFirst(name),
     Primary:(first author only), Roles:[{Term: role|default "author"}]}`, dedup on
     raw-`name|role`; `GenreForms`/tags -> uncontrolled tags carried for step-4 promotion;
     `Languages:["eng"]`. Subjects: see step 4.
   - `Instance() codexbf.Instance`: one Instance per collapsed format (first-seen per format,
     upgrade to an ISBN-bearing edition when the incumbent lacks ISBNs); `ISBNs` as
     `Identifier{Class:"Isbn", Value:...}`; a `hardcover` `Identifier{Class:"Identifier",
     Source:"hardcover", Value:<book id or edition id>}` for provenance/back-links (mirrors
     overdrive's source-tagged ids); `Media`/`Carrier` RDATerms per format.
     *(Note: the current `Record` contract returns one `Work` + one `Instance`; overdrive is
     also 1:1. Hardcover's per-format collapse yields multiple instances per book -- confirm
     whether to emit the primary Instance and fold the rest, or whether `ingest.Run` needs a
     multi-Instance path. Overdrive sidesteps this by being one item = one instance.)*
   - `Identity() identity.Record`: `ProviderKeys` = `[ProviderKey(SchemeID,"hardcover:"+id),
     ProviderKey(SchemeISBN, isbn13)...]` -- **hardcover id first** (most specific), **ISBN
     keys included** so a Hardcover book clusters with an OverDrive edition of the same ISBN;
     `Author`/`Title`/`Lang` for the fallback fuzzy key.

4. **Controlled subjects in-graph** (fold in `map-subjects.mjs` + `subject-map.json`).
   Ship the demo's table (24 `genreToSubject` keys -> 16 authority URIs; 20 `authorities`
   with `labels{en,es}` + optional `broader[]`) as `ingest/hardcover/testdata/subject-map.json`
   or an `authorities`-package asset. During crosswalk, promote each mapped genre tag to a
   `codexbf.Subject{Class:"Topic", Label:.., Source:"hardcover"}` **and emit the authority
   URI + labels + skos:broader into the graph** (so `project` resolves controlled Subjects
   with labels + Broader exactly as tasks/012/015 do -- no post-projection rewrite). Unmapped
   genres stay as tags. Data-driven table, not per-work hardcoding.

5. **Facets** come free from `cat.Facets()` (count-desc, label-asc) -- retire `gen-facets.mjs`.
   `project.SchemaVersion` is already 5 (what the module consumes). No projector facet change.

6. **`cmd/lcat`**: add `must(reg.Register(hardcover.ProviderName, hardcover.New))` +
   import in `providers.go`; optionally a `cmd/lcat/hardcover.go` subcommand (flags:
   `--token`/env, `--limit`, `--out`, `--introspect`) as a convenience alias over
   `runIngest`, dispatched from `main.go` + listed in `usage()`.

7. **Golden test** (`ingest/hardcover/hardcover_test.go`), mirroring
   `ingest/marc/marc_express_test.go`: commit a captured GraphQL response under
   `ingest/hardcover/testdata/`, construct the provider against it (inject the fixture via a
   seam -- e.g. `Config.Source` pointing at the JSON file, or an unexported `httpClient`
   field), run `Records(context.Background())` with **no network**, and assert on
   `Work()`/`Instance()`/`Identity()`: contributor normalization, format collapse, ISBN
   selection, promoted controlled subjects (labels + broader), and the extras. A captured
   fixture is preferable to a hand-written one so the GraphQL shape stays honest.

### The one open decision: how the `cover`/`rating`/`dateRead` extras survive projection

`project.Work` is a **fixed v5 field set** (id, title, subtitle, contributors, subjects,
tags, languages, classifications, formats, instances) with no `extra`. `codexbf.Work` is
likewise fixed BIBFRAME -- so a Record's `Work()` has nowhere to hang these. tasks/022 wired
only the **last hop** (Hugo adapter forwards a catalog.json `extra` object to page params;
tasks/025 renders `.Params.cover`). The graph->project->catalog.json hops are still missing.
This is the piece that makes 026 more than "just a provider," and it touches core packages,
so it needs sign-off. Options:

- **(A) Graph passthrough (single pipeline, recommended for cover).** Add an optional
  `Extras() map[string]string` to `ingest.Record`; `ingest.Run` writes each as a triple under
  a reserved predicate (e.g. `lcat:extra#cover`) on the minted Work node. Add
  `Work.Extra map[string]string `json:"extra,omitempty"`` to `project.Work` and harvest that
  namespace during projection. `omitempty` keeps existing catalogs byte-identical, so it
  stays **schema v5** (additive). Completes 022's chain end-to-end. Cost: touches `ingest` +
  `project` + the graph.
- **(B) Keep personal data out of the graph (respects ARCHITECTURE §5).** `rating`/`dateRead`
  are personal reading-log data, not bibliographic -- putting them in the shared BIBFRAME
  graph is arguably a layering violation (the same principle that keeps availability out of
  the graph). Alternative: the provider emits a `extras.json` sidecar keyed by a stable key
  (ISBN or `hardcover:<id>`), and `lcat project` joins it onto the projected Works by that
  key. Cover (bibliographic-adjacent) could still go via (A) as `bf:coverArt`/`schema:image`.
  Cost: a new project-time join input; more moving parts but keeps the graph pure.
- **(C) Hybrid:** cover modeled as a real Instance/Work cover property (graph); rating +
  dateRead via the (B) sidecar. Cleanest architecturally, most code.

Recommendation: **(A)** for a first cut (fewest moving parts, one pipeline, finishes the
022/025 chain), with a note that rating/dateRead-in-graph is a deliberate pragmatic call; fall
back to **(C)** if graph purity is a hard requirement. This is the decision to confirm before
building step 3's Instance/extras emission and the `project` change.

### Progress

Decision: **(A) graph passthrough with provenance** (per the user) -- extras ride in the
feed provenance graph, so their origin is tracked like every other feed statement.

- **DONE (0854457) -- extras infrastructure.** `ingest.ExtraProvider` (optional Record
  capability); `ingest.Run` writes a Record's extras into the Work's feed graph under
  `bibframe.ExtraPred`; `project` harvests that namespace (feed-scoped) into
  `Work.Extra` -> `catalog.json` `extra` (omitempty, schema stays v5). Round-trip tests
  in `ingest` + `project`; existing providers unchanged (byte-identical grains).
- **DONE (e76cf74) -- Hardcover provider (scope #1,2,3,5,6,7).** `ingest/hardcover`:
  GraphQL fetch (token via `--token`/`$HARDCOVER_API_TOKEN`, `me`->user, paginated
  `user_books` status_id=3, ctx cancel, `--introspect`); per-format edition explosion so
  formats cluster into one Work with an Instance each; crosswalk (contributors, genre
  tags from object/string `cached_tags`, RDA media->format, ISBN merge keys, hardcover
  provenance id); `cover`/`rating`/`dateRead`/`description` via ExtraProvider; registered
  in `lcat` + `lcat hardcover` subcommand + generic `lcat ingest --provider hardcover
  --source <shelf.json>` offline replay; golden test through the real ingest->project
  path (no network). Verified: `lcat hardcover` + `lcat project --provider hardcover`
  yields the demo's catalog (clustered formats, contributors, tags, extras, facet counts).

- **DONE (7fd642f) -- controlled subjects (scope #4).** Chosen approach: **(1)
  SubjectEnricher capability** (per the user). `ingest.SubjectEnricher` (optional Record
  capability) + `bibframe.AuthoritySubject`; `ingest.Run` collects a Work's controlled
  subjects (first record wins); `bibframe.BuildWorks` emits each as a `bf:subject` link to
  the authority URI plus its `skos:prefLabel`/`skos:broader` statements in the feed graph,
  so the projector resolves them as controlled subjects with labels + hierarchy
  (tasks/012/015). Hardcover ships the demo's genre->authority table (go:embed
  `subject-map.json`): mapped genres promote to controlled subjects (LCSH/Homosaurus, en/es
  labels, broader), unmapped genres stay tags -- both dimensions coexist. Golden test
  asserts the promoted subjects (labels + broader); facets show the subjects dimension.
- **DONE -- docs (scope #6).** `docs/hardcover-provider.md`: the two-step `lcat hardcover`
  (or `lcat ingest --provider hardcover`) + `lcat project` refresh, offline `--source`
  replay, the mapping summary, and the note that the demo can drop its `scripts/*.mjs`, the
  `data:*` npm scripts, and the `npx` step. Pointer added to the top-level README.

## Resolution

All scope items complete across four commits (0854457 extras infra, e76cf74 provider,
7fd642f controlled subjects, plus docs): a first-party `ingest/hardcover` provider pulls a
Hardcover Read shelf through the real identity/clustering + `lcat project` path. Verified
end-to-end (offline fixture, no network): `lcat hardcover --source … && lcat project
--provider hardcover` yields schema-v5 `catalog.json` + `facets.json` equivalent to the
demo's Node pipeline -- clustered formats (ebook+audiobook -> one Work, both formats),
`Last, First` contributors, controlled subjects with en/es labels + `skos:broader`, and the
`cover`/`rating`/`dateRead`/`description` extras -- so `libcatalog-demo` can delete
`scripts/*.mjs`, the `data:*` npm scripts, and the `npx` build step. Token is read from
`--token`/`$HARDCOVER_API_TOKEN` and never written to disk. Full suite + `go vet` green.

Deviations from the demo, by design: physical format projects as `print` (the framework's
RDA `unmediated` term) rather than `physical`; a book with no credited author emits no
contributor (rather than a literal "Unknown"); `rating` projects as a string (RDF literal),
e.g. "4.5". A follow-up (filed separately) could restore the Homosaurus `+` in "LGBTQ+
books" now that tasks/023 slugifies URL-safely.

### Suggested commit sequence

1. provider skeleton + fetch + structs (no crosswalk) behind the registry;
2. crosswalk `Work()`/`Instance()`/`Identity()` + golden test on a captured fixture;
3. controlled-subject promotion + shipped authorities table;
4. the extras decision above (ingest + project change) once confirmed;
5. `cmd/lcat` subcommand + `usage()`;
6. docs: `lcat ingest --provider hardcover` + `lcat project` two-step in README/ARCHITECTURE,
   noting the demo can then drop `scripts/*.mjs`, the `data:*` npm scripts, and the `npx` step.
