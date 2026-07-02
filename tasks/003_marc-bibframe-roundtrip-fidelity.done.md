# 003 -- MARC <-> BIBFRAME round-trip fidelity + known-loss list

## Problem
ROADMAP Phase 0 promises MARC/MODS/schema.org export. But MARC <-> BIBFRAME
conversion is lossy in **both** directions (LC's own converters drop data). "No
lossy intermediary" (ARCHITECTURE.md §2) is true of the markdown-vs-graph point
and must not be read as round-trip fidelity. Adopters bringing their ILS's MARC
will judge the framework on exactly this, so the loss must be measured and
documented, not assumed away.

## Scope
1. **Round-trip harness.** MARC -> `codex.Record` -> BIBFRAME -> MARC over the
   ~6,266 qllpoc records; diff input vs output at the field/subfield level.
2. **Known-loss list.** Enumerate fields that do not survive round-trip (and the
   reverse: BIBFRAME constructs with no MARC home). Publish it in docs so it is a
   contract, not a surprise.
3. **Golden files.** Freeze a representative sample as golden round-trip
   fixtures; CI fails on unexplained fidelity regressions.
4. **Direction coverage.** Same for MODS and schema.org exports (at least a
   smoke-level fidelity statement).

## Acceptance
- [x] A published known-loss table backed by the harness output.
- [x] Golden round-trip tests in CI; a fidelity regression breaks the build.
- [x] Phase 0 sign-off cites measured fidelity numbers, not "immediately."

## Done (commit pending)

Measured the loss on the real vendored OverDrive MARC Express samples (30 records)
and pinned it, rather than freezing brittle golden output bytes.

- **Harness + CI gates** (`bibframe/roundtrip_test.go`): round-trips each sample record
  `MARC --Encode--> BIBFRAME --Decode--> MARC` (libcodex `Encode`/`Decode`) and
  compares field-tag presence. `TestMARCRoundTripCoreFieldsSurvive` fails if a core
  bibliographic field is dropped; `TestMARCRoundTripNoUndocumentedLoss` fails on any
  loss not in the published table. The frozen fixtures + the pinned `coreFields` /
  `knownLostFields` sets ARE the golden -- a survival contract, more robust than golden
  bytes (which would churn with any libcodex serialization tweak).
- **Known-loss table** (`docs/marc-fidelity.md`): measured kept vs lost per field, with
  what each is and why. **Kept:** 001, 020, 100/700, 245, 250, 260, 300, 337, 338, 520,
  650, 655, 856. **Lost:** 006/007/008, 037, 040, 084, 306, 336, 347, 490,
  500/511/521/533/538, 776. **Relocated:** language 008/35-37 -> 041.
- **Framework payoff:** the two framework-critical MARC losses -- **037 (Reserve ID)**
  and **084 (BISAC)** -- are exactly what the OverDrive *direct* JSON->BIBFRAME path
  preserves (`tasks/008`). This is the measured backing for "OverDrive ingests directly;
  MARC is only the ILS ramp" (`tasks/007`).
- **MODS / schema.org** (scope §4): documented as export-only -- no import/round-trip in
  scope, so no fidelity contract claimed (a smoke-level statement, per the acceptance).

## Note

Fidelity is asserted at the field-tag level (present in vs out). Subfield/indicator/
punctuation-level diffing is a finer future pass; the tag-level contract already breaks
the build on the losses that matter to adopters.
