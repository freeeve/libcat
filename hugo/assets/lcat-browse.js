/*
 * Client-side ranked search over the RoaringRange WASM reader (libcatalog
 * tasks/158). Opt-in via [params.search] engine = "roaringrange".
 *
 * Progressive enhancement: the server-rendered work list (task 157) is the
 * default view. When the visitor types a query, this module replaces the results
 * with a ranked search served entirely client-side from the artifacts the build
 * emits (search.BuildBrowse): a global trigram index (browse-index.rrs), a facet
 * sidecar (browse-facets.rrsf), and a record store (browse-records.{idx,bin})
 * whose records are compact result-card JSON. Clearing the query restores the
 * static list. If the reader or artifacts are unavailable, the static list stays
 * and nothing regresses.
 *
 * Scope: this increment wires ranked *text* search + result rendering. Facet
 * *filtering* (passing filters_json to search) and facet-only browse ride the
 * facet-sidebar rework in tasks 157/158; the reader already returns facetCounts
 * here for when that lands.
 *
 * Status: NOT yet browser-verified end to end -- landed for review/testing.
 */
import init, { RrsCatalog } from "/lcat/roaringrange_reader.js";

const PAGE = 60;

/** esc HTML-escapes untrusted record text before insertion. */
function esc(s) {
  return String(s == null ? "" : s).replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c],
  );
}

/** card renders one result row from a decoded record (browseCard JSON). */
function card(dec, rec) {
  let c;
  try {
    c = JSON.parse(dec.decode(rec));
  } catch (e) {
    return "";
  }
  const href = "/works/" + encodeURIComponent(c.id) + "/";
  const contrib = (c.contributors || []).join(", ");
  return (
    '<li><a class="lcat-result" href="' +
    href +
    '">' +
    '<span class="lcat-result-title">' +
    esc(c.title || c.id) +
    "</span>" +
    (c.subtitle ? '<span class="lcat-result-subtitle">' + esc(c.subtitle) + "</span>" : "") +
    (contrib ? '<span class="lcat-result-contributors">' + esc(contrib) + "</span>" : "") +
    "</a></li>"
  );
}

function start() {
  const results = document.getElementById("lcat-results");
  const form = document.querySelector(".lcat-search");
  if (!results || !form) return;
  const input = form.querySelector('input[name="q"]');
  if (!input) return;

  const base = (results.getAttribute("data-lcat-browse") || "/search").replace(/\/+$/, "");
  const staticList = results.innerHTML; // restored when the query is cleared
  const countEl = document.querySelector(".lcat-resultcount");
  const staticCount = countEl ? countEl.textContent : "";
  const labels = {
    none: results.getAttribute("data-lcat-noresults") || "No matches",
    results: results.getAttribute("data-lcat-resultsword") || "results",
  };
  const dec = new TextDecoder();

  let catalog = null;
  let booting = null;
  function boot() {
    if (catalog) return Promise.resolve(true);
    if (!booting) {
      booting = init()
        .then(() =>
          RrsCatalog.openAll(
            base + "/browse-index.rrs",
            base + "/browse-facets.rrsf",
            base + "/browse-records.idx",
            base + "/browse-records.bin",
          ),
        )
        .then((c) => {
          catalog = c;
          return true;
        })
        .catch((e) => {
          console.warn("lcat-browse: reader unavailable, staying on static list", e);
          return false;
        });
    }
    return booting;
  }

  function restore() {
    results.innerHTML = staticList;
    if (countEl) countEl.textContent = staticCount;
  }

  function render(res) {
    const ids = (res && res.ids) || [];
    const recs = (res && res.records) || [];
    const html = [];
    for (let i = 0; i < ids.length; i++) {
      if (recs[i]) html.push(card(dec, recs[i]));
    }
    results.innerHTML = html.length ? html.join("") : '<li class="lcat-noresults">' + esc(labels.none) + "</li>";
    if (countEl) {
      countEl.textContent = ids.length + (ids.length >= PAGE ? "+ " : " ") + labels.results;
    }
  }

  let seq = 0;
  function query(q) {
    const mine = ++seq;
    boot()
      .then((ok) => {
        if (!ok || mine !== seq) return; // reader down, or a newer keystroke won
        return catalog.search(q, 0, PAGE, 0, null).then((res) => {
          if (mine === seq) render(res);
        });
      })
      .catch((e) => console.warn("lcat-browse: search failed", e));
  }

  function onChange() {
    const q = input.value.trim();
    if (q === "") {
      restore();
      return;
    }
    query(q);
  }

  form.addEventListener("submit", (e) => {
    e.preventDefault();
    onChange();
  });
  input.addEventListener("input", onChange);

  // Honor an initial ?q= (a deep link, or the no-JS form landing here).
  const initial = new URLSearchParams(window.location.search).get("q");
  if (initial) {
    input.value = initial;
    onChange();
  }
}

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", start);
} else {
  start();
}
