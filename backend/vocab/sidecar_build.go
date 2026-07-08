// Sidecar index builder (tasks/167): serializes one scheme's terms into
// range-servable roaringrange artifacts so the server never materializes a
// big vocabulary as Go maps. Layout under <prefix>sidecar/:
//
//	<scheme>.rrsr.bin/.idx  full Term JSON per doc (RRSR record store)
//	<scheme>.uri.rril       term URI -> doc, retired terms included
//	<scheme>.id1/2/3.rril   canon identifier tiers (own/exactMatch/closeMatch),
//	                        live terms only -- MatchIdentifier's precedence
//	<scheme>.search.bin     sorted (normLabel, doc, alt) entries (LCVS format)
//	<scheme>.manifest.json  source snapshot path+ETag; presence arms the scheme
//
// Doc ids are the scheme's term URIs in sorted order, so RRIL postings for
// one key surface the smallest URI first and output is deterministic.
package vocab

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sort"

	rr "github.com/freeeve/roaringrange"

	"github.com/freeeve/libcodex/rdf"

	"github.com/freeeve/libcat/storage/blob"
)

// SidecarManifest arms a scheme for sidecar serving: it names the source
// snapshot (and its ETag at build time) the artifacts were built from. A
// mismatched or missing source, or loose quads for the scheme elsewhere in
// the authorities tree, bypasses the sidecar for that snapshot build -- the
// map path remains the correctness backstop.
type SidecarManifest struct {
	Version    int    `json:"version"`
	Scheme     string `json:"scheme"`
	Source     string `json:"source"`
	SourceETag string `json:"sourceETag"`
	// SourceSchemes lists every authority scheme the source file carries --
	// the loader may skip parsing the file only when all of them are
	// sidecar-armed, so a shared source never silently drops a scheme.
	SourceSchemes []string `json:"sourceSchemes"`
	Terms         int      `json:"terms"`
	Live          int      `json:"live"`
}

const (
	sidecarVersion  = 1
	searchMagic     = "LCVS"
	sidecarDirPart  = "sidecar/"
	manifestSuffix  = ".manifest.json"
	identifierTiers = 3
)

func sidecarPath(prefix, scheme, suffix string) string {
	return prefix + sidecarDirPart + scheme + suffix
}

// BuildSidecar builds and stores the sidecar artifacts for scheme from the
// installed snapshot at source (usually <prefix>vocab/<scheme>.nq). It
// parses the snapshot with the same routing the map loader uses, so the two
// paths index identical terms.
func BuildSidecar(ctx context.Context, st blob.Store, prefix, scheme, source string) (*SidecarManifest, error) {
	data, etag, err := st.Get(ctx, source)
	if err != nil {
		return nil, fmt.Errorf("vocab: sidecar source %s: %w", source, err)
	}
	ds, err := rdf.ParseNQuads(data)
	if err != nil {
		return nil, fmt.Errorf("vocab: parse %s: %w", source, err)
	}
	tmp := &snapshot{schemes: map[string]map[string]*Term{}, search: map[string][]searchEntry{}}
	tmp.addDataset(ds, nil)
	tmp.finish()
	byURI := tmp.schemes[scheme]
	if len(byURI) == 0 {
		return nil, fmt.Errorf("vocab: %s carries no authority:%s terms", source, scheme)
	}
	sourceSchemes := make([]string, 0, len(tmp.schemes))
	for s := range tmp.schemes {
		sourceSchemes = append(sourceSchemes, s)
	}
	sort.Strings(sourceSchemes)
	return buildSidecarTerms(ctx, st, prefix, scheme, source, etag, sourceSchemes, byURI, tmp.search[scheme])
}

func buildSidecarTerms(ctx context.Context, st blob.Store, prefix, scheme, source, sourceETag string, sourceSchemes []string, byURI map[string]*Term, search []searchEntry) (*SidecarManifest, error) {
	uris := make([]string, 0, len(byURI))
	for uri := range byURI {
		uris = append(uris, uri)
	}
	sort.Strings(uris)
	doc := make(map[string]uint32, len(uris))
	for i, uri := range uris {
		doc[uri] = uint32(i)
	}

	// Records: full Term JSON in doc order.
	records := make([][]byte, len(uris))
	live := 0
	for i, uri := range uris {
		t := byURI[uri]
		if t.MergedInto == "" {
			live++
		}
		data, err := json.Marshal(t)
		if err != nil {
			return nil, fmt.Errorf("vocab: marshal %s: %w", uri, err)
		}
		records[i] = data
	}
	var bin, idx bytes.Buffer
	if err := rr.WriteRecords(&bin, &idx, records); err != nil {
		return nil, fmt.Errorf("vocab: write records: %w", err)
	}

	// URI lookup: every term, retired included (Lookup resolves them).
	uriEntries := make([]rr.LookupEntry, len(uris))
	for i, uri := range uris {
		uriEntries[i] = rr.LookupEntry{ID: uri, Doc: uint32(i)}
	}
	uriBuf := &bytes.Buffer{}
	if err := rr.WriteLookup(uriBuf, uriEntries); err != nil {
		return nil, fmt.Errorf("vocab: write uri lookup: %w", err)
	}

	// Identifier tiers, live terms only, canonicalized like buildMatch.
	tierIDs := func(t *Term, tier int) []string {
		switch tier {
		case 0:
			return []string{t.ID}
		case 1:
			return t.ExactMatch
		default:
			return t.CloseMatch
		}
	}
	tierBufs := make([]*bytes.Buffer, identifierTiers)
	for k := range identifierTiers {
		var entries []rr.LookupEntry
		for _, uri := range uris {
			t := byURI[uri]
			if t.MergedInto != "" {
				continue
			}
			for _, id := range tierIDs(t, k) {
				if key := canonIdentifier(id); key != "" {
					entries = append(entries, rr.LookupEntry{ID: key, Doc: doc[t.ID]})
				}
			}
		}
		tierBufs[k] = &bytes.Buffer{}
		if err := rr.WriteLookup(tierBufs[k], entries); err != nil {
			return nil, fmt.Errorf("vocab: write id tier %d: %w", k+1, err)
		}
	}

	searchBuf, err := encodeSearch(search, doc)
	if err != nil {
		return nil, err
	}

	puts := []struct {
		suffix string
		data   []byte
	}{
		{".rrsr.bin", bin.Bytes()},
		{".rrsr.idx", idx.Bytes()},
		{".uri.rril", uriBuf.Bytes()},
		{".id1.rril", tierBufs[0].Bytes()},
		{".id2.rril", tierBufs[1].Bytes()},
		{".id3.rril", tierBufs[2].Bytes()},
		{".search.bin", searchBuf},
	}
	for _, p := range puts {
		if _, err := st.Put(ctx, sidecarPath(prefix, scheme, p.suffix), p.data, blob.PutOptions{}); err != nil {
			return nil, fmt.Errorf("vocab: put sidecar %s: %w", p.suffix, err)
		}
	}
	m := &SidecarManifest{
		Version:       sidecarVersion,
		Scheme:        scheme,
		Source:        source,
		SourceETag:    sourceETag,
		SourceSchemes: sourceSchemes,
		Terms:         len(uris),
		Live:          live,
	}
	mdata, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	// The manifest lands last: its presence implies a complete artifact set.
	if _, err := st.Put(ctx, sidecarPath(prefix, scheme, manifestSuffix), mdata, blob.PutOptions{}); err != nil {
		return nil, fmt.Errorf("vocab: put sidecar manifest: %w", err)
	}
	return m, nil
}

// encodeSearch serializes sorted search entries as the LCVS blob: a shared
// norm-bytes arena plus parallel end-offset/doc/alt-bit columns, so the
// reader holds prefix search resident in a fraction of Go-string overhead.
func encodeSearch(entries []searchEntry, doc map[string]uint32) ([]byte, error) {
	var arena bytes.Buffer
	ends := make([]uint32, len(entries))
	docs := make([]uint32, len(entries))
	altBits := make([]byte, (len(entries)+7)/8)
	for i, e := range entries {
		arena.WriteString(e.norm)
		ends[i] = uint32(arena.Len())
		d, ok := doc[e.uri]
		if !ok {
			return nil, fmt.Errorf("vocab: search entry uri %s has no doc", e.uri)
		}
		docs[i] = d
		if e.alt {
			altBits[i/8] |= 1 << (i % 8)
		}
	}
	out := &bytes.Buffer{}
	out.WriteString(searchMagic)
	out.WriteByte(sidecarVersion)
	binary.Write(out, binary.LittleEndian, uint32(len(entries)))
	binary.Write(out, binary.LittleEndian, uint64(arena.Len()))
	out.Write(arena.Bytes())
	binary.Write(out, binary.LittleEndian, ends)
	binary.Write(out, binary.LittleEndian, docs)
	out.Write(altBits)
	return out.Bytes(), nil
}
