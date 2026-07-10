// The "more like this" rail's Shares line (libcat tasks/284, tasks/296).
//
// The sidecar names shared concepts by authority IRI, because a label is
// language-specific and the sidecar is not. page.html used to resolve those IRIs
// against the page's *own* subjects -- but the scorer reaches a neighbour through
// the concept tree and then reports the value the *neighbour* holds verbatim, so
// `shared` routinely names a concept the page does not carry. Those printed as
// bare authority URLs to readers: 5.6% of shared IRIs on a 62.6k-work catalog.
//
// Nothing could have caught it from inside a template. This builds the site for
// real, against a catalog whose every branch is deliberate, and reads the
// rendered Shares line the way a visitor does.
//
// Usage: node similar_seam_test.cjs   (requires `hugo` on PATH)
"use strict";
const fs = require("fs");
const os = require("os");
const path = require("path");
const { execFileSync } = require("child_process");

let failures = 0;
function check(name, fn) {
  try {
    fn();
    console.log(`ok   - ${name}`);
  } catch (e) {
    failures++;
    console.error(`FAIL - ${name}\n       ${e.message}`);
  }
}
function assert(cond, msg) {
  if (!cond) throw new Error(msg);
}

const HOMO = "https://homosaurus.org/v3/";
const FAST = "http://id.worldcat.org/fast/";
const GENDER = HOMO + "homoit0000282"; // ancestor: labeled, but no Work carries it
const TRANS = HOMO + "homoit0000669"; // carried by both Works
const TRANS_FAST = FAST + "1735592"; // the same concept, a different scheme
const UNDESCRIBED = HOMO + "homoit0009999"; // a bare URI the graph never described
const DUTCH_ONLY = HOMO + "homoit0000777"; // described, but in no site language

// A catalog where the ancestor GENDER is described only by the Terms sideband --
// no Work carries it -- so a page can only label it from the catalog, never from
// itself. That is the shape that broke.
const catalog = {
  version: 11,
  terms: [
    { id: GENDER, labels: { en: "Gender identity", es: "Identidad de genero" } },
    { id: TRANS, labels: { en: "Transgender people", es: "Personas trans" }, broader: [GENDER] },
    { id: DUTCH_ONLY, labels: { nl: "Transgender personen" } },
    { id: UNDESCRIBED }, // in the sideband, but with nothing to say
  ],
  works: [
    {
      id: "wone",
      title: "One",
      subjects: [{ id: TRANS, labels: { en: "Transgender people", es: "Personas trans" }, broader: [GENDER] }],
    },
    {
      id: "wtwo",
      title: "Two",
      subjects: [
        { id: TRANS, labels: { en: "Transgender people", es: "Personas trans" }, broader: [GENDER] },
        { id: TRANS_FAST, labels: { en: "Transgender people" } },
      ],
    },
    { id: "wthree", title: "Three" }, // no subjects at all
  ],
};

const similar = {
  version: 1,
  limit: 8,
  works: {
    // The reported bug: GENDER is reached through the tree and carried by neither
    // page. Only the catalog can label it.
    wone: [{ id: "wtwo", title: "Two", shared: [TRANS, GENDER] }],
    // The stutter: one concept in two schemes, both resolving to one label.
    // Plus free text (a tag), which is already human and passes through.
    wtwo: [{ id: "wone", title: "One", shared: [TRANS, TRANS_FAST, "lgbtq-books"] }],
    // A page with no subjects of its own still explains itself; and an IRI the
    // catalog cannot label is dropped rather than printed raw. DUTCH_ONLY has a
    // label, just not in a site language.
    wthree: [
      { id: "wone", title: "One", shared: [GENDER, DUTCH_ONLY] },
      { id: "wtwo", title: "Two", shared: [UNDESCRIBED] },
    ],
  },
};

const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "lcat-similar-seam-"));
const siteDir = path.join(tmp, "site");
fs.cpSync(path.join(__dirname, "exampleSite"), siteDir, { recursive: true });
// The site resolves the module by a relative replace; the copy must point back.
const gomod = path.join(siteDir, "go.mod");
fs.writeFileSync(gomod, fs.readFileSync(gomod, "utf8").replace("=> ../", `=> ${__dirname}`));
const assets = path.join(siteDir, "assets");
fs.writeFileSync(path.join(assets, "catalog.json"), JSON.stringify(catalog));
fs.writeFileSync(path.join(assets, "similar.json"), JSON.stringify(similar));
// facets.json is projected from the same catalog; an empty one keeps them honest.
fs.writeFileSync(path.join(assets, "facets.json"), JSON.stringify({ version: 11 }));

const out = path.join(tmp, "public");
execFileSync("hugo", ["--quiet", "--destination", out], { cwd: siteDir, stdio: ["ignore", "ignore", "inherit"] });

// shares reads the rendered Shares line for a work, as a list of terms.
function shares(work, lang) {
  const file = path.join(out, ...(lang === "en" ? [] : [lang]), "works", work, "index.html");
  const html = fs.readFileSync(file, "utf8");
  return [...html.matchAll(/<span class="lcat-similar-why">[^:]*:([^<]*)<\/span>/g)].map((m) =>
    m[1].split(",").map((s) => s.trim()),
  );
}
const cards = (work, lang) => {
  const file = path.join(out, ...(lang === "en" ? [] : [lang]), "works", work, "index.html");
  return (fs.readFileSync(file, "utf8").match(/<li class="lcat-similar-item">/g) ?? []).length;
};

// ---------------------------------------------------------------------------
// tasks/296 -- the rail must never print an authority URL at a reader.
// ---------------------------------------------------------------------------

check("no rendered Shares line contains a raw authority URL", () => {
  for (const lang of ["en", "es"]) {
    for (const w of ["wone", "wtwo", "wthree"]) {
      for (const line of shares(w, lang)) {
        const raw = line.filter((t) => t.startsWith("http://") || t.startsWith("https://"));
        assert(raw.length === 0, `${lang} /works/${w}/ shows ${JSON.stringify(raw)}`);
      }
    }
  }
});

check("a concept reached through the tree, carried by neither page, is labeled", () => {
  // wone -> wtwo shares GENDER, which no Work in this catalog carries.
  assert(
    JSON.stringify(shares("wone", "en")[0]) === JSON.stringify(["Transgender people", "Gender identity"]),
    `got ${JSON.stringify(shares("wone", "en")[0])}`,
  );
});

check("the same concept in two schemes collapses to one term", () => {
  // TRANS and TRANS_FAST both label "Transgender people". It used to print twice.
  const line = shares("wtwo", "en")[0];
  assert(JSON.stringify(line) === JSON.stringify(["Transgender people", "lgbtq-books"]), `got ${JSON.stringify(line)}`);
});

check("free text passes through unresolved", () => {
  assert(shares("wtwo", "en")[0].includes("lgbtq-books"), "the tag was dropped or rewritten");
});

check("a page with no subjects of its own still explains its rail", () => {
  // The old code built its label map from $.Params.subjectList; wthree has none,
  // so every term on this page would have rendered as a URL.
  assert(shares("wthree", "en")[0][0] === "Gender identity", `got ${JSON.stringify(shares("wthree", "en")[0])}`);
});

check("an IRI the catalog cannot label is dropped, and its card keeps no reason", () => {
  // wthree's second neighbour shares only UNDESCRIBED: card renders, Shares does not.
  assert(cards("wthree", "en") === 2, `expected 2 neighbour cards, got ${cards("wthree", "en")}`);
  assert(shares("wthree", "en").length === 1, "the unlabelable neighbour still printed a Shares line");
});

check("a label in no site language still beats printing the URL", () => {
  // DUTCH_ONLY is described, just not in en/es. The lexically-first tag wins.
  assert(shares("wthree", "en")[0].includes("Transgender personen"), `got ${JSON.stringify(shares("wthree", "en")[0])}`);
});

check("labels resolve per language", () => {
  assert(
    JSON.stringify(shares("wone", "es")[0]) === JSON.stringify(["Personas trans", "Identidad de genero"]),
    `got ${JSON.stringify(shares("wone", "es")[0])}`,
  );
});

fs.rmSync(tmp, { recursive: true, force: true });
console.log(failures === 0 ? "all similar-rail seam tests passed" : `${failures} failed`);
process.exit(failures === 0 ? 0 : 1);
