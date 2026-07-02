# 055 -- bump libcodex to v0.11.0: round-trip gains land (your 081 filing)

Filed from libcodex (its tasks/081, now done). Left uncommitted per the
cross-repo convention.

## What changed upstream

libcodex v0.11.0 implements the reconstructable set your fidelity gate asked
for, so `TestMARCRoundTripLossTableCurrent` will flag these on the bump --
each moves from known-lost to kept in your table + `docs/marc-fidelity.md`:

- **511 / 521 / 533 / 538** -- typed notes (`bf:noteType` performers/audience/
  reproduction/systemDetails) that decode back to their original tags. Note
  labels now join every subfield, so multi-subfield 533s keep their details.
- **490** -- `bf:seriesStatement` on the Instance; `$v` splits back out on
  decode (rejoined after " ; ").
- **776 `$z`** -- the OverDrive print/ebook pairing (776 with only `$c`/`$z`)
  now survives as a `bf:Isbn` on the associated resource.
- **306** -- `bf:duration`; **347 `$a`/`$b`** -- `bf:digitalCharacteristic`
  -> `bflc:FileType`/`bflc:EncodingFormat` (`$2` is not round-tripped).

Also relevant to your `lcat:marcVerbatim` sidecar: repeated `bf:relatedTo` and
`bf:relation` children previously collided on one JSON-LD object key (only the
last survived); v0.11.0 serializes them as arrays. If you ingest
libcodex-emitted JSON-LD produced before v0.11.0 with multiple 7xx-$t or
76x-78x fields, those graphs are missing entries at the source.

Deliberately NOT reconstructed (your stated non-goals): 040, 037/084 as
vendor conventions (037 decodes as an 024-shaped identifier; 084 as 072).
006/007 packed reconstruction is upstream's task 082 (electronic + sound
categories first).

Upstream now runs its own loss-gate matrix (`bibframe/lossgate_test.go`)
mirroring yours -- kitchen-sink + LC realdata through all four serializations
with a stale guard -- so future gains should be flagged on both sides.

## Acceptance

- [ ] go.mod bumped to libcodex v0.11.0.
- [ ] `TestMARCRoundTripLossTableCurrent` failures resolved by moving the tags
      above to coreFields; `docs/marc-fidelity.md` updated.
- [ ] `lcat:marcVerbatim` sidecar shrinks accordingly on the sample corpus.
