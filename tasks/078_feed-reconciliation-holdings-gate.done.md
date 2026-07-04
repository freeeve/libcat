# 078 -- Feed de-accession reconciliation and holdings-aware projection

## Context

Nothing in the model ties a record to being held/licensed. The public
projection (`project/project.go`) excludes only tombstoned/suppressed works;
the WASM search index is built from that projection, so every other minted
work is searchable. Two ways unowned records show in patron search today:
(1) OverDrive ingest never de-accessions -- `RunStore` only deletes grains
retired by clustering merges, so a title dropped from the Libby collection
persists until someone manually suppresses it; (2) copycat commits (050) and
future original records (077) create works with zero items and no
availability identifier -- fine while holdings are being added, wrong if they
linger. Live availability stays client-side by design (tasks/004), so the
fix is corpus hygiene, not runtime filtering.

## Scope

1. **Feed reconciliation pass**: after an OverDrive ingest, diff the incoming
   scan against works whose only bib source is `feed:overdrive`. Absent works
   get flagged `lcat:withdrawnFromFeed` (date-stamped), not deleted --
   editorial statements and identity survive a title returning to the
   collection.
2. **Withdrawal policy**: config-selectable per feed -- `review` (default:
   flag only, surface a review queue in the admin UI with one-key suppress /
   keep) or `auto-suppress` (flag and set `PredSuppressed` in one pass;
   clearing the flag un-suppresses only if suppression came from this pass).
3. **Holdings signal in projection**: project a `held` boolean per instance --
   true when it has >=1 item (physical) or a live-availability identifier
   whose feed still lists it (digital). Emitted into `catalog.json`/facets so
   the site can facet or badge; whether unheld works are hidden stays a site
   decision, not a projection hard-code.
4. **Admin visibility**: works list rows show withdrawn/suppressed/no-holdings
   badges (the editor `/v1/works` scan intentionally shows everything; it
   should say so per row).

## Out of scope

- Runtime availability filtering in search (index is static; view-time
  adapter already handles "unavailable now").
- Automatic grain deletion -- suppression/tombstone remain the only removal
  paths.

## Acceptance

- Ingesting a scan missing a previously-present Libby-only title flags it;
  in `review` mode it appears in the queue, and suppressing removes it from
  the next projection/search build; a later scan containing it again clears
  the flag (and auto-set suppression).
- A work with editorial statements or a second bib source is never flagged.
- A copycat-committed work with no items projects `held: false`; adding an
  item flips it without a re-ingest.
- Works list shows a "withdrawn" badge on flagged rows.

## Outcome

- `bibframe/visibility.go`: `lcat:withdrawnFromFeed` (date literal),
  `lcat:suppressedBy` (provenance of a suppression; "feed-reconcile" for the
  auto policy), `lcat:feedWithdrawalKept` (sticky keep decision); all read
  through `WorkVisibility` and set/cleared by the shared editorial-statement
  primitive. Manual unsuppress also drops the provenance mark.
- `ingest/reconcile.go`: `Reconcile(st, prefix, feed, present, policy, date)`
  diffs the corpus against the run's resolved Works (`Result.WorkIDs`, new).
  Protection: any other `feed:` graph or any editorial statement beyond the
  visibility marks themselves (items, tags, merges) exempts the grain;
  tombstoned and kept works are skipped. Returning works clear the flag, a
  reconcile-set suppression, and a stale keep. Policies `review` /
  `auto-suppress`; nothing deletes.
- CLI: `lcat ingest --reconcile=review|auto-suppress` (and the `overdrive`
  alias) runs the pass after the ingest and prints flag/suppress/clear
  counts plus the flagged ids.
- Projection (SchemaVersion 5 -> 6): `Instance.Held` / `Work.Held` -- >=1
  item, or an `overdrive-reserve` identifier while the work is not
  withdrawn. Cascade: hugo.toml catalogSchemaVersion, hugo/README (held
  semantics; hidden-vs-badged is the site's call), exampleSite fixtures to
  v6 with held, ARCHITECTURE §5. exampleSite builds on v6.
- Backend: `WorkSummary` gains Suppressed/Tombstoned/Withdrawn/Kept/Items/
  HasAvailability (the editor list shows everything, so rows say what
  projection would do); `GET /v1/withdrawn` review queue (undecided rows
  only); `POST /v1/works/{id}/withdrawn` with `keep` (clear flag + pin) or
  `suppress` (hide, flag stays as the reason), audited.
- UI: WorkSearch rows badge tombstoned/suppressed/withdrawn/no-holdings; new
  Withdrawals screen (`g t`, nav link) with s=suppress / p=keep / o=open
  keyboard triage.
- Per-feed policy config beyond the CLI flag (e.g. a server-side scheduled
  reconcile) can layer on later; the pass itself is store-agnostic.
