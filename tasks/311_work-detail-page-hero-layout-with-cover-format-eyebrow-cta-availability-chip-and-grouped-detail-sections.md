# 311 -- work detail page -- hero layout with cover, format eyebrow, CTA, availability chip, and grouped detail sections

Opened 2026-07-10, from a design reference the maintainer supplied ("I also liked
this layout that I had on the opac"). Sibling of tasks/310, which did the rail on
the same page.

## The reference, read off the screenshot

Top to bottom, on a Work detail page:

1. **Hero**, two columns on a wide screen: cover image left, everything else right.
   - Format **eyebrow** above the title, small caps, muted: `EBOOK`.
   - `<h1>` title, then the subtitle as a separate muted line beneath it -- not the
     `title: subtitle` run-on the current `page.html` renders inside the `<h1>`.
   - Contributors with roles: `Patrick Bex (Author)`, name linked.
   - A primary **CTA button** (`Open in Libby →`) beside an **availability chip**
     (`27 holds · several months wait`), both pill-shaped.
   - **On these lists**: chips linking to curated lists
     (`Ace Spectrum Representation`).
2. **About this title** -- section rule + small-caps heading, then the summary
   paragraphs.
3. **Details** -- a multi-column definition grid, not the current single `<dl>`:
   `PUBLISHER | IMPRINT | PUBLISHED | LANGUAGE` on one row, `FORMATS | ISBN` on the
   next, labels in small caps above their values, each cell underlined by a rule.
4. **Subjects** -- chips (rounded, tinted), not the current bare list.
5. **Homosaurus subjects** -- grouped **by broader term**, the broader term as a
   small-caps column heading with its narrower terms beneath, laid out in columns.
   Followed by a `Suggest a Homosaurus subject` link.
6. **More like this** -- done in tasks/310.

## What is already there

`page.html` renders every one of these facts. This is a layout and typography task,
not a data task, with these exceptions:

- **The format eyebrow** wants one carrier for the Work. `.Params.formats` is a
  list; tasks/310 already decided what to do when it has more than one (nothing).
  Reuse that rule, or hoist it into a partial both can call.
- **Publisher / imprint / published date** -- confirm the projector emits these per
  Instance. `docs/` says the detail page shows `formats`, `language`,
  `classification`; the reference wants publisher, imprint, publication date and
  ISBN too. If they are not in `catalog.json`, that is a projector change and this
  task should be split.
- **The CTA and the availability chip** are `lcat-availability.js` territory
  (tasks/004, tasks/288): the chip's text (`27 holds · several months wait`) is a
  richer render of what the availability adapter already returns, and the button's
  href is a provider deep link. Today the layout has an inert
  `<span class="lcat-availability">` placeholder. The reference gives it a shape.
- **On these lists** implies a curated-list concept. Check whether the reference
  site's lists were tags, a taxonomy, or a queerbooks-side extra. If it is an
  adopter concept, the module's job is the `work-extra.html` hook, not the chips.
- **Grouping Homosaurus subjects by broader term** needs the `terms` sideband's
  `broader` edges, which `catalog.json` already carries (tasks/178). The grouping
  is a template concern. A subject with several broader terms, or none, needs a
  stated rule.

## Constraints carried from the sibling tasks

- Restyling must not resurrect the `[hidden]` bug (tasks/303): any element the
  layout hides must stay hidden under an author `display`.
- Any assertion that something is absent needs a control asserting something else
  is present (tasks/304, tasks/308).
- Verify in a browser over HTTP, not `file://`: root-relative CSS does not load and
  every measurement is taken against an unstyled page (learned in tasks/310).
- The exampleSite is bilingual; check `/es/` for anything touching i18n.
