# 100 -- `lcat vocab-subset` produces a non-matching snapshot for https catalogs

> Filed from the libcatalog-demo repo (cross-repo note, uncommitted) while wiring the
> richer cataloging demo (demo tasks/011). Left uncommitted so a session working in
> this repo owns it.

## Symptom

`lcat vocab-subset --catalog catalog.json --out lcsh.nq` against a catalog whose LCSH
subjects use **https** URIs (`https://id.loc.gov/authorities/subjects/shNNNN`) reports
**"wrote 0 terms (of N subjects)"** and emits a snapshot the vocab index cannot match, so
the editor still shows the subjects as "not in local index" -- the exact thing the
subcommand exists to fix.

## Cause

- `catalogSubjectURIs` harvests the catalog's URIs as-is (https), and `fetchConcept`
  correctly fetches `<https-uri>.skos.nt`.
- But id.loc.gov's SKOS payload identifies the concept by its **canonical http** URI, so
  every emitted quad is keyed `http://id.loc.gov/...`.
- `subsetFromNT` counts a term only when the fetched triple's subject equals the input
  URI (`q.S.Value == uri`) -- http vs https never matches, hence "0 terms".
- The vocab index (`backend/vocab`) matches subject URIs **exactly** (`match` keyed by
  `Term.ID`), so an https catalog subject never resolves against the http-keyed snapshot.

The demo worked around it by rewriting the snapshot's `http://id.loc.gov/...` subject
URIs to `https://...` after generation (its catalog is https-native, from the upstream
`ingest/hardcover/subject-map.json`). That is a band-aid the tool should not require.

## Options

1. **Normalize on the way out.** In `subsetFromNT`, when a fetched concept's canonical
   URI differs from the requested URI only by scheme (http vs https), emit it under the
   **catalog's** URI (and fix the `hasLabel`/terms count to compare scheme-insensitively).
   Keeps the snapshot aligned with whatever URIs the catalog actually uses.
2. **Normalize on the way in.** Have the vocab index match subject URIs
   scheme-insensitively (treat http/https id.loc.gov as equal). Broader blast radius.
3. **Make the ingest subject-map canonical.** If https LCSH URIs are themselves the bug,
   `ingest/hardcover/subject-map.json` (and any projector output) should use the http
   canonical form -- then tool + catalog + LoC all agree. (Separate decision from the
   tool fix; the tool should still be robust to either.)

Option 1 is the smallest fix that makes `vocab-subset` correct for real catalogs.

## Acceptance

- `lcat vocab-subset` against an https-URI catalog writes a snapshot the editor resolves
  (real headings, no "not in local index") and reports a non-zero term count.

## Resolved (Option 1)

`subsetFromNT` now takes the namespace and re-schemes in-namespace URIs (subjects and
broader/narrower targets) to the **catalog's** scheme, comparing scheme-insensitively for
the term count. id.loc.gov's canonical-http concept is emitted under the catalog's https
URI, so the exact-match index resolves it; out-of-namespace URIs (wikidata/worldcat
matches) keep their own scheme. Verified: an https catalog now writes non-zero terms
keyed https (was "0 terms"). New `TestSubsetFromNTHTTPS` covers it. The demo's post-gen
http->https rewrite band-aid can be dropped. (Option 3 -- canonicalizing the ingest
subject-map -- stays a separate demo/ingest decision; the tool is now robust either way.)
