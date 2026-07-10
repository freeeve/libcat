# 330 -- v0.126.0 MARC export now globally sorted by work id -- sha changes once for all adopters

Filed from queerbooks-demo on 2026-07-10 (cross-repo ask).

Not a bug -- a heads-up worth a release note. Adopting v0.126.0, our
`catalog.mrc.gz` and `catalog.xml.gz` both changed sha, with **no suppressed works
and no corpus change**. We chased it down because a churning export sha is the 291
failure mode, and wanted to be sure it was not that.

It is not. It is a one-time canonicalization:

- **Same content.** 77,084 MARC records before and after, same total uncompressed
  bytes (108,819,109), 0 added, 0 removed.
- **Order changed.** v0.126.0 emits records **globally sorted by work id** (0
  descents). v0.124.1 emitted in blob-walk order (255 descents). Same records.
- **Deterministic.** Three consecutive `lcat build --only export` at v0.126.0 give
  the identical sha. So this is a single re-sort, not per-release churn -- the good
  kind of change, matching what 291 already did for `catalog.nq.gz` (which did *not*
  move here, being already sorted).

The reason it is worth a note: **every** adopter who publishes MARC/MARCXML will see
those two artifacts' sha shift exactly once on this upgrade, and any page that embeds
their hashes (our Hugo Downloads page does) will re-render with them. Someone running
a sha-pinned integrity check or a "did my downloads change?" alarm will trip on it
and, without the note, go hunting for a data problem that is not there -- which is the
half hour we just spent.

Suggested one-liner for the v0.126.0 notes: "MARC and MARCXML exports are now emitted
in canonical work-id order; their checksums change once on upgrade, with identical
record content."

If the sort was a deliberate part of 304's export rework, great -- just say so where
adopters will see it. If it was incidental, it is still an improvement worth keeping;
please just make it a documented one.
