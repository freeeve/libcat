/*
  Unit tests for the browse sidebar's full-vocabulary facet filter
  (lcat-browse.js treeifySidebar/wireFlatFilter). Run: node
  hugo/facet_filter_test.cjs (exit 0 = pass). Lives at the module root so Hugo
  never publishes it.

  lcat-browse.js reads the corpus through the RoaringRange wasm reader, which
  jsdom cannot run; the reader is mocked to the same facets() contract the real
  one exposes ([{field, cats:[{name,count}]}]). The fragment prints only the
  top rows of each group, so the test proves the filter matches a category that
  the reader holds but the fragment never rendered -- the flat-group bug where
  a real contributor came back "0 matches" (task 470).
*/
const assert = require("node:assert");
const fs = require("node:fs");
const { JSDOM } = require("jsdom");

// Swap the wasm import for window-provided mocks so the module evaluates as a
// plain script under jsdom.
const SRC = fs
  .readFileSync(__dirname + "/assets/lcat-browse.js", "utf8")
  .replace(
    /^import .*roaringrange\.js";$/m,
    "const init = window.__rr.init, RrsCatalog = window.__rr.RrsCatalog, RrfFacets = window.__rr.RrfFacets, RrsRecords = window.__rr.RrsRecords;",
  );

let passed = 0;
const tests = [];
function test(name, fn) {
  tests.push({ name, fn });
}
const settle = () => new Promise((r) => setTimeout(r, 30));

// 25 contributors: 20 common (top by count) the fragment renders, then 5 more
// -- including "Baldwin, James" -- that the reader holds beyond the rendered
// slice.
const RENDERED = Array.from({ length: 20 }, (_, i) => ({
  name: "Common, Author " + i,
  count: 100 - i,
}));
const HIDDEN = [
  { name: "Baldwin, James", count: 7 },
  { name: "Lorde, Audre", count: 6 },
  { name: "Rich, Adrienne", count: 5 },
  { name: "Winterson, Jeanette", count: 4 },
  { name: "Sarton, May", count: 3 },
];
const ALL = RENDERED.concat(HIDDEN);

// A hydratable (unlinked) contributor group: 20 rendered rows and a filter box,
// exactly as facets-body.html prints for a group over the facet limit.
function contributorGroup() {
  const rows = RENDERED.map(
    (c) =>
      `<li data-lcat-field="contributor" data-lcat-cat="${c.name}"><span class="lcat-facet-value">${c.name}</span> <span class="lcat-count">${c.count}</span></li>`,
  ).join("");
  return (
    '<nav class="lcat-facets" aria-label="Filter">' +
    '<details class="lcat-facet" open>' +
    '<summary>Contributors <span class="lcat-facet-total">25</span></summary>' +
    '<input type="search" class="lcat-facet-filter" data-lcat-facet-filter aria-label="Filter">' +
    "<ul>" +
    rows +
    "</ul></details></nav>"
  );
}

function page() {
  const dom = new JSDOM(
    "<!doctype html><html><body>" +
      '<form class="lcat-search"><input name="q"></form>' +
      '<div class="lcat-resultcount"></div>' +
      '<ol id="lcat-results" data-lcat-browse="/search"></ol>' +
      "<aside>" +
      contributorGroup() +
      "</aside>" +
      "</body></html>",
    { url: "https://example.org/works/", runScripts: "dangerously" },
  );
  const w = dom.window;
  // jsdom omits these globals the module uses on start().
  w.TextDecoder = TextDecoder;
  w.TextEncoder = TextEncoder;
  // Reader mock: only a contributor field, carrying every category with counts.
  w.__rr = {
    init: () => Promise.resolve(),
    RrsCatalog: {
      open: () => Promise.resolve({ openFacets: () => Promise.resolve({}) }),
    },
    RrfFacets: {
      open: () =>
        Promise.resolve({
          facets: () => [{ field: "contributor", cats: ALL.slice() }],
        }),
    },
    RrsRecords: { open: () => Promise.resolve({ len: () => 100 }) },
  };
  // No sidecars: subjectMeta degrades to {} (no subjects in this fixture).
  w.fetch = () => Promise.resolve({ ok: false, status: 404 });
  w.eval(SRC);
  return dom;
}

// Boot fires on first focus of the search input; return once the async reader
// pipeline (adoptSidebar -> treeifySidebar -> wireFlatFilter) has settled.
async function boot(dom) {
  // start() registers the focus->boot listener on DOMContentLoaded, which fires
  // a tick after eval; settle first so the listener exists to catch the focus.
  await settle();
  dom.window.document
    .querySelector('input[name="q"]')
    .dispatchEvent(new dom.window.Event("focus"));
  await settle();
}

function filterInput(dom) {
  return dom.window.document.querySelector(
    ".lcat-facets [data-lcat-facet-filter]",
  );
}
function valuesShown(dom) {
  return Array.from(
    dom.window.document.querySelectorAll(
      ".lcat-facets li[data-lcat-cat] .lcat-facet-value",
    ),
  ).map((s) => s.textContent);
}
function type(dom, q) {
  const inp = filterInput(dom);
  inp.value = q;
  inp.dispatchEvent(new dom.window.Event("input"));
}

test("the default view is untouched until the filter is used (task 470)", async () => {
  const dom = page();
  await boot(dom);
  const shown = valuesShown(dom);
  assert.equal(
    shown.length,
    20,
    "still the 20 rendered rows before any filter",
  );
  assert.ok(!shown.includes("Baldwin, James"));
});

test("filtering matches a category beyond the rendered rows (task 470)", async () => {
  const dom = page();
  await boot(dom);
  type(dom, "baldwin");
  const shown = valuesShown(dom);
  assert.deepEqual(
    shown,
    ["Baldwin, James"],
    "the hidden contributor surfaces",
  );
});

test("a filter with no full-set match yields an empty list, not the top rows", async () => {
  const dom = page();
  await boot(dom);
  type(dom, "nonesuch-zzz");
  assert.equal(valuesShown(dom).length, 0);
});

test("clearing the filter restores the top rows (task 470)", async () => {
  const dom = page();
  await boot(dom);
  type(dom, "baldwin");
  assert.equal(valuesShown(dom).length, 1);
  type(dom, "");
  assert.equal(valuesShown(dom).length, 20, "top rows come back on clear");
});

test("a selection made under a filter survives the rebuild (task 470)", async () => {
  const dom = page();
  await boot(dom);
  type(dom, "lorde");
  const cb = dom.window.document.querySelector(
    '.lcat-facets li[data-lcat-cat="Lorde, Audre"] input[data-cat]',
  );
  assert.ok(cb, "the matched row hydrated into a toggle");
  cb.checked = true;
  // Widen the query: Lorde stays a match and must keep its checked state.
  type(dom, "e");
  const still = dom.window.document.querySelector(
    '.lcat-facets li[data-lcat-cat="Lorde, Audre"] input[data-cat]',
  );
  assert.ok(still && still.checked, "checked state preserved across rebuild");
});

(async () => {
  for (const t of tests) {
    try {
      await t.fn();
      passed++;
      console.log("ok   - " + t.name);
    } catch (e) {
      console.error("FAIL - " + t.name + "\n  " + (e && e.stack ? e.stack : e));
      process.exitCode = 1;
    }
  }
  console.log(
    "\nall " +
      passed +
      " facet-filter tests passed" +
      (process.exitCode ? " (with failures)" : ""),
  );
})();
