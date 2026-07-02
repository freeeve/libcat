# 053 -- libcodex v0.10.0 bump (SRU + Z39.50 clients, crosswalk gains)

## Context

libcodex has moved v0.8.0 -> v0.10.0 (local sibling HEAD, `replace
../libcodex` active). v0.9.0 brought the SRU client plus crosswalk
improvements that change what survives MARC round-trips -- 5XX note family ->
`bf:Note`, 76X-78X linking entries -> `bf:relation`, title completeness
(nonSortNum, uniform parts, 246). v0.10.0 adds the pure-Go `z3950` package
(no crosswalk changes v0.9->v0.10). Together they fully unblock tasks/050.
Same shape as the v0.7.0 bump (tasks/013): move the require line, verify no
grain drift OR characterize it, and re-measure fidelity.

## Scope

1. go.mod v0.8.0 -> v0.10.0 in both modules (core + backend; keep the local
   replaces).
2. Re-run the corpus: fresh ingest, re-ingest no-churn gate, serialize/
   project/index -- characterize any grain changes from the new crosswalk
   triples (new bf:Note/bf:relation quads are *expected* on the MARC provider
   path; RDFC canonicalization absorbs ordering).
3. **Re-measure the round-trip loss table** (`bibframe/roundtrip_test.go`
   coreFields/knownLostFields): 490/5XX/776 may move from lost to kept;
   update the pinned sets and `docs/marc-fidelity.md`. This feeds the
   tasks/049 fidelity table and shrinks the `lcat:marcVerbatim` sidecar's
   workload.

## Acceptance

- Full test suite green on v0.9.0; re-ingest 0-minted.
- Loss table matches measured reality; marc-fidelity.md updated.
- Projector output stable or the delta documented.
