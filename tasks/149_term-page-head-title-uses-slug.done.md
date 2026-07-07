# 149 -- term page head <title>/og:title render the slug, not the display label

Filed from libcatalog-demo (tasks/024, 2026-07-06), seen adopting hugo/v0.21.0
with a schema v9 projection.

`layouts/term.html` resolves the human display title (subject label + vocabulary,
language name, classification label -- tasks/141/142) but only for the `<h1>`.
The SEO head (`_partials/head-seo.html:15` and the `og:title` meta) still uses
`.Title`, which for a taxonomy term page is Hugo's humanized term key. With
scheme-prefixed subject slugs that yields:

```html
<title>Homosaurus-Lgbtq-Books · Eve's Library</title>
<meta property="og:title" content="Homosaurus-Lgbtq-Books">
```

while the page h1 correctly reads "Subject: LGBTQ books (Homosaurus)".

Expected: head title matches the resolved display label ("LGBTQ books
(Homosaurus) · Eve's Library"). Likely fix: hoist the term.html title
resolution into a shared partial (or set the page title via a cascade/adapter)
so head-seo.html and term.html read the same value; languages and
classifications term pages have the same mismatch (e.g. `<title>Eng · ...`
vs an h1 of "Language: English").
