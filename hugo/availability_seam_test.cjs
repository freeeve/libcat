// The seam nothing crossed (libcat tasks/287).
//
// availability_test.cjs is 23 tests deep and every one hands the adapter a
// hand-written JavaScript object -- so it can never discover that the real
// producer spells the keys differently. The producer is:
//
//     TOML -> Hugo (lowercases param keys) -> jsonify -> readConfig -> adapter
//
// This test builds hugo/exampleSite with the availability block the README
// prescribes, pulls the config script out of a rendered page, and feeds exactly
// those bytes to readConfig. Nothing is hand-written but the TOML.
//
// Usage: node availability_seam_test.cjs
"use strict";
const fs = require("fs");
const os = require("os");
const path = require("path");
const { execFileSync } = require("child_process");

const A = require("./assets/lcat-availability.js");

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

const PROXY = "https://proxy.example/av";
const ACTION = "https://borrow.example/go/{id}";
const BASE = "https://thunder.example/v2";

// The README's own spelling, verbatim (hugo/README.md:305,326,352).
const OVERLAY = `
[params.availability]
  enabled = true
  [params.availability.overdrive]
    slug = "examplelib"
    transport = "proxied"
    proxyUrl = "${PROXY}"
    baseUrl = "${BASE}"
    actionUrlTemplate = "${ACTION}"
    timeoutMs = 4321
`;

const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "lcat-availability-seam-"));
const overlay = path.join(tmp, "availability.toml");
fs.writeFileSync(overlay, OVERLAY);
const out = path.join(tmp, "public");

try {
  execFileSync("hugo", ["--quiet", "--config", `hugo.toml,${overlay}`, "--destination", out], {
    cwd: path.join(__dirname, "exampleSite"),
    stdio: ["ignore", "ignore", "inherit"],
  });
} catch (e) {
  console.error("FAIL - could not build exampleSite (is `hugo` on PATH?)");
  process.exit(1);
}

// Pull the config script out of a rendered page, exactly as the browser sees it.
const page = fs.readFileSync(path.join(out, "works", "wexampleone", "index.html"), "utf8");
const m = page.match(/<script id="lcat-availability-config"[^>]*>([\s\S]*?)<\/script>/);
if (!m) {
  console.error("FAIL - no lcat-availability-config script in the rendered page");
  process.exit(1);
}
const emitted = m[1];

// A document with exactly the one node readConfig looks for.
const doc = { getElementById: (id) => (id === "lcat-availability-config" ? { textContent: emitted } : null) };

// This is the bug, stated as a fact about Hugo rather than an assumption. If Hugo
// ever stops lowercasing, this fails and the normalizer can go.
check("Hugo emits the param keys lowercased", () => {
  let raw = JSON.parse(emitted);
  if (typeof raw === "string") raw = JSON.parse(raw);
  const od = raw.overdrive;
  assert(od, "no overdrive block in the emitted config");
  const keys = Object.keys(od);
  assert(keys.includes("proxyurl"), `emitted keys are ${JSON.stringify(keys)}; expected the lowercased "proxyurl"`);
  assert(!keys.includes("proxyUrl"), "Hugo preserved camelCase; the normalizer is now dead code");
});

check("readConfig hands the adapter the spelling it reads", () => {
  const cfg = A.readConfig(doc);
  assert(cfg, "readConfig returned null for an enabled config");
  const od = cfg.overdrive;
  assert(od.proxyUrl === PROXY, `proxyUrl = ${JSON.stringify(od.proxyUrl)}`);
  assert(od.baseUrl === BASE, `baseUrl = ${JSON.stringify(od.baseUrl)}`);
  assert(od.actionUrlTemplate === ACTION, `actionUrlTemplate = ${JSON.stringify(od.actionUrlTemplate)}`);
  assert(od.timeoutMs === 4321, `timeoutMs = ${JSON.stringify(od.timeoutMs)}`);
  assert(od.transport === "proxied" && od.slug === "examplelib", "the already-lowercase keys regressed");
});

// The failure the report calls the one that matters: proxied transport configured
// per the README, and not one request ever issued.
check("a proxied transport built from real config posts to the proxy", () => {
  const cfg = A.readConfig(doc);
  const req = A.overdriveRequest(["abc"], cfg.overdrive);
  assert(req.url === PROXY, `request went to ${JSON.stringify(req.url)}, not the configured proxy`);
  assert(req.body.slug === "examplelib", "the proxy body lost the slug");
});

check("unknown keys survive canonicalization", () => {
  const cfg = A.readConfig({
    getElementById: () => ({ textContent: JSON.stringify({ enabled: true, overdrive: { proxyurl: PROXY, myOwnKey: 7 } }) }),
  });
  assert(cfg.overdrive.proxyUrl === PROXY, "known key not canonicalized");
  assert(cfg.overdrive.myOwnKey === 7, "a deployment's own key was renamed or dropped");
});

check("a disabled config is still null", () => {
  const cfg = A.readConfig({ getElementById: () => ({ textContent: JSON.stringify({ enabled: false, overdrive: {} }) }) });
  assert(cfg === null, "a disabled availability block produced a config");
});

fs.rmSync(tmp, { recursive: true, force: true });

if (failures) {
  console.error(`\n${failures} seam test(s) failed`);
  process.exit(1);
}
console.log("\nall availability seam tests passed");
