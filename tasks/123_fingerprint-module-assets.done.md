# 123 -- Fingerprint module-linked assets (lcat.css, lcat-search.js, lcat-availability.js)

Filed from libcatalog-demo. The module's baseof links its assets by stable URL:

    {{ with resources.Get "lcat.css" }}<link rel="stylesheet" href="{{ .RelPermalink }}">{{ end }}

Because the URL never changes, adopters cannot give these files long browser cache
lifetimes safely: after a module upgrade, a returning visitor gets fresh HTML with
a cached old stylesheet. This bit the demo live right after the hugo/v0.5.0 bump --
cached lcat.css had no `.lcat-btn` rules, so the new `.lcat-btn` markup rendered as
bare links (dim green on the hero, effectively invisible). The demo's stopgap is
short-TTL cache headers for all css/js in its deploy script, which costs
revalidation requests on every page view.

Ask: pipe the assets through Hugo's fingerprinting --

    {{ with resources.Get "lcat.css" }}{{ with . | fingerprint }}
    <link rel="stylesheet" href="{{ .RelPermalink }}" integrity="{{ .Data.Integrity }}">
    {{ end }}{{ end }}

-- so the filename carries a content hash, upgrades change the URL, and adopters
can serve `max-age=31536000,immutable` for everything under the pattern. Same for
lcat-search.js and lcat-availability.js. (Subresource integrity comes along free.)
