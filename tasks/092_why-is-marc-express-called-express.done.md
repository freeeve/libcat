# 092: Investigate why OverDrive "MARC Express" is called *Express*

## Context / the surprise

While comparing OverDrive's two ingest routes (catalog-scale sizing, tasks/085)
we assumed "MARC Express" would be the **lighter/leaner** feed -- the name reads
like "express = fast, minimal, stripped-down." The measurements say the
opposite: **MARC Express is the richer route.**

Measured on real data this session:

| | OverDrive MARC Express (marc provider) | OverDrive Thunder JSON (overdrive provider) |
|---|---|---|
| quads / work grain | **~125** | ~65 |
| Summary (520) | yes | no (Item has no description field) |
| Notes (5xx: audience/performer/system/language) | yes (176 across QLL 91-work sample) | no |
| Genre/form (LCGFT) | yes | no |
| Series statement | yes | no (Item.Series dropped by crosswalk) |
| Relations / added entries (7xx) | yes | no |
| Extent / duration | yes | no |
| Electronic locators (cover, samples, links) | yes (856) | no |
| Controlled subjects (LCSH $0) | no (only $2 vocab, no authority URI) | no |

So "Express" is **not** about record size or field count -- the Express file is
close to a full library catalog record. The name almost certainly refers to
something else (delivery/automation speed, or the Marketplace product tier), not
"lightweight." This task is to nail down what "Express" actually denotes and fix
any place our docs/comments imply it means "lighter."

## Questions to answer

1. **What does OverDrive mean by "Express" in "MARC Express"?** Leading
   hypotheses to confirm/refute:
   - *Delivery speed / automation*: "Express" = automated, scheduled delivery of
     MARC records to the ILS (vs. manually requesting/generating records) --
     express *delivery*, not express *content*. This is the strongest prior.
   - *Product tier*: a named Marketplace feature ("MARC Express deliveries") that
     is free/included, vs. a paid fuller cataloging service (e.g. OCLC-sourced
     records, or OverDrive's premium/enhanced MARC).
   - *Turnaround*: records available shortly after a title is added, vs. a
     batch/backfile process.
2. **Is there a "fuller" OverDrive MARC tier** that Express is the lightweight
   sibling of? If so, how does *its* fidelity compare -- would adopting it change
   libcatalog's loss table (docs/marc-fidelity.md)? If "Express" is lightweight
   *relative to that tier* but still richer than Thunder JSON, document both
   comparisons.
3. **How does MARC Express compare to a true full record** (OCLC/LC MARC with
   authority-linked 6xx $0)? We already know Express lacks controlled subject
   URIs; enumerate what else a "real" catalog record carries that Express omits,
   so "richer than Thunder, lighter than OCLC" is precisely stated.
4. **Does the name history matter for our routing decision?** ARCHITECTURE / the
   Phase-0 decision routes OverDrive through the *direct Thunder->BIBFRAME* path
   and treats MARC as the "existing-ILS onboarding ramp" (tasks/007). But the
   measurements show Express is the higher-fidelity source. Re-examine: for a
   real deployment, is MARC Express actually the *preferred* full-collection
   source (it is the free full-collection route per prior notes), and is the
   "MARC is just a ramp" framing still right? (See docs/marc-fidelity.md
   "Why this validates the OverDrive architecture" -- that section leans on
   Express's *losses*; it should also acknowledge Express's *richness*.)

## Where it shows up in the repo (audit + correct)

- `docs/marc-fidelity.md` -- the "validates the OverDrive architecture" section
  frames MARC as the lossy detour; add the richness finding and the corrected
  meaning of "Express."
- `ingest/marc/marc.go:87` and other comments that mention "MARC Express" -- make
  sure none imply it is a lighter feed.
- Sample data: `ingest/overdrive/testdata/marc-express/od-sample-{ebook,audiobook}.mrc`
  (15 records) -- the vendored Express samples; a good corpus for enumerating
  field coverage precisely.
- Memory: `[[sibling-repos]]` notes MARC Express as "the free full-collection
  MARC route" (Admin -> MARC Express deliveries -> backdated file). Reconcile the
  final answer with that note.

## Research sources

- OverDrive Marketplace / OverDrive Resource Center ("Marc Express" help pages,
  "MARC Express deliveries" admin docs) -- the authoritative product definition.
- Library cataloging community (listservs, Reddit r/libraries, code4lib) on
  OverDrive MARC Express vs. fuller records.
- Compare the vendored sample records' actual field set to OCLC WorldCat records
  for the same titles to quantify "fuller record" gaps.

## Deliverable

- A short written answer (a paragraph in `docs/marc-fidelity.md` or a new
  `docs/overdrive-provider.md`) stating what "Express" denotes, with a source.
- Corrections to any comment/doc that implies Express = lighter.
- If the investigation changes the routing recommendation (Express as preferred
  full-collection source vs. onboarding ramp), note it for ARCHITECTURE / the
  Phase-0 provider decision.

## Note

Thorough task, filed 2026-07-04 off the tasks/085 sizing + provider-comparison
work. No code change is required to answer the question -- it is a
naming/product investigation whose output is documentation and a possibly-revised
routing framing. Related: [[sibling-repos]], tasks/007 (MARC ramp), tasks/085
(sizing), tasks/089/081 (entity decode).

## Resolution (2026-07-04)

**"Express" = delivery speed/automation, not record size.** Per OverDrive's own
[MARC Express](https://resources.overdrive.com/library/apps-features/marc-express/)
docs, the service auto-generates free "minimum MARC records" from publisher-supplied
metadata and delivers them the day after a content order (Admin → MARC Express
deliveries; backdated file = whole collection). The 2012 pitch was "fast-turnaround
MARC records"
([Library Journal](https://www.libraryjournal.com/story/overdrive-to-feature-fast-turnaround-marc-records-new-apis-for-opac-customization)).
It is express *delivery*, not express (stripped-down) *content*.

Answers to the four questions:

1. **What "Express" denotes:** rapid, free, ready-to-load, same-cycle delivery to the
   ILS. Confirmed by the OverDrive Resource Center page.
2. **Fuller OverDrive MARC tier?** No -- OverDrive ships only MARC Express (minimum).
   Full RDA/authority records come from paid *third parties* (OCLC, TLC eBiblioFile,
   BDS), which OverDrive itself points libraries to. Express is the lightweight
   sibling of third-party full records, not of a premium OverDrive feed -- so no new
   comparison row is owed to the loss table.
3. **Express vs a true full record:** Express omits LC/Dewey classification, the OCLC
   control number, and authority-linked `6xx $0` subject URIs (already in the
   uncontrolled-subjects note), i.e. no authority control / full RDA description.
   Precise statement: **richer than the current Thunder-JSON crosswalk output,
   lighter than a full OCLC/LC record.**
4. **Routing recommendation:** **unchanged.** The direct Thunder→BIBFRAME path stays
   preferred because it *models* the two framework-critical fields (037 Reserve ID,
   084 BISAC); the fidelity gap the sizing table showed is a property of our lean
   Thunder crosswalk (closable by enriching it), not of the JSON source. No
   ARCHITECTURE / Phase-0 change -- only the "MARC is a lossy detour" framing in
   docs/marc-fidelity.md was amended to also credit Express's richness.

Corrections made: `docs/marc-fidelity.md` gained a "What 'Express' means" section and
a richness caveat in "Why this validates the OverDrive architecture"; `[[sibling-repos]]`
memory clarified ("full-collection" = coverage, not full-fidelity). No code comment
implied "Express = lighter" (marc.go / cmd/lcat mentions are neutral), so no code change.
