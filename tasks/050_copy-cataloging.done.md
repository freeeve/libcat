# 050 -- Copy cataloging + staged import

## Context

Koha's Z39.50/SRU copy cataloging and staged-import workflow. The protocol
clients live in libcodex, not here -- and **both have now shipped**, so this
task is fully unblocked once tasks/053 bumps the dependency.

**SRU (libcodex v0.9.0)**: `sru.NewClient(baseURL)` (configurable
version/schema/maxRecords), `Client.SearchRetrieve(ctx, sru.Request{Query,
StartRecord, MaximumRecords})` -> `Response{Records, NumberOfRecords}` with
`Record.Decode() (*codex.Record, error)` (MARCXML handled internally), a
streaming `Client.NewReader(ctx, query)` implementing `codex.RecordReader`
with `All() iter.Seq2`, and `sru.Quote` for CQL terms.

**Z39.50 (libcodex v0.10.0, pure Go -- no YAZ/cgo)**: `z3950.NewClient
(target)` -> `Connect(ctx) (*Conn)` -> `Conn.Search(ctx, Query)` /
`Present(ctx, start, count)`; `Record.Decode() (*codex.Record, error)`;
streaming `Client.NewReader(ctx, q)` mirroring the SRU reader; composable
bib-1 queries (`z3950.Term(index, term)`, `And/Or/AndNot`, modifiers
`.Phrase()/.Word()/.Truncated()/.Exact()`); idPass/open auth in Initialize.
The plan's YAZ-gateway-container escape hatch is now moot -- delete that
language from the design when implementing; targets config gains a
`protocol: sru|z3950` field and both routes converge on `*codex.Record ->
FromRecord -> editor draft`.

The staged file-import half (.mrc upload) remains independent of both.

## Scope

1. `backend/copycat/`: thin integration over libcodex search clients -- targets
   config `{name, url, protocol, index map, recordSchema}`, search fan-out,
   result -> `codex.Record` -> `FromRecord` -> editor **draft** (nothing
   committed until save).
2. Match banner: incoming record run through `identity.Resolver`
   ("would merge with existing Work w..."), choices: open existing /
   import as new / overlay.
3. Staged import batches: upload .mrc/N-Quads -> parsed server-side into staged
   records (datastore, not grains) with per-record match status; review screen;
   per-batch overlay policy (replace-feed / fill-holes-only / never; editorial
   always preserved); commit applies via the ingest pipeline.
4. SPA: external search screen, staged-batches screen, Targets admin config.

## Acceptance

- [x] Staged .mrc batch: stage -> review matches -> commit -> grains land through
  the shared identity/cluster pipeline; re-commit is byte-stable. (Tested on the
  vendored MARC Express sample: byte-stable re-commit rewrites nothing -- even
  with an editorial tag written between commits -- and fires no rebuild
  trigger. Verified live: 15 records staged with titles, committed, searchable,
  re-stage shows the match banner with concrete ids.)
- [x] External search -> import -> match banner flows. (Fan-out over the
  libcodex sru/z3950 clients behind an injectable seam; per-target failures
  reported, results stage into the same reviewable batches.)

## Delivered (2026-07-02, commits 6cfe82b + SPA)

- **`ingest.RunStore`**: the shared prior-load/resolve/cluster pipeline
  (extracted as `cluster`) over a blob.Store -- grains under ETag CAS,
  byte-identical grains untouched, retired grains deleted, changed paths
  returned for the rebuild trigger; catalog.nq stays the rebuild pipeline's.
  `bibframe.BuildWorkGrain` exposes the per-Work build; `marc.FromCodexRecords`
  / `marc.Identity` expose the file-ingest crosswalk and resolution keys.
- **`backend/copycat`**: targets `{name, url, protocol: sru|z3950}` in the
  datastore (the YAZ-gateway escape hatch is moot, per the plan note);
  concurrent search fan-out with per-target failure reporting; staged batches
  with per-record match banners from a throwaway resolver seeded off the live
  grain tree; review (import/skip per record; overlay policy replace-feed /
  fill-holes-only / never -- editorial always preserved by the pipeline);
  commit re-resolves matches against the current corpus and runs the shared
  pipeline under the "copycat" feed.
- **HTTP** `/v1/copycat/*`: targets (admin writes), search, stage (search
  results or base64 .mrc), batches list/get/review/commit/delete.
- **SPA**: Import screen -- targets admin, external search with staging
  checkboxes and per-target failure notices, .mrc file upload, staged-batch
  review (match badges linking the existing work, decision radios, policy
  select, commit outcome). Axe a11y covered (it caught an empty table
  header, fixed).
- **Note**: match-status "import as new despite a match" (forcing a split)
  is deliberately not offered -- clustering owns identity; an over-merge is
  corrected with the editor's split tool after commit.
