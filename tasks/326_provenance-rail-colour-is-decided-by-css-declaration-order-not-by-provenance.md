# 326 -- the provenance rail colour for a value asserted by more than one source is decided by CSS declaration order, not by provenance

Filed from libcat-e2e on 2026-07-10 (cross-repo ask).

A look-and-feel / UX defect, low severity (a 3px border colour), but a real one:
in a cataloging tool the provenance rail runs its precedence backwards, letting a
machine feed visually erase a human editorial assertion.

## Symptom

`ProfileForm` renders one field-value row per distinct value, wearing a
`ProvenanceBadge` for **every** source that asserted it (`mergeProv`,
tasks/196 -- "one display row wearing every provenance badge"). The row is a
`<li class="value">`, and `app.css` edges it with a coloured left rail meant to
show "the ink of whoever asserted it".

When a value carries more than one *kind* of badge, the rail colour is not chosen
by any provenance rule -- it is whichever `.value:has(.badge--…)` rule sits last
in the stylesheet.

Measured on `:8481`, read-only (opened two works in the editor; nothing saved),
2026-07-10:

| work | field | badges (left→right, as rendered) | rail colour | = token |
|---|---|---|---|---|
| `w62o34pka63lak` | language | `editorial`, `feed:marc` | `rgb(139,144,137)` | `--prov-feed` `#8b9089` (grey) |
| `w6h2fe3pe9pgmk` | language | `editorial`, `feed:marc` | `rgb(139,144,137)` | `--prov-feed` `#8b9089` (grey) |

`--prov-editorial` is `#1e6b4e` (green). The **editorial** badge renders first
(leftmost) on both rows, yet the rail is **feed grey**. The human editorial
provenance is present in the badges but erased from the rail.

### How common it is (stated honestly)

- `:8481` playground: **2 of 41** works have a cross-kind row (both editorial +
  feed on one value); 24 of 1153 value-rows carry multiple provenances, but 22 of
  those are multi-*feed* (all `.badge--feed`, so the rail is correctly feed).
- `:8501` queerbooks: **0 of 250** works -- every one of 3692 provenance values is
  `feed:*`, so the rail is never wrong there.

So the defect is real but rare, and the worse pairing -- editorial + *enrichment*,
where a machine enrichment would out-rank a human -- was not observed in either
corpus. It is nonetheless reachable: the cascade below out-ranks editorial under
both feed and enrichment.

## Root cause

`backend/ui/src/app.css:240-242`:

```css
.value:has(.badge--editorial) { border-left-color: var(--prov-editorial); }
.value:has(.badge--feed)      { border-left-color: var(--prov-feed); }
.value:has(.badge--enrichment){ border-left-color: var(--prov-enrichment); }
```

All three selectors have **identical specificity**, so when a row matches more
than one, the CSS cascade falls back to source order and the **last** rule wins.
Declaration order is editorial → feed → enrichment, so the effective precedence is:

```
enrichment  >  feed  >  editorial
```

i.e. the more machine-generated the source, the more it dominates the rail, and
`editorial` -- the human decision -- is the weakest. Nothing about the data or the
badge order influences the colour; reordering these three lines in the stylesheet
would silently change what every multi-source row looks like.

The badge component itself is fine (`components/ProvenanceBadge.svelte`); the
issue is purely the rail rule in `app.css`. The rail is derived by `:has()` off
the badge classes (the comment at `app.css:233-238` describes exactly this "needs
no extra markup" hook), so it inherited whatever order the three lines happened to
be written in.

## Why it matters

The rail exists to signal provenance at a glance, and the module's own comment
frames editorial provenance as something to preserve, not hide -- "an overridden
feed value keeps its rail but goes dotted: shadowed, not erased". Here an
editorial assertion is *erased* from the rail whenever a feed (or enrichment)
also touched the value. For a cataloger the editorial rail is the signal that a
human vetted a field; showing machine-feed grey instead understates exactly the
provenance a cataloging tool should surface most.

It is also fragile in the way tasks/294 (saved-query order) and the split
pin-order (tasks/323) are: a user-visible outcome is decided by an incidental
ordering (there, a random id; here, stylesheet line order) rather than by a stated
rule.

## Expected

The rail colour for a multi-source value should follow a **defined provenance
precedence**, not stylesheet declaration order. Given the tool's editorial-first
framing, `editorial` should not be the lowest-ranked; a natural precedence is
`editorial > enrichment > feed` (human decision first). Any of:

- raise `.value:has(.badge--editorial)` in specificity or move it last so it wins;
- or compute the rail from a single designated primary provenance in the component
  rather than off `:has()`, so the colour is data-driven;
- at minimum, make the rail match the leading (first-rendered) badge, so the colour
  and the badges never disagree.

## Repro

```
node harness/probe_prov_rail.mjs          # 2/3 -- P1 fails while the rail ignores editorial
node harness/retest.mjs                    # check t326
```

Both log in read-only, find a work whose `language` value carries both an
`editorial` and a `feed` badge (surveying the API first so the check is not tied to
one fixture id), open it in the editor, and read the rendered
`borderLeftColor` of that `.value` row. Nothing is written.
