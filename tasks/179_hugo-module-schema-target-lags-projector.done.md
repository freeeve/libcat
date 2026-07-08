# 179: hugo module's catalogSchemaVersion default still 9 after the v10 bump

## Done (2026-07-08)

hugo/hugo.toml default bumped to 10, with the release-checklist note pinned
as a comment on the param itself (project.SchemaVersion and this default move
together; the mismatch only surfaces at an adopter's render). README's
"currently **9**" updated and the schema-version history gained a v10
terms-sideband entry. The exampleSite's handcrafted catalog.json/facets.json
moved to version 10 with it -- they build against the module default, so
leaving them at 9 would have failed the default build exactly the way
adopters hit it. Verified: exampleSite builds clean; both e2e passes green.

---

Left by the queerbooks-demo session 2026-07-08 (uncommitted cross-repo note).

178 bumped project.SchemaVersion to 10 (the terms sideband), and the hugo
module's templates handle v10 -- but hugo/hugo.toml still ships
`params.catalogSchemaVersion = 9`, so any adopter on the lockstep v0.34.0
pair fails the render loudly:

    ERROR libcat: catalog.json schema version 10, module targets 9 --
    reproject with a matching lcat (tasks/009)

Your e2e stayed green because its site config overrides the param (which is
also how we've worked around it -- queerbooks site/hugo.toml sets 10, marked
INTERIM to drop when this lands).

Fix: bump the default to 10 in hugo/hugo.toml (and the README note that says
"currently **9**"); worth a release checklist line item that the schema
constant and the module default move together, since the mismatch only
surfaces at an adopter's render.
