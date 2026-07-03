# 075 -- Keyboard remapping, MARC editor op family, keymap presets

## Context

Koha's advanced editor ships a shortcut table (copy/cut field or subfield to
a clipboard, paste, insert (c)/(p)/delimiter, copy field on next line, link
to authorities, help on current subfield, save) and lets users redefine the
chords. Our keyboard layer (tasks/059/065) routes everything through one
registry in `keyboard.ts`, and the legend, "?" overlay, and `formatKey` all
render from the registered chord -- so a remap layer applied at bind time
propagates everywhere. Koha's default chords are not adoptable as-is:
Ctrl-C/Ctrl-X shadow system copy/cut (our grid uses real inputs where native
clipboard must keep working), and Ctrl-D/Ctrl-P/Ctrl-S are browser
bookmark/print/save. Adopt the operations with sane defaults; offer the Koha
chords as an opt-in preset.

## Scope

1. **Remap layer**: bindings gain a stable action id (`scope:default-key`
   implicitly; no call-site changes). `bindKeys` consults a user keymap and
   re-keys on registration. Keymap persists in localStorage; per-user server
   sync is future work. Reset-to-default per binding and wholesale.
2. **Redefine UI**: the "?" overlay grows an edit mode -- select a binding,
   press the new chord, conflict check against the binding's scope plus
   global, persist. Reserved chords (mod+c/x/v, browser-critical keys)
   refuse with an explanation.
3. **MARC editor op family**: copy field / cut field / paste field (an
   app-level field clipboard), duplicate-on-next-line (exists as Alt+D),
   insert (c) and (p) and the subfield delimiter, help on current field
   (opens the tasks/074 LOC page for the row's tag). Routed through the
   registry with an allow-in-inputs flag so they fire while focus is in the
   grid's form controls (today `MarcGrid` binds Alt+D via a local keydown,
   invisible to the registry, legend, and remapping).
4. **Presets**: named keymaps applied as a bundle -- "default" and
   "koha-advanced-editor" (Koha's table, minus the reserved chords, which
   stay on our defaults and are called out in the preset UI).

## Acceptance

- Remapping copycat's "pick" from x to p updates the footer legend and "?"
  overlay, survives reload, and x reverts to inert.
- Assigning a chord already bound in the same scope (or global) is refused
  with the conflicting action named; mod+c is refused as reserved.
- With focus in a MARC value input, the field-clipboard chords fire; plain
  navigation keys still type normally.
- Applying the Koha preset rebinds the op family (e.g. copy-field on
  shift+mod+c where safe) in one action; resetting restores defaults.

## Outcome

- `keyboard.ts`: bindings carry `id` (`scope:default-key`), `defaultKey`,
  `allowInInputs`, `legendHidden`; `bindKeys` re-keys through the
  localStorage keymap (`lcat.keymap`); `setKeymapEntry` re-keys live
  registrations in place (conflict/reserved checked, resets included);
  `conflictingBinding`/`reservedReason`/`applyKeymap`/`resetKeymap` back the
  UI. `normalizeChord` now reads the letter from `ev.code` under alt/mod
  (fixes macOS Option glyphs -- Alt+D was "∂" before) and keeps shift as a
  segment beside another modifier (`mod+shift+c`).
- `keymaps.ts`: `default` and `koha-advanced-editor` presets; per-entry
  apply so conflicts skip rather than fail, skips reported.
- `KeyboardHelp.svelte`: overlay edit mode -- click a key, press the chord,
  refusals name the reserved reason or the holding action; per-binding and
  wholesale reset; preset picker.
- MARC op family in `MarcGrid` (scope passed from WorkEditor via MarcPanel):
  alt+c/x/v copy/cut/paste through the shared `fieldClipboard.svelte.ts`
  (newest-first, capped, cloned entries -- the tasks/076 pane consumes it),
  alt+d duplicate (now registry-routed, remappable, and actually working on
  macOS), alt+g/© alt+r/℗ alt+k/$ caret inserts, alt+h LOC field help; all
  `allowInInputs` so they fire from the grid's form controls.
- Per-user server sync of the keymap remains future work, as scoped.
