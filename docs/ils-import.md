# Importing works from an ILS (and Aspen Discovery parity)

How a library's holdings flow from an integrated library system (ILS) into libcat's
catalog, what libcat already does, and which adapters would bring it to parity with
[Aspen Discovery](https://github.com/Aspen-Discovery/aspen-discovery) for ILS
integration.

The short version: libcat's architecture is **already Aspen-shaped** -- a bib
ingest layer that groups editions into works, a derived discovery index, and a
view-time real-time-availability layer. The seams exist (`ingest.Provider` registry,
`lcat-availability.js` adapter registry). What is missing is a set of concrete
*adapters* for live ILS protocols, and -- if full parity is a goal -- a patron /
circulation subsystem libcat deliberately does not have today.

## The theoretical flow

An ILS is the system of record for a library's bibliographic data and holdings. To
put those works in the libcat catalog, the flow is:

1. **Harvest** (a `RoleIngest` provider). Pull bib records from the ILS. Two modes:
   - *Full*: a complete MARC export, or OAI-PMH `ListRecords` over the bib set.
   - *Incremental*: OAI-PMH with a `from`/`until` window and resumption tokens, or
     the ILS's changed-records API, so only new/changed/deleted records sync.

   The **OAI-PMH harvester** (`ingest/oaipmh`, tasks/361) does the live end for any
   OAI-capable ILS: it issues `ListRecords` over the metadata prefix (default
   `marc21`), follows resumption-token pagination to the end, honours the `from`/
   `until` incremental window and `set` selector, decodes each record's MARCXML, and
   feeds it through the shared MARC crosswalk. Records the endpoint marks deleted are
   skipped. Run it:

   ```sh
   lcat ingest --provider oai --source https://ils.example/cgi-bin/koha/oai.pl \
     --feed koha --param set=biblios --param from=2026-07-01 --out data/out
   ```

   Per-ILS export APIs (Koha REST, Sierra) are the follow-ups where OAI is absent or
   thin. The `marc` file provider remains for staged exports.

2. **Convert**. MARC → BIBFRAME via the libcodex crosswalk (the path the `marc`
   provider already uses), producing `ingest.Record`s: stable two-tier identity keys
   plus the Work and Instance BIBFRAME. Crosswalk-lossy MARC fields ride the
   `lcat:marcVerbatim` sidecar (`docs/marc-fidelity.md`), so nothing is silently
   dropped and MARC export round-trips.

3. **Cluster into works ("Grouped Works")**. `identity.WorkKey` (normalized
   primary-creator + main-title + language) clusters a work's editions and formats
   into one Work with a stable minted `w…` id; each Instance keeps its ILS item/bib
   ids. This is libcat's equivalent of Aspen's *Grouped Works*.

4. **Grain store**. Each Work is written as a canonical RDFC-1.0 BIBFRAME grain in
   the blob store -- the source of truth (`docs/ARCHITECTURE.md` §3a). An incremental
   harvest upserts only the changed grains; a deletion tombstones one.

5. **Reconcile withdrawals**. Records absent from a full harvest (or flagged deleted
   incrementally) are withdrawn by the reconciliation arm (tasks/078), so a title
   pulled from the ILS leaves the catalog rather than lingering.

6. **Project**. `lcat project` builds `catalog.json` + `facets.json` + `similar.json`
   + `redirects.json`, which the Hugo module and the roaringrange client-side search
   render into the static OPAC (`docs/ARCHITECTURE.md` §7).

7. **Real-time availability** (a `lcat-availability.js` adapter, view-time). The OPAC
   asks the ILS live for each item's status, keyed by the ILS id the projector emits
   on the Instance as a `data-<scheme>-id` DOM attribute (the scheme→attribute table
   is `hugo/data/lcat/availabilityAttrs.toml`, tasks/288). Availability is **never
   stored in the graph** (ARCHITECTURE §5) -- it is volatile and fetched per view.
   Today libcat ships two availability adapters: OverDrive/Thunder and DAIA (a
   generic physical-ILS protocol).

8. **(Optional / future) Patron circulation**. Login, place/cancel holds, checkouts,
   renewals, fines and reading history, via the ILS's patron API. libcat has none of
   this -- it is a cataloging + discovery layer, and patron state is inherently
   live-only, outside the static catalog.

The important property: steps 1 and 7 are **the two adapter seams**. Adding an ILS is
"a `Register` call plus the provider's own package -- no libcat fork" (per
`ingest/provider.go`), and a new availability source is a `registerAdapter` call plus
one `availabilityAttrs.toml` row. Patron circulation (step 8) would be a genuinely new
backend subsystem.

## How Aspen Discovery does it

Aspen (open-source, ByWater Solutions; VuFind/Pika lineage) integrates an ILS in
three layers:

- **Bib indexing**: a full nightly MARC export from the ILS (plus incremental
  updates and non-MARC "side loads"), grouped into *Grouped Works* and indexed into
  Apache Solr.
- **ILS account integration (drivers)**: real-time, per-ILS, for both *item
  availability/status* and *patron accounts* (auth, checkouts, holds, fines,
  renewals, reading history). Drivers exist for Koha, Sierra (III), Symphony /
  SirsiDynix, Polaris, CARL.X, Evergreen, Horizon, Millennium, and others, over each
  ILS's API (ILS-DI, Koha REST, Sierra REST, SirsiDynix Web Services, Polaris API)
  and the broadly-supported **SIP2** protocol.
- **eContent drivers**: OverDrive, Hoopla, Cloud Library / Bibliotheca, Axis 360 /
  Boundless, Palace Project, Libby -- each providing that platform's bib records,
  availability, and checkout/hold.
- **Enrichment**: Novelist (series, recommendations), Syndetics / Content Café
  (covers, reviews, tables of contents), Wikipedia, ratings.

## Where libcat already aligns

| Aspen concept | libcat equivalent | State |
|---|---|---|
| MARC indexing | `ingest/marc` provider → BIBFRAME grains | present (file input) |
| Live bib harvest from an ILS | `ingest/oaipmh` (OAI-PMH ListRecords, resumption, incremental, deletions) | present (OAI); per-ILS APIs are follow-ups |
| Grouped Works | `identity.WorkKey` clustering | present |
| Solr discovery index | `lcat project` → static catalog.json + roaringrange search | present (static-first, no server) |
| eContent driver (OverDrive) | `ingest/overdrive` (direct JSON→BIBFRAME) + Thunder availability adapter | present |
| Real-time item availability | `lcat-availability.js` adapters (OverDrive, DAIA) | partial |
| Enrichment (covers/subjects/related) | cover extras, LCSH/OpenLibrary/LC-Hub enrichers, computed "more like this" | partial |
| Patron accounts / circulation | -- | absent by design |

## Adapters to add for parity

Grouped by layer, roughly in value order. Each slots into an existing seam except
where noted.

**A. Live bib harvest (import works FROM an ILS)** -- new `RoleIngest` providers:
- **OAI-PMH harvester** -- **shipped** (`ingest/oaipmh`, tasks/361). One adapter
  reaches every OAI-capable ILS (Koha, Evergreen, many others) with full +
  incremental `from`/`until` sync and deletions.
- Per-ILS export/changed-records APIs where OAI is absent or thin: **Koha REST**,
  **Sierra API**, **Symphony / SirsiDynix**, **Polaris API**, **Evergreen**.

**B. Real-time availability** -- new `lcat-availability.js` adapters:
- **SIP2** (broad ILS support via one protocol -- the availability parallel to OAI on
  the harvest side).
- **ILS-DI** availability (Koha, Symphony).
- Per-ILS REST availability: **Koha REST**, **Sierra API**, **Polaris API**.
- (Already shipped: DAIA, OverDrive/Thunder.)

**C. eContent** -- each a bib provider (A) + an availability adapter (B):
- **Hoopla**, **Cloud Library / Bibliotheca**, **Axis 360 / Boundless**,
  **Palace Project**.
- (Already shipped: OverDrive.)

**D. Patron / circulation** -- a *new backend subsystem*, not just an adapter, and a
scope decision: does libcat stay discovery-only, or add patron accounts? Parity would
need patron auth (SIP2 or the ILS patron API) and holds / checkouts / renewals /
fines. This is the largest gap and the one that most changes libcat's shape
(stateless static-first vs. live patron state).

**E. Commercial enrichment** (optional): **Novelist**, **Syndetics / Content Café**
adapters, as `RoleEnrich` sources, for covers / reviews / series where a library
licenses them.

## Recommended phasing

1. **OAI-PMH harvester + SIP2 availability** -- two generic protocol adapters that
   between them cover the largest number of ILSes, and both fit the existing seams
   with no architectural change.
2. **Koha REST + Sierra API** (harvest + availability) -- the two most common
   open/commercial ILSes, for richer incremental sync and item status than the
   generic protocols expose.
3. **Additional eContent** (Hoopla, Cloud Library, Axis 360, Palace).
4. **Patron / circulation** -- gated on the scope decision above; the biggest lift.
