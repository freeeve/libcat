# 133 -- hugo module: build performance at ~50k works (14-minute builds)

Filed from queerbooks-demo (2026-07-06). Do not let a queerbooks session edit
this repo -- investigate/implement here.

## Measurements (queerbooks-demo, first real large-corpus consumer)

- Corpus: 48,515 works / 54,763 instances; catalog.json 70MB, facets.json
  4.2MB; 10,064 subject terms, 45,347 contributor terms; bilingual en+es.
- hugo v0.148.2+extended, module hugo/v0.7.0, Apple Silicon laptop:
  **839.77s real / 1097.51s user / 114.09s sys**, public/ 1.7GB, 172,493
  HTML files. (Caveat: that run also had a suspected missing-es-tree issue
  under investigation queerbooks-side; en-only page volume may account for
  the 172k. Treat the 14 min as a lower bound for a correct bilingual
  build.)
- User/real ratio ~1.3 on a many-core machine -- the build looks badly
  underparallelized or template-bound, not I/O-bound.

## Things worth profiling (suggestions, not conclusions)

- `hugo --templateMetrics --templateMetricsHints` over the big corpus: which
  partials dominate. Candidates: facets.html (sidebar rendered per page?
  4.2MB facets data), work-card.html, head-seo.html.
- `partialCached` for the facet sidebar and any other page-invariant
  partials (vary key: section/language) -- if facets render per page that is
  ~100k+ renders of a large partial.
- The content adapter (_content.gotmpl) unmarshals the 70MB catalog.json
  once per language via resources.Get + transform.Unmarshal -- is that
  cached across languages, and how much of the 14 min is adapter time vs
  render time? (hugo --logLevel info timing lines / build stats.)
- Term-page volume: contributors alone mint ~45k term pages x 2 languages.
  A site-side lever already exists (drop facets), but check whether term
  pages render anything expensive per page.
- Taxonomy assignment cost: each work page carries 5-6 taxonomies; Hugo's
  taxonomy building at 100k pages may dominate -- measure before assuming.
- GC pressure at 70MB unmarshaled JSON x languages: try GOGC / HUGO memory
  limits guidance for adopters; document a recommended `hugo --gc
  --minify=false` baseline for large corpora.
- Consider documenting `renderSegments` guidance (e.g. segment per language)
  so CI can parallelize/shard large builds across jobs.

## Acceptance-ish

A documented large-corpus build recipe in the module README (flags, expected
wall-clock per 10k works, memory) plus whatever partialCached/template wins
profiling actually justifies. Target: meaningfully under the ~10 min/lang
observed, or a clear writeup of why Hugo's floor is what it is at this scale.

## Findings (2026-07-06, 10k-work subset of the real corpus, en-only)

Profiled with --templateMetrics and --profile-cpu on the actual queerbooks
catalog.json (first 10k works; full facets.json).

1. **Template layer (fixed in the module).** facets.html re-rendered per
   list/term page: 4m04s cumulative over 24,901 renders, 100% cache
   potential. work-card.html: 2m25s over 94,244 renders for 10k unique works.
   The tasks/128 lcat-term-url lookups ran 1.66M times, almost all inside
   those two. Now partialCached: facets per language (verified no
   cross-language leak -- es sidebar renders es labels), cards keyed by
   .RelPermalink, search/theme-toggle cached; nav and the adopter hooks
   deliberately NOT cached (page-dependent / override contract). Result:
   71s -> 57s wall, 168 -> 107 user-CPU-s.
2. **After caching, page/file VOLUME is the wall.** CPU profile: 39.5%
   syscall (writing 71,727 files), 23% doctree event bookkeeping, ~10% GC.
   Template execution is no longer dominant.
3. **Site levers (measured, now in README "Large catalogs"):** RSS off
   removes 18,419 files (sys -26%); dropping contributor/classification/
   language taxonomies: 57s/162cpu/71.7k files -> 21s/65cpu/25k files.
   Gotcha discovered: Hugo MERGES [taxonomies] across --config files -- an
   override file cannot remove a dimension (measured identical file counts);
   trim must happen in the main config.
4. **Non-findings:** the 70MB catalog.json unmarshal is ~1.1s (adapter is
   not the problem); wall-clock at this file volume is machine-noisy (same
   config measured 91s and 153s back-to-back) -- CPU seconds and file counts
   are the honest metrics, noted in the README.

Full-scale bilingual extrapolation: caching + both site levers should land
~50k works x 2 languages in the low single-digit minutes vs the observed 14;
not measured end-to-end (laptop noise), left to the queerbooks deploy to
confirm on real hardware.
