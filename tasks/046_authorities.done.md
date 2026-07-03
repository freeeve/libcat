# 046 -- Authorities: local authority editing, merge, linking

## Context

Koha's authorities module, SKOS-native: local authority grains at
`data/authorities/` (ARCHITECTURE §3), ids minted `a...`, edited with the same
profile mechanism; cross-references are altLabel (used-for) and broader/
narrower/related (BT/NT/RT). Global heading update is free -- bibs reference by
URI, so relabels propagate at projection; merges rewrite references.

## Scope

1. `backend/authoritiesvc/`: authority CRUD over authority grains (blob.Store),
   label search, `identity.Mint("a")` for local terms; authority profiles
   (authority-topic shipped default).
2. Authority merge: `lcat:mergedInto` on the loser + batch rewrite of
   `bf:subject` references loser -> winner across work grains (one batch op).
3. Auto-linking suggestions (never auto-write): on Work save, string subject
   values matched against authority labels -> suggestion queue items; unmatched
   heading -> one-keystroke "create local authority" flow.
4. SPA: Authorities search screen + AuthorityEditor (prefLabel/altLabel/
   definition/broader/narrower/related/exactMatch), merge tool.

## Acceptance

- [x] Create/edit/merge round-trips through grains; merged references rewritten
  corpus-wide; vocab index reflects changes after reload.
- [x] Auto-link suggestions land in the queue with provenance PIPELINE.

## Delivered (2026-07-02)

- **Grain substrate (root module).** `identity.AuthorityPrefix` ("a");
  `bibframe/authority.go`: `AuthorityGrainPath` (`data/authorities/<xx>/<id>.nq`),
  `LocalAuthorityIRI` (absolute IRIs under the framework's local-authority
  namespace, so `bf:subject` references stay linked data), `AuthorityTerm`
  (SKOS description incl. `narrower`/`exactMatch`) with grain build/parse
  round-trip, `AddAuthorityMergeMarker` (`lcat:mergedInto` in the loser's
  authority graph, so it survives description edits and the index sees it),
  and `ReplaceSubjectReference` -- retracts the editorial `bf:subject` +
  authority-graph statements about the loser and appends the winner through
  the same path suggestion publishing uses; feed graphs are never touched.
  `ingest.WorkSummary` gains `Subjects` (controlled IRIs) and `SummarizeGrain`
  is exported for single-grain callers.
- **Vocab index is now snapshot-swapped** (`vocab.Index.Reload`): reads stay
  lock-free over an immutable snapshot; a reload atomically swaps it, so every
  holder (terms handler, suggestion gate, publisher) sees authority edits
  without rewiring. Loader handles `skos:exactMatch` + `lcat:mergedInto`;
  retired terms resolve via Lookup but leave search. New `Terms` (management
  listing) and `MatchLabel` (whole-heading exact match, pref-vs-alt flagged).
- **`backend/authoritiesvc`**: CRUD over authority grains (mint-and-check
  create, If-Match update that preserves retirement, ETag concurrency), Merge
  (marker + `ScanSummaries`/`MutateGrain` batch rewrite of every carrier in
  one pass + grains-changed trigger + audit), AutoLink (string subjects vs
  every scheme's labels; exact pref match 0.9, used-for 0.75 ->
  `PipelineSuggest`, create-only + tombstone-aware, never auto-writes), and
  Reload. Winner may be in any scheme -- local->established-vocabulary merge is
  the expected promotion path.
- **HTTP surface** (`/v1/authorities`: list/search, POST create, GET/PUT with
  ETag + 412-with-fresh-state, `/merge`, `/reload`, `/profile` serving the
  authority-topic profile, librarian-gated); the record write paths (PUT +
  ops) hand saved grains to the auto-linker via the `WorkSaveHook` seam.
  `authority-topic` profile gains `narrower` + `exactMatch` (750). lcatd
  always mounts the index (a fresh deployment can mint its first term) and
  force-includes `local` in a configured scheme filter.
- **SPA**: Authorities search screen (keyboard-navigable, retired badge,
  one-keystroke "create local authority" for an unmatched heading -- lands in
  the editor) and AuthorityEditor (lang-tagged prefLabel/altLabel/definition
  rows, URI lists with VocabPicker for broader/narrower/related/exactMatch,
  profile-driven field labels, If-Match save with conflict reload, merge tool
  with inline confirm). Axe a11y tests cover both screens.
- **Tests**: grain round-trip/marker/rewrite (bibframe), summary subjects
  (ingest), reload/retirement/match (vocab), service CRUD/merge/autolink
  (authoritiesvc), full HTTP flows incl. the on-save PIPELINE suggestion
  (httpapi), 6 axe screens (ui). Both modules green.
