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
	"fmt"
	"io"
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

// Doer is the HTTP seam, injectable for tests.
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Enricher resolves creators via the Wikidata Query Service.
type Enricher struct {
	client   Doer
	endpoint string
	// batch is how many ISBNs one SPARQL query carries; delay is the
	// politeness pause between queries (WDQS is a shared public service).
	batch int
	delay time.Duration
	now   func() time.Time
}

// Option configures the enricher.
type Option func(*Enricher)

// WithClient injects the HTTP client (tests; a caller with its own limits).
func WithClient(d Doer) Option { return func(e *Enricher) { e.client = d } }

// WithEndpoint points at a different SPARQL endpoint (a mirror, a test stub).
func WithEndpoint(u string) Option { return func(e *Enricher) { e.endpoint = u } }

// WithDelay overrides the politeness pause between SPARQL batches.
func WithDelay(d time.Duration) Option { return func(e *Enricher) { e.delay = d } }

// New returns the Wikidata creator-demographics enricher.
func New(opts ...Option) *Enricher {
	e := &Enricher{
		client:   http.DefaultClient,
		endpoint: DefaultEndpoint,
		batch:    40,
		delay:    time.Second,
		now:      time.Now,
	}
	for _, o := range opts {
		o(e)
	}
	return e
}

// Name implements ingest.Enricher.
func (e *Enricher) Name() string { return Name }

// Enrich resolves each Work's ISBNs and returns creator-claim enrichments for
// the Works that matched. Works with no ISBN, or whose ISBNs Wikidata does not
// know, are absent from the result -- RunEnrich leaves them untouched, and the
// audit reports them as unmatched rather than guessing.
func (e *Enricher) Enrich(ctx context.Context, works []ingest.WorkSummary) ([]ingest.Enrichment, error) {
	isbnToWorks := map[string][]string{}
	var order []string
	for _, w := range works {
		for _, raw := range w.ISBNs {
			isbn := normalizeISBN(raw)
			if isbn == "" {
				continue
			}
			if _, seen := isbnToWorks[isbn]; !seen {
				order = append(order, isbn)
			}
			isbnToWorks[isbn] = append(isbnToWorks[isbn], w.WorkID)
		}
	}
	if len(order) == 0 {
		return nil, nil
	}

	retrieved := e.now().UTC().Format("2006-01-02")
	byWork := map[string]map[string]*ingest.CreatorClaim{} // workID -> QID -> claim
	for start := 0; start < len(order); start += e.batch {
		if start > 0 && e.delay > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(e.delay):
			}
		}
		end := min(start+e.batch, len(order))
		rows, err := e.query(ctx, order[start:end])
		if err != nil {
			return nil, fmt.Errorf("wikidata: %w", err)
		}
		for _, row := range rows {
			qid := strings.TrimPrefix(row.author, entityPrefix)
			if qid == row.author || qid == "" {
				continue // not an entity IRI: never synthesize an identity
			}
			for _, workID := range isbnToWorks[row.isbn] {
				claims := byWork[workID]
				if claims == nil {
					claims = map[string]*ingest.CreatorClaim{}
					byWork[workID] = claims
				}
				c := claims[qid]
				if c == nil {
					c = &ingest.CreatorClaim{
						QID:        qid,
						MatchedVia: "isbn:" + row.isbn,
						Retrieved:  retrieved,
					}
					claims[qid] = c
				}
				// OPTIONAL bindings arrive on some rows and not others;
				// take the label from whichever row carries it.
				if c.Label == "" && row.authorLabel != "" {
					c.Label = row.authorLabel
				}
				if row.prop != "" && row.value != "" {
					c.AddClaim(ingest.DemographicClaim{
						Property:   row.prop,
						ValueQID:   strings.TrimPrefix(row.value, entityPrefix),
						ValueLabel: row.valueLabel,
					})
				}
			}
		}
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
	isbn, author, authorLabel, prop, value, valueLabel string
}

// query runs one batched resolution: ISBN -> edition -> work -> author,
// with the demographic claims OPTIONAL so a resolved author with no stated
// claims still comes back (the audit counts them as matched-but-unknown).
func (e *Enricher) query(ctx context.Context, isbns []string) ([]row, error) {
	var values strings.Builder
	for _, i := range isbns {
		fmt.Fprintf(&values, "%q ", i)
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
	sparql := fmt.Sprintf(`SELECT ?isbn ?author ?authorLabel ?prop ?value ?valueLabel WHERE {
  VALUES ?isbn { %s}
  ?edition wdt:P212|wdt:P957 ?i .
  FILTER(REPLACE(STR(?i), "-", "") = ?isbn)
  { ?edition wdt:P629 ?bwork . ?bwork wdt:P50 ?author . } UNION { ?edition wdt:P50 ?author . }
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
}`, values.String(), props.String())

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
		return nil, fmt.Errorf("sparql status %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
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
			isbn:        get("isbn"),
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
