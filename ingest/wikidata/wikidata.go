// Package wikidata resolves a Work's creators to Wikidata entities and caches
// their EXPLICITLY-STATED demographic claims as enrichment statements -- the
// creator-demographics half of the diversity-audit feature.
//
// The contract, in order of importance:
//
//   - No name inference, ever. Resolution goes ISBN -> edition (P212/P957) ->
//     work (P629) -> author (P50): every hop is a cataloged identifier or an
//     explicit Wikidata statement. A creator's name is never matched against
//     anything, and a work without a resolvable identifier yields nothing.
//   - Explicit claims only, with provenance. Only the values Wikidata states
//     outright for P21 (sex or gender), P27 (country of citizenship), P91
//     (sexual orientation), and P172 (ethnic group) are recorded, each as the
//     claim's own entity URI plus label, alongside the QID it came from, what
//     identifier matched it, and the retrieval date. Birth/death dates are
//     deliberately not fetched.
//   - Aggregate use. These statements exist so a collection-level audit can
//     report distributions with coverage; they are enrichment-graph data, not
//     display fields, and the projector does not surface them on work pages.
//
// Coverage will be partial and skewed (Wikidata's own coverage is); the audit
// reading this data is responsible for reporting match rate and unknowns
// first. Statements land in the enrichment:wikidata graph, dropped and
// replaced on each run, so a re-run refreshes and a removed claim upstream
// disappears here too.
package wikidata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/freeeve/libcat/ingest"
)

// Name is the enrichment source name; statements land in enrichment:wikidata.
const Name = "wikidata"

// DefaultEndpoint is the public Wikidata Query Service SPARQL endpoint.
const DefaultEndpoint = "https://query.wikidata.org/sparql"

// userAgent identifies libcat to WDQS per its user-agent policy.
const userAgent = "libcat-diversity-audit/1.0 (https://github.com/freeeve/libcat)"

// entityPrefix is the Wikidata entity IRI namespace QIDs expand under.
const entityPrefix = "http://www.wikidata.org/entity/"

// claimProps are the only properties fetched: the explicitly-stated
// demographic claims the audit aggregates. Order is the emission order.
var claimProps = []string{"P21", "P27", "P91", "P172"}

// isbnMatcher resolves a key that is a hyphenless ISBN: edition -> work ->
// author. Wikidata stores P212 hyphenated and computing the canonical
// hyphenation needs the ISBN range table, so match on the stripped form; the
// property-path scan costs seconds per query regardless of batch size.
const isbnMatcher = `?edition wdt:P212|wdt:P957 ?i .
  FILTER(REPLACE(STR(?i), "-", "") = ?key)
  { ?edition wdt:P629 ?bwork . ?bwork wdt:P50 ?author . } UNION { ?edition wdt:P50 ?author . }`

// authorIDProps maps an author authority-identifier scheme to its Wikidata
// property: the direct, indexed resolution hops. Order fixes pass order.
var authorIDProps = []struct{ scheme, prop string }{
	{"viaf", "P214"},
	{"lcnaf", "P244"},
	{"isni", "P213"},
	{"orcid", "P496"},
}

// classifyAuthorID extracts (scheme, wikidata-formatted value) from an agent
// authority IRI, or ok=false for namespaces the resolver does not speak.
// ISNI is stored space-grouped on Wikidata; ORCID keeps its hyphens.
func classifyAuthorID(iri string) (scheme, value string, ok bool) {
	u := strings.TrimPrefix(strings.TrimPrefix(iri, "https://"), "http://")
	u = strings.TrimSuffix(u, "/")
	switch {
	case strings.HasPrefix(u, "viaf.org/viaf/"):
		return "viaf", strings.TrimPrefix(u, "viaf.org/viaf/"), true
	case strings.HasPrefix(u, "id.loc.gov/authorities/names/"):
		return "lcnaf", strings.TrimPrefix(u, "id.loc.gov/authorities/names/"), true
	case strings.HasPrefix(u, "isni.org/isni/"):
		raw := strings.ReplaceAll(strings.TrimPrefix(u, "isni.org/isni/"), " ", "")
		if len(raw) != 16 {
			return "", "", false
		}
		return "isni", raw[0:4] + " " + raw[4:8] + " " + raw[8:12] + " " + raw[12:16], true
	case strings.HasPrefix(u, "orcid.org/"):
		return "orcid", strings.TrimPrefix(u, "orcid.org/"), true
	}
	return "", "", false
}

// authorIDMatcher resolves a key that is an identifier value under one
// authority property: a direct indexed lookup, no edition hop.
func authorIDMatcher(prop string) string {
	return "?author wdt:" + prop + " ?key ."
}

// Doer is the HTTP seam, injectable for tests.
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// maxRetries is how many extra times one SPARQL batch is attempted after a
// transient failure (a network error, a 429, or a 5xx -- WDQS 504s are
// routine on a shared public service); the pause between attempts starts at
// the enricher's retryBase and doubles. A batch that still fails is SKIPPED,
// not fatal: its works stay untouched and a re-run backfills them, so an
// hours-long corpus run survives a bad stretch.
const maxRetries = 5

// Enricher resolves creators via the Wikidata Query Service.
type Enricher struct {
	client   Doer
	endpoint string
	// batch is how many ISBNs one SPARQL query carries; delay is the
	// politeness pause between queries (WDQS is a shared public service).
	batch     int
	delay     time.Duration
	retryBase time.Duration
	now       func() time.Time
	log       *slog.Logger
	// stats accumulates the last Enrich call's run counters.
	stats ingest.EnrichStats
}

// Skipped reports how many batches the last Enrich call abandoned after
// retries; their works were left untouched for a re-run to backfill.
func (e *Enricher) Skipped() int { return e.stats.SkippedBatches }

// RunStats implements ingest.StatsReporter: the last run's counters, surfaced
// in the run endpoint's result.
func (e *Enricher) RunStats() ingest.EnrichStats { return e.stats }

// Option configures the enricher.
type Option func(*Enricher)

// WithClient injects the HTTP client (tests; a caller with its own limits).
func WithClient(d Doer) Option { return func(e *Enricher) { e.client = d } }

// WithEndpoint points at a different SPARQL endpoint (a mirror, a test stub).
func WithEndpoint(u string) Option { return func(e *Enricher) { e.endpoint = u } }

// WithDelay overrides the politeness pause between SPARQL batches.
func WithDelay(d time.Duration) Option { return func(e *Enricher) { e.delay = d } }

// WithRetryBase overrides the first retry pause (tests use 0).
func WithRetryBase(d time.Duration) Option { return func(e *Enricher) { e.retryBase = d } }

// WithLogger wires progress logging: per-batch INFO lines, retry and
// skipped-batch WARNs -- a multi-minute synchronous run must be observable
// from the server log. Nil (the default) logs nothing.
func WithLogger(l *slog.Logger) Option { return func(e *Enricher) { e.log = l } }

// New returns the Wikidata creator-demographics enricher.
func New(opts ...Option) *Enricher {
	e := &Enricher{
		client:    http.DefaultClient,
		endpoint:  DefaultEndpoint,
		batch:     40,
		delay:     time.Second,
		retryBase: 2 * time.Second,
		now:       time.Now,
	}
	for _, o := range opts {
		o(e)
	}
	return e
}

// Name implements ingest.Enricher.
func (e *Enricher) Name() string { return Name }

// Enrich resolves each Work's creators by cataloged identifiers, in two
// passes: author authority IDs first (VIAF/LCNAF/ISNI/ORCID -> the direct
// wdt:P214/P244/P213/P496 lookups -- indexed exact matches, the high-recall
// path), then ISBN -> edition -> author for works the first pass left
// unmatched. Works with no resolvable identifier, or whose identifiers
// Wikidata does not know, are absent from the result -- RunEnrich leaves
// them untouched, and the audit reports them as unmatched rather than
// guessing. Never a name, in either pass.
func (e *Enricher) Enrich(ctx context.Context, works []ingest.WorkSummary) ([]ingest.Enrichment, error) {
	// Pass-1 keys: scheme -> wikidata-formatted value -> work ids.
	idKeys := map[string]map[string][]string{}
	for _, w := range works {
		for _, iri := range w.ContributorIDs {
			scheme, value, ok := classifyAuthorID(iri)
			if !ok {
				continue
			}
			byValue := idKeys[scheme]
			if byValue == nil {
				byValue = map[string][]string{}
				idKeys[scheme] = byValue
			}
			byValue[value] = append(byValue[value], w.WorkID)
		}
	}
	// Pass-2 keys: hyphenless ISBN -> work ids.
	isbnKeys := map[string][]string{}
	var isbnOrder []string
	for _, w := range works {
		for _, raw := range w.ISBNs {
			isbn := normalizeISBN(raw)
			if isbn == "" {
				continue
			}
			if _, seen := isbnKeys[isbn]; !seen {
				isbnOrder = append(isbnOrder, isbn)
			}
			isbnKeys[isbn] = append(isbnKeys[isbn], w.WorkID)
		}
	}

	retrieved := e.now().UTC().Format("2006-01-02")
	e.stats = ingest.EnrichStats{}
	started := e.now()
	succeeded := 0
	firstBatch := true
	var lastErr error
	byWork := map[string]map[string]*ingest.CreatorClaim{} // workID -> QID -> claim
	creators := map[string]bool{}
	claimCount := 0

	// runPass batches one matcher over its keys, attributing rows to works.
	runPass := func(pass, matcher, keyScheme string, order []string, keyToWorks map[string][]string) error {
		total := (len(order) + e.batch - 1) / e.batch
		for start := 0; start < len(order); start += e.batch {
			if !firstBatch && e.delay > 0 {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(e.delay):
				}
			}
			firstBatch = false
			end := min(start+e.batch, len(order))
			batchStart := e.now()
			rows, err := e.queryRetry(ctx, matcher, order[start:end])
			e.stats.Batches++
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				e.stats.SkippedBatches++
				lastErr = err
				if e.log != nil {
					e.log.Warn("wikidata batch skipped after retries",
						"pass", pass, "batch", e.stats.Batches, "of", total, "keys", end-start, "err", err)
				}
				continue
			}
			succeeded++
			if e.log != nil {
				e.log.Info("wikidata batch resolved",
					"pass", pass, "batch", e.stats.Batches, "of", total, "keys", end-start,
					"rows", len(rows), "creators", len(creators), "claims", claimCount,
					"batchElapsed", e.now().Sub(batchStart).Round(time.Millisecond),
					"elapsed", e.now().Sub(started).Round(time.Second))
			}
			for _, row := range rows {
				qid := strings.TrimPrefix(row.author, entityPrefix)
				if qid == row.author || qid == "" {
					continue // not an entity IRI: never synthesize an identity
				}
				for _, workID := range keyToWorks[row.key] {
					claims := byWork[workID]
					if claims == nil {
						claims = map[string]*ingest.CreatorClaim{}
						byWork[workID] = claims
					}
					c := claims[qid]
					if c == nil {
						c = &ingest.CreatorClaim{
							QID:        qid,
							MatchedVia: keyScheme + ":" + row.key,
							Retrieved:  retrieved,
						}
						claims[qid] = c
						creators[qid] = true
					}
					// OPTIONAL bindings arrive on some rows and not others;
					// take the label from whichever row carries it.
					if c.Label == "" && row.authorLabel != "" {
						c.Label = row.authorLabel
					}
					if row.prop != "" && row.value != "" {
						before := len(c.Claims)
						c.AddClaim(ingest.DemographicClaim{
							Property:   row.prop,
							ValueQID:   strings.TrimPrefix(row.value, entityPrefix),
							ValueLabel: row.valueLabel,
						})
						if len(c.Claims) > before {
							claimCount++
						}
					}
				}
			}
		}
		return nil
	}

	// Pass 1: author authority ids, per scheme.
	for _, sp := range authorIDProps {
		byValue := idKeys[sp.scheme]
		if len(byValue) == 0 {
			continue
		}
		order := make([]string, 0, len(byValue))
		for v := range byValue {
			order = append(order, v)
		}
		sort.Strings(order)
		if err := runPass(sp.scheme, authorIDMatcher(sp.prop), sp.scheme, order, byValue); err != nil {
			return nil, err
		}
	}
	// Pass 2: ISBN fallback, only for works pass 1 resolved nothing on --
	// recall without re-querying what the direct hops already answered.
	pendingISBN := map[string][]string{}
	var pendingOrder []string
	for _, isbn := range isbnOrder {
		var pending []string
		for _, id := range isbnKeys[isbn] {
			if len(byWork[id]) == 0 {
				pending = append(pending, id)
			}
		}
		if len(pending) > 0 {
			pendingOrder = append(pendingOrder, isbn)
			pendingISBN[isbn] = pending
		}
	}
	if len(pendingOrder) > 0 {
		if err := runPass("isbn", isbnMatcher, "isbn", pendingOrder, pendingISBN); err != nil {
			return nil, err
		}
	}
	if len(idKeys) == 0 && len(isbnOrder) == 0 {
		return nil, nil
	}

	e.stats.ResolvedCreators = len(creators)
	e.stats.Claims = claimCount
	e.stats.ElapsedMS = e.now().Sub(started).Milliseconds()
	if e.log != nil {
		e.log.Info("wikidata enrichment finished",
			"batches", e.stats.Batches, "skipped", e.stats.SkippedBatches,
			"creators", e.stats.ResolvedCreators, "claims", e.stats.Claims,
			"elapsed", e.now().Sub(started).Round(time.Second))
	}
	// Every batch failing is configuration-shaped (bad endpoint, outage),
	// not weather; partial failure is survivable and a re-run backfills.
	if succeeded == 0 && lastErr != nil {
		return nil, fmt.Errorf("wikidata: every batch failed, last: %w", lastErr)
	}

	workIDs := make([]string, 0, len(byWork))
	for id := range byWork {
		workIDs = append(workIDs, id)
	}
	sort.Strings(workIDs)
	out := make([]ingest.Enrichment, 0, len(workIDs))
	for _, id := range workIDs {
		claims := byWork[id]
		qids := make([]string, 0, len(claims))
		for q := range claims {
			qids = append(qids, q)
		}
		sort.Strings(qids)
		enr := ingest.Enrichment{WorkID: id}
		for _, q := range qids {
			enr.Creators = append(enr.Creators, *claims[q])
		}
		out = append(out, enr)
	}
	return out, nil
}

// row is one SPARQL result binding set, flattened.
type row struct {
	key, author, authorLabel, prop, value, valueLabel string
}

// queryRetry wraps query with backoff on transient failures: network errors,
// 429s, and 5xx statuses retry up to maxRetries with doubling pauses; other
// statuses (a 400 means the query itself is malformed) fail immediately.
func (e *Enricher) queryRetry(ctx context.Context, matcher string, keys []string) ([]row, error) {
	backoff := e.retryBase
	var err error
	for attempt := 0; ; attempt++ {
		var rows []row
		rows, err = e.query(ctx, matcher, keys)
		if err == nil {
			return rows, nil
		}
		if !transient(err) || attempt >= maxRetries {
			return nil, err
		}
		if e.log != nil {
			e.log.Warn("wikidata batch retrying",
				"attempt", attempt+1, "of", maxRetries, "backoff", backoff, "err", err)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
		backoff *= 2
	}
}

// statusError marks a non-200 SPARQL response with its code, so the retry
// wrapper can tell WDQS weather (5xx/429) from a broken query (4xx).
type statusError struct {
	code int
	body string
}

func (e *statusError) Error() string {
	return fmt.Sprintf("sparql status %d: %s", e.code, e.body)
}

// transient reports whether an error is worth retrying: any transport error,
// or a 429/5xx status.
func transient(err error) bool {
	var se *statusError
	if errors.As(err, &se) {
		return se.code == http.StatusTooManyRequests || se.code >= 500
	}
	return true // transport-level: connection reset, timeout, DNS blip
}

// query runs one batched resolution: ISBN -> edition -> work -> author,
// with the demographic claims OPTIONAL so a resolved author with no stated
// claims still comes back (the audit counts them as matched-but-unknown).
func (e *Enricher) query(ctx context.Context, matcher string, keys []string) ([]row, error) {
	var values strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&values, "%q ", k)
	}
	var props strings.Builder
	for _, p := range claimProps {
		fmt.Fprintf(&props, "wdt:%s ", p)
	}
	// Wikidata stores P212 hyphenated; grains carry hyphenless ISBNs.
	// Computing the canonical hyphenation needs the ISBN range table, so
	// match on the stripped form instead. The property-path scan plus
	// FILTER costs seconds per query regardless of batch size, which the
	// batch amortizes. The author label is an explicit OPTIONAL rather
	// than the label service, which does not reliably bind through the
	// UNION. The claims are OPTIONAL so a resolved author with none stated
	// still returns (matched-but-unknown, which the audit reports).
	// Explicit PREFIX declarations: the official WDQS auto-registers the
	// Wikidata prefixes as a convenience, but spec-compliant endpoints (a
	// QLever mirror via LCATD_ENRICH_WIKIDATA_ENDPOINT) reject bare wdt:/rdfs:.
	// The label service was already avoided for the same portability reason.
	sparql := fmt.Sprintf(`PREFIX wdt: <http://www.wikidata.org/prop/direct/>
PREFIX rdfs: <http://www.w3.org/2000/01/rdf-schema#>
SELECT ?key ?author ?authorLabel ?prop ?value ?valueLabel WHERE {
  VALUES ?key { %s}
  %s
  OPTIONAL { ?author rdfs:label ?aEn . FILTER(LANG(?aEn) = "en") }
  OPTIONAL { ?author rdfs:label ?aMul . FILTER(LANG(?aMul) = "mul") }
  BIND(COALESCE(?aEn, ?aMul) AS ?authorLabel)
  OPTIONAL {
    VALUES ?prop { %s}
    ?author ?prop ?value .
    OPTIONAL { ?value rdfs:label ?vEn . FILTER(LANG(?vEn) = "en") }
    OPTIONAL { ?value rdfs:label ?vMul . FILTER(LANG(?vMul) = "mul") }
  }
  BIND(COALESCE(?vEn, ?vMul) AS ?valueLabel)
}`, values.String(), matcher, props.String())

	form := url.Values{"query": {sparql}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.endpoint,
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/sparql-results+json")
	req.Header.Set("User-Agent", userAgent)

	res, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return nil, &statusError{code: res.StatusCode, body: strings.TrimSpace(string(body))}
	}

	var parsed struct {
		Results struct {
			Bindings []map[string]struct {
				Value string `json:"value"`
			} `json:"bindings"`
		} `json:"results"`
	}
	if err := json.NewDecoder(res.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode sparql results: %w", err)
	}
	rows := make([]row, 0, len(parsed.Results.Bindings))
	for _, b := range parsed.Results.Bindings {
		get := func(k string) string { return b[k].Value }
		rows = append(rows, row{
			key:         get("key"),
			author:      get("author"),
			authorLabel: get("authorLabel"),
			prop:        propLocal(get("prop")),
			value:       get("value"),
			valueLabel:  get("valueLabel"),
		})
	}
	return rows, nil
}

// propLocal reduces a wdt property IRI to its P-id ("...prop/direct/P21" ->
// "P21"); anything else returns "".
func propLocal(iri string) string {
	if iri == "" {
		return ""
	}
	i := strings.LastIndexByte(iri, '/')
	p := iri[i+1:]
	if !strings.HasPrefix(p, "P") {
		return ""
	}
	return p
}

// normalizeISBN strips hyphens and spaces and upcases a trailing X. Anything
// that then is not 10 or 13 digits (final X allowed) is dropped.
func normalizeISBN(raw string) string {
	s := strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(strings.TrimSpace(raw)))
	if len(s) != 10 && len(s) != 13 {
		return ""
	}
	for i, c := range s {
		if c >= '0' && c <= '9' {
			continue
		}
		if c == 'X' && i == 9 && len(s) == 10 {
			continue
		}
		return ""
	}
	return s
}
