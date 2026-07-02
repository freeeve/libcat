// Package project turns the canonical BIBFRAME graph into the catalog's derived
// data: the JSON a static site (the Hugo module's content adapter) and the search
// index consume (ARCHITECTURE §7). It is a read-only projection -- the graph
// stays the source of truth -- and it flattens each clustered Work, with its
// Instances and the union of its feed and editorial statements, into one record.
package project

import (
	"sort"

	"github.com/freeeve/libcatalog/bibframe"
	"github.com/freeeve/libcodex/rdf"
)

// BIBFRAME / RDF vocabulary the projection reads. These mirror libcodex's stable
// output IRIs; kept local so the projector depends only on the rdf toolkit.
const (
	bfNS          = "http://id.loc.gov/ontologies/bibframe/"
	bflcNS        = "http://id.loc.gov/ontologies/bflc/"
	rdfNS         = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
	rdfsNS        = "http://www.w3.org/2000/01/rdf-schema#"
	skosNS        = "http://www.w3.org/2004/02/skos/core#"
	classWork     = bfNS + "Work"
	pTitle        = bfNS + "title"
	pMainTitle    = bfNS + "mainTitle"
	pSubtitle     = bfNS + "subtitle"
	pContribution = bfNS + "contribution"
	pAgent        = bfNS + "agent"
	pRole         = bfNS + "role"
	pSubject      = bfNS + "subject"
	pLanguage     = bfNS + "language"
	pClassif      = bfNS + "classification"
	pClassPortion = bfNS + "classificationPortion"
	pHasInstance  = bfNS + "hasInstance"
	pIdentifiedBy = bfNS + "identifiedBy"
	pSource       = bfNS + "source"
	classIsbn     = bfNS + "Isbn"
	pLabel        = rdfsNS + "label"
	pPrefLabel    = skosNS + "prefLabel"
	pValue        = rdfNS + "value"
	primaryContr  = bflcNS + "PrimaryContribution"
)

// SchemaVersion is the catalog.json / facets.json / redirects.json schema version.
// The Hugo module and search-index builder read it to detect a projector/consumer
// mismatch. v2 added the per-Instance identifier scheme (ProviderID.Source) for the
// availability adapter (tasks/004/008). v3 split controlled subjects (authority
// URIs + resolved labels) from uncontrolled feed tags (tasks/012).
const SchemaVersion = 3

// Catalog is the projected corpus: one record per Work, sorted by id.
type Catalog struct {
	Version int    `json:"version"`
	Works   []Work `json:"works"`
}

// Work is the discovery unit as the static site sees it -- the display and facet
// fields of a bf:Work plus its Instances (the borrowable editions/formats).
type Work struct {
	ID              string        `json:"id"`
	Title           string        `json:"title"`
	Subtitle        string        `json:"subtitle,omitempty"`
	Contributors    []Contributor `json:"contributors,omitempty"`
	Subjects        []Subject     `json:"subjects,omitempty"`
	Tags            []string      `json:"tags,omitempty"`
	Languages       []string      `json:"languages,omitempty"`
	Classifications []string      `json:"classifications,omitempty"`
	Instances       []Instance    `json:"instances,omitempty"`
}

// Contributor is an agent's display name and role.
type Contributor struct {
	Name string `json:"name"`
	Role string `json:"role,omitempty"`
}

// Subject is a controlled-vocabulary subject: a stable authority URI plus the
// human labels resolved from the authority's skos:prefLabel / rdfs:label statements
// in the graph, keyed by language tag (e.g. "en", "es"; "" for an untagged label).
// Links and facets key on ID; display uses Labels, falling back to ID when the
// authority provides none (tasks/012). Distinct from an uncontrolled feed Tag.
type Subject struct {
	ID     string            `json:"id"`
	Labels map[string]string `json:"labels,omitempty"`
}

// Instance is one edition/format: its id, ISBNs, and the scheme-tagged provider
// ids the runtime availability adapter keys on.
type Instance struct {
	ID          string       `json:"id"`
	ISBNs       []string     `json:"isbns,omitempty"`
	ProviderIDs []ProviderID `json:"providerIds,omitempty"`
}

// ProviderID is one non-ISBN identifier with its bf:source scheme, so a client-side
// availability adapter selects its key by scheme (e.g. OverDrive's "overdrive-reserve"
// Reserve ID vs the "overdrive" title id) rather than guessing from a flat list
// (ARCHITECTURE §9, tasks/004). Source is empty for an untagged identifier.
type ProviderID struct {
	Source string `json:"source,omitempty"`
	Value  string `json:"value"`
}

// Facets is the precomputed facet index: for each facetable dimension, the
// distinct values and how many Works carry each. Emitting it saves the static
// site from aggregating the whole corpus in templates at build time.
type Facets struct {
	Version         int            `json:"version"`
	Languages       []FacetValue   `json:"languages,omitempty"`
	Subjects        []SubjectFacet `json:"subjects,omitempty"`
	Tags            []FacetValue   `json:"tags,omitempty"`
	Contributors    []FacetValue   `json:"contributors,omitempty"`
	Classifications []FacetValue   `json:"classifications,omitempty"`
}

// FacetValue is one facet value and the number of Works that carry it.
type FacetValue struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

// SubjectFacet is one controlled-subject facet value: the authority URI (the key),
// its resolved labels, and the number of Works carrying it. Facets key on ID so a
// relabel does not churn the facet; display uses Labels (tasks/012).
type SubjectFacet struct {
	ID     string            `json:"id"`
	Labels map[string]string `json:"labels,omitempty"`
	Count  int               `json:"count"`
}

// Facets aggregates the catalog into per-dimension value counts, each value
// counted once per Work. Values are ordered by descending count then value, so
// the output is deterministic.
func (c *Catalog) Facets() Facets {
	lang, tag, contrib, cls := map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}
	subj := map[string]*SubjectFacet{}
	for _, w := range c.Works {
		countDistinct(lang, w.Languages)
		countDistinct(tag, w.Tags)
		countDistinct(cls, w.Classifications)
		names := make([]string, len(w.Contributors))
		for i, con := range w.Contributors {
			names[i] = con.Name
		}
		countDistinct(contrib, names)
		seen := map[string]bool{}
		for _, s := range w.Subjects {
			if s.ID == "" || seen[s.ID] {
				continue
			}
			seen[s.ID] = true
			sf := subj[s.ID]
			if sf == nil {
				sf = &SubjectFacet{ID: s.ID, Labels: s.Labels}
				subj[s.ID] = sf
			}
			sf.Count++
		}
	}
	return Facets{
		Version:         SchemaVersion,
		Languages:       facetValues(lang),
		Subjects:        subjectFacets(subj),
		Tags:            facetValues(tag),
		Contributors:    facetValues(contrib),
		Classifications: facetValues(cls),
	}
}

// subjectFacets turns the URI->SubjectFacet map into a slice ordered by descending
// count, then id, so the output is deterministic.
func subjectFacets(m map[string]*SubjectFacet) []SubjectFacet {
	if len(m) == 0 {
		return nil
	}
	out := make([]SubjectFacet, 0, len(m))
	for _, sf := range m {
		out = append(out, *sf)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// countDistinct increments m once for each distinct non-empty value in vals.
func countDistinct(m map[string]int, vals []string) {
	seen := map[string]bool{}
	for _, v := range vals {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		m[v]++
	}
}

// facetValues turns a value->count map into a slice ordered by descending count,
// then ascending value.
func facetValues(m map[string]int) []FacetValue {
	if len(m) == 0 {
		return nil
	}
	out := make([]FacetValue, 0, len(m))
	for v, n := range m {
		out = append(out, FacetValue{Value: v, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Value < out[j].Value
	})
	return out
}

// Redirect is one retired-Work -> surviving-Work URL redirect (ARCHITECTURE §4):
// after an editorial merge the retired id must still resolve, so shared links and
// SEO survive.
type Redirect struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// RedirectMap is the redirect artifact emitted alongside catalog.json: every
// retired Work id and the surviving id it now resolves to, chains collapsed to the
// final survivor and sorted by retired id. The static host turns each into a 301
// (per the tasks/001 decision: the projector emits the map, the host serves it).
type RedirectMap struct {
	Version   int        `json:"version"`
	Redirects []Redirect `json:"redirects"`
}

// Redirects builds the redirect map from a catalog.nq dataset by reading the
// editorial graph's lcat:mergedInto statements and collapsing merge chains
// (A->B->C yields A->C and B->C) to the final survivor. A merge cycle terminates
// at the last id reached rather than looping.
func Redirects(catalogNQ []byte) (RedirectMap, error) {
	ds, err := rdf.ParseNQuads(catalogNQ)
	if err != nil {
		return RedirectMap{}, err
	}
	ed := bibframe.EditorialGraph()
	raw := map[string]string{}
	for _, q := range ds.Quads {
		if q.G == ed && q.P.Value == bibframe.PredMergedInto && q.S.IsIRI() && q.O.IsIRI() {
			raw[fragID(q.S.Value, "Work")] = fragID(q.O.Value, "Work")
		}
	}
	rm := RedirectMap{Version: SchemaVersion, Redirects: []Redirect{}}
	froms := make([]string, 0, len(raw))
	for from := range raw {
		froms = append(froms, from)
	}
	sort.Strings(froms)
	for _, from := range froms {
		if to := follow(raw, from); to != from {
			rm.Redirects = append(rm.Redirects, Redirect{From: from, To: to})
		}
	}
	return rm, nil
}

// follow chases the merge chain from start to the final survivor -- the last id
// with no onward mapping. It stops on a missing link or a cycle (returning the id
// reached), so a malformed overlay cannot loop.
func follow(raw map[string]string, start string) string {
	seen := map[string]bool{}
	cur := start
	for {
		to, ok := raw[cur]
		if !ok || to == "" || seen[cur] {
			return cur
		}
		seen[cur] = true
		cur = to
	}
}

// Project reads a catalog.nq dataset and projects each Work into a Catalog record.
// Display/facet fields are drawn from the union of the provider's feed graph and
// the editorial graph, so curated subjects appear alongside feed data.
func Project(catalogNQ []byte, provider string) (*Catalog, error) {
	ds, err := rdf.ParseNQuads(catalogNQ)
	if err != nil {
		return nil, err
	}
	p := &projector{
		feed:   ds.Graph(bibframe.FeedGraph(provider)),
		ed:     ds.Graph(bibframe.EditorialGraph()),
		labels: buildLabelIndex(ds),
	}
	cat := &Catalog{Version: SchemaVersion}
	if p.feed == nil {
		return cat, nil
	}
	for _, w := range p.feed.SubjectsOfType(classWork) {
		cat.Works = append(cat.Works, p.work(w))
	}
	sort.Slice(cat.Works, func(i, j int) bool { return cat.Works[i].ID < cat.Works[j].ID })
	return cat, nil
}

type projector struct {
	feed   *rdf.Graph
	ed     *rdf.Graph                   // editorial graph; nil when the corpus has no editorial statements
	labels map[string]map[string]string // authority URI -> language tag -> label
}

func (p *projector) work(w rdf.Term) Work {
	wk := Work{ID: fragID(w.Value, "Work")}
	if t, ok := p.feed.Object(w, pTitle); ok {
		wk.Title, _ = p.feed.Literal(t, pMainTitle)
		wk.Subtitle, _ = p.feed.Literal(t, pSubtitle)
	}
	wk.Contributors = p.contributors(w)
	wk.Subjects, wk.Tags = p.subjectsAndTags(w)
	wk.Languages = p.languages(w)
	wk.Classifications = p.classifications(w)
	wk.Instances = p.instances(w)
	return wk
}

// contributors returns the Work's agents, primary contributions first (as a MARC
// 1XX would lead), then the rest by name.
func (p *projector) contributors(w rdf.Term) []Contributor {
	type entry struct {
		c       Contributor
		primary bool
	}
	var es []entry
	for _, node := range p.feed.Objects(w, pContribution) {
		agent, ok := p.feed.Object(node, pAgent)
		if !ok {
			continue
		}
		name, _ := p.feed.Literal(agent, pLabel)
		if name == "" {
			continue
		}
		var role string
		if r, ok := p.feed.Object(node, pRole); ok {
			role, _ = p.feed.Literal(r, pLabel)
		}
		es = append(es, entry{Contributor{Name: name, Role: role}, p.feed.HasType(node, primaryContr)})
	}
	// Sort by (primary desc, name, role) -- a total order over the distinguishing
	// fields, so the projection is independent of contribution statement order: two
	// equivalent serializations of the same graph must yield identical catalog.json.
	sort.Slice(es, func(i, j int) bool {
		if es[i].primary != es[j].primary {
			return es[i].primary
		}
		if es[i].c.Name != es[j].c.Name {
			return es[i].c.Name < es[j].c.Name
		}
		return es[i].c.Role < es[j].c.Role
	})
	out := make([]Contributor, len(es))
	for i, e := range es {
		out[i] = e.c
	}
	return out
}

// subjectsAndTags splits a Work's bf:subject objects (across the feed and editorial
// graphs) into two dimensions (tasks/012). An IRI object is a controlled-vocabulary
// subject: its authority URI plus labels resolved from the graph (buildLabelIndex).
// A labeled blank node is an uncontrolled feed tag: its label string. Subjects are
// deduped by URI and sorted by URI; tags are deduped and sorted.
func (p *projector) subjectsAndTags(w rdf.Term) ([]Subject, []string) {
	subj := map[string]Subject{}
	tags := map[string]bool{}
	collect := func(g *rdf.Graph) {
		if g == nil {
			return
		}
		for _, s := range g.Objects(w, pSubject) {
			if s.IsIRI() {
				if _, ok := subj[s.Value]; !ok {
					subj[s.Value] = Subject{ID: s.Value, Labels: p.labels[s.Value]}
				}
			} else if label, ok := g.Literal(s, pLabel); ok && label != "" {
				tags[label] = true
			}
		}
	}
	collect(p.feed)
	collect(p.ed)

	ids := make([]string, 0, len(subj))
	for id := range subj {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	subjects := make([]Subject, len(ids))
	for i, id := range ids {
		subjects[i] = subj[id]
	}
	if len(subjects) == 0 {
		subjects = nil
	}
	return subjects, sortedKeys(tags)
}

// buildLabelIndex indexes the human labels of controlled-vocabulary terms across
// every graph in the dataset (tasks/012): for each IRI subject of skos:prefLabel or
// rdfs:label, it maps the term URI -> language tag -> label. prefLabel wins over
// rdfs:label for the same (URI, language). These labels come from authority
// statements (e.g. an authority:<vocab> graph merged into catalog.nq); the index is
// empty when no authority data is present, so subjects fall back to their URI.
func buildLabelIndex(ds *rdf.Dataset) map[string]map[string]string {
	idx := map[string]map[string]string{}
	put := func(uri, lang, label string, pref bool) {
		if label == "" {
			return
		}
		byLang := idx[uri]
		if byLang == nil {
			byLang = map[string]string{}
			idx[uri] = byLang
		}
		if _, ok := byLang[lang]; ok && !pref {
			return // keep prefLabel over rdfs:label
		}
		byLang[lang] = label
	}
	// Two passes so prefLabel always wins regardless of statement order.
	for _, q := range ds.Quads {
		if q.P.Value == pPrefLabel && q.S.IsIRI() && q.O.IsLiteral() {
			put(q.S.Value, q.O.Lang, q.O.Value, true)
		}
	}
	for _, q := range ds.Quads {
		if q.P.Value == pLabel && q.S.IsIRI() && q.O.IsLiteral() {
			put(q.S.Value, q.O.Lang, q.O.Value, false)
		}
	}
	if len(idx) == 0 {
		return nil
	}
	return idx
}

func (p *projector) languages(w rdf.Term) []string {
	set := map[string]bool{}
	for _, l := range p.feed.Objects(w, pLanguage) {
		if code := rdf.LocalName(l.Value); code != "" {
			set[code] = true
		}
	}
	return sortedKeys(set)
}

func (p *projector) classifications(w rdf.Term) []string {
	set := map[string]bool{}
	for _, c := range p.feed.Objects(w, pClassif) {
		if v, ok := p.feed.Literal(c, pClassPortion); ok && v != "" {
			set[v] = true
		}
	}
	return sortedKeys(set)
}

func (p *projector) instances(w rdf.Term) []Instance {
	var out []Instance
	for _, inst := range p.feed.Objects(w, pHasInstance) {
		i := Instance{ID: fragID(inst.Value, "Instance")}
		var isbns []string
		var pids []ProviderID
		for _, id := range p.feed.Objects(inst, pIdentifiedBy) {
			v, ok := p.feed.Literal(id, pValue)
			if !ok || v == "" {
				continue
			}
			if p.feed.HasType(id, classIsbn) {
				isbns = append(isbns, v)
				continue
			}
			pids = append(pids, ProviderID{Source: p.identifierSource(id), Value: v})
		}
		sort.Strings(isbns)
		sort.Slice(pids, func(a, b int) bool {
			if pids[a].Source != pids[b].Source {
				return pids[a].Source < pids[b].Source
			}
			return pids[a].Value < pids[b].Value
		})
		i.ISBNs, i.ProviderIDs = isbns, pids
		out = append(out, i)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].ID < out[b].ID })
	return out
}

// identifierSource returns the rdfs:label of an identifier node's bf:source scheme,
// or "" when the identifier carries no scheme.
func (p *projector) identifierSource(id rdf.Term) string {
	if src, ok := p.feed.Object(id, pSource); ok {
		if label, ok := p.feed.Literal(src, pLabel); ok {
			return label
		}
	}
	return ""
}

// fragID extracts an id from a node IRI of the form "#<id><suffix>".
func fragID(iri, suffix string) string {
	if len(iri) > 0 && iri[0] == '#' {
		iri = iri[1:]
	}
	if n := len(iri) - len(suffix); n >= 0 && iri[n:] == suffix {
		return iri[:n]
	}
	return iri
}

func sortedKeys(m map[string]bool) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
