# Availability provider feasibility matrix

Live availability is fetched **client-side at view time** and kept out of the graph
(ARCHITECTURE §5), behind one normalized model and a per-provider adapter (`tasks/004`).
Whether a provider can be called **`direct`** from the browser or needs the optional
**`proxied`** edge function depends on four things per source: CORS from the deploy
origin, auth mode, a batch endpoint, and rate limits. This matrix records the current
understanding so a deployment can pick a transport; **verify the flagged cells against
your own origin before shipping** -- CORS and auth in particular are deployment- and
account-specific.

Confidence: **[V]** verified against a real client/spec · **[R]** reasoned from the
provider's known model · **[?]** unverified, confirm per deployment.

## Matrix

| Provider | Kind | Transport | Auth | Batch | CORS (browser) | Availability semantics | Confidence |
|----------|------|-----------|------|-------|----------------|------------------------|------------|
| **OverDrive / Thunder** | digital | `direct` (fall back `proxied`) | none | yes, `POST /libraries/{slug}/media/availability`, <=25 ids | unverified from a static origin | copies owned/available, holds, est. wait | [V] endpoint + batch cap + unauth (deeplibby `overdrive_client.go`); CORS **[?]** |
| **Physical ILS -- DAIA** | physical | `proxied` | none/scoped-token | per-item (some servers list) | library-hosted, usually not permissive | `locations[]`: library, call number, on-shelf/loaned, due date | [R] DAIA is a read spec; hosting/CORS varies **[?]** |
| **Physical ILS -- ILS-DI / PAIA** | physical | `proxied` | scoped-token | per-item | not permissive | `locations[]` + loan/hold status | [R] auth'd patron API -> proxy **[?]** |
| **hoopla** | digital | `proxied` | scoped-token | n/a (mostly always-available) | not permissive | typically instant-borrow, no holds -> status `available` | [R] simultaneous-access model; API is patron-authed **[?]** |
| **Boundless / Axis 360** | digital | `proxied` | scoped-token | vendor API | not permissive | copies/holds like OverDrive | [R] patron-authed vendor API **[?]** |
| **cloudLibrary** | digital | `proxied` | scoped-token | vendor API | not permissive | copies/holds | [R] patron-authed vendor API **[?]** |

## How the transport choice flows to the adapter

- **`direct`** providers issue the browser fetch straight to the source. Only viable when
  the source is unauthenticated (or public-key) **and** its CORS allows the deploy origin.
  OverDrive/Thunder is the reference `direct` adapter -- but its CORS from a static site
  origin is unverified, so the adapter supports a `proxied` fallback with **identical**
  normalized output (`tasks/004`, proven by test).
- **`proxied`** providers POST the batch id list to the optional edge function
  (`{provider, slug, ids}`), which calls the source, strips secrets, and returns the raw
  response for the client to normalize. This is required whenever auth is a scoped token
  (must not ship in the browser) or CORS blocks a direct call -- i.e. every physical ILS
  and every patron-authed digital vendor above. A pure-`direct` deployment stays
  backend-free; the proxy is enabled only for the providers that need it.

## Digital vs. physical

The normalized model is a superset: digital sources fill `copiesOwned/Available`,
`holdsCount`, `estimatedWaitDays`; a physical ILS fills `locations[]` (per-branch call
number + shelf status). One adapter interface serves both; the UI renders only the
normalized shape, so adding a physical ILS does not change the templates.

That last clause was aspirational until `tasks/288`: `page.html` hardcoded
`overdrive-reserve` and emitted no other scheme's attribute, so the bundled DAIA adapter
collected nothing and never ran. An adapter is now wired by a row in
`hugo/data/lcat/availabilityAttrs.toml` mapping its `bf:source` scheme to its
`registerAdapter` `domAttr` -- no layout edit. `hugo/availability_seam_test.cjs` fails if
any registered adapter's attribute is absent from a real rendered page.

## Open items (tracked in `tasks/004`)

- The proxy **function** itself is a deployment artifact (an edge/serverless handler),
  not shipped by the module; the client contract is defined and tested.
- A **physical-ILS adapter** (DAIA/ILS-DI) populating `locations[]` proves the superset.
  Shipped and reachable since `tasks/288` (`exampleSite` carries a print edition with a
  `daia` document id); still wants a live endpoint and the proxy to validate against a
  real ILS.
- A coarse **"available now" facet sidecar** (periodically refreshed, explicitly stale)
  is the only way to facet/sort by availability from the static index -- a Tier 2
  add-on; scope decision pending.
- **Live CORS checks** for each `direct` candidate against a real deploy origin.
