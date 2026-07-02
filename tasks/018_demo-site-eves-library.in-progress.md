# 018 -- Demo site: "Eve's Library" (libcatalog + Hugo showcase)

## Progress

Kicked off: the demo lives in its own repo, **https://github.com/freeeve/libcatalog-demo**
(public), scaffolded as a libcatalog Hugo-module adopter -- config, taxonomies, Pagefind
search enabled (tasks/017), a placeholder schema-v5 catalog (public-domain classics) +
generated facets, and a build that verifies (6 Work pages, Pagefind indexed). The granular
follow-up work is tracked in that repo's `tasks/`: 001 Hardcover data pipeline, 002
generic-library chrome/branding, 003 S3+CloudFront deploy, 004 controlled-subject mapping,
005 quality/SEO/a11y + link-back. This libcatalog-side task tracks the effort to "live at
libcatalog.evefreeman.com"; close it when the demo is deployed and the READMEs link to it.

## Context

libcatalog has a runnable `hugo/exampleSite`, but it is a **minimal fixture** (3 hand-
authored Works) meant to exercise the module, not to sell it. We want a **public,
real-content demo** that shows an adopter what a finished Tier 1 catalog looks like:
faceted, accessible, multilingual-capable, searchable (Pagefind, tasks/017), built the
way a real library would build it. It doubles as living documentation and a link to drop
in the README.

## Goal

Stand up **`libcatalog.evefreeman.com`** -- a generic-looking library website, "Eve's
Library", filled with the books Eve has actually read (sourced from Hardcover), that is
transparently a **demo of libcatalog + Hugo**.

## Decisions (from planning)

- **Its own repo** (e.g. `libcatalog-demo`), consuming the libcatalog Hugo module the
  way any adopter would (`module.imports` -> `github.com/freeeve/libcatalog/hugo`). This
  keeps the framework repo clean and makes the demo itself a reference adopter. Do **not**
  build the site inside this repo; this file is the driving task, the build happens in the
  new repo.
- **Book data from the Hardcover GraphQL API** (`https://api.hardcover.app/v1/graphql`,
  `Authorization: Bearer <token>`). Fetch Eve's **read** shelf and transform to the
  libcatalog catalog schema.
- **Deploy to S3 + CloudFront** (the ARCHITECTURE §6 static-tier target): built site +
  Pagefind index synced to S3, served via CloudFront with an ACM cert for the subdomain.

## Scope

1. **New repo scaffold.** A Hugo site that imports the module, mounts
   `assets/catalog.json` + `assets/facets.json`, and declares the `[taxonomies]` block
   (per hugo/README "Setup"). Enable Pagefind: `[params.search] engine = "pagefind"`
   (tasks/017). Availability stays **off** (no live external calls from the demo).
2. **Book data pipeline (Hardcover -> catalog).** A small build-time fetcher:
   - Query the authenticated user's read books (`user_books` where status = read),
     joining book -> title/subtitle, contributions -> authors (+ role), editions ->
     ISBN-13/10, cover image, description, genres/tags, rating, date read.
   - Map to `catalog.json` (schema **version 5**; keys: `id`, `title`, `subtitle`,
     `contributors[]`, `subjects[]`, `tags[]`, `languages[]`, `classifications[]`,
     `formats[]`, `instances[]`). Genres -> `tags`; keep `subjects` (controlled) optional
     for the first pass, or map a few to LCSH/Homosaurus to show the controlled-vocabulary
     dimension (tasks/012).
   - **Preferred, showcases the onboarding ramp:** where an ISBN resolves to a real bib
     record, pull MARC/BIBFRAME (via libcodex / LoC / OpenLibrary) and run it through
     `lcat project` so the demo exercises the real BIBFRAME->project pipeline, not a
     bespoke JSON shim. Fall back to direct Hardcover->catalog mapping for records with no
     retrievable MARC. Regenerate `facets.json` from the projector so counts stay correct
     (do not hand-maintain it at demo scale).
3. **Generic library chrome.** Brand as "Eve's Library" and model a typical public-
   library site: a homepage hero / "Recently read" strip, an **About** page that states
   plainly this is a demo of the libcatalog framework + Hugo (with links to the repos),
   and a light theme override of `assets/lcat.css` for identity. Reuse the module's
   templates; only shadow what branding needs. Keep the "built with libcatalog + Hugo"
   note in the footer on every page.
4. **Build + deploy.** A reproducible pipeline (Makefile or GitHub Actions):
   `hugo --minify` -> `pagefind --site public` (or `npm run search:index` equivalent) ->
   `aws s3 sync public/ s3://<bucket>` -> CloudFront invalidation. TLS via ACM for
   `libcatalog.evefreeman.com`; DNS alias (Route 53 or the domain's DNS) to the CloudFront
   distribution. Keep AWS infra as code (a small Terraform/CloudFormation stack) if
   practical so it is reproducible.
5. **Link back.** Once live, add the demo URL to the top-level README and hugo/README.

## Inputs needed before build (gather at execution time)

- **Hardcover**: an API token + the account/user identifier for Eve's read shelf, and
  confirmation of the current `user_books`/`books` schema shape (it evolves).
- **AWS**: account + credentials with S3/CloudFront/ACM (and Route 53 if DNS lives there)
  access, or the target bucket/distribution if they already exist.
- **DNS**: control of `evefreeman.com` to point `libcatalog` at CloudFront and validate
  the ACM cert.
- **Repo**: the GitHub org/owner and final repo name.
- Optional: whether the demo is English-only or bilingual (the module supports either;
  English-only is fine for a first cut).

## Acceptance

- `https://libcatalog.evefreeman.com` serves a faceted, accessible, **Pagefind-searchable**
  catalog of Eve's real read books, looking like a generic library site and clearly
  labeled as a libcatalog + Hugo demo.
- The site is a **separate repo** that imports the published Hugo module as an adopter
  would; its build is reproducible (one command / one CI workflow) and includes the
  Pagefind post-build step.
- Book data derives from Hardcover; where feasible it flows through the real
  BIBFRAME -> `lcat project` pipeline rather than a one-off transform.
- README links to the live demo.

## Refs

- hugo/README "Setup" / "Search" / "Multilingual"; ARCHITECTURE §6 (Tier 1, S3/CloudFront)
  and §7 (projector + module); `tasks/017` (Pagefind search, enabled here), `tasks/012`
  (controlled subjects vs tags), `tasks/009` (projector contract), `tasks/016`
  (multilingual, if the demo goes bilingual).
- Hardcover API: https://api.hardcover.app/v1/graphql (docs at https://docs.hardcover.app).
- `libcodex` for MARC/BIBFRAME retrieval + conversion from ISBNs.
