#!/bin/sh
# End-to-end run of the RoaringRange client browse path (tasks/158): builds the
# exampleSite with the roaringrange engine, emits the search/browse artifacts
# from the fixture catalog, serves over a Range-capable server (required by the
# reader; python http.server is not one), and drives it in headless Chromium.
# A second pass rebuilds under the minimal profile (disabled taxonomy/term
# kinds + shared sidebar) and drives the hydrated-sidebar facet UI (tasks/170).
#
# Needs: hugo, go, node, and Playwright with Chromium. If playwright is not
# npm-resolvable, point PLAYWRIGHT_PKG at an install, e.g. from the npx cache:
#   npx playwright install chromium
#   PLAYWRIGHT_PKG=$(ls -d ~/.npm/_npx/*/node_modules/playwright/index.js | head -1) ./run.sh
set -e
here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/../.." && pwd)
out=${TMPDIR:-/tmp}/lcat-e2e-$$
port=${PORT:-8510}
minport=$((port + 1))
bigport=$((port + 2))
trap 'kill $srv $minsrv $bigsrv 2>/dev/null || true; rm -rf "$out" "$out-min" "$out-big" "${out}-lcat" "${out}-big-catalog.json"' EXIT

printf '[params.search]\n  engine = "roaringrange"\n' > "${out}.toml"
(cd "$repo/hugo/exampleSite" && hugo --quiet --config hugo.toml,"${out}.toml" --destination "$out")
rm "${out}.toml"

(cd "$repo" && go build -o "${out}-lcat" ./cmd/lcat)
"${out}-lcat" index --catalog "$here/fixture-catalog.json" --out "$out/search"

node "$here/range-server.mjs" "$out" "$port" &
srv=$!
sleep 1
node "$here/browse.spec.mjs" "http://127.0.0.1:$port"

# The generic section index (tasks/290) must not be a Works browse. Driven against
# this same roaringrange build, so "did not hydrate" means the reader declined
# rather than never booted -- the spec's first check is /works/ hydrating.
node "$here/section-index.spec.mjs" "http://127.0.0.1:$port"

# Minimal profile (tasks/157): no taxonomy/term pages, shared sidebar. The
# sidebar's unlinked rows must hydrate into reader toggles and replace the
# fallback panel (tasks/170), including the exclude toggles (tasks/173) --
# negatives is on in the exampleSite config, restated here so this pass never
# depends on the params merge strategy.
printf 'disableKinds = ["taxonomy","term"]\n[params.search]\n  engine = "roaringrange"\n[params.facets]\n  shared = true\n  negatives = true\n' > "${out}.toml"
(cd "$repo/hugo/exampleSite" && hugo --quiet --config hugo.toml,"${out}.toml" --destination "$out-min")
rm "${out}.toml"
cp -R "$out/search" "$out-min/search"

node "$here/range-server.mjs" "$out-min" "$minport" &
minsrv=$!
sleep 1
node "$here/browse-minimal.spec.mjs" "http://127.0.0.1:$minport"

# Base-set scope (tasks/281). The fixture catalog has three works, so it cannot
# tell "the base set is the match set" from "the base set is its first page" --
# the page size is 60. This pass indexes a 600-work catalog instead, and forces a
# static paginator on screen (three works, pagerSize 2) so the check that browse
# hides it cannot pass by there being no pager.
printf '[pagination]\n  pagerSize = 2\n[params.search]\n  engine = "roaringrange"\n' > "${out}.toml"
(cd "$repo/hugo/exampleSite" && hugo --quiet --config hugo.toml,"${out}.toml" --destination "$out-big")
rm "${out}.toml"
node "$here/make-large-catalog.mjs" "${out}-big-catalog.json"
"${out}-lcat" index --catalog "${out}-big-catalog.json" --out "$out-big/search"

node "$here/range-server.mjs" "$out-big" "$bigport" &
bigsrv=$!
sleep 1
node "$here/browse-scope.spec.mjs" "http://127.0.0.1:$bigport"

# Result pager (tasks/301). Same 600-work build: the reader must reach past the
# first page of a match set, the pages must partition it, a page must be
# deep-linkable, and clearing must restore the static list and its corpus pager.
node "$here/browse-pager.spec.mjs" "http://127.0.0.1:$bigport"

# Facet selections in the URL (tasks/349). A faceted page must reconstruct from a
# cold ?f=/?x= deep link (alongside ?q= and ?page=), pager links must carry the
# facet, and clearing must drop it from the URL.
node "$here/browse-facet-url.spec.mjs" "http://127.0.0.1:$bigport"
