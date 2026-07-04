package vocabsrc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Suggest response dialects.
const (
	// FlavorSuggest2 is the id.loc.gov suggest2 API (any of its datasets --
	// and anything else speaking the same shape).
	FlavorSuggest2 = "suggest2"
	// FlavorWikidata is the Wikidata wbsearchentities action API.
	FlavorWikidata = "wikidata"
	// FlavorVIAF is the VIAF AutoSuggest API.
	FlavorVIAF = "viaf"
)

// Suggestion is one live typeahead hit, source-tagged for the picker badge.
// ExactMatch carries sibling identifiers when the source exposes them (VIAF
// clusters map to LCNAF/GND/Wikidata) so a term created from the pick can
// record skos:exactMatch cross-references.
type Suggestion struct {
	Source      string `json:"source"`
	Scheme      string `json:"scheme"`
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	// Variants are the heading's variant/"used for" labels when the source
	// exposes them (suggest2's more.variantLabels) -- often the only context
	// a bare authorized heading carries.
	Variants   []string `json:"variants,omitempty"`
	ExactMatch []string `json:"exactMatch,omitempty"`
}

// SuggestClient queries a source's live typeahead API.
type SuggestClient struct {
	// Client overrides the HTTP client (tests). Default: 10s timeout.
	Client *http.Client
}

const suggestUserAgent = "libcatalog-vocabsrc"

var defaultSuggestHTTP = &http.Client{Timeout: 10 * time.Second}

func (c *SuggestClient) client() *http.Client {
	if c != nil && c.Client != nil {
		return c.Client
	}
	return defaultSuggestHTTP
}

// Suggest queries one source for up to limit typeahead hits.
func (c *SuggestClient) Suggest(ctx context.Context, src Source, q string, limit int) ([]Suggestion, error) {
	if !src.CanSuggest() {
		return nil, fmt.Errorf("%w: source %q has no suggest endpoint", ErrValidation, src.Name)
	}
	if limit <= 0 || limit > 20 {
		limit = 10
	}
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, nil
	}
	var u string
	switch src.SuggestFlavor {
	case FlavorSuggest2:
		u = fmt.Sprintf("%s/%s/suggest2?q=%s&count=%d",
			strings.TrimSuffix(src.SuggestURL, "/"), strings.Trim(src.SuggestDataset, "/"),
			url.QueryEscape(q), limit)
	case FlavorWikidata:
		u = fmt.Sprintf("%s/w/api.php?action=wbsearchentities&format=json&language=en&uselang=en&type=item&limit=%d&search=%s",
			strings.TrimSuffix(src.SuggestURL, "/"), limit, url.QueryEscape(q))
	case FlavorVIAF:
		u = fmt.Sprintf("%s/viaf/AutoSuggest?query=%s",
			strings.TrimSuffix(src.SuggestURL, "/"), url.QueryEscape(q))
	default:
		return nil, fmt.Errorf("%w: unknown suggest flavor %q", ErrValidation, src.SuggestFlavor)
	}
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, fmt.Errorf("vocabsrc: suggest %s %q: %w", src.Name, q, err)
	}
	switch src.SuggestFlavor {
	case FlavorSuggest2:
		return parseSuggest2(src, body, limit)
	case FlavorWikidata:
		return parseWikidata(src, body, limit)
	default:
		return parseVIAF(src, body, limit)
	}
}

func (c *SuggestClient) get(ctx context.Context, u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", suggestUserAgent)
	resp, err := c.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	const maxBody = 4 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxBody {
		return nil, fmt.Errorf("response exceeds %d bytes", maxBody)
	}
	return data, nil
}

func parseSuggest2(src Source, body []byte, limit int) ([]Suggestion, error) {
	var res struct {
		Hits []struct {
			URI          string `json:"uri"`
			ALabel       string `json:"aLabel"`
			SuggestLabel string `json:"suggestLabel"`
			More         struct {
				VariantLabels []string `json:"variantLabels"`
			} `json:"more"`
		} `json:"hits"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("vocabsrc: suggest2 decode: %w", err)
	}
	out := make([]Suggestion, 0, len(res.Hits))
	for _, hit := range res.Hits {
		label := hit.ALabel
		if label == "" {
			label = hit.SuggestLabel
		}
		if hit.URI == "" || label == "" {
			continue
		}
		out = append(out, Suggestion{
			Source: src.Name, Scheme: src.Scheme, ID: hit.URI, Label: label,
			Variants: hit.More.VariantLabels,
		})
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func parseWikidata(src Source, body []byte, limit int) ([]Suggestion, error) {
	var res struct {
		Search []struct {
			ID          string `json:"id"`
			Label       string `json:"label"`
			Description string `json:"description"`
			ConceptURI  string `json:"concepturi"`
		} `json:"search"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("vocabsrc: wikidata decode: %w", err)
	}
	out := make([]Suggestion, 0, len(res.Search))
	for _, hit := range res.Search {
		uri := hit.ConceptURI
		if uri == "" && hit.ID != "" {
			uri = "http://www.wikidata.org/entity/" + hit.ID
		}
		if uri == "" || hit.Label == "" {
			continue
		}
		out = append(out, Suggestion{
			Source: src.Name, Scheme: src.Scheme,
			ID: uri, Label: hit.Label, Description: hit.Description,
		})
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

// parseVIAF reads AutoSuggest hits; per-source cluster ids become exactMatch
// URIs (LCNAF, GND, Wikidata).
func parseVIAF(src Source, body []byte, limit int) ([]Suggestion, error) {
	var res struct {
		Result []struct {
			Term        string `json:"term"`
			DisplayForm string `json:"displayForm"`
			NameType    string `json:"nametype"`
			VIAFID      string `json:"viafid"`
			LC          string `json:"lc"`
			DNB         string `json:"dnb"`
			WKP         string `json:"wkp"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("vocabsrc: viaf decode: %w", err)
	}
	out := make([]Suggestion, 0, len(res.Result))
	for _, hit := range res.Result {
		label := hit.DisplayForm
		if label == "" {
			label = hit.Term
		}
		if hit.VIAFID == "" || label == "" {
			continue
		}
		sugg := Suggestion{
			Source: src.Name, Scheme: src.Scheme,
			ID: "http://viaf.org/viaf/" + hit.VIAFID, Label: label,
			Description: hit.NameType,
		}
		if id := compactID(hit.LC); id != "" {
			sugg.ExactMatch = append(sugg.ExactMatch, "http://id.loc.gov/authorities/names/"+id)
		}
		if id := compactID(hit.DNB); id != "" {
			sugg.ExactMatch = append(sugg.ExactMatch, "https://d-nb.info/gnd/"+id)
		}
		if id := compactID(hit.WKP); id != "" {
			sugg.ExactMatch = append(sugg.ExactMatch, "http://www.wikidata.org/entity/"+id)
		}
		out = append(out, sugg)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

// compactID strips the embedded spaces VIAF leaves in source ids ("n  79021164").
func compactID(id string) string {
	return strings.ReplaceAll(id, " ", "")
}
