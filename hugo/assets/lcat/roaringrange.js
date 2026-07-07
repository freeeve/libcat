/* @ts-self-types="./roaringrange.d.ts" */

/**
 * Result of [`RrsIndex::count_estimate`]: a result count and whether it is
 * exact (single-trigram unfiltered query) or an upper bound.
 */
export class CountEstimate {
    static __wrap(ptr) {
        const obj = Object.create(CountEstimate.prototype);
        obj.__wbg_ptr = ptr;
        CountEstimateFinalization.register(obj, obj.__wbg_ptr, obj);
        return obj;
    }
    __destroy_into_raw() {
        const ptr = this.__wbg_ptr;
        this.__wbg_ptr = 0;
        CountEstimateFinalization.unregister(this);
        return ptr;
    }
    free() {
        const ptr = this.__destroy_into_raw();
        wasm.__wbg_countestimate_free(ptr, 0);
    }
    /**
     * The exact count, or the upper bound when `exact` is false.
     * @returns {number}
     */
    get count() {
        const ret = wasm.countestimate_count(this.__wbg_ptr);
        return ret;
    }
    /**
     * Whether `count` is exact rather than an upper bound.
     * @returns {boolean}
     */
    get exact() {
        const ret = wasm.countestimate_exact(this.__wbg_ptr);
        return ret !== 0;
    }
}
if (Symbol.dispose) CountEstimate.prototype[Symbol.dispose] = CountEstimate.prototype.free;

/**
 * Result of [`RrsIndex::filter_ids`]: the surviving doc IDs (input ranking
 * order preserved) and search-filtered facet counts over them. The `ids` and
 * `facetCounts` getters copy across the wasm boundary on every access, so read each
 * once into a JS variable rather than re-touching them in a loop.
 */
export class FilteredIds {
    static __wrap(ptr) {
        const obj = Object.create(FilteredIds.prototype);
        obj.__wbg_ptr = ptr;
        FilteredIdsFinalization.register(obj, obj.__wbg_ptr, obj);
        return obj;
    }
    __destroy_into_raw() {
        const ptr = this.__wbg_ptr;
        this.__wbg_ptr = 0;
        FilteredIdsFinalization.unregister(this);
        return ptr;
    }
    free() {
        const ptr = this.__destroy_into_raw();
        wasm.__wbg_filteredids_free(ptr, 0);
    }
    /**
     * Search-filtered facet counts over the survivors, as a JS array of
     * `{ field, cats: [{ name, count }] }` (same shape as `facets()`); an empty array when no
     * facet sidecar is open.
     * @returns {any}
     */
    facetCounts() {
        const ret = wasm.filteredids_facetCounts(this.__wbg_ptr);
        return ret;
    }
    /**
     * The surviving doc IDs as a `Uint32Array`, in the input ranking order.
     * @returns {Uint32Array}
     */
    get ids() {
        const ret = wasm.filteredids_ids(this.__wbg_ptr);
        var v1 = getArrayU32FromWasm0(ret[0], ret[1]).slice();
        wasm.__wbindgen_free(ret[0], ret[1] * 4, 4);
        return v1;
    }
}
if (Symbol.dispose) FilteredIds.prototype[Symbol.dispose] = FilteredIds.prototype.free;

/**
 * The in-browser model2vec query embedder (mode 2) exposed to JavaScript: turns
 * query text into a `Float32Array` vector with no backend, to feed
 * [`RrviIndex::search`]. Built with `wasm-pack build --features "wasm vector"`.
 */
export class Model2vecEmbedder {
    static __wrap(ptr) {
        const obj = Object.create(Model2vecEmbedder.prototype);
        obj.__wbg_ptr = ptr;
        Model2vecEmbedderFinalization.register(obj, obj.__wbg_ptr, obj);
        return obj;
    }
    __destroy_into_raw() {
        const ptr = this.__wbg_ptr;
        this.__wbg_ptr = 0;
        Model2vecEmbedderFinalization.unregister(this);
        return ptr;
    }
    free() {
        const ptr = this.__destroy_into_raw();
        wasm.__wbg_model2vecembedder_free(ptr, 0);
    }
    /**
     * Vector dimensionality (must match the `RRVI` index it queries).
     * @returns {number}
     */
    dim() {
        const ret = wasm.model2vecembedder_dim(this.__wbg_ptr);
        return ret >>> 0;
    }
    /**
     * Embeds `text` into a `Float32Array` query vector (BERT tokenize → static
     * embedding mean-pool → L2-normalize). Pass it to `RrviIndex.search`.
     * @param {string} text
     * @returns {Float32Array}
     */
    embed(text) {
        const ptr0 = passStringToWasm0(text, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.model2vecembedder_embed(this.__wbg_ptr, ptr0, len0);
        var v2 = getArrayF32FromWasm0(ret[0], ret[1]).slice();
        wasm.__wbindgen_free(ret[0], ret[1] * 4, 4);
        return v2;
    }
    /**
     * Downloads the `RRM2` artifact at `url` once (a plain GET; ~tens of MB,
     * browser-cached) and builds the embedder. Returns a `Promise<Model2vecEmbedder>`.
     * @param {string} url
     * @returns {Promise<Model2vecEmbedder>}
     */
    static open(url) {
        const ptr0 = passStringToWasm0(url, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.model2vecembedder_open(ptr0, len0);
        return ret;
    }
}
if (Symbol.dispose) Model2vecEmbedder.prototype[Symbol.dispose] = Model2vecEmbedder.prototype.free;

/**
 * Result of [`RrtIndex::searchPrefixCapped`]: the matching doc IDs and whether the
 * prefix matched more dictionary terms than the union cap. When `truncated` is
 * `true`, `ids` is a bounded approximation (the first cap terms lexicographically),
 * so a UI can surface "showing partial matches".
 */
export class PrefixSearch {
    static __wrap(ptr) {
        const obj = Object.create(PrefixSearch.prototype);
        obj.__wbg_ptr = ptr;
        PrefixSearchFinalization.register(obj, obj.__wbg_ptr, obj);
        return obj;
    }
    __destroy_into_raw() {
        const ptr = this.__wbg_ptr;
        this.__wbg_ptr = 0;
        PrefixSearchFinalization.unregister(this);
        return ptr;
    }
    free() {
        const ptr = this.__destroy_into_raw();
        wasm.__wbg_prefixsearch_free(ptr, 0);
    }
    /**
     * The matching doc IDs as a `Uint32Array`, most popular first.
     * @returns {Uint32Array}
     */
    get ids() {
        const ret = wasm.prefixsearch_ids(this.__wbg_ptr);
        var v1 = getArrayU32FromWasm0(ret[0], ret[1]).slice();
        wasm.__wbindgen_free(ret[0], ret[1] * 4, 4);
        return v1;
    }
    /**
     * Whether the prefix matched more terms than the cap, making `ids` a bounded
     * approximation of the full prefix union.
     * @returns {boolean}
     */
    get truncated() {
        const ret = wasm.prefixsearch_truncated(this.__wbg_ptr);
        return ret !== 0;
    }
}
if (Symbol.dispose) PrefixSearch.prototype[Symbol.dispose] = PrefixSearch.prototype.free;

/**
 * The `RRSB` BM25 impact sidecar (`.rrb`): one quantized impact byte per
 * (word, doc), addressed through the posting bitmaps the term search already
 * fetched. Boot is the 64 B header plus the resident sparse entry index
 * (~8 B per 512 terms — ~3 MB for the 187M-term OpenAlex corpus).
 */
export class RrbIndex {
    static __wrap(ptr) {
        const obj = Object.create(RrbIndex.prototype);
        obj.__wbg_ptr = ptr;
        RrbIndexFinalization.register(obj, obj.__wbg_ptr, obj);
        return obj;
    }
    __destroy_into_raw() {
        const ptr = this.__wbg_ptr;
        this.__wbg_ptr = 0;
        RrbIndexFinalization.unregister(this);
        return ptr;
    }
    free() {
        const ptr = this.__destroy_into_raw();
        wasm.__wbg_rrbindex_free(ptr, 0);
    }
    /**
     * Total documents in the corpus the sidecar was built over (BM25's N).
     * @returns {number}
     */
    docCount() {
        const ret = wasm.rrbindex_docCount(this.__wbg_ptr);
        return ret >>> 0;
    }
    /**
     * Boots the sidecar at `url` (header + resident sparse index). Returns a
     * `Promise<RrbIndex>`.
     * @param {string} url
     * @returns {Promise<RrbIndex>}
     */
    static open(url) {
        const ptr0 = passStringToWasm0(url, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrbindex_open(ptr0, len0);
        return ret;
    }
}
if (Symbol.dispose) RrbIndex.prototype[Symbol.dispose] = RrbIndex.prototype.free;

/**
 * A standalone facet sidecar (`RRSF`) exposed to JavaScript, opened on its own
 * without the text index. Lets the vector/semantic path filter results and show
 * facet counts even when the (much larger) `.rrs` text index isn't present —
 * they share the doc-ID space, so the `.rrf` applies directly.
 */
export class RrfFacets {
    static __wrap(ptr) {
        const obj = Object.create(RrfFacets.prototype);
        obj.__wbg_ptr = ptr;
        RrfFacetsFinalization.register(obj, obj.__wbg_ptr, obj);
        return obj;
    }
    __destroy_into_raw() {
        const ptr = this.__wbg_ptr;
        this.__wbg_ptr = 0;
        RrfFacetsFinalization.unregister(this);
        return ptr;
    }
    free() {
        const ptr = this.__destroy_into_raw();
        wasm.__wbg_rrffacets_free(ptr, 0);
    }
    /**
     * Exact head+tail filtered counts for specific `[field, category]` pairs over
     * `ids` — the on-demand companion to `filterIds().facetCounts()`, which prices
     * only the top categories per field. Use it to fetch the exact count of a
     * long-tail category the user expands or searches for (each pair ≈ one tail
     * fetch). Returns one count per pair, in order, as a `Uint32Array`; an unknown
     * `[field, category]` yields 0.
     * @param {Uint32Array} ids
     * @param {Array<any>} pairs
     * @returns {Promise<Uint32Array>}
     */
    countsFor(ids, pairs) {
        const ptr0 = passArray32ToWasm0(ids, wasm.__wbindgen_malloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrffacets_countsFor(this.__wbg_ptr, ptr0, len0, pairs);
        return ret;
    }
    /**
     * Facet fields and categories with full-corpus counts, as a JS array of
     * `{ field, cats: [{ name, count }] }` (same shape as `RrsIndex.facets()`).
     * @returns {any}
     */
    facets() {
        const ret = wasm.rrffacets_facets(this.__wbg_ptr);
        return ret;
    }
    /**
     * Filters a ranked doc-ID list by the selected facets (same contract as
     * `RrsIndex.filterIds`, including the `wantCounts` count-wave gate). Resolves to a
     * `FilteredIds`.
     * @param {Uint32Array} ids
     * @param {Array<any>} filters
     * @param {boolean} want_counts
     * @returns {Promise<FilteredIds>}
     */
    filterIds(ids, filters, want_counts) {
        const ptr0 = passArray32ToWasm0(ids, wasm.__wbindgen_malloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrffacets_filterIds(this.__wbg_ptr, ptr0, len0, filters, want_counts);
        return ret;
    }
    /**
     * Boots the facet sidecar at `url` — **meta region only** (header + field +
     * category tables + string blob), so `facets()` (the global list of names +
     * full-corpus counts the browse UI shows) is ready in a couple of ranged reads
     * even on a sidecar with 100k+ categories. The head/tail postings are NOT loaded
     * here; `filterIds`/`facetCounts`/`countsFor` range-fetch exactly the postings
     * they touch (see task 053). Resolves to an `RrfFacets`.
     * @param {string} url
     * @returns {Promise<RrfFacets>}
     */
    static open(url) {
        const ptr0 = passStringToWasm0(url, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrffacets_open(ptr0, len0);
        return ret;
    }
}
if (Symbol.dispose) RrfFacets.prototype[Symbol.dispose] = RrfFacets.prototype.free;

/**
 * A resident `RRHC` boot bundle exposed to JavaScript: **one GET** fetches every
 * member's boot region (index sparse, facet meta, record header, dict, lookup
 * header); [`inlined`](Self::inlined) hands each reader its bytes for the
 * `fromBoot` constructors, so a whole catalog boots in a single round trip
 * instead of one cold open per member. Content-hash the URL (immutable) and warm
 * visits boot from the browser cache with zero network.
 */
export class RrhcBundle {
    static __wrap(ptr) {
        const obj = Object.create(RrhcBundle.prototype);
        obj.__wbg_ptr = ptr;
        RrhcBundleFinalization.register(obj, obj.__wbg_ptr, obj);
        return obj;
    }
    __destroy_into_raw() {
        const ptr = this.__wbg_ptr;
        this.__wbg_ptr = 0;
        RrhcBundleFinalization.unregister(this);
        return ptr;
    }
    free() {
        const ptr = this.__destroy_into_raw();
        wasm.__wbg_rrhcbundle_free(ptr, 0);
    }
    /**
     * The inlined boot bytes for the member whose data-file name is `name` (a
     * `Uint8Array`), or `undefined` when the bundle carries it by range
     * reference only — the caller falls back to that member's cold open.
     * @param {string} name
     * @returns {Uint8Array | undefined}
     */
    inlined(name) {
        const ptr0 = passStringToWasm0(name, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrhcbundle_inlined(this.__wbg_ptr, ptr0, len0);
        let v2;
        if (ret[0] !== 0) {
            v2 = getArrayU8FromWasm0(ret[0], ret[1]).slice();
            wasm.__wbindgen_free(ret[0], ret[1] * 1, 1);
        }
        return v2;
    }
    /**
     * Fetches and parses the bundle at `url`. Returns a `Promise<RrhcBundle>`.
     * @param {string} url
     * @returns {Promise<RrhcBundle>}
     */
    static open(url) {
        const ptr0 = passStringToWasm0(url, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrhcbundle_open(ptr0, len0);
        return ret;
    }
}
if (Symbol.dispose) RrhcBundle.prototype[Symbol.dispose] = RrhcBundle.prototype.free;

/**
 * A range-fetchable [`Catalog`] exposed to JavaScript: one object bundling the
 * `RRS` index with an optional `RRSF` facet sidecar and `RRSR` record store, so
 * the whole "search → ranked IDs + records + facet counts" flow is one call.
 * Mirrors [`RrsIndex`]/[`RrsRecords`]; adopt it in place of wiring those three
 * together by hand.
 */
export class RrsCatalog {
    static __wrap(ptr) {
        const obj = Object.create(RrsCatalog.prototype);
        obj.__wbg_ptr = ptr;
        RrsCatalogFinalization.register(obj, obj.__wbg_ptr, obj);
        return obj;
    }
    __destroy_into_raw() {
        const ptr = this.__wbg_ptr;
        this.__wbg_ptr = 0;
        RrsCatalogFinalization.unregister(this);
        return ptr;
    }
    free() {
        const ptr = this.__destroy_into_raw();
        wasm.__wbg_rrscatalog_free(ptr, 0);
    }
    /**
     * Returns the facet fields and their full-corpus category counts as a JS array of
     * `{ field, cats: [{ name, count }] }`, or an empty array when no facet sidecar is attached.
     * Mirrors [`RrsIndex::facets`].
     * @returns {any}
     */
    facets() {
        const ret = wasm.rrscatalog_facets(this.__wbg_ptr);
        if (ret[2]) {
            throw takeFromExternrefTable0(ret[1]);
        }
        return takeFromExternrefTable0(ret[0]);
    }
    /**
     * Number of n-grams in the index dictionary.
     * @returns {number}
     */
    ngramCount() {
        const ret = wasm.rrscatalog_ngramCount(this.__wbg_ptr);
        if (ret[2]) {
            throw takeFromExternrefTable0(ret[1]);
        }
        return ret[0] >>> 0;
    }
    /**
     * Boots a catalog over the index at `index_url` alone (header + sparse
     * dictionary). Attach the optional sidecars with [`RrsCatalog::open_facets`]
     * and [`RrsCatalog::open_records`]. Returns a `Promise<RrsCatalog>`.
     * @param {string} index_url
     * @returns {Promise<RrsCatalog>}
     */
    static open(index_url) {
        const ptr0 = passStringToWasm0(index_url, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrscatalog_open(ptr0, len0);
        return ret;
    }
    /**
     * Boots the catalog with all three resources at once: the index at
     * `index_url`, the facet sidecar at `facets_url`, and the record store
     * (`records_idx_url` offset index + `records_bin_url` blob). Returns a
     * `Promise<RrsCatalog>`.
     * @param {string} index_url
     * @param {string} facets_url
     * @param {string} records_idx_url
     * @param {string} records_bin_url
     * @returns {Promise<RrsCatalog>}
     */
    static openAll(index_url, facets_url, records_idx_url, records_bin_url) {
        const ptr0 = passStringToWasm0(index_url, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ptr1 = passStringToWasm0(facets_url, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len1 = WASM_VECTOR_LEN;
        const ptr2 = passStringToWasm0(records_idx_url, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len2 = WASM_VECTOR_LEN;
        const ptr3 = passStringToWasm0(records_bin_url, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len3 = WASM_VECTOR_LEN;
        const ret = wasm.rrscatalog_openAll(ptr0, len0, ptr1, len1, ptr2, len2, ptr3, len3);
        return ret;
    }
    /**
     * Opens the facet sidecar at `url` and attaches it, enabling filtered search
     * and facet counts.
     * @param {string} url
     * @returns {Promise<void>}
     */
    openFacets(url) {
        const ptr0 = passStringToWasm0(url, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrscatalog_openFacets(this.__wbg_ptr, ptr0, len0);
        return ret;
    }
    /**
     * Opens the record store (`idx_url` offset index + `bin_url` record blob)
     * and attaches it, so [`RrsCatalog::search`] returns record bytes.
     * @param {string} idx_url
     * @param {string} bin_url
     * @returns {Promise<void>}
     */
    openRecords(idx_url, bin_url) {
        const ptr0 = passStringToWasm0(idx_url, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ptr1 = passStringToWasm0(bin_url, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len1 = WASM_VECTOR_LEN;
        const ret = wasm.rrscatalog_openRecords(this.__wbg_ptr, ptr0, len0, ptr1, len1);
        return ret;
    }
    /**
     * Opens the record store (`idx_url` offset index + `bin_url` record blob)
     * with the shared zstd dictionary `dict` (the `*.dict` sidecar's bytes,
     * passed as a `Uint8Array`) and attaches it, so a version-2 compressed store
     * inflates records transparently in [`RrsCatalog::search`]. Requires the
     * crate to be built with the `zstd` feature for a compressed store; a raw
     * store ignores the dictionary.
     * @param {string} idx_url
     * @param {string} bin_url
     * @param {Uint8Array} dict
     * @returns {Promise<void>}
     */
    openRecordsWithDict(idx_url, bin_url, dict) {
        const ptr0 = passStringToWasm0(idx_url, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ptr1 = passStringToWasm0(bin_url, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len1 = WASM_VECTOR_LEN;
        const ptr2 = passArray8ToWasm0(dict, wasm.__wbindgen_malloc);
        const len2 = WASM_VECTOR_LEN;
        const ret = wasm.rrscatalog_openRecordsWithDict(this.__wbg_ptr, ptr0, len0, ptr1, len1, ptr2, len2);
        return ret;
    }
    /**
     * Runs the full search flow and resolves to a JS object:
     * `{ ids: Uint32Array, records: Array<Uint8Array|null> | null,
     * facetCounts: Array<{field, cats:[{name, count}]}> | null }`.
     *
     * `filters` is an array of `[field, category]` pairs (e.g.
     * `[["format","ebook"],["language","en"]]`); an empty array `[]` means no filter, and a
     * malformed entry throws. Within a field categories OR, across fields they AND. The page
     * covers ranked doc IDs `[offset, offset+len)`; `max_missing` is the fuzzy
     * tolerance (0 = strict). `records`/`facetCounts` are `null` unless the
     * matching sidecar is attached.
     * @param {string} query
     * @param {number} offset
     * @param {number} len
     * @param {number} max_missing
     * @param {Array<any>} filters
     * @returns {Promise<any>}
     */
    search(query, offset, len, max_missing, filters) {
        const ptr0 = passStringToWasm0(query, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrscatalog_search(this.__wbg_ptr, ptr0, len0, offset, len, max_missing, filters);
        return ret;
    }
}
if (Symbol.dispose) RrsCatalog.prototype[Symbol.dispose] = RrsCatalog.prototype.free;

/**
 * A stateful pagination cursor exposed to JavaScript.
 */
export class RrsCursor {
    static __wrap(ptr) {
        const obj = Object.create(RrsCursor.prototype);
        obj.__wbg_ptr = ptr;
        RrsCursorFinalization.register(obj, obj.__wbg_ptr, obj);
        return obj;
    }
    __destroy_into_raw() {
        const ptr = this.__wbg_ptr;
        this.__wbg_ptr = 0;
        RrsCursorFinalization.unregister(this);
        return ptr;
    }
    free() {
        const ptr = this.__destroy_into_raw();
        wasm.__wbg_rrscursor_free(ptr, 0);
    }
    /**
     * The search-filtered facet counts as a JS array of `{ field, cats: [{ name, count }] }` —
     * how many of this query's results fall in each category; an empty array when no facet
     * sidecar is open.
     * @returns {any}
     */
    facetCounts() {
        const ret = wasm.rrscursor_facetCounts(this.__wbg_ptr);
        return ret;
    }
    /**
     * Number of head (popular) results — available immediately, no tail fetch.
     * @returns {number}
     */
    headCount() {
        const ret = wasm.rrscursor_headCount(this.__wbg_ptr);
        return ret >>> 0;
    }
    /**
     * Fetches the lazy tail intersection on demand; afterwards `loaded`/`page`
     * cover the full result set.
     * @returns {Promise<void>}
     */
    loadTail() {
        const ret = wasm.rrscursor_loadTail(this.__wbg_ptr);
        return ret;
    }
    /**
     * Number of doc IDs materialized so far.
     * @returns {number}
     */
    loaded() {
        const ret = wasm.rrscursor_loaded(this.__wbg_ptr);
        return ret >>> 0;
    }
    /**
     * Returns the next `n` doc IDs as a `Uint32Array`. Pages within the head
     * cost no fetches; crossing into the tail triggers one concurrent wave.
     * @param {number} n
     * @returns {Promise<Uint32Array>}
     */
    next(n) {
        const ret = wasm.rrscursor_next(this.__wbg_ptr, n);
        return ret;
    }
    /**
     * Random-access page: up to `limit` doc IDs starting at `offset`. Paging
     * backward never fetches; paging past the head fetches the tail once.
     * @param {number} offset
     * @param {number} limit
     * @returns {Promise<Uint32Array>}
     */
    page(offset, limit) {
        const ret = wasm.rrscursor_page(this.__wbg_ptr, offset, limit);
        return ret;
    }
    /**
     * Whether loading the tail could still add results (its intersection is unfetched).
     * @returns {boolean}
     */
    pendingTail() {
        const ret = wasm.rrscursor_pendingTail(this.__wbg_ptr);
        return ret !== 0;
    }
}
if (Symbol.dispose) RrsCursor.prototype[Symbol.dispose] = RrsCursor.prototype.free;

/**
 * A range-fetchable RRS index exposed to JavaScript. Optionally carries an
 * opened facet sidecar (`RRSF`) used to filter queries.
 */
export class RrsIndex {
    static __wrap(ptr) {
        const obj = Object.create(RrsIndex.prototype);
        obj.__wbg_ptr = ptr;
        RrsIndexFinalization.register(obj, obj.__wbg_ptr, obj);
        return obj;
    }
    __destroy_into_raw() {
        const ptr = this.__wbg_ptr;
        this.__wbg_ptr = 0;
        RrsIndexFinalization.unregister(this);
        return ptr;
    }
    free() {
        const ptr = this.__destroy_into_raw();
        wasm.__wbg_rrsindex_free(ptr, 0);
    }
    /**
     * Exact-or-bounded result count for a strict-AND `query` (+ optional facet
     * `filters`), **without fetching any posting body**: KB-scale dictionary +
     * posting-header reads, plus the resident facet counts. `exact` is true only
     * for a single-trigram unfiltered query; otherwise `count` is an upper bound
     * (the smallest per-trigram cardinality, min'd with the filter's resident
     * count bound). Not valid for fuzzy (`max_missing > 0`) matching.
     * @param {string} query
     * @param {Array<any> | null} [filters]
     * @returns {Promise<CountEstimate>}
     */
    countEstimate(query, filters) {
        const ptr0 = passStringToWasm0(query, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrsindex_countEstimate(this.__wbg_ptr, ptr0, len0, isLikeNone(filters) ? 0 : addToExternrefTable0(filters));
        return ret;
    }
    /**
     * Exact head+tail filtered counts for specific `[field, category]` pairs over
     * `ids` — the on-demand companion to `filterIds().facetCounts()` for long-tail
     * categories the cap leaves head-only. One count per pair, in order
     * (`Uint32Array`); an unknown pair, or no facet sidecar open, yields 0.
     * @param {Uint32Array} ids
     * @param {Array<any>} pairs
     * @returns {Promise<Uint32Array>}
     */
    countsFor(ids, pairs) {
        const ptr0 = passArray32ToWasm0(ids, wasm.__wbindgen_malloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrsindex_countsFor(this.__wbg_ptr, ptr0, len0, pairs);
        return ret;
    }
    /**
     * Returns the facet fields and their categories as a JS array of
     * `{ field, cats: [{ name, count }] }`. An empty array when no sidecar is open. Counts are
     * full-corpus and free (served from the in-memory meta region).
     * @returns {any}
     */
    facets() {
        const ret = wasm.rrsindex_facets(this.__wbg_ptr);
        return ret;
    }
    /**
     * Filters a ranked doc-ID list (e.g. semantic/vector hits) by the selected
     * facets, preserving the input order, and returns the survivors plus
     * search-filtered facet counts over them. Because `vector_id == doc_id`, the
     * vector path reuses the same `RRSF` sidecar the trigram path uses — no
     * remapping. `filters` is an array of `[field, category]` pairs (within a field categories
     * OR, across fields they AND); a malformed entry throws. With no sidecar open or no filters,
     * the IDs pass through unchanged. `wantCounts` gates the facet-count fetch wave: pass `false`
     * when the caller will not display counts (e.g. before the facet heads are resident) to skip
     * hundreds of range GETs; filtering still runs. Resolves to a `FilteredIds`.
     * @param {Uint32Array} ids
     * @param {Array<any>} filters
     * @param {boolean} want_counts
     * @returns {Promise<FilteredIds>}
     */
    filterIds(ids, filters, want_counts) {
        const ptr0 = passArray32ToWasm0(ids, wasm.__wbindgen_malloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrsindex_filterIds(this.__wbg_ptr, ptr0, len0, filters, want_counts);
        return ret;
    }
    /**
     * Boots from a **resident** boot region (header + sparse index, an `RRHC`
     * bundle member) — zero fetches; per-query dict/posting reads still go to
     * `url`. Facets attach via [`openFacets`](Self::open_facets) or
     * [`openFacetsFromBoot`](Self::open_facets_from_boot).
     * @param {Uint8Array} boot
     * @param {string} url
     * @returns {RrsIndex}
     */
    static fromBoot(boot, url) {
        const ptr0 = passArray8ToWasm0(boot, wasm.__wbindgen_malloc);
        const len0 = WASM_VECTOR_LEN;
        const ptr1 = passStringToWasm0(url, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len1 = WASM_VECTOR_LEN;
        const ret = wasm.rrsindex_fromBoot(ptr0, len0, ptr1, len1);
        if (ret[2]) {
            throw takeFromExternrefTable0(ret[1]);
        }
        return RrsIndex.__wrap(ret[0]);
    }
    /**
     * Loads the facet category heads that search-filtered counts intersect
     * against — on a large sidecar a wave of hundreds of scattered small reads,
     * which is why a bundle boot defers it here instead of blocking first
     * paint. A no-op without an open sidecar.
     * @returns {Promise<void>}
     */
    loadFacetHeads() {
        const ret = wasm.rrsindex_loadFacetHeads(this.__wbg_ptr);
        return ret;
    }
    /**
     * Number of n-grams in the index dictionary.
     * @returns {number}
     */
    ngramCount() {
        const ret = wasm.rrsindex_ngramCount(this.__wbg_ptr);
        return ret >>> 0;
    }
    /**
     * Boots the index at `url`: fetches the header and sparse index. Returns a
     * `Promise<RrsIndex>` to JavaScript. Facets are not opened here; call
     * [`RrsIndex::open_facets`] afterward if a sidecar is available.
     * @param {string} url
     * @returns {Promise<RrsIndex>}
     */
    static open(url) {
        const ptr0 = passStringToWasm0(url, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrsindex_open(ptr0, len0);
        return ret;
    }
    /**
     * Boots the optional facet sidecar at `url` and attaches it to this index,
     * enabling [`RrsIndex::facets_json`] and filtered search.
     * @param {string} url
     * @returns {Promise<void>}
     */
    openFacets(url) {
        const ptr0 = passStringToWasm0(url, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrsindex_openFacets(this.__wbg_ptr, ptr0, len0);
        return ret;
    }
    /**
     * Like [`openFacets`](Self::open_facets) but booting from a **resident**
     * meta region (an `RRHC` bundle member) — zero fetches: full-corpus counts
     * and filtering work immediately. Call
     * [`loadFacetHeads`](Self::load_facet_heads) (off the boot critical path)
     * before search-filtered counts report non-zero.
     * @param {Uint8Array} meta
     * @param {string} url
     */
    openFacetsFromBoot(meta, url) {
        const ptr0 = passArray8ToWasm0(meta, wasm.__wbindgen_malloc);
        const len0 = WASM_VECTOR_LEN;
        const ptr1 = passStringToWasm0(url, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len1 = WASM_VECTOR_LEN;
        const ret = wasm.rrsindex_openFacetsFromBoot(this.__wbg_ptr, ptr0, len0, ptr1, len1);
        if (ret[1]) {
            throw takeFromExternrefTable0(ret[0]);
        }
    }
    /**
     * Estimated client-side bytes a search for `query` (plus the optional facet
     * `filters`, the same `[field, category]` pairs `searchCursorFiltered` takes)
     * would fetch — priced from KB-scale dictionary reads and the resident facet
     * table only; **no posting is fetched**. Compare against a routing threshold
     * to send expensive queries to a server-side search instead. `0` when a query
     * trigram is absent (the strict-AND search short-circuits to empty).
     * @param {string} query
     * @param {Array<any> | null} [filters]
     * @returns {Promise<number>}
     */
    queryCost(query, filters) {
        const ptr0 = passStringToWasm0(query, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrsindex_queryCost(this.__wbg_ptr, ptr0, len0, isLikeNone(filters) ? 0 : addToExternrefTable0(filters));
        return ret;
    }
    /**
     * Returns up to `limit` matching doc IDs, most-popular first. Resolves to a
     * `Uint32Array` in JavaScript.
     * @param {string} query
     * @param {number} limit
     * @returns {Promise<Uint32Array>}
     */
    search(query, limit) {
        const ptr0 = passStringToWasm0(query, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrsindex_search(this.__wbg_ptr, ptr0, len0, limit);
        return ret;
    }
    /**
     * Opens a stateful pagination cursor for `query` (one head fetch wave up
     * front). Resolves to an `RrsCursor`.
     * @param {string} query
     * @param {number} max_missing
     * @returns {Promise<RrsCursor>}
     */
    searchCursor(query, max_missing) {
        const ptr0 = passStringToWasm0(query, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrsindex_searchCursor(this.__wbg_ptr, ptr0, len0, max_missing);
        return ret;
    }
    /**
     * Like [`RrsIndex::search_cursor`] but ANDs the selected facets into the
     * result. `filters` is an array of `[field, category]` pairs (within a field categories OR,
     * across fields they AND); a malformed entry throws. The filter is applied only when a
     * sidecar is open and `filters` is non-empty. Resolves to an `RrsCursor`.
     * @param {string} query
     * @param {number} max_missing
     * @param {Array<any>} filters
     * @returns {Promise<RrsCursor>}
     */
    searchCursorFiltered(query, max_missing, filters) {
        const ptr0 = passStringToWasm0(query, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrsindex_searchCursorFiltered(this.__wbg_ptr, ptr0, len0, max_missing, filters);
        return ret;
    }
}
if (Symbol.dispose) RrsIndex.prototype[Symbol.dispose] = RrsIndex.prototype.free;

/**
 * A range-fetchable identifier exact-match index (`RRIL`) exposed to JavaScript:
 * resolves an ISBN/ASIN/… to the ranked doc IDs of the title(s) carrying it, over
 * HTTP Range. Pairs with the trigram index, which no longer carries identifiers.
 */
export class RrsLookup {
    static __wrap(ptr) {
        const obj = Object.create(RrsLookup.prototype);
        obj.__wbg_ptr = ptr;
        RrsLookupFinalization.register(obj, obj.__wbg_ptr, obj);
        return obj;
    }
    __destroy_into_raw() {
        const ptr = this.__wbg_ptr;
        this.__wbg_ptr = 0;
        RrsLookupFinalization.unregister(this);
        return ptr;
    }
    free() {
        const ptr = this.__destroy_into_raw();
        wasm.__wbg_rrslookup_free(ptr, 0);
    }
    /**
     * Boots from a **resident** copy of the 16-byte header (an `RRHC` bundle
     * member) — zero fetches; per-query probes still go to `url`.
     * @param {Uint8Array} header
     * @param {string} url
     * @returns {RrsLookup}
     */
    static fromBoot(header, url) {
        const ptr0 = passArray8ToWasm0(header, wasm.__wbindgen_malloc);
        const len0 = WASM_VECTOR_LEN;
        const ptr1 = passStringToWasm0(url, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len1 = WASM_VECTOR_LEN;
        const ret = wasm.rrslookup_fromBoot(ptr0, len0, ptr1, len1);
        if (ret[2]) {
            throw takeFromExternrefTable0(ret[1]);
        }
        return RrsLookup.__wrap(ret[0]);
    }
    /**
     * Whether the index holds no entries.
     * @returns {boolean}
     */
    isEmpty() {
        const ret = wasm.rrslookup_isEmpty(this.__wbg_ptr);
        return ret !== 0;
    }
    /**
     * Number of index entries.
     * @returns {number}
     */
    len() {
        const ret = wasm.rrslookup_len(this.__wbg_ptr);
        return ret >>> 0;
    }
    /**
     * Resolves `identifier` to the doc IDs of the title(s) carrying it (most
     * popular first), as a `Uint32Array`. Empty if none.
     * @param {string} identifier
     * @returns {Promise<Uint32Array>}
     */
    lookup(identifier) {
        const ptr0 = passStringToWasm0(identifier, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrslookup_lookup(this.__wbg_ptr, ptr0, len0);
        return ret;
    }
    /**
     * Boots the index at `url` (reads the 16-byte header). Returns a
     * `Promise<RrsLookup>`.
     * @param {string} url
     * @returns {Promise<RrsLookup>}
     */
    static open(url) {
        const ptr0 = passStringToWasm0(url, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrslookup_open(ptr0, len0);
        return ret;
    }
}
if (Symbol.dispose) RrsLookup.prototype[Symbol.dispose] = RrsLookup.prototype.free;

/**
 * A range-fetchable `RRSR` record store exposed to JavaScript: maps a ranked
 * doc ID to its raw record bytes over HTTP Range. The offset index (`.idx`) and
 * the record blob (`.bin`) are each backed by their own [`WasmFetch`] URL.
 */
export class RrsRecords {
    static __wrap(ptr) {
        const obj = Object.create(RrsRecords.prototype);
        obj.__wbg_ptr = ptr;
        RrsRecordsFinalization.register(obj, obj.__wbg_ptr, obj);
        return obj;
    }
    __destroy_into_raw() {
        const ptr = this.__wbg_ptr;
        this.__wbg_ptr = 0;
        RrsRecordsFinalization.unregister(this);
        return ptr;
    }
    free() {
        const ptr = this.__destroy_into_raw();
        wasm.__wbg_rrsrecords_free(ptr, 0);
    }
    /**
     * Boots from a **resident** copy of the 16-byte `RRSR` header (an `RRHC`
     * bundle member) plus the dictionary bytes — zero fetches; per-record reads
     * still go to `idx_url`/`bin_url`.
     * @param {Uint8Array} header
     * @param {Uint8Array} dict
     * @param {string} idx_url
     * @param {string} bin_url
     * @returns {RrsRecords}
     */
    static fromBootWithDict(header, dict, idx_url, bin_url) {
        const ptr0 = passArray8ToWasm0(header, wasm.__wbindgen_malloc);
        const len0 = WASM_VECTOR_LEN;
        const ptr1 = passArray8ToWasm0(dict, wasm.__wbindgen_malloc);
        const len1 = WASM_VECTOR_LEN;
        const ptr2 = passStringToWasm0(idx_url, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len2 = WASM_VECTOR_LEN;
        const ptr3 = passStringToWasm0(bin_url, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len3 = WASM_VECTOR_LEN;
        const ret = wasm.rrsrecords_fromBootWithDict(ptr0, len0, ptr1, len1, ptr2, len2, ptr3, len3);
        if (ret[2]) {
            throw takeFromExternrefTable0(ret[1]);
        }
        return RrsRecords.__wrap(ret[0]);
    }
    /**
     * Raw record bytes for doc `id` as a `Uint8Array`, or `undefined` (a JS
     * `null`) when `id` is out of range. One Range read of the offset pair, one
     * of the record slice.
     * @param {number} id
     * @returns {Promise<any>}
     */
    get(id) {
        const ret = wasm.rrsrecords_get(this.__wbg_ptr, id);
        return ret;
    }
    /**
     * Raw record bytes for several doc IDs (a results page is the typical
     * input). Resolves to a JS `Array` aligned with `ids`: each element is a
     * `Uint8Array`, or `null` for an out-of-range doc ID.
     * @param {Uint32Array} ids
     * @returns {Promise<Array<any>>}
     */
    getMany(ids) {
        const ptr0 = passArray32ToWasm0(ids, wasm.__wbindgen_malloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrsrecords_getMany(this.__wbg_ptr, ptr0, len0);
        return ret;
    }
    /**
     * Record bytes for doc `id` decoded as a UTF-8 string, or `undefined` (a JS
     * `null`) when `id` is out of range. Convenience for JSON/text records;
     * invalid UTF-8 is replaced lossily.
     * @param {number} id
     * @returns {Promise<any>}
     */
    getText(id) {
        const ret = wasm.rrsrecords_getText(this.__wbg_ptr, id);
        return ret;
    }
    /**
     * Whether the store holds no records.
     * @returns {boolean}
     */
    isEmpty() {
        const ret = wasm.rrsrecords_isEmpty(this.__wbg_ptr);
        return ret !== 0;
    }
    /**
     * Number of records (doc IDs `0..len`).
     * @returns {number}
     */
    len() {
        const ret = wasm.rrsrecords_len(this.__wbg_ptr);
        return ret >>> 0;
    }
    /**
     * Boots the record store: reads and validates the 16-byte `RRSR` header of
     * the offset index at `idx_url`, with records served from the blob at
     * `bin_url`. Returns a `Promise<RrsRecords>` to JavaScript.
     * @param {string} idx_url
     * @param {string} bin_url
     * @returns {Promise<RrsRecords>}
     */
    static open(idx_url, bin_url) {
        const ptr0 = passStringToWasm0(idx_url, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ptr1 = passStringToWasm0(bin_url, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len1 = WASM_VECTOR_LEN;
        const ret = wasm.rrsrecords_open(ptr0, len0, ptr1, len1);
        return ret;
    }
    /**
     * Boots a record store and attaches the shared zstd dictionary `dict` (the
     * `*.dict` sidecar's bytes, e.g. fetched once at boot, passed as a
     * `Uint8Array`), so version-2 compressed records inflate transparently.
     * Requires the crate to be built with the `zstd` feature for a compressed
     * store; a raw store ignores the dictionary. Returns a `Promise<RrsRecords>`.
     * @param {string} idx_url
     * @param {string} bin_url
     * @param {Uint8Array} dict
     * @returns {Promise<RrsRecords>}
     */
    static openWithDict(idx_url, bin_url, dict) {
        const ptr0 = passStringToWasm0(idx_url, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ptr1 = passStringToWasm0(bin_url, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len1 = WASM_VECTOR_LEN;
        const ptr2 = passArray8ToWasm0(dict, wasm.__wbindgen_malloc);
        const len2 = WASM_VECTOR_LEN;
        const ret = wasm.rrsrecords_openWithDict(ptr0, len0, ptr1, len1, ptr2, len2);
        return ret;
    }
}
if (Symbol.dispose) RrsRecords.prototype[Symbol.dispose] = RrsRecords.prototype.free;

/**
 * A pagination cursor over a secondary-ordered result set whose pages are mapped
 * back to primary doc IDs. Mirrors [`RrsCursor`]; [`RrsSecondaryCursor::page`]
 * returns a `Uint32Array` of **primary** doc IDs.
 */
export class RrsSecondaryCursor {
    static __wrap(ptr) {
        const obj = Object.create(RrsSecondaryCursor.prototype);
        obj.__wbg_ptr = ptr;
        RrsSecondaryCursorFinalization.register(obj, obj.__wbg_ptr, obj);
        return obj;
    }
    __destroy_into_raw() {
        const ptr = this.__wbg_ptr;
        this.__wbg_ptr = 0;
        RrsSecondaryCursorFinalization.unregister(this);
        return ptr;
    }
    free() {
        const ptr = this.__destroy_into_raw();
        wasm.__wbg_rrssecondarycursor_free(ptr, 0);
    }
    /**
     * The search-filtered facet counts as a JS array of `{ field, cats: [{ name, count }] }`
     * (same shape as `facets()`, counts restricted to this query's result); an empty array when
     * no secondary sidecar is open.
     * @returns {any}
     */
    facetCounts() {
        const ret = wasm.rrssecondarycursor_facetCounts(this.__wbg_ptr);
        return ret;
    }
    /**
     * Number of head results — available with no tail fetch.
     * @returns {number}
     */
    headCount() {
        const ret = wasm.rrssecondarycursor_headCount(this.__wbg_ptr);
        return ret >>> 0;
    }
    /**
     * Forces the lazy tail to be fetched; afterwards `loaded`/`page` span the full
     * result set.
     * @returns {Promise<void>}
     */
    loadTail() {
        const ret = wasm.rrssecondarycursor_loadTail(this.__wbg_ptr);
        return ret;
    }
    /**
     * Number of secondary results materialized so far (head, plus tail once fetched).
     * @returns {number}
     */
    loaded() {
        const ret = wasm.rrssecondarycursor_loaded(this.__wbg_ptr);
        return ret >>> 0;
    }
    /**
     * The next `n` primary doc IDs in secondary rank order, advancing an internal position —
     * the sequential counterpart of [`page`](Self::page), matching `RrsCursor.next`.
     * @param {number} n
     * @returns {Promise<Uint32Array>}
     */
    next(n) {
        const ret = wasm.rrssecondarycursor_next(this.__wbg_ptr, n);
        return ret;
    }
    /**
     * The page of primary doc IDs for the secondary-ordered results
     * `[offset, offset+limit)`. Head pages cost no posting fetch; crossing into the
     * tail fetches it once. Always one small coalesced permutation gather per page.
     * @param {number} offset
     * @param {number} limit
     * @returns {Promise<Uint32Array>}
     */
    page(offset, limit) {
        const ret = wasm.rrssecondarycursor_page(this.__wbg_ptr, offset, limit);
        return ret;
    }
    /**
     * Whether an unfetched tail could still add results.
     * @returns {boolean}
     */
    pendingTail() {
        const ret = wasm.rrssecondarycursor_pendingTail(this.__wbg_ptr);
        return ret !== 0;
    }
}
if (Symbol.dispose) RrsSecondaryCursor.prototype[Symbol.dispose] = RrsSecondaryCursor.prototype.free;

/**
 * A secondary full index exposed to JavaScript: a second `RRS` reindexed in an
 * alternate rank order (e.g. newest-first), the permutation back to primary doc
 * IDs, and an optional secondary-space facet sidecar for filtered search. Search it
 * like [`RrsIndex`]; the cursor's pages come back as **primary** doc IDs, so
 * records are fetched through the existing primary-keyed store unchanged. Facet
 * counts are identical to the primary order's. See `SORTCOLS.md`.
 */
export class RrsSecondaryIndex {
    static __wrap(ptr) {
        const obj = Object.create(RrsSecondaryIndex.prototype);
        obj.__wbg_ptr = ptr;
        RrsSecondaryIndexFinalization.register(obj, obj.__wbg_ptr, obj);
        return obj;
    }
    __destroy_into_raw() {
        const ptr = this.__wbg_ptr;
        this.__wbg_ptr = 0;
        RrsSecondaryIndexFinalization.unregister(this);
        return ptr;
    }
    free() {
        const ptr = this.__destroy_into_raw();
        wasm.__wbg_rrssecondaryindex_free(ptr, 0);
    }
    /**
     * The facet fields with full-corpus counts as a JS array of `{ field, cats: [{ name, count }]
     * }` (same shape as [`RrsIndex::facets`]); an empty array when no sidecar is open.
     * @returns {any}
     */
    facets() {
        const ret = wasm.rrssecondaryindex_facets(this.__wbg_ptr);
        return ret;
    }
    /**
     * Boots the secondary index over the text index at `rrs_url` and the
     * permutation store at `perm_url`. Returns a `Promise<RrsSecondaryIndex>`.
     * @param {string} rrs_url
     * @param {string} perm_url
     * @returns {Promise<RrsSecondaryIndex>}
     */
    static open(rrs_url, perm_url) {
        const ptr0 = passStringToWasm0(rrs_url, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ptr1 = passStringToWasm0(perm_url, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len1 = WASM_VECTOR_LEN;
        const ret = wasm.rrssecondaryindex_open(ptr0, len0, ptr1, len1);
        return ret;
    }
    /**
     * Opens the secondary-space facet sidecar at `url` and attaches it, enabling
     * `facetsJson` and filtered secondary search.
     * @param {string} url
     * @returns {Promise<void>}
     */
    openFacets(url) {
        const ptr0 = passStringToWasm0(url, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrssecondaryindex_openFacets(this.__wbg_ptr, ptr0, len0);
        return ret;
    }
    /**
     * Opens an unfiltered pagination cursor for `query` over the secondary order.
     * `max_missing` is the fuzzy tolerance (0 = strict).
     * @param {string} query
     * @param {number} max_missing
     * @returns {Promise<RrsSecondaryCursor>}
     */
    searchCursor(query, max_missing) {
        const ptr0 = passStringToWasm0(query, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrssecondaryindex_searchCursor(this.__wbg_ptr, ptr0, len0, max_missing);
        return ret;
    }
    /**
     * Like [`RrsSecondaryIndex::search_cursor`] but ANDs the selected facets into
     * the result. `filters` accepts the same entry shapes as the primary index (a
     * `[field, category]` array or a `{field, category, exclude?}` object); within a field
     * categories OR, across fields they AND. A malformed entry throws. The secondary path has
     * no exclude support, so an `exclude: true` selection throws rather than being silently
     * ignored. Applied only when a secondary sidecar is open and `filters` is non-empty.
     * @param {string} query
     * @param {number} max_missing
     * @param {Array<any>} filters
     * @returns {Promise<RrsSecondaryCursor>}
     */
    searchCursorFiltered(query, max_missing, filters) {
        const ptr0 = passStringToWasm0(query, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrssecondaryindex_searchCursorFiltered(this.__wbg_ptr, ptr0, len0, max_missing, filters);
        return ret;
    }
}
if (Symbol.dispose) RrsSecondaryIndex.prototype[Symbol.dispose] = RrsSecondaryIndex.prototype.free;

/**
 * A range-fetchable [`SortCols`] store exposed to JavaScript: dense columns
 * indexed by doc ID, used to re-rank a materialized candidate set client-side
 * (sort by rating / date / any secondary metric) and to map a secondary index's
 * doc IDs back to the primary space. See `SORTCOLS.md`.
 */
export class RrsSortCols {
    static __wrap(ptr) {
        const obj = Object.create(RrsSortCols.prototype);
        obj.__wbg_ptr = ptr;
        RrsSortColsFinalization.register(obj, obj.__wbg_ptr, obj);
        return obj;
    }
    __destroy_into_raw() {
        const ptr = this.__wbg_ptr;
        this.__wbg_ptr = 0;
        RrsSortColsFinalization.unregister(this);
        return ptr;
    }
    free() {
        const ptr = this.__destroy_into_raw();
        wasm.__wbg_rrssortcols_free(ptr, 0);
    }
    /**
     * The index of the column named `name`, or `-1` if absent.
     * @param {string} name
     * @returns {number}
     */
    columnIndex(name) {
        const ptr0 = passStringToWasm0(name, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrssortcols_columnIndex(this.__wbg_ptr, ptr0, len0);
        return ret;
    }
    /**
     * A JS array of the columns' `{ name, type }` (`type` is one of
     * `"u16"`/`"u32"`/`"i32"`/`"f32"`), in stored order.
     * @returns {any}
     */
    columns() {
        const ret = wasm.rrssortcols_columns(this.__wbg_ptr);
        return ret;
    }
    /**
     * Boots the store at `url`: reads the header + column meta (a few KB; the dense
     * data is range-fetched per query). Returns a `Promise<RrsSortCols>`.
     * @param {string} url
     * @returns {Promise<RrsSortCols>}
     */
    static open(url) {
        const ptr0 = passStringToWasm0(url, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrssortcols_open(ptr0, len0);
        return ret;
    }
    /**
     * Number of rows (doc IDs `0..rows`) every column holds.
     * @returns {number}
     */
    rows() {
        const ret = wasm.rrssortcols_rows(this.__wbg_ptr);
        return ret >>> 0;
    }
    /**
     * The contiguous run `[start, start+len)` of a `u32` column as a `Uint32Array`
     * — the permutation-page fast path. Clamps to the row count.
     * @param {number} col
     * @param {number} start
     * @param {number} len
     * @returns {Promise<Uint32Array>}
     */
    sliceU32(col, start, len) {
        const ret = wasm.rrssortcols_sliceU32(this.__wbg_ptr, col, start, len);
        return ret;
    }
    /**
     * The top `k` of `candidates` by column `col` as a `Uint32Array`, descending
     * when `descending` (else ascending); ties keep ascending doc-ID order.
     * @param {number} col
     * @param {Uint32Array} candidates
     * @param {number} k
     * @param {boolean} descending
     * @returns {Promise<Uint32Array>}
     */
    topk(col, candidates, k, descending) {
        const ptr0 = passArray32ToWasm0(candidates, wasm.__wbindgen_malloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrssortcols_topk(this.__wbg_ptr, col, ptr0, len0, k, descending);
        return ret;
    }
    /**
     * Values for `ids` in column `col`, as a `Float64Array` aligned with `ids`
     * (every stored type is exactly representable in `f64`). One coalesced wave of
     * ranged reads.
     * @param {number} col
     * @param {Uint32Array} ids
     * @returns {Promise<Float64Array>}
     */
    values(col, ids) {
        const ptr0 = passArray32ToWasm0(ids, wasm.__wbindgen_malloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrssortcols_values(this.__wbg_ptr, col, ptr0, len0);
        return ret;
    }
}
if (Symbol.dispose) RrsSortCols.prototype[Symbol.dispose] = RrsSortCols.prototype.free;

/**
 * A range-fetchable `RRSS` split set exposed to JavaScript. Boots the manifest in two ranged
 * reads; each query opens (and prunes) the splits it needs, resolved as `base_url/<name>`.
 */
export class RrssIndex {
    static __wrap(ptr) {
        const obj = Object.create(RrssIndex.prototype);
        obj.__wbg_ptr = ptr;
        RrssIndexFinalization.register(obj, obj.__wbg_ptr, obj);
        return obj;
    }
    __destroy_into_raw() {
        const ptr = this.__wbg_ptr;
        this.__wbg_ptr = 0;
        RrssIndexFinalization.unregister(this);
        return ptr;
    }
    free() {
        const ptr = this.__destroy_into_raw();
        wasm.__wbg_rrssindex_free(ptr, 0);
    }
    /**
     * Number of split boots resident from the boot bundle (`0` when booted without one) — the
     * count of per-split header GETs the bundle collapsed into its single GET.
     * @returns {number}
     */
    bundledBootCount() {
        const ret = wasm.rrssindex_bundledBootCount(this.__wbg_ptr);
        return ret >>> 0;
    }
    /**
     * A header-only count estimate for `query` across the whole split set — `{ count, exact }`,
     * the same shape as `RrsIndex.countEstimate`. `exact` is true only for a single-n-gram query
     * over a base-only, tombstone-free set; otherwise `count` is an upper bound (summed per-split
     * minima, or an over-count once deltas/tombstones supersede base docs). Reads only roaring
     * descriptive headers (KBs per surviving split); Bloom-pruned splits cost no fetch. The count
     * is over the unfiltered query (no facet filter is applied); term-bodied split sets have no
     * header-only count primitive and reject with an error. Resolves to `{ count, exact }`.
     * @param {string} query
     * @returns {Promise<CountEstimate>}
     */
    countEstimate(query) {
        const ptr0 = passStringToWasm0(query, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrssindex_countEstimate(this.__wbg_ptr, ptr0, len0);
        return ret;
    }
    /**
     * The EXACT match count for `query` (ANDed with `filters`) — a full intersection scan across
     * every split, the on-demand counterpart to `countEstimate`. Reads posting bodies (can be
     * hundreds of MB on a broad query), so it backs a deliberate "exact count" action, not an
     * every-keystroke count. `filters` takes the same shape as `searchFiltered` (exclude filters
     * are unsupported and throw). Resolves to the count as a number.
     * @param {string} query
     * @param {Array<any>} filters
     * @returns {Promise<number>}
     */
    countExact(query, filters) {
        const ptr0 = passStringToWasm0(query, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrssindex_countExact(this.__wbg_ptr, ptr0, len0, filters);
        return ret;
    }
    /**
     * Number of delta splits flushed since the base (0 for a base-only set).
     * @returns {number}
     */
    deltaCount() {
        const ret = wasm.rrssindex_deltaCount(this.__wbg_ptr);
        return ret >>> 0;
    }
    /**
     * Total documents the manifest's splits hold (Σ per-split doc counts) — what
     * a tier-pruned manifest (e.g. the lite tier-prefix set) actually searches,
     * as opposed to the record store's full corpus size.
     * @returns {number}
     */
    docCount() {
        const ret = wasm.rrssindex_docCount(this.__wbg_ptr);
        return ret;
    }
    /**
     * Per-(field, category) facet counts over `ids` (global doc IDs — typically a query's ranked
     * result from `search`/`searchFiltered`), as a JS `Array<{ field, cats: [{ name, count }] }>`
     * — the same shape the monolith's facet accessors return. Each contributing split's own
     * `‹split›.rrf` sidecar is opened and counted; counts are summed by category name (split sets
     * carry no global category table). Categories the result never hits are omitted (the demo
     * renders missing keys as `0`). Resolves to that array.
     * @param {Uint32Array} ids
     * @returns {Promise<any>}
     */
    facetCounts(ids) {
        const ptr0 = passArray32ToWasm0(ids, wasm.__wbindgen_malloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrssindex_facetCounts(this.__wbg_ptr, ptr0, len0);
        return ret;
    }
    /**
     * Whether this set was booted from an `RRHC` boot bundle ([`openBundle`](Self::open_bundle)),
     * i.e. its split boots are resident and split opens skip the per-split header GET.
     * @returns {boolean}
     */
    hasBundle() {
        const ret = wasm.rrssindex_hasBundle(this.__wbg_ptr);
        return ret !== 0;
    }
    /**
     * Boots the split-set manifest at `manifest_url`; per-split files (and the sort-column
     * store, if any) are fetched from `base_url/<name>`. Each queried split cold-opens its own
     * header; for the boot-bundle path that collapses those opens, see
     * [`openBundle`](Self::open_bundle). Returns a `Promise<RrssIndex>`.
     * @param {string} manifest_url
     * @param {string} base_url
     * @returns {Promise<RrssIndex>}
     */
    static open(manifest_url, base_url) {
        const ptr0 = passStringToWasm0(manifest_url, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ptr1 = passStringToWasm0(base_url, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len1 = WASM_VECTOR_LEN;
        const ret = wasm.rrssindex_open(ptr0, len0, ptr1, len1);
        return ret;
    }
    /**
     * Boots the split set with an `RRHC` boot bundle: the manifest at `manifest_url` and the
     * bundle at `rrhc_url` are fetched in **one parallel wave** (two GETs, one round trip of
     * latency), then each split the query opens takes its boot from the bundle's inlined blob —
     * no per-split header fetch. A split the bundle didn't inline falls back to a cold open, so
     * the path degrades gracefully. Per-split data files still resolve as `base_url/<name>`.
     * Returns a `Promise<RrssIndex>`.
     * @param {string} manifest_url
     * @param {string} base_url
     * @param {string} rrhc_url
     * @returns {Promise<RrssIndex>}
     */
    static openBundle(manifest_url, base_url, rrhc_url) {
        const ptr0 = passStringToWasm0(manifest_url, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ptr1 = passStringToWasm0(base_url, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len1 = WASM_VECTOR_LEN;
        const ptr2 = passStringToWasm0(rrhc_url, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len2 = WASM_VECTOR_LEN;
        const ret = wasm.rrssindex_openBundle(ptr0, len0, ptr1, len1, ptr2, len2);
        return ret;
    }
    /**
     * Returns up to `limit` matching global doc IDs, ranked by policy (tiered short-circuit or
     * stable-key sort, with delta supersession). Resolves to a `Uint32Array`.
     * @param {string} query
     * @param {number} limit
     * @returns {Promise<Uint32Array>}
     */
    search(query, limit) {
        const ptr0 = passStringToWasm0(query, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrssindex_search(this.__wbg_ptr, ptr0, len0, limit);
        return ret;
    }
    /**
     * Like [`search`](Self::search) but ANDs a facet filter in. Args are `(query, limit,
     * filters)` — `limit` adjacent to `query`, options trailing, matching
     * `RrsIndex.searchCursorFiltered`. `filters` accepts the same entry shapes as the primary
     * index (a `[field, category]` array or a `{field, category, exclude?}` object); within a
     * field categories OR, across fields AND; a malformed entry throws. The split path has no
     * exclude support, so an `exclude: true` selection throws rather than being silently ignored.
     * Each surviving split's own `‹split›.rrf` sidecar resolves the filter, and a split lacking a
     * selected field's categories is pruned without a fetch. An empty `filters` is exactly
     * [`search`](Self::search).
     * @param {string} query
     * @param {number} limit
     * @param {Array<any>} filters
     * @returns {Promise<Uint32Array>}
     */
    searchFiltered(query, limit, filters) {
        const ptr0 = passStringToWasm0(query, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrssindex_searchFiltered(this.__wbg_ptr, ptr0, len0, limit, filters);
        return ret;
    }
    /**
     * Names a **global term-Bloom sidecar** (resolved as `base_url/<name>`, the
     * `build_global_bloom` layout) covering the whole set's vocabulary. It is never
     * downloaded: the tiered query path range-probes `k` byte positions per query
     * term, and only after the top tier yields nothing — so an absent/typo term ends
     * the tier descent in a handful of one-byte reads instead of opening every split,
     * while present-term queries never touch it.
     * @param {string} name
     */
    setGlobalBloom(name) {
        const ptr0 = passStringToWasm0(name, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        wasm.rrssindex_setGlobalBloom(this.__wbg_ptr, ptr0, len0);
    }
    /**
     * Number of splits named by the manifest (base + delta).
     * @returns {number}
     */
    splitCount() {
        const ret = wasm.rrssindex_splitCount(this.__wbg_ptr);
        return ret >>> 0;
    }
    /**
     * Total on-S3 size of every split in bytes (the split set's footprint).
     * @returns {bigint}
     */
    totalBytes() {
        const ret = wasm.rrssindex_totalBytes(this.__wbg_ptr);
        return BigInt.asUintN(64, ret);
    }
}
if (Symbol.dispose) RrssIndex.prototype[Symbol.dispose] = RrssIndex.prototype.free;

/**
 * Aligned local doc IDs + BM25 scores from a term-index BM25 search, best-first.
 * In JavaScript `ids` is a `Uint32Array` and `scores` a `Float32Array` (index `i`
 * of each is the same hit), mirroring the vector reader's `RrviHits` so the two
 * scored-search shapes match. Both getters copy across the wasm boundary on every
 * access — read each once into a JS variable rather than re-touching them in a loop.
 */
export class RrtHits {
    static __wrap(ptr) {
        const obj = Object.create(RrtHits.prototype);
        obj.__wbg_ptr = ptr;
        RrtHitsFinalization.register(obj, obj.__wbg_ptr, obj);
        return obj;
    }
    __destroy_into_raw() {
        const ptr = this.__wbg_ptr;
        this.__wbg_ptr = 0;
        RrtHitsFinalization.unregister(this);
        return ptr;
    }
    free() {
        const ptr = this.__destroy_into_raw();
        wasm.__wbg_rrthits_free(ptr, 0);
    }
    /**
     * The matching local doc IDs (`Uint32Array`), best-first.
     * @returns {Uint32Array}
     */
    get ids() {
        const ret = wasm.rrthits_ids(this.__wbg_ptr);
        var v1 = getArrayU32FromWasm0(ret[0], ret[1]).slice();
        wasm.__wbindgen_free(ret[0], ret[1] * 4, 4);
        return v1;
    }
    /**
     * The BM25 scores (`Float32Array`) aligned with `ids`; higher is better.
     * @returns {Float32Array}
     */
    get scores() {
        const ret = wasm.rrthits_scores(this.__wbg_ptr);
        var v1 = getArrayF32FromWasm0(ret[0], ret[1]).slice();
        wasm.__wbindgen_free(ret[0], ret[1] * 4, 4);
        return v1;
    }
}
if (Symbol.dispose) RrtHits.prototype[Symbol.dispose] = RrtHits.prototype.free;

/**
 * A range-fetchable `RRTI` term-level inverted index exposed to JavaScript. Boot
 * holds only the small resident block router in memory (O(#blocks), not O(vocab));
 * each query range-fetches the dict blocks and postings it needs. Built with
 * `wasm-pack build --target web --features "wasm terms"`.
 */
export class RrtIndex {
    static __wrap(ptr) {
        const obj = Object.create(RrtIndex.prototype);
        obj.__wbg_ptr = ptr;
        RrtIndexFinalization.register(obj, obj.__wbg_ptr, obj);
        return obj;
    }
    __destroy_into_raw() {
        const ptr = this.__wbg_ptr;
        this.__wbg_ptr = 0;
        RrtIndexFinalization.unregister(this);
        return ptr;
    }
    free() {
        const ptr = this.__destroy_into_raw();
        wasm.__wbg_rrtindex_free(ptr, 0);
    }
    /**
     * Autocompletes `prefix`: up to `max_terms` dictionary terms that start with
     * it, in lexicographic order, as a JS `string[]`. Range-fetches only the dict
     * blocks spanning the prefix. Resolves to a `Promise<string[]>`. (Typo/substring
     * search is the trigram `RRS` index's job — it composes over the same doc IDs.)
     * @param {string} prefix
     * @param {number} max_terms
     * @returns {Promise<string[]>}
     */
    complete(prefix, max_terms) {
        const ptr0 = passStringToWasm0(prefix, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrtindex_complete(this.__wbg_ptr, ptr0, len0, max_terms);
        return ret;
    }
    /**
     * Whether the dictionary holds no terms.
     * @returns {boolean}
     */
    isEmpty() {
        const ret = wasm.rrtindex_isEmpty(this.__wbg_ptr);
        return ret !== 0;
    }
    /**
     * Number of distinct terms in the dictionary.
     * @returns {number}
     */
    len() {
        const ret = wasm.rrtindex_len(this.__wbg_ptr);
        return ret >>> 0;
    }
    /**
     * Boots the index at `url`: one boot read of the small block router, held
     * resident so a term resolves to its dict block with a single ranged read.
     * Returns a `Promise<RrtIndex>`.
     * @param {string} url
     * @returns {Promise<RrtIndex>}
     */
    static open(url) {
        const ptr0 = passStringToWasm0(url, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrtindex_open(ptr0, len0);
        return ret;
    }
    /**
     * Reranks caller-supplied candidate doc IDs (ANY mode's results — the shared
     * doc-ID space makes one sidecar serve trigram/term/hybrid) by BM25 against
     * this index's resolution of `query`. Query terms missing from the dictionary
     * contribute no score; if NO term resolves the candidates come back unchanged
     * (truncated to `k`) — a typo'd query degrades to static rank, never errors.
     * Resolves to a `Uint32Array`.
     * @param {RrbIndex} impacts
     * @param {string} query
     * @param {Uint32Array} ids
     * @param {number} k
     * @returns {Promise<Uint32Array>}
     */
    rerankIds(impacts, query, ids, k) {
        _assertClass(impacts, RrbIndex);
        const ptr0 = passStringToWasm0(query, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ptr1 = passArray32ToWasm0(ids, wasm.__wbindgen_malloc);
        const len1 = WASM_VECTOR_LEN;
        const ret = wasm.rrtindex_rerankIds(this.__wbg_ptr, impacts.__wbg_ptr, ptr0, len0, ptr1, len1, k);
        return ret;
    }
    /**
     * Returns up to `limit` doc IDs matching every query term (whole-word AND),
     * most popular first (ascending doc ID == descending rank). Resolves to a
     * `Uint32Array`. A query term absent from the dictionary yields no results.
     * @param {string} query
     * @param {number} limit
     * @returns {Promise<Uint32Array>}
     */
    search(query, limit) {
        const ptr0 = passStringToWasm0(query, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrtindex_search(this.__wbg_ptr, ptr0, len0, limit);
        return ret;
    }
    /**
     * BM25 search via the `.rrb` impact sidecar: intersect the query terms'
     * postings, take the first `m` candidates in static-rank order (the
     * candidate window bounding the rerank cost), and return the top `k` doc IDs
     * by BM25 score (ties keep static rank), as aligned ids + scores (`RrtHits`).
     * @param {RrbIndex} impacts
     * @param {string} query
     * @param {number} m
     * @param {number} k
     * @returns {Promise<RrtHits>}
     */
    searchBm25(impacts, query, m, k) {
        _assertClass(impacts, RrbIndex);
        const ptr0 = passStringToWasm0(query, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrtindex_searchBm25(this.__wbg_ptr, impacts.__wbg_ptr, ptr0, len0, m, k);
        return ret;
    }
    /**
     * Min-should-match BM25 search: keep docs present in **≥ `min_match`** of the
     * query's resolved terms (clamped to `[1, M]`; `min_match == M` is strict AND
     * like [`Self::search_bm25`], `min_match == 1` is the union), take the first
     * `m` qualifiers in static-rank order, and return the top `k` hits by BM25
     * score as aligned ids + scores. Same contract as `searchBm25`; → `RrtHits`.
     * @param {RrbIndex} impacts
     * @param {string} query
     * @param {number} m
     * @param {number} k
     * @param {number} min_match
     * @returns {Promise<RrtHits>}
     */
    searchBm25MinMatch(impacts, query, m, k, min_match) {
        _assertClass(impacts, RrbIndex);
        const ptr0 = passStringToWasm0(query, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrtindex_searchBm25MinMatch(this.__wbg_ptr, impacts.__wbg_ptr, ptr0, len0, m, k, min_match);
        return ret;
    }
    /**
     * Returns up to `limit` doc IDs matching any term that starts with `prefix`
     * (the union of every prefix-matching term's posting), most popular first.
     * Resolves to a `Uint32Array`.
     * @param {string} prefix
     * @param {number} limit
     * @returns {Promise<Uint32Array>}
     */
    searchPrefix(prefix, limit) {
        const ptr0 = passStringToWasm0(prefix, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrtindex_searchPrefix(this.__wbg_ptr, ptr0, len0, limit);
        return ret;
    }
    /**
     * Like [`searchPrefix`](Self::search_prefix) but returns a [`PrefixSearch`]
     * carrying a `truncated` flag: `true` when the prefix matched more dictionary
     * terms than the union cap, so `ids` is a bounded approximation of the full
     * prefix union rather than the exact set.
     * @param {string} prefix
     * @param {number} limit
     * @returns {Promise<PrefixSearch>}
     */
    searchPrefixCapped(prefix, limit) {
        const ptr0 = passStringToWasm0(prefix, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrtindex_searchPrefixCapped(this.__wbg_ptr, ptr0, len0, limit);
        return ret;
    }
}
if (Symbol.dispose) RrtIndex.prototype[Symbol.dispose] = RrtIndex.prototype.free;

/**
 * The result of [`RrviIndex::search`]: aligned doc IDs and similarity scores,
 * best-first. In JavaScript `ids` is a `Uint32Array` and `scores` a
 * `Float32Array`. Both getters copy across the wasm boundary on every access — read
 * each once into a JS variable rather than re-touching them in a loop.
 */
export class RrviHits {
    static __wrap(ptr) {
        const obj = Object.create(RrviHits.prototype);
        obj.__wbg_ptr = ptr;
        RrviHitsFinalization.register(obj, obj.__wbg_ptr, obj);
        return obj;
    }
    __destroy_into_raw() {
        const ptr = this.__wbg_ptr;
        this.__wbg_ptr = 0;
        RrviHitsFinalization.unregister(this);
        return ptr;
    }
    free() {
        const ptr = this.__destroy_into_raw();
        wasm.__wbg_rrvihits_free(ptr, 0);
    }
    /**
     * The matching doc IDs (`Uint32Array`), best-first.
     * @returns {Uint32Array}
     */
    get ids() {
        const ret = wasm.rrvihits_ids(this.__wbg_ptr);
        var v1 = getArrayU32FromWasm0(ret[0], ret[1]).slice();
        wasm.__wbindgen_free(ret[0], ret[1] * 4, 4);
        return v1;
    }
    /**
     * The similarity scores (`Float32Array`) aligned with `ids`; higher is better.
     * @returns {Float32Array}
     */
    get scores() {
        const ret = wasm.rrvihits_scores(this.__wbg_ptr);
        var v1 = getArrayF32FromWasm0(ret[0], ret[1]).slice();
        wasm.__wbindgen_free(ret[0], ret[1] * 4, 4);
        return v1;
    }
}
if (Symbol.dispose) RrviHits.prototype[Symbol.dispose] = RrviHits.prototype.free;

/**
 * A range-fetchable RRVI similarity (vector) index exposed to JavaScript. Built
 * with `wasm-pack build --target web --features "wasm vector"`.
 */
export class RrviIndex {
    static __wrap(ptr) {
        const obj = Object.create(RrviIndex.prototype);
        obj.__wbg_ptr = ptr;
        RrviIndexFinalization.register(obj, obj.__wbg_ptr, obj);
        return obj;
    }
    __destroy_into_raw() {
        const ptr = this.__wbg_ptr;
        this.__wbg_ptr = 0;
        RrviIndexFinalization.unregister(this);
        return ptr;
    }
    free() {
        const ptr = this.__destroy_into_raw();
        wasm.__wbg_rrviindex_free(ptr, 0);
    }
    /**
     * Vector dimensionality the index was built with.
     * @returns {number}
     */
    dim() {
        const ret = wasm.rrviindex_dim(this.__wbg_ptr);
        return ret >>> 0;
    }
    /**
     * Whether the index holds no vectors.
     * @returns {boolean}
     */
    isEmpty() {
        const ret = wasm.rrviindex_isEmpty(this.__wbg_ptr);
        return ret !== 0;
    }
    /**
     * Total number of indexed vectors.
     * @returns {number}
     */
    len() {
        const ret = wasm.rrviindex_len(this.__wbg_ptr);
        return ret >>> 0;
    }
    /**
     * Number of coarse (IVF) clusters.
     * @returns {number}
     */
    nlist() {
        const ret = wasm.rrviindex_nlist(this.__wbg_ptr);
        return ret >>> 0;
    }
    /**
     * Boots the RRVI index at `url`: one boot read of the coarse centroids, PQ
     * codebooks, optional OPQ rotation, and cluster directory. Returns a
     * `Promise<RrviIndex>`.
     * @param {string} url
     * @returns {Promise<RrviIndex>}
     */
    static open(url) {
        const ptr0 = passStringToWasm0(url, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrviindex_open(ptr0, len0);
        return ret;
    }
    /**
     * Opens the optional `RRVR` re-rank sidecar at `url` and attaches it, enabling
     * [`RrviIndex::search_rerank`].
     * @param {string} url
     * @returns {Promise<void>}
     */
    openRerank(url) {
        const ptr0 = passStringToWasm0(url, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrviindex_openRerank(this.__wbg_ptr, ptr0, len0);
        return ret;
    }
    /**
     * Searches for the `k` nearest vectors to `query` (a `Float32Array` of length
     * `dim`), probing the `nprobe` nearest clusters in one concurrent wave of
     * ranged reads. Resolves to an `RrviHits` with aligned `ids`/`scores`,
     * best-first. An inner-product index normalizes the query for you; `doc_id`
     * matches the text index's doc ID, so hits map straight to the record store.
     * @param {Float32Array} query
     * @param {number} k
     * @param {number} nprobe
     * @returns {Promise<RrviHits>}
     */
    search(query, k, nprobe) {
        const ptr0 = passArrayF32ToWasm0(query, wasm.__wbindgen_malloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrviindex_search(this.__wbg_ptr, ptr0, len0, k, nprobe);
        return ret;
    }
    /**
     * Like [`RrviIndex::search`] but re-ranks the ADC top-`r` candidates against
     * the higher-precision re-rank sidecar (open it first with `openRerank`),
     * returning the exact top-`k`. Rejects if no sidecar is open.
     * @param {Float32Array} query
     * @param {number} k
     * @param {number} nprobe
     * @param {number} r
     * @returns {Promise<RrviHits>}
     */
    searchRerank(query, k, nprobe, r) {
        const ptr0 = passArrayF32ToWasm0(query, wasm.__wbindgen_malloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.rrviindex_searchRerank(this.__wbg_ptr, ptr0, len0, k, nprobe, r);
        return ret;
    }
}
if (Symbol.dispose) RrviIndex.prototype[Symbol.dispose] = RrviIndex.prototype.free;

/**
 * A standalone portable RoaringBitmap exposed to JavaScript for client-side set
 * operations over external `.bm` bitmaps — e.g. the per-library bitmaps a static
 * catalog ships for library diff / intersection / collection paging. The bytes
 * are the portable serialization written by Go's `RoaringBitmap/roaring/v2`
 * `WriteTo` (the same format the index postings use), so they deserialize here
 * byte-for-byte with no glue.
 */
export class WasmBitmap {
    static __wrap(ptr) {
        const obj = Object.create(WasmBitmap.prototype);
        obj.__wbg_ptr = ptr;
        WasmBitmapFinalization.register(obj, obj.__wbg_ptr, obj);
        return obj;
    }
    __destroy_into_raw() {
        const ptr = this.__wbg_ptr;
        this.__wbg_ptr = 0;
        WasmBitmapFinalization.unregister(this);
        return ptr;
    }
    free() {
        const ptr = this.__destroy_into_raw();
        wasm.__wbg_wasmbitmap_free(ptr, 0);
    }
    /**
     * Intersection (`self ∩ other`) as a new bitmap.
     * @param {WasmBitmap} other
     * @returns {WasmBitmap}
     */
    and(other) {
        _assertClass(other, WasmBitmap);
        const ret = wasm.wasmbitmap_and(this.__wbg_ptr, other.__wbg_ptr);
        return WasmBitmap.__wrap(ret);
    }
    /**
     * Difference (`self \ other`) as a new bitmap.
     * @param {WasmBitmap} other
     * @returns {WasmBitmap}
     */
    andnot(other) {
        _assertClass(other, WasmBitmap);
        const ret = wasm.wasmbitmap_andnot(this.__wbg_ptr, other.__wbg_ptr);
        return WasmBitmap.__wrap(ret);
    }
    /**
     * Deserializes a portable RoaringBitmap from `bytes`.
     * @param {Uint8Array} bytes
     * @returns {WasmBitmap}
     */
    static fromBytes(bytes) {
        const ptr0 = passArray8ToWasm0(bytes, wasm.__wbindgen_malloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.wasmbitmap_fromBytes(ptr0, len0);
        if (ret[2]) {
            throw takeFromExternrefTable0(ret[1]);
        }
        return WasmBitmap.__wrap(ret[0]);
    }
    /**
     * Whether the bitmap holds no doc IDs.
     * @returns {boolean}
     */
    isEmpty() {
        const ret = wasm.wasmbitmap_isEmpty(this.__wbg_ptr);
        return ret !== 0;
    }
    /**
     * Number of doc IDs set (cardinality).
     * @returns {number}
     */
    len() {
        const ret = wasm.wasmbitmap_len(this.__wbg_ptr);
        return ret >>> 0;
    }
    /**
     * Union (`self ∪ other`) as a new bitmap.
     * @param {WasmBitmap} other
     * @returns {WasmBitmap}
     */
    or(other) {
        _assertClass(other, WasmBitmap);
        const ret = wasm.wasmbitmap_or(this.__wbg_ptr, other.__wbg_ptr);
        return WasmBitmap.__wrap(ret);
    }
    /**
     * Doc IDs in ascending order (== rank order, since doc IDs are popularity-
     * ranked), skipping `offset` and taking up to `limit`. Resolves to a
     * `Uint32Array`. Uses rank/`select` (O(limit·log n)) rather than re-walking the
     * `offset` prefix each call, so deep-paging a multi-million-doc bitmap does not
     * re-iterate everything before the page.
     * @param {number} offset
     * @param {number} limit
     * @returns {Uint32Array}
     */
    page(offset, limit) {
        const ret = wasm.wasmbitmap_page(this.__wbg_ptr, offset, limit);
        var v1 = getArrayU32FromWasm0(ret[0], ret[1]).slice();
        wasm.__wbindgen_free(ret[0], ret[1] * 4, 4);
        return v1;
    }
}
if (Symbol.dispose) WasmBitmap.prototype[Symbol.dispose] = WasmBitmap.prototype.free;

/**
 * `[payloadBytes, entryCount, hits, misses]` for the shared range cache, or `[0, 0, 0, 0]` when no
 * cache is enabled — a JS-side readout of cache effectiveness.
 * @returns {Float64Array}
 */
export function rangeCacheStats() {
    const ret = wasm.rangeCacheStats();
    var v1 = getArrayF64FromWasm0(ret[0], ret[1]).slice();
    wasm.__wbindgen_free(ret[0], ret[1] * 8, 8);
    return v1;
}

/**
 * Reciprocal-rank fusion of **N** ranked doc-ID lists (each a `Uint32Array`) into one
 * best-first ranking — the no-score-normalization hybrid, ties broken by ascending
 * id. This is the same [`crate::vector::reciprocal_rank_fusion`] every native
 * consumer uses, so the browser can fuse any number of arms (e.g. trigram + BM25 +
 * min-should-match + semantic) in one call without drifting from the library's
 * ranking. `kParam` is conventionally ~60.
 *
 * `weights` is optional: omit it (or pass `undefined`) for the equal-vote fusion;
 * pass a `Float64Array` parallel to `lists` to up- or down-weight individual arms
 * (e.g. boost a lone semantic list against several lexical ones). Weights may be
 * fractional or `> 1`; a length mismatch is a clean error. Returns a `Uint32Array`.
 * @param {Uint32Array[]} lists
 * @param {number} k_param
 * @param {Float64Array | null} [weights]
 * @returns {Uint32Array}
 */
export function reciprocalRankFusion(lists, k_param, weights) {
    const ptr0 = passArrayJsValueToWasm0(lists, wasm.__wbindgen_malloc);
    const len0 = WASM_VECTOR_LEN;
    const ret = wasm.reciprocalRankFusion(ptr0, len0, k_param, isLikeNone(weights) ? 0 : addToExternrefTable0(weights));
    if (ret[3]) {
        throw takeFromExternrefTable0(ret[2]);
    }
    var v2 = getArrayU32FromWasm0(ret[0], ret[1]).slice();
    wasm.__wbindgen_free(ret[0], ret[1] * 4, 4);
    return v2;
}

/**
 * Sets the shared range-cache budget in **mebibytes**, enabling caching for every range read
 * across every index type (trigram, term, facet, vector, records, split-set, ...). `0` or negative
 * disables and clears the cache. Resizing keeps warm entries, evicting LRU-first if the new budget
 * is smaller. Already-open indexes are affected too: each read resolves the cache live.
 * @param {number} mb
 */
export function setRangeCacheMb(mb) {
    wasm.setRangeCacheMb(mb);
}

/**
 * Runs once when the wasm module is instantiated: installs the default range cache so every index
 * type benefits without any JS setup. `setRangeCacheMb(0)` disables it; `setRangeCacheMb(n)` resizes.
 */
export function wasm_start() {
    wasm.wasm_start();
}
function __wbg_get_imports() {
    const import0 = {
        __proto__: null,
        __wbg_Error_ef53bc310eb298a0: function(arg0, arg1) {
            const ret = Error(getStringFromWasm0(arg0, arg1));
            return ret;
        },
        __wbg___wbindgen_boolean_get_1a45e2c38d4d41b9: function(arg0) {
            const v = arg0;
            const ret = typeof(v) === 'boolean' ? v : undefined;
            return isLikeNone(ret) ? 0xFFFFFF : ret ? 1 : 0;
        },
        __wbg___wbindgen_is_function_754e9f305ff6029e: function(arg0) {
            const ret = typeof(arg0) === 'function';
            return ret;
        },
        __wbg___wbindgen_is_object_56732c2bc353f41d: function(arg0) {
            const val = arg0;
            const ret = typeof(val) === 'object' && val !== null;
            return ret;
        },
        __wbg___wbindgen_is_undefined_67b456be8673d3d7: function(arg0) {
            const ret = arg0 === undefined;
            return ret;
        },
        __wbg___wbindgen_string_get_72bdf95d3ae505b1: function(arg0, arg1) {
            const obj = arg1;
            const ret = typeof(obj) === 'string' ? obj : undefined;
            var ptr1 = isLikeNone(ret) ? 0 : passStringToWasm0(ret, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
            var len1 = WASM_VECTOR_LEN;
            getDataViewMemory0().setInt32(arg0 + 4 * 1, len1, true);
            getDataViewMemory0().setInt32(arg0 + 4 * 0, ptr1, true);
        },
        __wbg___wbindgen_throw_1506f2235d1bdba0: function(arg0, arg1) {
            throw new Error(getStringFromWasm0(arg0, arg1));
        },
        __wbg__wbg_cb_unref_61db23ac97f16c31: function(arg0) {
            arg0._wbg_cb_unref();
        },
        __wbg_arrayBuffer_05927079aabe6d46: function() { return handleError(function (arg0) {
            const ret = arg0.arrayBuffer();
            return ret;
        }, arguments); },
        __wbg_call_9c758de292015997: function() { return handleError(function (arg0, arg1, arg2) {
            const ret = arg0.call(arg1, arg2);
            return ret;
        }, arguments); },
        __wbg_countestimate_new: function(arg0) {
            const ret = CountEstimate.__wrap(arg0);
            return ret;
        },
        __wbg_filteredids_new: function(arg0) {
            const ret = FilteredIds.__wrap(arg0);
            return ret;
        },
        __wbg_get_2b48c7d0d006a781: function(arg0, arg1) {
            const ret = arg0[arg1 >>> 0];
            return ret;
        },
        __wbg_get_de6a0f7d4d18a304: function() { return handleError(function (arg0, arg1) {
            const ret = Reflect.get(arg0, arg1);
            return ret;
        }, arguments); },
        __wbg_get_unchecked_33f6e5c9e2f2d6b2: function(arg0, arg1) {
            const ret = arg0[arg1 >>> 0];
            return ret;
        },
        __wbg_instanceof_ArrayBuffer_8f49811467741499: function(arg0) {
            let result;
            try {
                result = arg0 instanceof ArrayBuffer;
            } catch (_) {
                result = false;
            }
            const ret = result;
            return ret;
        },
        __wbg_instanceof_Promise_d0db99486956c8e8: function(arg0) {
            let result;
            try {
                result = arg0 instanceof Promise;
            } catch (_) {
                result = false;
            }
            const ret = result;
            return ret;
        },
        __wbg_instanceof_Response_cb984bd66d7bd408: function(arg0) {
            let result;
            try {
                result = arg0 instanceof Response;
            } catch (_) {
                result = false;
            }
            const ret = result;
            return ret;
        },
        __wbg_isArray_67c2c9c4313f4448: function(arg0) {
            const ret = Array.isArray(arg0);
            return ret;
        },
        __wbg_length_280688879ee7deb5: function(arg0) {
            const ret = arg0.length;
            return ret;
        },
        __wbg_length_33096ac1966bb961: function(arg0) {
            const ret = arg0.length;
            return ret;
        },
        __wbg_length_4a591ecaa01354d9: function(arg0) {
            const ret = arg0.length;
            return ret;
        },
        __wbg_length_66f1a4b2e9026940: function(arg0) {
            const ret = arg0.length;
            return ret;
        },
        __wbg_model2vecembedder_new: function(arg0) {
            const ret = Model2vecEmbedder.__wrap(arg0);
            return ret;
        },
        __wbg_new_578aeef4b6b94378: function(arg0) {
            const ret = new Uint8Array(arg0);
            return ret;
        },
        __wbg_new_ce1ab61c1c2b300d: function() {
            const ret = new Object();
            return ret;
        },
        __wbg_new_d90091b82fdf5b91: function() {
            const ret = new Array();
            return ret;
        },
        __wbg_new_e436d06bc8e77460: function() { return handleError(function () {
            const ret = new Headers();
            return ret;
        }, arguments); },
        __wbg_new_from_slice_18fa1f71286d66b8: function(arg0, arg1) {
            const ret = new Uint8Array(getArrayU8FromWasm0(arg0, arg1));
            return ret;
        },
        __wbg_new_from_slice_47be4219028de35d: function(arg0, arg1) {
            const ret = new Uint32Array(getArrayU32FromWasm0(arg0, arg1));
            return ret;
        },
        __wbg_new_typed_bf31d18f92484486: function(arg0, arg1) {
            try {
                var state0 = {a: arg0, b: arg1};
                var cb0 = (arg0, arg1) => {
                    const a = state0.a;
                    state0.a = 0;
                    try {
                        return wasm_bindgen__convert__closures_____invoke__h134a34389e58ea02(a, state0.b, arg0, arg1);
                    } finally {
                        state0.a = a;
                    }
                };
                const ret = new Promise(cb0);
                return ret;
            } finally {
                state0.a = 0;
            }
        },
        __wbg_new_with_length_690552eb9e6aeac9: function(arg0) {
            const ret = new Array(arg0 >>> 0);
            return ret;
        },
        __wbg_new_with_str_and_init_bcd02b79a793d27f: function() { return handleError(function (arg0, arg1, arg2) {
            const ret = new Request(getStringFromWasm0(arg0, arg1), arg2);
            return ret;
        }, arguments); },
        __wbg_ok_fb13c30bc1893039: function(arg0) {
            const ret = arg0.ok;
            return ret;
        },
        __wbg_prefixsearch_new: function(arg0) {
            const ret = PrefixSearch.__wrap(arg0);
            return ret;
        },
        __wbg_prototypesetcall_3249fc62a0fafa30: function(arg0, arg1, arg2) {
            Uint8Array.prototype.set.call(getArrayU8FromWasm0(arg0, arg1), arg2);
        },
        __wbg_prototypesetcall_59640349d2c6d881: function(arg0, arg1, arg2) {
            Uint32Array.prototype.set.call(getArrayU32FromWasm0(arg0, arg1), arg2);
        },
        __wbg_prototypesetcall_d1ae8885a2e9d458: function(arg0, arg1, arg2) {
            Float64Array.prototype.set.call(getArrayF64FromWasm0(arg0, arg1), arg2);
        },
        __wbg_queueMicrotask_35c611f4a14830b2: function(arg0) {
            queueMicrotask(arg0);
        },
        __wbg_queueMicrotask_404ed0a58e0b63cc: function(arg0) {
            const ret = arg0.queueMicrotask;
            return ret;
        },
        __wbg_resolve_25a7e548d5881dca: function(arg0) {
            const ret = Promise.resolve(arg0);
            return ret;
        },
        __wbg_rrbindex_new: function(arg0) {
            const ret = RrbIndex.__wrap(arg0);
            return ret;
        },
        __wbg_rrffacets_new: function(arg0) {
            const ret = RrfFacets.__wrap(arg0);
            return ret;
        },
        __wbg_rrhcbundle_new: function(arg0) {
            const ret = RrhcBundle.__wrap(arg0);
            return ret;
        },
        __wbg_rrscatalog_new: function(arg0) {
            const ret = RrsCatalog.__wrap(arg0);
            return ret;
        },
        __wbg_rrscursor_new: function(arg0) {
            const ret = RrsCursor.__wrap(arg0);
            return ret;
        },
        __wbg_rrsindex_new: function(arg0) {
            const ret = RrsIndex.__wrap(arg0);
            return ret;
        },
        __wbg_rrslookup_new: function(arg0) {
            const ret = RrsLookup.__wrap(arg0);
            return ret;
        },
        __wbg_rrsrecords_new: function(arg0) {
            const ret = RrsRecords.__wrap(arg0);
            return ret;
        },
        __wbg_rrssecondarycursor_new: function(arg0) {
            const ret = RrsSecondaryCursor.__wrap(arg0);
            return ret;
        },
        __wbg_rrssecondaryindex_new: function(arg0) {
            const ret = RrsSecondaryIndex.__wrap(arg0);
            return ret;
        },
        __wbg_rrssindex_new: function(arg0) {
            const ret = RrssIndex.__wrap(arg0);
            return ret;
        },
        __wbg_rrssortcols_new: function(arg0) {
            const ret = RrsSortCols.__wrap(arg0);
            return ret;
        },
        __wbg_rrthits_new: function(arg0) {
            const ret = RrtHits.__wrap(arg0);
            return ret;
        },
        __wbg_rrtindex_new: function(arg0) {
            const ret = RrtIndex.__wrap(arg0);
            return ret;
        },
        __wbg_rrvihits_new: function(arg0) {
            const ret = RrviHits.__wrap(arg0);
            return ret;
        },
        __wbg_rrviindex_new: function(arg0) {
            const ret = RrviIndex.__wrap(arg0);
            return ret;
        },
        __wbg_set_25ef40a9aeff260d: function() { return handleError(function (arg0, arg1, arg2, arg3, arg4) {
            arg0.set(getStringFromWasm0(arg1, arg2), getStringFromWasm0(arg3, arg4));
        }, arguments); },
        __wbg_set_6e30c9374c26414c: function() { return handleError(function (arg0, arg1, arg2) {
            const ret = Reflect.set(arg0, arg1, arg2);
            return ret;
        }, arguments); },
        __wbg_set_dca99999bba88a9a: function(arg0, arg1, arg2) {
            arg0[arg1 >>> 0] = arg2;
        },
        __wbg_set_headers_7c1e39ece7826bec: function(arg0, arg1) {
            arg0.headers = arg1;
        },
        __wbg_set_method_7a6811dec7a4feff: function(arg0, arg1, arg2) {
            arg0.method = getStringFromWasm0(arg1, arg2);
        },
        __wbg_set_mode_c90e3667002857d4: function(arg0, arg1) {
            arg0.mode = __wbindgen_enum_RequestMode[arg1];
        },
        __wbg_static_accessor_GLOBAL_9d53f2689e622ca1: function() {
            const ret = typeof global === 'undefined' ? null : global;
            return isLikeNone(ret) ? 0 : addToExternrefTable0(ret);
        },
        __wbg_static_accessor_GLOBAL_THIS_a1a35cec07001a8a: function() {
            const ret = typeof globalThis === 'undefined' ? null : globalThis;
            return isLikeNone(ret) ? 0 : addToExternrefTable0(ret);
        },
        __wbg_static_accessor_SELF_4c59f6c7ea29a144: function() {
            const ret = typeof self === 'undefined' ? null : self;
            return isLikeNone(ret) ? 0 : addToExternrefTable0(ret);
        },
        __wbg_static_accessor_WINDOW_e70ae9f2eb052253: function() {
            const ret = typeof window === 'undefined' ? null : window;
            return isLikeNone(ret) ? 0 : addToExternrefTable0(ret);
        },
        __wbg_status_00549d55b78d949e: function(arg0) {
            const ret = arg0.status;
            return ret;
        },
        __wbg_stringify_8286df6dcc591521: function() { return handleError(function (arg0) {
            const ret = JSON.stringify(arg0);
            return ret;
        }, arguments); },
        __wbg_then_18f476d590e58992: function(arg0, arg1, arg2) {
            const ret = arg0.then(arg1, arg2);
            return ret;
        },
        __wbg_then_ac7b025999b52837: function(arg0, arg1) {
            const ret = arg0.then(arg1);
            return ret;
        },
        __wbindgen_cast_0000000000000001: function(arg0, arg1) {
            // Cast intrinsic for `Closure(Closure { owned: true, function: Function { arguments: [Externref], shim_idx: 650, ret: Result(Unit), inner_ret: Some(Result(Unit)) }, mutable: true }) -> Externref`.
            const ret = makeMutClosure(arg0, arg1, wasm_bindgen__convert__closures_____invoke__h604311912c671172);
            return ret;
        },
        __wbindgen_cast_0000000000000002: function(arg0) {
            // Cast intrinsic for `F64 -> Externref`.
            const ret = arg0;
            return ret;
        },
        __wbindgen_cast_0000000000000003: function(arg0, arg1) {
            // Cast intrinsic for `Ref(String) -> Externref`.
            const ret = getStringFromWasm0(arg0, arg1);
            return ret;
        },
        __wbindgen_cast_0000000000000004: function(arg0, arg1) {
            var v0 = getArrayF64FromWasm0(arg0, arg1).slice();
            wasm.__wbindgen_free(arg0, arg1 * 8, 8);
            // Cast intrinsic for `Vector(F64) -> Externref`.
            const ret = v0;
            return ret;
        },
        __wbindgen_cast_0000000000000005: function(arg0, arg1) {
            var v0 = getArrayJsValueFromWasm0(arg0, arg1).slice();
            wasm.__wbindgen_free(arg0, arg1 * 4, 4);
            // Cast intrinsic for `Vector(NamedExternref("string")) -> Externref`.
            const ret = v0;
            return ret;
        },
        __wbindgen_cast_0000000000000006: function(arg0, arg1) {
            var v0 = getArrayU32FromWasm0(arg0, arg1).slice();
            wasm.__wbindgen_free(arg0, arg1 * 4, 4);
            // Cast intrinsic for `Vector(U32) -> Externref`.
            const ret = v0;
            return ret;
        },
        __wbindgen_init_externref_table: function() {
            const table = wasm.__wbindgen_externrefs;
            const offset = table.grow(4);
            table.set(0, undefined);
            table.set(offset + 0, undefined);
            table.set(offset + 1, null);
            table.set(offset + 2, true);
            table.set(offset + 3, false);
        },
    };
    return {
        __proto__: null,
        "./roaringrange_bg.js": import0,
    };
}

function wasm_bindgen__convert__closures_____invoke__h604311912c671172(arg0, arg1, arg2) {
    const ret = wasm.wasm_bindgen__convert__closures_____invoke__h604311912c671172(arg0, arg1, arg2);
    if (ret[1]) {
        throw takeFromExternrefTable0(ret[0]);
    }
}

function wasm_bindgen__convert__closures_____invoke__h134a34389e58ea02(arg0, arg1, arg2, arg3) {
    wasm.wasm_bindgen__convert__closures_____invoke__h134a34389e58ea02(arg0, arg1, arg2, arg3);
}


const __wbindgen_enum_RequestMode = ["same-origin", "no-cors", "cors", "navigate"];
const CountEstimateFinalization = (typeof FinalizationRegistry === 'undefined')
    ? { register: () => {}, unregister: () => {} }
    : new FinalizationRegistry(ptr => wasm.__wbg_countestimate_free(ptr, 1));
const FilteredIdsFinalization = (typeof FinalizationRegistry === 'undefined')
    ? { register: () => {}, unregister: () => {} }
    : new FinalizationRegistry(ptr => wasm.__wbg_filteredids_free(ptr, 1));
const Model2vecEmbedderFinalization = (typeof FinalizationRegistry === 'undefined')
    ? { register: () => {}, unregister: () => {} }
    : new FinalizationRegistry(ptr => wasm.__wbg_model2vecembedder_free(ptr, 1));
const PrefixSearchFinalization = (typeof FinalizationRegistry === 'undefined')
    ? { register: () => {}, unregister: () => {} }
    : new FinalizationRegistry(ptr => wasm.__wbg_prefixsearch_free(ptr, 1));
const RrbIndexFinalization = (typeof FinalizationRegistry === 'undefined')
    ? { register: () => {}, unregister: () => {} }
    : new FinalizationRegistry(ptr => wasm.__wbg_rrbindex_free(ptr, 1));
const RrfFacetsFinalization = (typeof FinalizationRegistry === 'undefined')
    ? { register: () => {}, unregister: () => {} }
    : new FinalizationRegistry(ptr => wasm.__wbg_rrffacets_free(ptr, 1));
const RrhcBundleFinalization = (typeof FinalizationRegistry === 'undefined')
    ? { register: () => {}, unregister: () => {} }
    : new FinalizationRegistry(ptr => wasm.__wbg_rrhcbundle_free(ptr, 1));
const RrsCatalogFinalization = (typeof FinalizationRegistry === 'undefined')
    ? { register: () => {}, unregister: () => {} }
    : new FinalizationRegistry(ptr => wasm.__wbg_rrscatalog_free(ptr, 1));
const RrsCursorFinalization = (typeof FinalizationRegistry === 'undefined')
    ? { register: () => {}, unregister: () => {} }
    : new FinalizationRegistry(ptr => wasm.__wbg_rrscursor_free(ptr, 1));
const RrsIndexFinalization = (typeof FinalizationRegistry === 'undefined')
    ? { register: () => {}, unregister: () => {} }
    : new FinalizationRegistry(ptr => wasm.__wbg_rrsindex_free(ptr, 1));
const RrsLookupFinalization = (typeof FinalizationRegistry === 'undefined')
    ? { register: () => {}, unregister: () => {} }
    : new FinalizationRegistry(ptr => wasm.__wbg_rrslookup_free(ptr, 1));
const RrsRecordsFinalization = (typeof FinalizationRegistry === 'undefined')
    ? { register: () => {}, unregister: () => {} }
    : new FinalizationRegistry(ptr => wasm.__wbg_rrsrecords_free(ptr, 1));
const RrsSecondaryCursorFinalization = (typeof FinalizationRegistry === 'undefined')
    ? { register: () => {}, unregister: () => {} }
    : new FinalizationRegistry(ptr => wasm.__wbg_rrssecondarycursor_free(ptr, 1));
const RrsSecondaryIndexFinalization = (typeof FinalizationRegistry === 'undefined')
    ? { register: () => {}, unregister: () => {} }
    : new FinalizationRegistry(ptr => wasm.__wbg_rrssecondaryindex_free(ptr, 1));
const RrsSortColsFinalization = (typeof FinalizationRegistry === 'undefined')
    ? { register: () => {}, unregister: () => {} }
    : new FinalizationRegistry(ptr => wasm.__wbg_rrssortcols_free(ptr, 1));
const RrssIndexFinalization = (typeof FinalizationRegistry === 'undefined')
    ? { register: () => {}, unregister: () => {} }
    : new FinalizationRegistry(ptr => wasm.__wbg_rrssindex_free(ptr, 1));
const RrtHitsFinalization = (typeof FinalizationRegistry === 'undefined')
    ? { register: () => {}, unregister: () => {} }
    : new FinalizationRegistry(ptr => wasm.__wbg_rrthits_free(ptr, 1));
const RrtIndexFinalization = (typeof FinalizationRegistry === 'undefined')
    ? { register: () => {}, unregister: () => {} }
    : new FinalizationRegistry(ptr => wasm.__wbg_rrtindex_free(ptr, 1));
const RrviHitsFinalization = (typeof FinalizationRegistry === 'undefined')
    ? { register: () => {}, unregister: () => {} }
    : new FinalizationRegistry(ptr => wasm.__wbg_rrvihits_free(ptr, 1));
const RrviIndexFinalization = (typeof FinalizationRegistry === 'undefined')
    ? { register: () => {}, unregister: () => {} }
    : new FinalizationRegistry(ptr => wasm.__wbg_rrviindex_free(ptr, 1));
const WasmBitmapFinalization = (typeof FinalizationRegistry === 'undefined')
    ? { register: () => {}, unregister: () => {} }
    : new FinalizationRegistry(ptr => wasm.__wbg_wasmbitmap_free(ptr, 1));

function addToExternrefTable0(obj) {
    const idx = wasm.__externref_table_alloc();
    wasm.__wbindgen_externrefs.set(idx, obj);
    return idx;
}

function _assertClass(instance, klass) {
    if (!(instance instanceof klass)) {
        throw new Error(`expected instance of ${klass.name}`);
    }
}

const CLOSURE_DTORS = (typeof FinalizationRegistry === 'undefined')
    ? { register: () => {}, unregister: () => {} }
    : new FinalizationRegistry(state => wasm.__wbindgen_destroy_closure(state.a, state.b));

function getArrayF32FromWasm0(ptr, len) {
    ptr = ptr >>> 0;
    return getFloat32ArrayMemory0().subarray(ptr / 4, ptr / 4 + len);
}

function getArrayF64FromWasm0(ptr, len) {
    ptr = ptr >>> 0;
    return getFloat64ArrayMemory0().subarray(ptr / 8, ptr / 8 + len);
}

function getArrayJsValueFromWasm0(ptr, len) {
    ptr = ptr >>> 0;
    const mem = getDataViewMemory0();
    const result = [];
    for (let i = ptr; i < ptr + 4 * len; i += 4) {
        result.push(wasm.__wbindgen_externrefs.get(mem.getUint32(i, true)));
    }
    wasm.__externref_drop_slice(ptr, len);
    return result;
}

function getArrayU32FromWasm0(ptr, len) {
    ptr = ptr >>> 0;
    return getUint32ArrayMemory0().subarray(ptr / 4, ptr / 4 + len);
}

function getArrayU8FromWasm0(ptr, len) {
    ptr = ptr >>> 0;
    return getUint8ArrayMemory0().subarray(ptr / 1, ptr / 1 + len);
}

let cachedDataViewMemory0 = null;
function getDataViewMemory0() {
    if (cachedDataViewMemory0 === null || cachedDataViewMemory0.buffer.detached === true || (cachedDataViewMemory0.buffer.detached === undefined && cachedDataViewMemory0.buffer !== wasm.memory.buffer)) {
        cachedDataViewMemory0 = new DataView(wasm.memory.buffer);
    }
    return cachedDataViewMemory0;
}

let cachedFloat32ArrayMemory0 = null;
function getFloat32ArrayMemory0() {
    if (cachedFloat32ArrayMemory0 === null || cachedFloat32ArrayMemory0.byteLength === 0) {
        cachedFloat32ArrayMemory0 = new Float32Array(wasm.memory.buffer);
    }
    return cachedFloat32ArrayMemory0;
}

let cachedFloat64ArrayMemory0 = null;
function getFloat64ArrayMemory0() {
    if (cachedFloat64ArrayMemory0 === null || cachedFloat64ArrayMemory0.byteLength === 0) {
        cachedFloat64ArrayMemory0 = new Float64Array(wasm.memory.buffer);
    }
    return cachedFloat64ArrayMemory0;
}

function getStringFromWasm0(ptr, len) {
    return decodeText(ptr >>> 0, len);
}

let cachedUint32ArrayMemory0 = null;
function getUint32ArrayMemory0() {
    if (cachedUint32ArrayMemory0 === null || cachedUint32ArrayMemory0.byteLength === 0) {
        cachedUint32ArrayMemory0 = new Uint32Array(wasm.memory.buffer);
    }
    return cachedUint32ArrayMemory0;
}

let cachedUint8ArrayMemory0 = null;
function getUint8ArrayMemory0() {
    if (cachedUint8ArrayMemory0 === null || cachedUint8ArrayMemory0.byteLength === 0) {
        cachedUint8ArrayMemory0 = new Uint8Array(wasm.memory.buffer);
    }
    return cachedUint8ArrayMemory0;
}

function handleError(f, args) {
    try {
        return f.apply(this, args);
    } catch (e) {
        const idx = addToExternrefTable0(e);
        wasm.__wbindgen_exn_store(idx);
    }
}

function isLikeNone(x) {
    return x === undefined || x === null;
}

function makeMutClosure(arg0, arg1, f) {
    const state = { a: arg0, b: arg1, cnt: 1 };
    const real = (...args) => {

        // First up with a closure we increment the internal reference
        // count. This ensures that the Rust closure environment won't
        // be deallocated while we're invoking it.
        state.cnt++;
        const a = state.a;
        state.a = 0;
        try {
            return f(a, state.b, ...args);
        } finally {
            state.a = a;
            real._wbg_cb_unref();
        }
    };
    real._wbg_cb_unref = () => {
        if (--state.cnt === 0) {
            wasm.__wbindgen_destroy_closure(state.a, state.b);
            state.a = 0;
            CLOSURE_DTORS.unregister(state);
        }
    };
    CLOSURE_DTORS.register(real, state, state);
    return real;
}

function passArray32ToWasm0(arg, malloc) {
    const ptr = malloc(arg.length * 4, 4) >>> 0;
    getUint32ArrayMemory0().set(arg, ptr / 4);
    WASM_VECTOR_LEN = arg.length;
    return ptr;
}

function passArray8ToWasm0(arg, malloc) {
    const ptr = malloc(arg.length * 1, 1) >>> 0;
    getUint8ArrayMemory0().set(arg, ptr / 1);
    WASM_VECTOR_LEN = arg.length;
    return ptr;
}

function passArrayF32ToWasm0(arg, malloc) {
    const ptr = malloc(arg.length * 4, 4) >>> 0;
    getFloat32ArrayMemory0().set(arg, ptr / 4);
    WASM_VECTOR_LEN = arg.length;
    return ptr;
}

function passArrayJsValueToWasm0(array, malloc) {
    const ptr = malloc(array.length * 4, 4) >>> 0;
    for (let i = 0; i < array.length; i++) {
        const add = addToExternrefTable0(array[i]);
        getDataViewMemory0().setUint32(ptr + 4 * i, add, true);
    }
    WASM_VECTOR_LEN = array.length;
    return ptr;
}

function passStringToWasm0(arg, malloc, realloc) {
    if (realloc === undefined) {
        const buf = cachedTextEncoder.encode(arg);
        const ptr = malloc(buf.length, 1) >>> 0;
        getUint8ArrayMemory0().subarray(ptr, ptr + buf.length).set(buf);
        WASM_VECTOR_LEN = buf.length;
        return ptr;
    }

    let len = arg.length;
    let ptr = malloc(len, 1) >>> 0;

    const mem = getUint8ArrayMemory0();

    let offset = 0;

    for (; offset < len; offset++) {
        const code = arg.charCodeAt(offset);
        if (code > 0x7F) break;
        mem[ptr + offset] = code;
    }
    if (offset !== len) {
        if (offset !== 0) {
            arg = arg.slice(offset);
        }
        ptr = realloc(ptr, len, len = offset + arg.length * 3, 1) >>> 0;
        const view = getUint8ArrayMemory0().subarray(ptr + offset, ptr + len);
        const ret = cachedTextEncoder.encodeInto(arg, view);

        offset += ret.written;
        ptr = realloc(ptr, len, offset, 1) >>> 0;
    }

    WASM_VECTOR_LEN = offset;
    return ptr;
}

function takeFromExternrefTable0(idx) {
    const value = wasm.__wbindgen_externrefs.get(idx);
    wasm.__externref_table_dealloc(idx);
    return value;
}

let cachedTextDecoder = new TextDecoder('utf-8', { ignoreBOM: true, fatal: true });
cachedTextDecoder.decode();
const MAX_SAFARI_DECODE_BYTES = 2146435072;
let numBytesDecoded = 0;
function decodeText(ptr, len) {
    numBytesDecoded += len;
    if (numBytesDecoded >= MAX_SAFARI_DECODE_BYTES) {
        cachedTextDecoder = new TextDecoder('utf-8', { ignoreBOM: true, fatal: true });
        cachedTextDecoder.decode();
        numBytesDecoded = len;
    }
    return cachedTextDecoder.decode(getUint8ArrayMemory0().subarray(ptr, ptr + len));
}

const cachedTextEncoder = new TextEncoder();

if (!('encodeInto' in cachedTextEncoder)) {
    cachedTextEncoder.encodeInto = function (arg, view) {
        const buf = cachedTextEncoder.encode(arg);
        view.set(buf);
        return {
            read: arg.length,
            written: buf.length
        };
    };
}

let WASM_VECTOR_LEN = 0;

let wasmModule, wasmInstance, wasm;
function __wbg_finalize_init(instance, module) {
    wasmInstance = instance;
    wasm = instance.exports;
    wasmModule = module;
    cachedDataViewMemory0 = null;
    cachedFloat32ArrayMemory0 = null;
    cachedFloat64ArrayMemory0 = null;
    cachedUint32ArrayMemory0 = null;
    cachedUint8ArrayMemory0 = null;
    wasm.__wbindgen_start();
    return wasm;
}

async function __wbg_load(module, imports) {
    if (typeof Response === 'function' && module instanceof Response) {
        if (typeof WebAssembly.instantiateStreaming === 'function') {
            try {
                return await WebAssembly.instantiateStreaming(module, imports);
            } catch (e) {
                const validResponse = module.ok && expectedResponseType(module.type);

                if (validResponse && module.headers.get('Content-Type') !== 'application/wasm') {
                    console.warn("`WebAssembly.instantiateStreaming` failed because your server does not serve Wasm with `application/wasm` MIME type. Falling back to `WebAssembly.instantiate` which is slower. Original error:\n", e);

                } else { throw e; }
            }
        }

        const bytes = await module.arrayBuffer();
        return await WebAssembly.instantiate(bytes, imports);
    } else {
        const instance = await WebAssembly.instantiate(module, imports);

        if (instance instanceof WebAssembly.Instance) {
            return { instance, module };
        } else {
            return instance;
        }
    }

    function expectedResponseType(type) {
        switch (type) {
            case 'basic': case 'cors': case 'default': return true;
        }
        return false;
    }
}

function initSync(module) {
    if (wasm !== undefined) return wasm;


    if (module !== undefined) {
        if (Object.getPrototypeOf(module) === Object.prototype) {
            ({module} = module)
        } else {
            console.warn('using deprecated parameters for `initSync()`; pass a single object instead')
        }
    }

    const imports = __wbg_get_imports();
    if (!(module instanceof WebAssembly.Module)) {
        module = new WebAssembly.Module(module);
    }
    const instance = new WebAssembly.Instance(module, imports);
    return __wbg_finalize_init(instance, module);
}

async function __wbg_init(module_or_path) {
    if (wasm !== undefined) return wasm;


    if (module_or_path !== undefined) {
        if (Object.getPrototypeOf(module_or_path) === Object.prototype) {
            ({module_or_path} = module_or_path)
        } else {
            console.warn('using deprecated parameters for the initialization function; pass a single object instead')
        }
    }

    if (module_or_path === undefined) {
        module_or_path = new URL('roaringrange_bg.wasm', import.meta.url);
    }
    const imports = __wbg_get_imports();

    if (typeof module_or_path === 'string' || (typeof Request === 'function' && module_or_path instanceof Request) || (typeof URL === 'function' && module_or_path instanceof URL)) {
        module_or_path = fetch(module_or_path);
    }

    const { instance, module } = await __wbg_load(await module_or_path, imports);

    return __wbg_finalize_init(instance, module);
}

export { initSync, __wbg_init as default };
