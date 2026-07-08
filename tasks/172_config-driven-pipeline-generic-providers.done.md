# 172: Config-driven pipeline + generic providers -- adopters shouldn't need Go

Left by the queerbooks-demo session 2026-07-08 (uncommitted cross-repo note).
Eve's framing: "not really expecting implementers of libcat to work on Go."

queerbooks-demo's cmd/qbd is the evidence. After a full adoption cycle, its
inventory splits cleanly:

## Should be lcat verbs / config (we wrote Go wrappers for these)

- **project, multi-feed**: project.Project views one feed graph; multi-feed
  deployments need the union (per-feed projection merged by work id, first
  feed wins a shared work). We built `qbd project -provider a,b` -- belongs
  in lcat project.
- **export/downloads**: grains -> catalog.nq.gz + catalog.mrc.gz +
  catalog.xml.gz + integrity manifest, with per-record skip for records ISO
  2709 cannot encode. Fully generic; nothing deployment-specific except:
- **public-provenance allowlist**: strip lcat:extra/sources values not in a
  configured allowlist from the public projection AND the nq download
  (privacy: community-source attribution must not leak on any public
  surface). Pure config. Ours: LOC/OverDrive/QLL only.
- **pipeline orchestration**: ingest -> project -> export -> index -> hugo
  as one `lcat build` driven by a deployment config file, instead of every
  adopter's shell script (we rebuilt ours 13 times tonight).

## Should be config-driven providers (today: mandatory Go)

- The registry/factory pattern ("copy cmd/lcat/providers.go") makes a Go
  binary the price of admission. Precedent: Aspen Discovery side loads --
  librarians load MARC exports with an indexing profile, not PHP.
- Generic providers worth shipping: **MARC file** (the sideload case),
  **N-Quads/dcterms** (our collnq provider is 90% a declarative mapping:
  work-IRI prefix, predicate->field map, identifier URN schemes, named-graph
  tier handling), **CSV+mapping**.
- Keep the Go Provider seam for genuinely bespoke sources (our coll.db
  SQLite provider stays code and that's fine) -- but it should be the
  exception, not the on-ramp.

## Offer

Happy to contribute the generic halves of qbd upstream as a starting point
(export/downloads, multi-feed project union, sources allowlist, and the
collnq provider generalized to a mapped nquads provider) -- say the word in
a task drop and we'll prepare them against your HEAD.
