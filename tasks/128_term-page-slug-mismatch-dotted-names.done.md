# 128 -- Taxonomy links 404 for terms with periods (link slug != Hugo term-page slug)

Filed from libcatalog-demo (user-reported:
https://libcatalog.evefreeman.com/contributors/kuang-r-f/ is a 404 -> renders as
an "empty" page via the error document).

tasks/023 (term URL-safety, marked done) doesn't cover periods. For a contributor
named "Kuang, R.F.":

- The module's link side (facets.html / detail pages, via `lcat-slug.html | urlize`)
  emits `/contributors/kuang-r-f/` -- urlize strips the dots.
- Hugo mints the actual term page at `/contributors/kuang-r.f./` -- Hugo's own
  term slugging KEEPS dots (and the trailing one).

Every dotted name is affected (`kuang-r.f.` vs `kuang-r-f`, `brite-poppy-z.` vs
`brite-poppy-z`); in the demo's 49 contributors at least those two 404 from every
facet link and work page. Non-dotted names match, which is why 023's testing
likely passed.

Fix direction: make one side agree with the other -- either link with the term
page's actual .RelPermalink (look the term up in site.Taxonomies instead of
reconstructing the URL from the name), or give term pages explicit url/slugs that
match lcat-slug (adapter/cascade). Linking via .RelPermalink is the robust one:
no slug function to keep in sync ever again. Add dotted-name fixtures
("Kuang, R.F.", "Brite, Poppy Z.") to whatever test covered 023.
