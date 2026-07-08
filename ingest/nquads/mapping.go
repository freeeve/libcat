package nquads

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// Mapping declares how a deployment's N-Quads export maps onto ingest records
// -- the whole provider config, written as a TOML file so adopting a new
// dcterms-shaped export means editing a mapping, not writing Go (tasks/172).
// Only subjects under WorkPrefix are read as work records; skos prefLabels on
// any other subject are harvested as authority labels for the work's
// controlled subjects.
type Mapping struct {
	// WorkPrefix is the IRI prefix of work subjects; the remainder is the
	// export's work id.
	WorkPrefix string `toml:"work-prefix"`
	// IDScheme namespaces the durable provider id ("<scheme>:<workid>")
	// minted for each work. Defaults to the feed name; keep it stable across
	// exports or every work re-mints.
	IDScheme string `toml:"id-scheme"`
	// Class is the BIBFRAME work class (default "Text").
	Class string `toml:"class"`
	// DefaultLanguage is the ISO 639-2/B code used when the export carries no
	// (mappable) language (default "eng").
	DefaultLanguage string `toml:"default-language"`
	// IDOrder orders records for deterministic ingest: "lexical" (default) or
	// "numeric" (decimal ids without padding, shorter ids first).
	IDOrder string `toml:"id-order"`
	// Predicates maps record fields to the predicate IRIs that carry them.
	// Fields: title, creator, identifier, subject, source, language,
	// prefLabel. A field may list several IRIs.
	Predicates map[string]StringList `toml:"predicates"`
	// Identifiers maps object URN prefixes to identifier schemes; scheme
	// "isbn" clusters cross-feed, any other scheme becomes a durable
	// source-tagged id key ("<scheme>:<value>").
	Identifiers map[string]string `toml:"identifiers"`
	// Languages maps the export's language codes to ISO 639-2/B. An unmapped
	// three-letter code passes through; anything else falls back to
	// DefaultLanguage.
	Languages map[string]string `toml:"languages"`
	// Sources describes provenance attestation objects.
	Sources SourcesMapping `toml:"sources"`
}

// SourcesMapping describes the source-attestation objects a "source" field
// predicate points at.
type SourcesMapping struct {
	// Prefix is stripped from the source IRI to form the source slug.
	Prefix string `toml:"prefix"`
	// ExtraKey is the work extra the joined slugs land under (default
	// "sources" -- the key the public-provenance allowlist governs).
	ExtraKey string `toml:"extra-key"`
	// Tentative lists source IRIs that do not confer confidence: a work
	// attested only by these is marked with the "tentative" extra and can be
	// dropped wholesale via Params["tentative"]="drop".
	Tentative []string `toml:"tentative"`
}

// StringList decodes from either a single TOML string or an array of strings,
// so the common one-IRI field stays one line.
type StringList []string

// UnmarshalTOML implements toml.Unmarshaler.
func (s *StringList) UnmarshalTOML(v any) error {
	switch val := v.(type) {
	case string:
		*s = StringList{val}
	case []any:
		for _, e := range val {
			str, ok := e.(string)
			if !ok {
				return fmt.Errorf("nquads mapping: predicate list holds a non-string %v", e)
			}
			*s = append(*s, str)
		}
	default:
		return fmt.Errorf("nquads mapping: predicate value %v is neither string nor list", v)
	}
	return nil
}

// mappedFields are the record fields Predicates may target.
var mappedFields = map[string]bool{
	"title": true, "creator": true, "identifier": true, "subject": true,
	"source": true, "language": true, "prefLabel": true,
}

// LoadMapping reads and validates a mapping TOML file.
func LoadMapping(path string) (*Mapping, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("nquads mapping: %w", err)
	}
	var m Mapping
	if err := toml.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("nquads mapping %s: %w", path, err)
	}
	if err := m.validate(); err != nil {
		return nil, fmt.Errorf("nquads mapping %s: %w", path, err)
	}
	m.applyDefaults()
	return &m, nil
}

func (m *Mapping) validate() error {
	if m.WorkPrefix == "" {
		return fmt.Errorf("work-prefix is required")
	}
	if len(m.Predicates) == 0 {
		return fmt.Errorf("at least one [predicates] entry is required")
	}
	for field := range m.Predicates {
		if !mappedFields[field] {
			return fmt.Errorf("unknown predicate field %q (want title, creator, identifier, subject, source, language, prefLabel)", field)
		}
	}
	switch m.IDOrder {
	case "", "lexical", "numeric":
	default:
		return fmt.Errorf("id-order %q (want lexical or numeric)", m.IDOrder)
	}
	return nil
}

func (m *Mapping) applyDefaults() {
	if m.Class == "" {
		m.Class = "Text"
	}
	if m.DefaultLanguage == "" {
		m.DefaultLanguage = "eng"
	}
	if m.IDOrder == "" {
		m.IDOrder = "lexical"
	}
	if m.Sources.ExtraKey == "" {
		m.Sources.ExtraKey = "sources"
	}
}

// fieldFor inverts Predicates into the predicate-IRI -> field lookup the scan
// loop uses.
func (m *Mapping) fieldFor() map[string]string {
	out := map[string]string{}
	for field, iris := range m.Predicates {
		for _, iri := range iris {
			out[iri] = field
		}
	}
	return out
}

// language maps an export language code to ISO 639-2/B per the mapping table.
func (m *Mapping) language(code string) string {
	if v, ok := m.Languages[code]; ok {
		return v
	}
	if len(code) == 3 {
		return code
	}
	return m.DefaultLanguage
}
