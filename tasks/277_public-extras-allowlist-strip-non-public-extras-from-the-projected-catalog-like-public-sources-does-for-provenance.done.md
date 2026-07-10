# 277 -- public-extras allowlist: strip non-public extras from the projected catalog

Filed from queerbooks-demo on 2026-07-09 (cross-repo ask).

`[project] public-sources` already gives us exactly the right shape for
provenance: project.SanitizeSources drops every extra.sources name not on
the allowlist, so private community sources never reach catalog.json, the
facets, or the dumps. There is no equivalent for the other extras.

Our need (queerbooks tasks/054): the corpus carries holdings extras --
`inQll`, `qllEbook`, `qllAudiobook` (62,602 / 12,539 / 7,100 works) --
that are institution-private: they say which titles one library already
holds. They must stay in the grains (the cataloging backend is exactly
where librarians need them, and they drive the acquisition map) but they
must not ride the projected catalog.json, the browse artifacts derived
from it, or the downloads.

Today the only way to keep them out of the public face is not to emit
them in ingest at all, which loses them for the backend too -- the two
faces project from the same grains, which is the design.

Ask: `[project] public-extras` (and the matching `--public-extras` flag
plus an `[export]` override, mirroring public-sources exactly):

    [project]
    public-sources = ["loc", "overdrive queer scan"]
    public-extras  = ["cover", "rating", "ratingsCount", "series",
                      "seriesOrder", "audience", "authorLiving"]
    # everything else in extra{} is dropped from the public catalog

Semantics we would want, again by analogy:
- Empty/absent = keep everything (today's behavior; no silent break).
- The allowlist governs catalog.json, whatever the browse/index build
  derives from it, and the three dumps -- the same surfaces
  public-sources governs. `sources` itself stays under public-sources.
- Grains untouched: this is a projection-time filter, not an ingest one.
- A count of what was stripped in the build log, like SanitizeSources
  reports (`stripped N sources`), so a misconfigured allowlist is loud.

Sharp edge worth handling: extras also feed [params.extraFacets] and any
site template reading .Params.<extra>. A stripped extra simply becomes
absent, so templates guarding with `with` degrade cleanly -- but a facet
configured on a stripped extra should probably warn at build time rather
than render an empty rail.

Until this lands we are deleting the chip/badge templates on our side, so
the data stops being *rendered* -- but inQll/qllEbook/qllAudiobook still
sit in the published catalog.json for anyone who fetches it. That is the
gap this ask closes.

## Outcome

Shipped in `223239e`. Every requested semantic, plus one the ask implied.

`project.SanitizeExtras(cat, allow)` drops extras by key and returns the count;
`export`'s `nqFilter` applies the same allowlist to `catalog.nq.gz`. `sources`
is exempt from both, so `public-extras` cannot silently undo a configured
`public-sources`. Grains untouched -- projection-time only, like SanitizeSources.
Absent or empty list keeps everything; no silent break.

Config: `--public-extras` on `lcat project` and `lcat export`,
`[project] public-extras` with an `[export]` override. `[export]` inherits when
the key is absent and declines to inherit when set to `[]`, exactly as
`public-sources` does.

**Covers follow `cover`, which the ask did not name.** If the allowlist omits
`cover`, the export no longer copies the blobs and logs why. The public catalog
cannot name a cover it stripped, so no public page renders it -- publishing the
image anyway would disclose exactly what the allowlist withheld, which is the
tasks/304 class of leak.

**The extraFacets build-time warning is deliberately not implemented.** `lcat`
reads no Hugo config, so it cannot see `[params.extraFacets]` to compare against.
Documented in docs/build-pipeline.md instead, alongside the fact that a stripped
extra is simply absent and `with`-guarded templates degrade cleanly.

### Verified on a copy-on-write clone of the playground store

40 works, two private extras planted on a cover-bearing work:

| config | inQll | qllEbook | cover quads | covers published |
|---|---|---|---|---|
| no allowlist | 1 | 1 | 1 | 1 |
| `--public-extras cover` | 0 | 0 | 1 | 1 |
| `--public-extras rating` | 0 | 0 | 0 | 0 (logged) |

The projected work set and the cover *set* (not count) are identical with and
without the allowlist -- a count would have passed while the filter ate `cover`.

### Mutation-tested

Every guard was stubbed out and the suite re-run; each mutation killed tests.

- extras filter passes everything: 2 fail
- `sources` exemption removed from `allowsExtra`: 2 fail
- covers copied unconditionally: 1 fail
- `extraKey` by substring search instead of N-Quads field 2: 1 fail
- `sources` exemption removed from `SanitizeExtras`: 2 fail

Two findings worth recording. The `sources` exemption originally lived in *two*
places, and the copy in `allowsExtra` was dead for the nq path -- `apply` returned
early. Mutating it changed nothing. Restructured so the exemption exists once and
both callers reach it.

And `TestExtraKeyReadsThePredicateNotTheObject` **survived its own mutation**: the
IRI sat at the end of the literal, so the closing quote made the substring parse
fail by luck and the test passed for the wrong reason. `FuzzExtraKey` caught it.
The case now puts a space after the IRI inside the literal, which is what a
substring search actually mis-parses. Seed corpus committed.
