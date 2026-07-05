# 103 -- lcat: an empty provider scan plus --reconcile withdraws the whole feed

> Filed from the 2026-07-05 full-code review.

## Symptom

`lcat overdrive --cache /wrong/path --out corpus --reconcile auto-suppress` reads
zero items without erroring, then flags every feed-only, uncurated Work in the
corpus as withdrawn and auto-suppresses it -- a catalog-wide visibility flip from
one misconfigured run.

## Cause

`runIngest` (cmd/lcat/ingest.go:61-72) calls `ingest.Reconcile` whenever
`--reconcile` is set, with no check that the scan actually produced records.
`overdrive.ReadCache` (ingest/overdrive/overdrive.go:74-92) returns an empty
slice and nil error when `filepath.Glob("page-*.json")` matches nothing, so a
wrong/missing `--cache` dir is indistinguishable from an empty feed. With
`present` empty, `reconcileGrain` (ingest/reconcile.go:117-137) withdraws every
grain that is not `grainProtected`. The next good scan self-heals, but the bad
run already published a mass-withdrawn catalog.

## Fix sketch

Refuse to reconcile against a zero-record scan: error out (with an explicit
`--reconcile-allow-empty` override for genuinely emptied feeds). Consider also
making `ReadCache` error when the cache dir exists but contains no page files,
or does not exist at all.

## Acceptance

- A reconcile-enabled run whose scan yields zero records exits non-zero without
  touching withdrawal flags, unless explicitly overridden.
- Test covers the empty-scan + reconcile path.
