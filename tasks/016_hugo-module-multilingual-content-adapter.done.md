# 016 -- Make the Hugo module's content adapter work multilingual out of the box

> Filed by the qllpoc session as a cross-repo handoff (uncommitted, per the repo
> boundary). Surfaced wiring the module into a bilingual (en+es) site (qllpoc
> `tasks/045`). Related to `tasks/005` (multilingual search), distinct concern
> (rendering).

## Problem

The module's content adapter (`hugo/content/works/_content.gotmpl`) already
localizes data -- it picks `subjects[].labels[site.Language.Lang]` (tasks/012).
But a **Hugo module's `content/` mounts to the default content language only**, so
in a multi-language site the adapter runs for the default language and every other
language gets **zero Work pages** (verified in qllpoc's bridge: en 20,717 pages,
es 15 before the workaround).

The consumer workaround is ugly: **shadow the module's adapter** into the site's
own `content/works/_content.gotmpl` (a byte-for-byte copy -- drift risk against the
schema the adapter parses) and re-mount `content` per language:

```toml
[[module.mounts]]
  source = "content"
  target = "content"
[[module.mounts]]
  source = "content"
  target = "content"
  lang = "es"
# + re-declare assets/i18n/layouts/static/data (declaring mounts disables defaults)
```

Copying the adapter defeats the point of shipping it in the module.

## Ask (pick one)

1. **Best: the module mounts its own content for every configured language.** If a
   module can self-declare `content` mounts per `site.Languages` (or Hugo already
   runs a module content adapter per language and this is a mount-config gap we can
   fix in the module's `hugo.toml`), do that so importing sites get multilingual
   Work pages with **no local adapter copy**.
2. **If Hugo can't do (1) from the module:** document the exact consumer mount
   block in `hugo/README.md` **and** expose the adapter body as an includable
   partial (e.g. `_partials/work-pages.gotmpl`) that a site's one-line
   `content/works/_content.gotmpl` calls, so the site owns only a stub, not a copy
   that drifts.

## UI chrome i18n (adjacent, smaller)

The module's templates hardcode English chrome ("Subjects", "Formats",
"Languages", "Search the catalog", "Editions & formats", …). A multilingual
adopter must override `facets.html` / `search.html` / `page.html` to swap in
`{{ i18n }}` (qllpoc did, with its own `i18n/*.toml`). Consider having the module
templates use `{{ i18n }}` with shipped `i18n/en.toml` defaults, so adopters
translate by adding tables, not by forking templates.

## Acceptance

- [x] A two-language site importing the module gets a full Work-page set in **each**
  language with **no** copied `_content.gotmpl`.
- [x] (chrome) Facet/detail chrome localizes via i18n tables without template forks.

## Delivered (commit pending)

**Ask 1 solved better than proposed -- zero site config.** Hugo content adapters have
an **`.EnableAllLanguages`** method (docs: "Create one content adapter for all
languages"): calling it makes the single adapter run once per configured language, each
run seeing its own `site.Language.Lang`. So `hugo/content/works/_content.gotmpl` gained
one line -- `{{- .EnableAllLanguages -}}` -- and a multi-language importing site now
gets a full Work-page set in every language with **no per-language content mount, no
component-mount redeclaration, and no copy of the adapter** (the "ugly workaround" the
task describes is fully removed; the fallback partial/stub of Ask 2 is unnecessary).
Verified empirically on Hugo v0.148.2 against a bilingual (en+es) site: es went from
**0 -> full** Work pages, and per-language data localization works (en detail shows
"Transgender people", es shows "Personas trans").

**UI chrome i18n.** The templates' hardcoded English chrome (facet titles, search form,
detail-page section headings, `Facets`/`Filter works` aria-labels, `Back to all works`,
`N works` counts, the authority-link aria-label) now come from `{{ i18n }}` keys. The
module ships **`hugo/i18n/en.toml`** as the defaults (Hugo merges a module's i18n bundle
into importing sites); an adopter adds `i18n/<lang>.toml` with the same keys -- omitted
keys fall back to the default content language, so a partial table still builds. **No
template fork.** `workCount` uses CLDR one/other plural forms.

**Demo + guard: exampleSite is now bilingual (en + es).** `exampleSite/hugo.toml` declares
`[languages]` en+es; `exampleSite/i18n/es.toml` translates the chrome (the adopter
workflow). Build: `/works/` English, `/es/works/` Spanish (chrome + subject labels);
`<html lang>` correct per language; a11y audit now covers **77 pages** (was 36) with **0
WCAG violations**; 17 availability JS tests still pass.

**Known minor gap (documented in README):** taxonomy term-page headings derived from the
taxonomy *name* itself (`term.html`'s `Subject:` prefix via `.Data.Singular`, the
`taxonomy.html` "N subjects" term count via `.Data.Plural`) still use Hugo's config-defined
taxonomy singular/plural, not `i18n` keys -- localizing those needs a taxonomy->key map or
a `term.html`/`taxonomy.html` override. Out of scope for the chrome pass; noted for adopters.

## Refs

- `hugo/content/works/_content.gotmpl`, `hugo/hugo.toml` (`[module]`),
  `hugo/layouts/{page,_partials/facets,_partials/search}.html`; `tasks/005`
  (multilingual search); qllpoc `tasks/045`, `site-graph/` (the bilingual bridge
  + the workaround this would remove).
