# 076 -- MARC text-mode editor (Koha advanced-editor style)

## Context

Koha's advanced editor presents the whole record as an editable text buffer:
one line per field, subfields typed inline with a visible delimiter, fixed
fields rendered as inline widgets, cut fields accumulating in a clipboard
pane. Expert catalogers are much faster in this mode than in a form grid.
Our write path makes this a frontend-only alternate surface: `MarcPanel`
(tasks/049) already loads `MarcRecordDoc`s, dry-runs edits to a quad delta,
and saves under If-Match -- text mode serializes the same doc to mrk-style
text and parses it back into the identical save path. Macros (tasks/047)
and the field-op keyboard family (tasks/075) apply to both surfaces.

## Scope

1. **Serializer/parser**: `MarcRecordDoc` <-> mrk-style text (`245 14 $a
   The Dutch house : $b a novel /`; LDR and control fields as raw lines).
   Round-trip is lossless over every doc the grid can hold, including the
   lossy-tag annotations. Parse failures report per line (bad tag,
   missing/duplicate indicators, subfield syntax) without discarding the
   buffer.
2. **Text mode toggle in MarcPanel**: grid and text are two views of the
   same in-memory doc; switching re-serializes, so edits carry across.
   Plain textarea, no editor dependency; monospace, per-line error gutter.
   Lightweight overlay syntax highlighting only as a later follow-up if the
   textarea proves insufficient.
3. **Fixed-field builders from text mode**: LDR/006/007/008 lines get an
   affordance that opens the existing `FixedFieldGrid` builder against that
   line; the result writes back into the buffer. No inline-widget
   reimplementation.
4. **Field clipboard pane**: cut/copied fields (tasks/075 ops) show in a
   side pane and paste back into either surface.

## Acceptance

- Grid -> text -> grid round-trips a record byte-identically (field order,
  indicators, subfield text, lossy annotations preserved).
- A syntactically broken line blocks save with a line-anchored error; other
  lines stay editable.
- Editing 008 via the builder from text mode updates the buffer line; the
  quad-delta preview matches the same edit made in grid mode.
- Cut in grid mode, paste in text mode (and vice versa) inserts the field.

## Outcome

- `lib/mrk.ts`: `serializeRecord`/`serializeField`/`parseRecord`. Lines are
  `LDR <leader>`, `TAG <raw>` for control fields (trailing 008 blanks
  survive), `TAG II $a … $b …` for data fields using the grid's own
  subfield-line helpers, so the two surfaces share one syntax by
  construction. Parse errors (bad tag, missing/bad indicators, no subfield
  content, duplicate LDR) are 1-based line-anchored; any error withholds the
  record so the buffer stays authoritative. Lossy annotations reattach from
  knownLoss by tag.
- `MarcTextEditor.svelte`: plain monospace textarea + scroll-synced line
  gutter that tints error lines (message on hover) and lists them below;
  every clean parse flows into the shared doc via onchange, onvalid gates
  the host. The "agreed text" handshake re-serializes the buffer only for
  outside changes (mount, clipboard-pane paste), never mid-typing. LDR/006/
  007/008 lines get "positions" buttons opening the existing FixedFieldGrid
  against that line, writing back into the buffer. Caret-line field ops
  (alt+c/x/v, alt+h help) register in the host scope with the same action
  ids as the grid's -- one surface mounted at a time, remaps hit both.
- `FieldClipboardPane.svelte`: entries newest-first as text lines with
  paste/remove/clear; hosted by MarcPanel above either surface, pasting
  appends to the shared doc (text mode re-serializes off it).
- `MarcPanel.svelte`: Grid|Text toggle; parse errors disable Preview/Save
  and the switch back to grid until fixed.
- Overlay syntax highlighting deliberately deferred, as scoped.
