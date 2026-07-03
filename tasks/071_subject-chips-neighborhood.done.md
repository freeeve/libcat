# 071 -- Subject chips with labels and inline SKOS neighborhood

## Context

Subject values render as raw URIs (homoit0000506) in the editor even though
the vocab index holds every installed term's prefLabel, altLabels,
definition, and broader/narrower/related -- and tasks/067 snapshots keep
exactly those predicates. Catalogers crosswalk headings by hand; the data to
accelerate broaden/narrow/switch-to-sibling moves is already loaded.

## Scope

1. **Batch resolve**: `GET /v1/terms/resolve?ids=uri1,uri2,...` looks URIs
   up across every loaded scheme (a scheme-agnostic index wrapper).
   Unresolvable URIs are simply absent from the response.
2. **Chips**: an IRI field value that resolves renders as a chip -- prefLabel
   primary, scheme badge, URI in the tooltip. Unresolved URIs keep today's
   rendering plus a muted "not in local index" hint.
3. **Inline neighborhood**: a chip's disclosure expands a panel under its
   row (one open at a time): also-known-as labels, definition, then
   Broader / Narrower / Related / Siblings (broader's narrower minus self),
   each neighbor with **Replace** and **Add** actions that stage ordinary
   ops (replace = remove + add), so preview/drafts/MARC pane are untouched.
4. Live-only URIs (no local scheme) get no expansion -- live neighborhood
   fetch is a follow-up.

## Acceptance

- A homosaurus subject shows its label, not its id; hovering shows the URI.
- Expanding a subject and clicking a broader term's Replace stages two ops
  (remove old, add new) visible in the save bar and the diff preview.
- Sibling terms appear when the term has a broader parent with other
  children.
- An unresolvable URI renders as today and offers no expansion.
