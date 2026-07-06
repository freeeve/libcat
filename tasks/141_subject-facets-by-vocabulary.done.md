# 141 -- subject facets separated by vocabulary (scheme-aware subjects)

(Renumbered from 139, which collided with the pushed-hugo-tag task.)

Filed from queerbooks-demo (2026-07-06, Eve's report). Do not let a
queerbooks session edit this repo -- implement here.

## Symptom (both faces)

A corpus with two controlled vocabularies (homosaurus + FAST) facets them
into ONE "Subjects" dimension. Eve's sidebar:

    SUBJECTS 10064
    Fiction          11619   <- fast
    Lesbians          8094   <- fast
    Gay men           7673   <- fast
    Lesbians          6362   <- homosaurus (same label, different term!)
    Gay men           6137
    Gender identity   4020

Duplicate labels with different counts read as a bug. Worse on the hugo
side: term pages slug by LABEL (lcat-slug), so fast "Lesbians" and
homosaurus "Lesbians" collapse onto one /subjects/lesbians/ term page even
though facets.json keeps them distinct -- facet counts and term-page counts
disagree.

## Data is ready for this

project.Subject.ID is the full authority URI; scheme is derivable from the
namespace (id.worldcat.org/fast -> fast, homosaurus.org -> homosaurus,
id.loc.gov/authorities/subjects -> lcsh, ...). Nothing carries it forward.

## Suggested shape

- Projector: add `scheme` to project.Subject (namespace -> scheme table,
  overridable); facets.json groups subjects per scheme.
- Hugo adapter: mint one taxonomy per scheme (e.g. `homosaurus`, `fast`)
  when schemes are present -- or keep one taxonomy but slug scheme-prefixed
  (`fast/lesbians`) so term pages stop colliding; facet sidebar renders one
  collapsible group per scheme with a display name from site config.
- Backend work-index facet panel: same grouping (the editor already knows
  schemes -- chips show a HOMOSAURUS badge).
  [Resolution note: the admin SPA's work search has no facet panel today --
  nothing to group. When one grows, group subjects by project.SchemeForURI
  like the hugo sidebar; the addendum's filter box likewise applies to the
  hugo sidebar only for now.]
- queerbooks preference for defaults: homosaurus first (community
  vocabulary, the catalog's heart), then FAST; scheme display names
  "Homosaurus" / "FAST".
- Note [taxonomies] merge gotcha from tasks/133 in whatever config this
  grows.

## Addendum (same day, Eve): filter box inside the facet group

At 10k subject terms the sidebar is unscannable -- add a small type-to-
filter input per facet group (client-side substring over the already-
rendered facet entries is enough; no index needed). Applies to both the
hugo facets.html sidebar and the backend work-index panel. If facets.html
grows the input, mind the tasks/133 partialCached contract: the markup must
stay page-invariant, so the filtering is purely client-side JS.
