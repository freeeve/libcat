# 074 -- Copycat fielded search, result MARC preview, LOC field help

## Context

Koha's Z39.50/SRU search offers per-field access points (ISBN, ISSN, title,
author, subject, keyword, LCCN, ...) against selectable targets, a MARC
preview of each hit before importing, and per-field "?" links from the MARC
editor to the LOC MARC 21 documentation. Copycat (tasks/050) already covers
target config, profile-scoped multi-target fan-out, and stage->review->commit;
its search is a single free-text string sent as `sru.Quote(query)` /
`z3950.Term("any", query)`. libcodex already exposes what fielded search
needs: `bib1Use` maps title/author/subject/isbn/issn/lccn/id to bib-1 use
attributes, and both protocols have `Term(index, term)` plus `And`/`Or` --
no libcodex changes required for these access points.

## Scope

1. **Fielded search (backend)**: extend the copycat search request from a
   bare string to a set of (index, term) pairs ANDed together, with indexes
   limited to what libcodex supports on both protocols: `any`, `title`,
   `author`, `subject`, `isbn`, `issn`, `lccn`, `id`. `protocolSearch`
   assembles the per-protocol query (CQL for SRU, RPN for Z39.50). A bare
   string stays valid and means `any`.
2. **Fielded search (UI)**: CopyCat screen grows a compact multi-field form
   (keyword + the fielded inputs, collapsed by default behind an "advanced"
   toggle so the single search box stays the fast path). Enter anywhere
   searches; empty fields are omitted.
3. **Result MARC preview**: results already carry the full RecordDoc --
   selected result gets an expandable MARC preview reusing MarcPreviewPane
   (tasks/070), so a cataloger can inspect before staging. Same affordance
   in the staged-batch review list.
4. **Edition + LCCN result columns**: add 250$a (edition) and 010$a (LCCN)
   to `SearchResult` and the results row.
5. **LOC field help links**: in the MARC grid/panel/preview, each tag links
   to its LOC MARC 21 Bibliographic page
   (`https://www.loc.gov/marc/bibliographic/bd{tag}.html`, new tab).
   Handle the grouped pages (09X -> bd09x.html, 59X -> bd59x.html; LDR ->
   bdleader.html); unknown/local tags get no link rather than a 404.
   Authority MARC views (tasks/046) link to
   `https://www.loc.gov/marc/authority/ad{tag}.html`.

## Out of scope / future

- Pub-year, Dewey, and LC-call-number access points: no bib-1 mapping in
  libcodex yet; if wanted, raise as an uncommitted task file in libcodex.
- "New record from nothing" (hosting non-Libby works): defer; dovetails with
  tasks/066 external-work-identity-links, since such works need an identity
  not derived from the OverDrive feed.

## Acceptance

- Searching ISBN=9780062963673 against a configured target returns only
  matching hits on both an SRU and a Z39.50 target; combining title+author
  ANDs the terms.
- A picked result shows its full MARC before staging; the batch review shows
  the same per record.
- Results list edition and LCCN when the record carries 250$a/010$a.
- Clicking a 246 tag in the work editor's MARC view opens
  loc.gov/marc/bibliographic/bd246.html in a new tab; a 599 tag goes to
  bd59x.html; a made-up tag is plain text.

## Outcome

- Backend: `copycat.FieldTerm` + `searchTerms` normalization (bare query =
  `any`, fields AND on, unknown index/empty term refused); `SearchFunc` now
  takes the term list; `sruQuery`/`z3950Query` assemble CQL/RPN per protocol,
  with lccn passed to SRU as `bath.lccn` (the dc set has no LCCN index).
  `SearchResult` gains `edition` (250$a) and `lccn` (010$a). The tasks/073
  subject lookup now searches the `isbn` access point instead of free text.
- UI: advanced toggle on the CopyCat screen with seven fielded inputs
  (collapsed fields are ignored so leftovers can't narrow a quick search);
  new shared `MarcRecordView` renders a RecordDoc read-only (extracted from
  MarcPreviewPane) and backs "v"-key MARC previews in both the results list
  and the batch review; `lochelp.ts` maps documented tags to LOC pages
  (bd/ad, 09X/59X groups, LDR -> bdleader) with curated tag sets so local
  tags stay plain; MarcGrid rows and the LDR row carry a "?" help link.
- Note: no authority MARC record view exists in the UI yet; `locFieldHelpUrl`
  already speaks `kind="authority"` for when one lands.
