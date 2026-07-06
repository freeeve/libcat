# 140 -- duplicates compare: subject names, not URIs; more distinguishing detail

(Renumbered from 138, which collided with the contributor-roles task.)

Filed from Eve's report (2026-07-06, screenshot of the Duplicates screen):
subjects render as raw authority URIs (id.worldcat.org/fast/..., homosaurus
IRIs) and the two columns look byte-identical -- nothing tells the cataloger
which record to keep.

## Shape

- Surface grain-carried names: extend the editor doc's display annotation to
  direct IRI-valued fields (resolved from the value node), and give the
  subjects profile field an annotation chain of skos:prefLabel. The ingest
  emission already writes `<authority-iri> skos:prefLabel "..."` next to
  every bf:subject link, so the doc then carries names with zero lookups --
  this is also fix 1 of tasks/137 (editor chips fall back to the annotation
  when the vocab index misses).
- Duplicates compare renders the annotation when present (URI in a tooltip),
  and adds per-column distinguishing detail: the instance summaries (format,
  ISBN, publication) that actually differ between records minted separately.
