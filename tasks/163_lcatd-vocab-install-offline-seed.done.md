# 163 -- lcatd vocab-install: offline vocabulary seeding for serverless deploys

Companion to tasks/162 (same shape: seed tooling that talks to the target
store directly). Motivating case: queerbooks wants Homosaurus, and on their
Lambda deployment neither server-side path works:

- The click-to-download flow queues an async job that only the container
  worker loop drains (appdeps ticker) -- a frozen Lambda never runs it.
- Without a Dynamo table the source registry lives in `store.NewMem()`, so a
  POSTed drop-in source resets on every cold start anyway.

The durable artifacts are blob-side: the converted snapshot
(`<prefix>vocab/<name>.nq`) and its sidecar (`.json`). Installed sidecars
already widen the effective scheme filter at boot, so a blob-only install
loads without any registry entry.

## Plan

1. `lcatd vocab-install (--blob-dir <dir> | --s3-bucket <bucket>)
   [--aws-endpoint <url>] --name <source> [--scheme <scheme>]
   (--url <dump-url> | --file <dump-path>) [--authorities-prefix ...]
   [--max-mb N]` -- construct a `vocabsrc.Service` over the target store with
   a mem registry and nil index (Reload no-ops), register the source, and run
   the existing download/upload install path. Same converter, caps, layout,
   and sidecar as a server-side install.
2. Vocabularies screen visibility: `Views()` gains rows for orphan installs
   (sidecar present, no registered source), synthesized from the sidecar's
   name + scheme -- an offline-seeded vocabulary shows its install state and
   stays removable/refreshable-by-upload like any other.

## Outcome (2026-07-07)

Shipped as planned. `vocab-install --url` reuses CreateDownload/RunDownload
against the in-memory registry (full fetch caps and error typing), `--file`
reuses InstallUpload (gzip sniffing included). Verified end-to-end against a
dir store with the real Homosaurus v5 dump: 4286 terms installed in ~3s, a
fresh lcatd boot logs `vocabularies loaded schemes=[homosaurus]`, the
Vocabularies list shows the orphan row (installed, 4286 terms), and
`/v1/terms/resolve` + `/v1/terms?q=` serve the terms with multilingual
labels. Views() orphan rows are unit-tested including removability.

Cross-version note for queerbooks: homosaurus term IDs are stable across
v4/v5 but the URI namespaces differ, and `canonIdentifier` folds only
protocol/trailing-slash -- so records carrying `/v4/` IRIs do not resolve
against a v5-only install. Install both versions (both dumps are published),
or rewrite stored IRIs v4->v5 (safe: IDs carry over).
