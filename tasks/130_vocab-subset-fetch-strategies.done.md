# 124 -- vocab-subset: support non-id.loc.gov authorities (fetch suffix / whole-dump mode)

Filed from libcatalog-demo (its tasks/020, bundling Homosaurus in the sandbox).

`lcat vocab-subset` harvests a catalog's subject URIs and fetches each concept
from `<uri>.skos.nt` -- an id.loc.gov convention. Other authorities differ:
Homosaurus serves per-term `<uri>.nt` (no `.skos` infix) and also publishes a
full n-triples dump (`https://homosaurus.org/v3.nt`, 7.4MB), which for a ~4k-term
vocabulary is one request instead of thousands.

The demo worked around it with a bespoke script (libcatalog-demo
`deploy/lcatd/gen-homosaurus.sh`): download the dump, keep the subsetKeep SKOS
predicates, tag quads `<authority:homosaurus>`. Fine for one vocab, but every
adopter with a non-LoC authority will rewrite it.

Asks (either or both):

1. `--fetch-suffix` (default `.skos.nt`) so per-term harvesting works against
   authorities like Homosaurus (`--fetch-suffix .nt`).
2. `--dump <url-or-file>` mode: read a whole n-triples/nquads dump, filter to the
   catalog's URIs (or `--all` to keep the entire vocabulary), emit the same
   graph-tagged snapshot. Reuses the existing subsetKeep filter; content
   negotiation (`Accept: application/n-triples`) would cover authorities without
   suffix conventions.
