# 166 -- Adopt the cat-on-books logo (README + default site theme)

Left by the queerbooks-demo session 2026-07-08 (uncommitted cross-repo note;
the SVGs are already dropped in this repo, also uncommitted -- commit both
when adopting).

Eve had a cat-sitting-on-books logo drawn for libcat: a single-color
silhouette (cat in profile, raised curled tail, three-book stack), transparent
background, viewBox 0 0 512 512, no external refs. Two files at
`hugo/static/logo.svg` (black) and `hugo/static/logo-dark.svg` (white), same
geometry -- only the fill differs.

## Work

1. README: show the logo at the top (GitHub supports the
   `#gh-light-mode-only` / `#gh-dark-mode-only` fragment trick, or a
   `<picture>` with `prefers-color-scheme` sources, so each GitHub theme gets
   the right variant).
2. Default site theme (hugo module): wire it as the default brand mark --
   header/nav partial and favicon. Two options:
   a. Keep the two static files and swap via CSS
      (`@media (prefers-color-scheme: dark)` / the theme's dark-mode class).
   b. Cleaner: inline the paths as one SVG partial with
      `fill="currentColor"`, so it follows the theme's text color for free
      and the static files stay for README/external use.
   Leave it overridable the usual Hugo way (site can shadow the partial or
   the static file) so downstream deployments (queerbooks etc.) can rebrand.
3. Favicon: emit an .ico/PNG render or serve the SVG favicon directly
   (`<link rel="icon" type="image/svg+xml">`); dark-mode-aware SVG favicons
   can embed the media query in a <style> block inside the SVG.

## Notes

- Geometry is hand-authored (head+ears as one path so the ear notch
  survives; tail is a stroked path with round caps). Edit coordinates
  directly if you want to tweak the pose.
- If you adopt the currentColor partial (2b), consider regenerating
  logo-dark.svg from logo.svg at build time (sed the fill) instead of
  keeping two sources.
