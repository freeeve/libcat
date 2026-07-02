// Dev-only internal-link checker (tasks/023). Walks a built Hugo site and asserts every
// root-relative link resolves to a generated file -- catching facet/term links whose
// slug does not match the page Hugo minted (e.g. a `+`/`/` in a subject/tag label that a
// CDN would 404). Not shipped: Hugo consumes only the templates and assets, never this.
//
// Usage: node link_check.cjs <built-site-dir>   (e.g. exampleSite/public)
"use strict";
const fs = require("fs");
const path = require("path");

const root = process.argv[2] || "exampleSite/public";

function walk(dir, out) {
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, e.name);
    if (e.isDirectory()) walk(p, out);
    else if (e.name.endsWith(".html")) out.push(p);
  }
  return out;
}

// Decode the few HTML entities Hugo emits inside href values so a literal `+` (`&#43;`)
// or `&` (`&amp;`) is checked as the character a browser would send.
function decode(s) {
  return s
    .replace(/&#43;/g, "+")
    .replace(/&#x2b;/gi, "+")
    .replace(/&amp;/g, "&");
}

function resolves(href) {
  const clean = decode(href).split("#")[0].split("?")[0];
  if (!clean.startsWith("/") || clean.startsWith("//")) return true; // external/protocol-relative: skip
  const rel = clean.slice(1);
  const target = /\.[a-z0-9]+$/i.test(clean)
    ? path.join(root, rel) // links to a file (e.g. .xml)
    : path.join(root, rel, "index.html"); // pretty dir URL
  return fs.existsSync(target);
}

const files = walk(root, []);
const broken = [];
const plusPaths = new Set();
const hrefRe = /href="([^"]+)"/g;

for (const f of files) {
  const html = fs.readFileSync(f, "utf8");
  let m;
  while ((m = hrefRe.exec(html)) !== null) {
    const href = m[1];
    if (/^(https?:|mailto:|tel:|#|data:)/i.test(href)) continue;
    const clean = decode(href).split("#")[0].split("?")[0];
    if (!clean.startsWith("/") || clean.startsWith("//")) continue;
    // /pagefind/ assets are emitted by the post-build `pagefind` step, not Hugo, so they
    // legitimately do not exist in a Hugo-only build -- skip them here (tasks/017).
    if (clean.startsWith("/pagefind/")) continue;
    // The bug this guards: an unsafe `+` left in a facet/term path (CDN 404).
    if (clean.includes("+")) plusPaths.add(clean);
    if (!resolves(href)) broken.push({ file: path.relative(root, f), href: decode(href) });
  }
}

let failed = false;
if (plusPaths.size) {
  failed = true;
  console.error(`Found ${plusPaths.size} link path(s) containing a literal '+' (CDN-unsafe):`);
  for (const p of [...plusPaths].sort()) console.error(`  ${p}`);
}
if (broken.length) {
  failed = true;
  console.error(`\n${broken.length} broken internal link(s):`);
  const seen = new Set();
  for (const b of broken) {
    const key = b.href;
    if (seen.has(key)) continue;
    seen.add(key);
    console.error(`  ${b.href}  (e.g. in ${b.file})`);
  }
}

if (failed) process.exit(1);
console.log(`===== ${files.length} pages checked =====`);
console.log("All internal facet/term/work links resolve; no CDN-unsafe '+' paths.");
