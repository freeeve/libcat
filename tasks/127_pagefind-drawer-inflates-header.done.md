# 127 -- Pagefind results drawer renders in-flow inside the header (page blows up)

Filed from libcatalog-demo (user-reported on the live site; screenshot showed the
header squeezed into a left column with the search results occupying the right
half of the viewport).

`search-pagefind.html` mounts the whole Pagefind Component UI -- input, filter
accordions, results list -- inside `.lcat-search-pagefind`, which sits in the
`.lcat-header` flex row. Nothing positions the drawer, so as soon as a query has
results (or filters render), the drawer participates in header layout: the header
grows to drawer height, its items re-align against it, and the page content
reflows. Any search on any site using engine="pagefind" hits this.

Two gaps, one fix each:

1. Position `.pagefind-ui__drawer:not(.pagefind-ui__hidden)` as an overlay
   dropdown anchored to the search box (absolute; below the input; themed
   surface/border/shadow; max-height + overflow-y; z-index above content).
   The demo is carrying exactly this as adopter CSS in its `lcat-theme.css`
   (tasks/022 there) -- lift it into lcat.css and the demo will drop it.
2. `--pagefind-ui-tag` is not mapped (module sets primary/text/background/
   border only), so result metadata chips keep Pagefind's light default
   background and are unreadable in dark mode. Map it to
   `var(--lcat-surface-alt)` alongside the other vars.

## Update (2026-07-06): more evidence; demo stopgap withdrawn

Follow-up user screenshot (browser-zoomed viewport) shows it is worse than a
sizing problem: with the Contributor filter expanded, the drawer paints filter
checkbox rows and the results column overlapping each other -- doubled/garbled
text ("Legends" superimposed on contributor labels), empty filter pills stretched
full-width. Confirmed a single PagefindUI mount on the page (one #lcat-pagefind,
one init), so this is the Component UI misrendering inside the constrained header
container, not a double-mount.

Recommendation upgraded: don't just absolutely-position the drawer -- move the
Component UI out of the header flow entirely (own region below the header, or a
modal), since its filter grid assumes page-scale width.

The demo repo briefly shipped an overlay-dropdown stopgap (its v0.9.1) and has
REVERTED it by owner policy: base-theme bugs get filed here and the base behavior
is left as-is. Nothing downstream papers over this now, so the fix lands cleanly.
