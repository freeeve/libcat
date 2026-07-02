# 022 -- Content adapter: forward extra catalog fields to page params

## Context

Filed from the **libcatalog-demo** adopter site while wiring in its real Hardcover data
(that repo's `tasks/001`). Left uncommitted so a concurrent session here isn't disrupted;
a request, not in-progress.

`content/works/_content.gotmpl` maps a fixed allow-list of catalog fields into each
Work page's `params` (subtitle, languages, subjects, tags, formats, contributors,
classifications, contributorList, subjectList, instances, workID). An adopter whose
projected `catalog.json` carries additional fields -- the demo adds `cover`, `rating`,
`dateRead`, `description` from Hardcover -- cannot surface them in templates without
**shadowing the entire adapter**, which forks the multilingual + schema-guard logic and
reintroduces module-bump merge pain.

## Proposed change

Let the adapter pass adopter-defined fields through to params without a fork. Options:

1. **Passthrough map.** Forward a reserved object, e.g. `.extra` (or `.params`), from each
   Work verbatim into page params:
   `... "params" (merge (dict ...the fixed set...) (.extra | default dict))`.
   Adopters put custom fields under `extra` in their projection.
2. **Config allow-list.** A site param, e.g. `params.catalogExtraFields = ["cover",
   "rating", ...]`, that the adapter loops over and copies from each Work if present.

Option 1 is simplest and keeps top-level catalog keys clean if `extra` is namespaced;
option 2 avoids changing the projection shape. Either keeps existing sites unchanged
(no `extra` / empty allow-list = current behavior).

## Acceptance

- An adopter can add fields to Work page params via projection/config alone -- no copy of
  `_content.gotmpl`.
- `exampleSite` builds unchanged; documented under README "Providing the projected data".

## Related

- Demo also proposes `tasks/020` (baseof footer/head hooks) and `tasks/021` (link-back).
  With 020 + 022 the demo could drop both its `baseof` and adapter shadows.

## Resolution

Implemented **Option 1 (namespaced passthrough)** in `content/works/_content.gotmpl`: a
Work's reserved `extra` object is merged into the page params, with the fixed set merged
**last** so reserved keys (`workID`, `subjects`, etc.) always win -- `extra` can add keys
but never clobber one. A Work without `extra` is byte-for-byte unchanged (`with .extra`
guards the merge).

Verified on a built exampleSite (temporary fixture): a Work carrying
`extra: {rating, cover, workID: "<collision>"}` surfaced `.Params.rating`/`.Params.cover`,
while `.Params.workID` kept the reserved value (collision rejected); a Work without `extra`
was unchanged. `npm run test:a11y` and `npm run test:links` stay green (95 pages each).
Documented under `hugo/README.md` "Provide the projected data".

Note: HTML comments can't be used to debug params in a Hugo template -- Go's `html/template`
strips them; use a real element/attribute instead.
