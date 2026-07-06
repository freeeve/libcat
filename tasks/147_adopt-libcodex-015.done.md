# 147 -- adopt libcodex v0.15.0 (their 086 locator labels + 089 native SKOS)

libcodex v0.15.0 ships both tasks this repo filed:

- **their 089** (from our tasks/136): the crosswalk reads SKOS-shaped
  subjects natively (prefLabel, untyped -> Topic), emits `$0` on 6xx from
  IRI subject nodes, derives the thesaurus from well-known IRI prefixes,
  and FromRecord keeps a URI-shaped 6xx `$0` as an IRI subject. Our
  bibframe/marcsubjects.go shim becomes dead weight -- delete it and rely
  on the native path (keep the DecodeGrainMARC e2e test as the
  equivalence proof).
- **their 086** (from our tasks/090): `Instance.ElectronicLocator` became
  a struct (breaking, 0.x); each 856 is a locator node carrying $3 as
  rdfs:label, $z/$y as bf:note(s). The editor's links field can annotate
  from the grain (rdfs:label chain), demoting the links.ts URL-shape
  heuristic to a fallback for label-less locators and pre-0.15 grains.

## Plan

- Bump both go.mods to libcodex v0.15.0 (no libcatalog code touches
  ElectronicLocator directly; the breaking change lands in the graph
  shape, which only libcodex reads/writes).
- Delete marcsubjects.go shim + its shim-internal test; keep/adjust the
  e2e MARC-output test to the native crosswalk's output.
- instance-ebook profile: links field gains annotation [rdfs:label];
  ProfileForm prefers the annotation over linkInfo()'s heuristic label.
- Release lockstep v0.20.0 after adoption.

## Done

- Both go.mods on libcodex v0.15.0; nothing here touches ElectronicLocator
  directly, so the breaking struct change cost zero code.
- marcsubjects.go shim deleted; DecodeGrainMARC relies on the native
  crosswalk. The e2e test (TestDecodeGrainMARCControlledSubjects) passes
  against native output unchanged: `650 _7 $a Label $2 homosaurus|fast $0
  <iri>`, unknown authority keeps $0 with no $2.
- One native gap found and worked around: the crosswalk mints one heading
  PER prefLabel with no language preference (Spanish label became a second
  650). Filed as libcodex tasks/091; until it lands, DecodeGrainMARC
  pre-filters each subject's prefLabels to the preferred language
  (en > untagged > first tag), decode-local -- ~30 lines replacing the
  ~150-line shim, delete when 091 ships.
- Links: instance-ebook's links field gained the rdfs:label annotation
  chain, so 856 $3 (Image/Thumbnail/Excerpt) reaches the doc from the
  grain; ProfileForm prefers the annotation over the links.ts URL-shape
  heuristic (kept as fallback for label-less locators and pre-0.15 grains,
  and it still drives thumbnail detection).
  TestLinksAnnotatedFromLocatorLabels proves the labels flow from the real
  OverDrive MARC samples; the golden grain round-trip is stable under the
  new locator node shape.
- Released as lockstep v0.20.0.
