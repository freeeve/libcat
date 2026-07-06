/*
  Unit tests for the negative facet filters (lcat-negatives.js, tasks/144).
  Run: node hugo/negatives_test.cjs  (exit 0 = pass). Lives at the module
  root, not under assets/, so Hugo never publishes it. Mounts a minimal
  sidebar + results DOM in jsdom and exercises URL-state parsing, card
  hiding, chips, link rewriting, and toggling.
*/
const assert = require("node:assert");
const fs = require("node:fs");
const { JSDOM } = require("jsdom");

const SRC = fs.readFileSync(__dirname + "/assets/lcat-negatives.js", "utf8");

let passed = 0;
function test(name, fn) {
  try {
    fn();
    passed++;
    console.log("ok   - " + name);
  } catch (e) {
    console.error("FAIL - " + name + "\n  " + (e && e.stack ? e.stack : e));
    process.exitCode = 1;
  }
}

function page(url) {
  const dom = new JSDOM(
    `<!doctype html><html><body>
    <nav class="lcat-facets"><ul>
      <li><a href="/tags/fiction/">Fiction</a><button type="button" data-lcat-exclude data-lcat-taxonomy="tags" data-lcat-term="fiction" data-lcat-label="Fiction" aria-pressed="false">&#x2212;</button></li>
      <li><a href="/languages/eng/">English</a><button type="button" data-lcat-exclude data-lcat-taxonomy="languages" data-lcat-term="eng" data-lcat-label="English" aria-pressed="false">&#x2212;</button></li>
    </ul></nav>
    <script id="lcat-negatives-config" type="application/json">{"excluded":"Not %s","remove":"Remove exclusion of %s"}</script>
    <main>
      <ol id="lcat-results">
        <li><article class="lcat-card" data-lcat-tags="fiction|family" data-lcat-languages="eng"></article></li>
        <li><article class="lcat-card" data-lcat-tags="family" data-lcat-languages="spa"></article></li>
      </ol>
    </main>
    </body></html>`,
    { url: url, runScripts: "outside-only" },
  );
  dom.window.eval(SRC);
  return dom;
}

function hiddenFlags(dom) {
  return Array.from(dom.window.document.querySelectorAll("#lcat-results > li")).map((li) =>
    li.classList.contains("lcat-neg-hidden"),
  );
}

test("x-param on load hides matching cards, chips + pressed state render", () => {
  const dom = page("https://example.org/works/?xtags=fiction");
  assert.deepEqual(hiddenFlags(dom), [true, false]);
  const chips = dom.window.document.getElementById("lcat-excluded");
  assert.ok(chips && !chips.hidden);
  assert.ok(chips.textContent.includes("Not Fiction"));
  const btn = dom.window.document.querySelector('[data-lcat-term="fiction"]');
  assert.equal(btn.getAttribute("aria-pressed"), "true");
});

test("sidebar links carry active exclusions", () => {
  const dom = page("https://example.org/works/?xtags=fiction");
  const a = dom.window.document.querySelector('a[href*="/languages/eng/"]');
  assert.ok(a.getAttribute("href").includes("xtags=fiction"));
});

test("exclude button toggles URL state and card visibility", () => {
  const dom = page("https://example.org/works/");
  const btn = dom.window.document.querySelector('[data-lcat-term="eng"]');
  btn.click();
  assert.ok(dom.window.location.search.includes("xlanguages=eng"));
  assert.deepEqual(hiddenFlags(dom), [true, false]);
  btn.click();
  assert.equal(dom.window.location.search, "");
  assert.deepEqual(hiddenFlags(dom), [false, false]);
});

test("chip dismiss removes the exclusion", () => {
  const dom = page("https://example.org/works/?xtags=fiction&xlanguages=eng");
  assert.deepEqual(hiddenFlags(dom), [true, false]);
  const chip = dom.window.document.querySelector("#lcat-excluded li button");
  chip.click();
  assert.equal(dom.window.location.search, "?xlanguages=eng");
  assert.deepEqual(hiddenFlags(dom), [true, false]);
  dom.window.document.querySelector("#lcat-excluded li button").click();
  assert.deepEqual(hiddenFlags(dom), [false, false]);
  assert.ok(dom.window.document.getElementById("lcat-excluded").hidden);
});

test("unknown x-params and foreign params are ignored", () => {
  const dom = page("https://example.org/works/?xfoo=bar&q=sea");
  assert.deepEqual(hiddenFlags(dom), [false, false]);
  const chips = dom.window.document.getElementById("lcat-excluded");
  assert.ok(!chips || chips.hidden);
  const a = dom.window.document.querySelector('a[href*="/tags/fiction/"]');
  assert.ok(!a.getAttribute("href").includes("xfoo"));
});

console.log("\nall " + passed + " negative-filter tests passed" + (process.exitCode ? " (with failures)" : ""));
