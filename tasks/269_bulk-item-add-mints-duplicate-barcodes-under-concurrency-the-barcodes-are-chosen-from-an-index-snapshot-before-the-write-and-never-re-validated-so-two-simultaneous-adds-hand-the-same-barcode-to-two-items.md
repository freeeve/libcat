# 269 -- bulk item add mints duplicate barcodes under concurrency: the barcodes are chosen from an index snapshot before the write and never re-validated, so two simultaneous adds hand the same barcode to two items

Filed from libcat-e2e on 2026-07-09 (cross-repo ask).
