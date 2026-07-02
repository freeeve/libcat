# 012 -- Separate controlled subjects from tags; resolve subject labels in the projector

> Filed by the qllpoc session as a cross-repo handoff (uncommitted, per the repo
> boundary), mirroring how `tasks/009` execution was handed to qllpoc as its
> `tasks/045`. Fold into a projector task or keep as the spec.

## Problem

`lcat project` flattens two different things into one `catalog.json` `subjects`
string list (`project/project.go` `subjects()`, ~line 309):

- **Controlled-vocabulary subjects** -- IRI-valued `bf:subject` from the
  `editorial:` graph (Homosaurus term URIs, e.g.
  `https://homosaurus.org/v4/homoit0001378`). These have **no `rdfs:label`** in
  the record, so `subjects()` falls through to `s.Value` and emits the **raw
  URI**. Downstream (the Hugo module facet + Work detail) shows the URI as the
  subject text -- see qllpoc `site-graph/` (`tasks/045`), where subjects render
  as `https://homosaurus.org/v4/...`.
- **Uncontrolled feed tags** -- label-only blank `bf:Topic` nodes from
  `feed:<provider>` (OverDrive genre/topic strings: "Fiction", "Romance",
  "Historical Fiction", "LGBTQIA+ (Fiction)"). Free strings, not authorities.

Conflating them means a facet page mixes a controlled Homosaurus authority with a
loose vendor genre string, and the controlled ones aren't human-readable.

## The graph already distinguishes them

No new modeling needed -- the discriminator is structural and already present:

| Kind | Graph shape | Discriminator |
|---|---|---|
| Controlled subject | `<#wWork> bf:subject <IRI> <editorial:>` | object is an **IRI** |
| Tag (feed) | `<#wWork> bf:subject _:b <feed:overdrive>` ; `_:b a bf:Topic` ; `_:b rdfs:label "…"` | object is a **labeled blank node** |

So `subjects()` can split on `s.IsIRI()` vs literal-labeled deterministically.

## Scope

1. **Two dimensions in `catalog.json`.** Split `Work.Subjects` into:
   - `subjects` -- controlled, authority-backed. Each entry carries a stable
     **id (the URI)** and a resolved **label** (see below), so links/facets key
     on the URI while display shows the label. Suggest
     `[]{ID string; Label string}` (or `Label map[string]string` for i18n).
   - `tags` (or `genres`) -- the uncontrolled feed label strings, as today's flat
     `[]string`.
2. **`facets.json`** -- emit `subjects` and `tags` as separate facet dimensions
   (subject facet values keyed by URI + label; tag facet values plain strings).
3. **Hugo module (`tasks/009`)** -- add a `tags` taxonomy alongside `subjects`;
   the subject facet/term + Work-detail render the **label** and link by URI; tags
   render as plain terms. Update `hugo/hugo.toml` `[taxonomies]` (and the README's
   canonical block importing sites must copy) and the exampleSite.
4. **Schema version** -- bump `project.SchemaVersion` (v2 -> v3); the adapter
   already fails loudly on mismatch, so consumers reproject.

## Label resolution (decision for this session)

A controlled subject's label must come from the authority, not the record. Two
paths -- recommend (a), with (b) as an interim:

- **(a) Authority triples in the graph (linked-data, ARCHITECTURE §5).** Resolve
  the subject IRI's label from `skos:prefLabel` / `rdfs:label` statements on that
  IRI, language-tagged (`@en`, `@es`). This assumes the vocabulary is present as
  authority records (the `data/authorities/` shard §5 anticipates). Companion
  qllpoc work: materialize Homosaurus authority grains (term URI -> prefLabel +
  altLabel + broader, en/es) so the projector has labels to read. This keeps the
  projector provider/vocabulary-agnostic.
- **(b) Optional label input.** `lcat project --labels <uri->label file>` (or
  `--authorities <dir>`); qllpoc passes a map derived from its
  `site/data/homosaurus.json` (`id -> {label, label_es}`). Pragmatic, but a
  bespoke input the graph model would rather not need.

Either way: **multilingual** (qllpoc renders en + es), and **URI-not-found**
falls back to emitting the URI as the label (today's behavior) so nothing breaks.

## Consumer impact

- qllpoc reprojects via `make graph-project` (already wired to `lcat serialize` +
  `lcat project`); `site-graph/` picks up labels + the tag facet with no qllpoc
  code change beyond the schema-version target and any theme override.
- If (a): qllpoc adds a Homosaurus-authority migration (a follow-up qllpoc task);
  its editorial overlay already emits `bf:subject <uri>` (qllpoc
  `internal/graphmigrate`), so only the authority label triples are new.

## Acceptance

- [x] `catalog.json` (v3) has distinct `subjects` (URI + labels) and `tags`; a
  Homosaurus subject shows its human label, not the URI (once authority labels are
  in the graph; else it falls back to the URI).
- [x] `facets.json` + the Hugo module expose subject and tag facets separately.
  Subject **identity** keys on the URI (facets.json `subjects` are URI-keyed; the
  Work-detail subject links to its authority URI). The Hugo **facet term pages** are
  label-keyed for usable/localized URLs -- a deliberate base-module choice; a theme
  can re-key on a URI-slug if it prefers.
- [x] Provider/vocabulary-agnostic: the projector reads generic
  `skos:prefLabel`/`rdfs:label`; no Homosaurus specifics in `project/`.

## Done (commits `51c0090` projector, `9c865c0` Hugo)

Decision (with the session): **authority labels are provenance-tracked quads**, not
a `--labels` side-channel -- consistent with §5 (4th column = provenance). They live
in an `authority:<vocab>` graph **merged into `catalog.nq`** (chosen over a separate
`--authorities` input), which `lcat serialize` already folds in since it merges every
grain. So no new CLI surface: `buildLabelIndex` scans the whole dataset for
`skos:prefLabel` (preferred) / `rdfs:label` on IRI subjects, keyed by language tag,
and `subjectsAndTags` splits on `s.IsIRI()`. URI fallback when no label.

- `project`: `Work.Subjects []Subject{ID, Labels map[lang]string}` + `Work.Tags
  []string`; `Facets.Subjects []SubjectFacet{ID, Labels, Count}` + `Facets.Tags`;
  `searchText` indexes subject labels + tags; `SchemaVersion` 2->3.
- `hugo`: `tags` taxonomy; subjects taxonomy-indexed by localized label; Work detail
  shows label + authority-URI link + a tags section; facet sidebar renders labels.
  Module target -> v3; README + exampleSite updated.
- Validated: unit tests (controlled subject with en/es prefLabels resolves; feed tag
  splits out); corpus reproject (5,659 works, the OverDrive feed's 93 genres all
  route to `tags`, `subjects` empty -- no Homosaurus URIs in the vendor feed).

**qllpoc companion (its side):** materialize Homosaurus authority grains (term URI ->
prefLabel/altLabel/broader, en/es) into `authority:homosaurus` so the merged
`catalog.nq` carries labels for the projector to read; `internal/graphmigrate`
already emits `bf:subject <uri>`.

## Refs

- `project/project.go` (`subjects()` ~L309, `Work.Subjects` L61, `Facets.Subjects`
  L96); `hugo/hugo.toml` `[taxonomies]`, `hugo/layouts` (`page`, `term`,
  `_partials/facets.html`); ARCHITECTURE §5 (controlled vocabularies as linked
  data; `data/authorities/`).
- qllpoc `tasks/045`/`tasks/041` (the bridge that surfaced this),
  `internal/graphmigrate` (emits `bf:subject <homosaurus-uri>` in `editorial:`).
