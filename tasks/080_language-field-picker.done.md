# 080: Language fields -- names and pickers instead of raw typing

Two raw-entry warts in the work editor (maintainer screenshots, 2026-07-04):

- The LANGUAGE field renders stored values as `id.loc.gov › eng` and adding
  one means pasting a full `http://id.loc.gov/vocabulary/languages/…` URI.
- The summary (langLiteral) row asks for a hand-typed `lang (en)` tag.

Both get the rdaterms treatment (closed list shipped as data + picker with
an "Other…" escape -- the pattern flagged pending in the design notes).

## Plan

- `lib/languages.ts`: the current MARC language code list (485 terms,
  generated from the id.loc.gov/vocabulary/languages bulk JSON) as
  `RdaTerm`s -- a "common" optgroup fronting the full alphabetical list --
  plus `languageTerm(iri)` and `LANG_TAGS` (common BCP-47 primary tags).
- ProfileForm: Language field gains `options: LANGUAGES` (existing select +
  "Other IRI…" machinery); stored IRIs render as the language name with the
  code badge via a combined rdaterms/languages lookup.
- langLiteral rows: the free-text lang box becomes a select of common tags
  with "Other tag…" free entry; empty stays "no language tag".
