# 126 -- Pagefind search UI ignores the ?q= the SearchAction JSON-LD advertises

Filed from libcatalog-demo (noticed verifying its tasks/021).

`head-seo.html` (tasks/119) emits WebSite JSON-LD with a SearchAction whose
`urlTemplate` is `{baseURL}works/?q={query}` -- but `search-pagefind.html` never
reads `?q=` on load: landing on `/works/?q=dragons` renders the plain works list
with an empty search box. So the deep link the site itself advertises to crawlers
(and anything else that constructs a search URL) is a no-op. The interim
substring-filter path (`lcat-search.js`) has the same gap if it also ignores the
param.

Ask: on the works list, read `q` from `location.search` at init; if present,
populate the search input, run the query, and keep the URL in sync (pushState) as
the user types -- making search results shareable/bookmarkable as a bonus.
Alternatively (weaker), drop the SearchAction from the JSON-LD so the head does
not advertise a capability the page lacks.
