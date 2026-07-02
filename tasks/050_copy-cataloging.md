# 050 -- Copy cataloging + staged import

## Context

Koha's Z39.50/SRU copy cataloging and staged-import workflow. The protocol
clients live in libcodex, not here. **SRU shipped in libcodex v0.9.0**:
`sru.NewClient(baseURL)` (configurable version/schema/maxRecords),
`Client.SearchRetrieve(ctx, sru.Request{Query, StartRecord, MaximumRecords})`
-> `Response{Records, NumberOfRecords}` with `Record.Decode() (*codex.Record,
error)` (MARCXML handled internally), a streaming `Client.NewReader(ctx,
query)` implementing `codex.RecordReader` with `All() iter.Seq2`, and
`sru.Quote` for CQL terms. Z39.50 is tracked upstream (libcodex tasks/075,
YAZ as an external test oracle -- not cgo); the external-search half here can
proceed on SRU now (needs the libcodex v0.9.0 bump, tasks/053). The staged
file-import half (.mrc upload) is independent of both.

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

- Staged .mrc batch: stage -> review matches -> commit -> grains land through
  the shared identity/cluster pipeline; re-commit is byte-stable.
- External search -> import-to-draft -> match banner flows (once libcodex
  clients exist).
