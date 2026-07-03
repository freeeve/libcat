# 059 -- UI keyboard core v2 + live kbd legend

## Context

First phase of the admin UX overhaul (keyboard-first, compact, drillable -- plan of 2026-07-03).
The scope-stack dispatcher in `backend/ui/src/lib/keyboard.ts` only handles plain unmodified
keys, so Cmd+K (App) and Cmd+S (WorkEditor) live as ad-hoc window listeners invisible to the
`?` help overlay. There is no persistent on-screen hint of what keys are live, unlike the
tag-review app's footer legend.

## Scope

- `keyboard.ts` v2: one grammar for plain keys (`"j"`), modifier chords (`"mod+s"`, mod =
  meta or ctrl), and two-step sequences (`"g w"`, 900ms window). Mod chords bypass the
  form-control guard; plain keys keep it. `BindingSpec` gains `legend` (short footer label)
  and `hidden` (alias keys excluded from footer + help). `keymapVersion` store bumps on any
  scope/binding change; `legendBindings()` collapses sequences into one `prefix …` entry.
- New `components/KbdLegend.svelte`: fixed footer ("drawer rail") rendering the live
  bindings as `kbd` chips; cross-fades on scope change (reduced-motion gated).
- App: declarative `mod+k` palette binding replaces the window listener; global `g`
  sequences (`g d/w/a/q/b/m/e/i/u/p`) navigate to every screen; legend mounted when
  signed in.
- WorkEditor: `mod+s` binding replaces the window listener.
- Existing scopes (works, authorities, queue) get `hidden` on alias keys and short legends.
- Tokens: `--legend-h`; SaveBar/PublishBar sit above the legend.

## Acceptance

- Chord normalization + sequence timeout + legend alias-hiding covered in keyboard.test.ts.
- Cmd+S still fires while focus is in a form input; plain keys still do not.
- `?` overlay and footer legend derive from the same registry.
- `npm run check`, `npm run test`, `npm run build` green; axe suite green.
