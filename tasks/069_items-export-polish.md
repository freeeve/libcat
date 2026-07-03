# 069 -- Item templates, bulk add, authority export (Koha parity gaps)

## Context

Remaining gaps from the Koha cataloging review (2026-07) that fall outside
tasks/058 item polish. The bf:Item model (tasks/051) stays minimal -- no
circulation state -- but multi-copy physical workflows need less typing,
and export jobs (tasks/038, 048) cover works only.

## Scope

1. **Item templates**: saved item field sets (call number pattern,
   location, note), personal or library-shared on the macros sharing model
   (tasks/047); apply pre-fills the ItemsPanel form.
2. **Bulk add**: create N copies in one action with an auto-incrementing
   barcode pattern (prefix + zero-padded counter, collision-checked
   against existing barcodes); preview the generated list before create.
3. **Authority export**: extend export jobs with an authorities selection
   (all / by-vocabulary / label search); formats: MARC authority, SKOS
   N-Quads, JSON-LD.

## Acceptance

- Applying a shared template then bulk-adding 12 copies yields 12 items
  with sequential barcodes and no collisions; preview matches the result.
- An authorities export job produces valid MARC authority records and
  round-trips labels/cross-references in the SKOS output.
