# 021 -- Link back to the live Eve's Library demo

## Context

Filed from the **libcatalog-demo** adopter repo while doing its quality pass (that
repo's `tasks/005 §5`). The link-back must land in **this** (libcatalog) repo, not the
demo repo, per the workspace convention -- hence this task file. Left uncommitted so a
concurrent session here isn't disrupted; treat as a pending request, not in-progress.

## Scope

Once https://libcatalog.evefreeman.com is live and verified, add a "Live demo" line to:

- top-level `README.md` -- near the top, e.g.
  `**Live demo:** https://libcatalog.evefreeman.com (Eve's Library -- a public adopter site).`
- `hugo/README.md` -- under the intro or "What it renders", pointing at the same URL as a
  runnable reference adopter (imports the module, provides projected data, adds light
  branding).

## Acceptance

- Both READMEs link to https://libcatalog.evefreeman.com.
- Wording makes clear it is a demo/reference adopter of the framework + Hugo module.

## Related

- The demo also proposes `tasks/020` here (adopter footer/head template hooks), which
  would let it drop its `baseof.html` shadow.

## Resolution

Verified the demo is live first (HTTP 200; `<title>Eve's Library -- a libcatalog + Hugo
demo</title>`), then added the link-back to both READMEs:

- top-level `README.md` -- a **Live demo** line under the intro.
- `hugo/README.md` -- a **Live reference adopter** line under the module intro, framing it
  as a runnable real-site example next to `exampleSite/`.

Both point at https://libcatalog.evefreeman.com and make clear it is a demo/reference
adopter of the framework + Hugo module.
