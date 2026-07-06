/*
 * Opt-in negative facet filters (libcatalog tasks/144): filter works OUT by a
 * facet term. URL state is x<taxonomy>=<term>, repeatable, matching the qllpoc
 * convention, so exclusions are shareable and bookmarkable. Exclusions are
 * buttons, not links -- "hide X" URLs stay out of crawlers -- and everything
 * is client-side over the already-rendered page: cards carrying an excluded
 * term hide, active exclusions render as dismissible "Not X" chips above the
 * results, and sidebar/pagination links are rewritten to carry the exclusions
 * along while browsing. Like lcat-search.js this filters the CURRENT page's
 * cards only; a fully server-shaped result set is the roaringrange reader's
 * job (tasks/010). No results list on the page (taxonomy landings) still
 * toggles the URL state, which the rewritten links then carry.
 */
(function () {
  "use strict";
  var buttons = Array.prototype.slice.call(document.querySelectorAll("[data-lcat-exclude]"));
  if (buttons.length === 0) return;
  var results = document.getElementById("lcat-results");
  var strings = { excluded: "Not %s", remove: "Remove exclusion of %s" };
  var cfgEl = document.getElementById("lcat-negatives-config");
  if (cfgEl) {
    try { strings = JSON.parse(cfgEl.textContent); } catch (e) { /* defaults */ }
  }
  var taxonomies = {};
  buttons.forEach(function (b) { taxonomies[b.getAttribute("data-lcat-taxonomy")] = true; });

  function exclusions() {
    var out = [];
    new URLSearchParams(window.location.search).forEach(function (term, key) {
      if (key.charAt(0) === "x" && taxonomies[key.slice(1)] && term !== "") {
        out.push({ taxonomy: key.slice(1), term: term });
      }
    });
    return out;
  }

  function isExcluded(xs, taxonomy, term) {
    return xs.some(function (ex) { return ex.taxonomy === taxonomy && ex.term === term; });
  }

  function labelFor(ex) {
    var label = ex.term;
    buttons.forEach(function (b) {
      if (b.getAttribute("data-lcat-taxonomy") === ex.taxonomy && b.getAttribute("data-lcat-term") === ex.term) {
        label = b.getAttribute("data-lcat-label") || ex.term;
      }
    });
    return label;
  }

  function chipsContainer() {
    var chips = document.getElementById("lcat-excluded");
    if (!chips && results) {
      chips = document.createElement("ul");
      chips.id = "lcat-excluded";
      chips.className = "lcat-excluded";
      chips.setAttribute("role", "status");
      results.parentNode.insertBefore(chips, results);
    }
    return chips;
  }

  function renderChips(xs) {
    var chips = chipsContainer();
    if (!chips) return;
    chips.textContent = "";
    chips.hidden = xs.length === 0;
    xs.forEach(function (ex) {
      var label = labelFor(ex);
      var li = document.createElement("li");
      li.appendChild(document.createTextNode(strings.excluded.replace("%s", label) + " "));
      var rm = document.createElement("button");
      rm.type = "button";
      rm.textContent = "×";
      rm.setAttribute("aria-label", strings.remove.replace("%s", label));
      rm.addEventListener("click", function () { toggle(ex.taxonomy, ex.term, false); });
      li.appendChild(rm);
      chips.appendChild(li);
    });
  }

  function hideCards(xs) {
    if (!results) return;
    for (var li = results.firstElementChild; li; li = li.nextElementSibling) {
      var card = li.querySelector(".lcat-card");
      var hide = false;
      if (card) {
        xs.forEach(function (ex) {
          var attr = card.getAttribute("data-lcat-" + ex.taxonomy);
          if (attr && attr.split("|").indexOf(ex.term) !== -1) hide = true;
        });
      }
      li.classList.toggle("lcat-neg-hidden", hide);
    }
  }

  // Static links do not know about exclusions, so browsing to another facet
  // or page would drop them -- carry the x-params on sidebar and pagination
  // links (never work links: a detail page has no result list to filter).
  function rewriteLinks(xs) {
    var links = document.querySelectorAll(".lcat-facets a, .pagination a");
    Array.prototype.forEach.call(links, function (a) {
      var url = new URL(a.getAttribute("href"), window.location.href);
      Object.keys(taxonomies).forEach(function (t) { url.searchParams.delete("x" + t); });
      xs.forEach(function (ex) { url.searchParams.append("x" + ex.taxonomy, ex.term); });
      a.setAttribute("href", url.pathname + url.search + url.hash);
    });
  }

  function apply() {
    var xs = exclusions();
    renderChips(xs);
    hideCards(xs);
    rewriteLinks(xs);
    buttons.forEach(function (b) {
      var on = isExcluded(xs, b.getAttribute("data-lcat-taxonomy"), b.getAttribute("data-lcat-term"));
      b.setAttribute("aria-pressed", on ? "true" : "false");
    });
  }

  function toggle(taxonomy, term, add) {
    var params = new URLSearchParams(window.location.search);
    var key = "x" + taxonomy;
    var vals = params.getAll(key).filter(function (v) { return v !== term; });
    if (add) vals.push(term);
    params.delete(key);
    vals.forEach(function (v) { params.append(key, v); });
    var q = params.toString();
    history.replaceState(null, "", window.location.pathname + (q ? "?" + q : "") + window.location.hash);
    apply();
  }

  buttons.forEach(function (b) {
    b.addEventListener("click", function () {
      var taxonomy = b.getAttribute("data-lcat-taxonomy");
      var term = b.getAttribute("data-lcat-term");
      toggle(taxonomy, term, !isExcluded(exclusions(), taxonomy, term));
    });
  });
  apply();
})();
