# 145 -- editorial subject adds write no in-grain label quad

Found while diagnosing Eve's duplicates-URIs re-report (2026-07-06; that one
was queerbooks running pre-v0.11.0 -- their data was fine). The playground
showed the actual gap in THIS repo:

- Feed/enrichment paths write `<iri> skos:prefLabel "..."` next to every
  bf:subject link (ingest/enrich.go enrichmentQuads), so tasks/137/140
  grain-carried annotations work for ingested subjects.
- Subjects added through the editor (SubjectLookup, neighborhood "Add",
  batch ops) stage only the bf:subject quad into `editorial:` -- no label
  companion. Repro on the playground: w1dh6vtir43o8i has five editorial:
  subject IRIs, all `annotation: None` in the doc; the Duplicates compare
  and any other annotation consumer shows raw URIs for them. The editor
  screen itself hides this because ProfileForm falls back to the vocab
  index (resolveTermURIs), which Duplicates deliberately does not call.

## Shape

Server-side, at patch apply (or in the ops layer): when an add lands on a
field whose profile carries a skos:prefLabel annotation chain and the value
is an IRI the vocab index resolves, also write the
`<iri> skos:prefLabel "..."` quad into the grain (authority:<scheme> graph,
like vocabsrc does), so the grain stays self-describing for Duplicates,
exports, and the public projection's label index. Grains whose subjects
were added before the fix stay bare -- consider a backfill batch op, or
accept that a vocab re-enrich pass covers them.

## Done

- editor.ApplyOps takes a LabelResolver (nil = off): after the editorial
  patch, every IRI the patch asserted on a direct field whose annotation
  chain is exactly skos:prefLabel gets its vocabulary's labels (all
  languages -- the projection localizes from them) written into the grain's
  authority:<scheme> graph. Unresolvable IRIs stay bare links; additions are
  idempotent, and labels persist after the subject is removed (they describe
  the term, not the link).
- vocab.Index.LabelResolver() adapts the index (nil-safe); threaded from
  deps.Vocab through registerRecords, registerMARC (preview parity -- the
  650 shim sees the same grain a save produces), and batch.Service.Labels
  (appdeps).
- annotationLabel now picks ONE language for display (en, then untagged,
  then first tag -- the tasks/116 PickLabel order) instead of concatenating
  translations; the multi-value join within a language (e.g. role chains)
  is unchanged.
- Verified end-to-end on the playground: POST /v1/works/{id}/ops adding
  homoit0000934 wrote both prefLabels into authority:homosaurus and the doc
  annotates "LGBTQ+ science fiction"; Duplicates reads the same annotation.
- Backfill of pre-fix grains: not included; a batch re-assert of existing
  subjects (or a vocab re-enrich pass) covers them when wanted.
