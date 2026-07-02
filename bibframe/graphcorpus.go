package bibframe

import (
	"fmt"
	"sort"

	"github.com/freeeve/libcatalog/storage"
	codexbf "github.com/freeeve/libcodex/bibframe"
	"github.com/freeeve/libcodex/rdf"
)

// WorkGroup is one clustered Work ready to serialize: its minted id, its shared
// Work-level BIBFRAME, and the Instances (each with its own minted id) that
// realize it. It is the direct-BIBFRAME, two-tier-identity unit a native
// provider produces after resolution (ARCHITECTURE §4).
type WorkGroup struct {
	WorkID    string
	Work      codexbf.Work
	Instances []GroupInstance
}

// GroupInstance is one Instance of a WorkGroup: its minted id and Instance-level
// BIBFRAME.
type GroupInstance struct {
	InstanceID string
	Instance   codexbf.Instance
}

// GrainFromGraph canonicalizes one BIBFRAME graph into its N-Quads grain, every
// statement tagged with the given provenance graph and RDFC-1.0 canonicalized so
// an unchanged input re-serializes to identical bytes.
func GrainFromGraph(g *rdf.Graph, graph rdf.Term) ([]byte, error) {
	ds, err := rdf.ParseNQuads(g.NQuads(graph))
	if err != nil {
		return nil, fmt.Errorf("parse n-quads: %w", err)
	}
	return ds.Canonical()
}

// BuildWorks writes one canonical N-Quads grain per Work into sink (at
// GrainPath(WorkID)) in the provider's feed graph, plus a bulk catalog.nq. Each
// grain carries the shared Work and its Instances via libcodex's WorkInstances,
// so a clustered Work (multiple editions/formats) is one per-Work file with
// minted, provider-independent ids at both tiers. It reports the number of Works
// (grains) and Instances written.
func BuildWorks(sink storage.Sink, works []WorkGroup, provider string) (BuildStats, error) {
	feed := FeedGraph(provider)
	stats := BuildStats{}

	type built struct {
		id string
		g  *rdf.Graph
	}
	graphs := make([]built, 0, len(works))
	for _, wg := range works {
		wi := codexbf.WorkInstances{Work: wg.Work}
		bases := make([]string, len(wg.Instances))
		for i, gi := range wg.Instances {
			wi.Instances = append(wi.Instances, gi.Instance)
			bases[i] = gi.InstanceID
		}
		g := wi.Graph(wg.WorkID, bases)
		grain, err := GrainFromGraph(g, feed)
		if err != nil {
			return stats, fmt.Errorf("grain %s: %w", wg.WorkID, err)
		}
		if err := writeSink(sink, GrainPath(wg.WorkID), grain); err != nil {
			return stats, err
		}
		stats.Grains++
		stats.Records += len(wg.Instances)
		graphs = append(graphs, built{wg.WorkID, g})
	}

	sort.Slice(graphs, func(i, j int) bool { return graphs[i].id < graphs[j].id })
	w, err := sink.Create("catalog.nq")
	if err != nil {
		return stats, fmt.Errorf("create catalog.nq: %w", err)
	}
	// One shared encoder across the corpus keeps blank-node labels unique, so the
	// bulk file is a valid merge of the grains rather than a collision-prone
	// concatenation (ARCHITECTURE §3).
	var enc rdf.Encoder
	for _, b := range graphs {
		if _, err := w.Write(enc.AppendNQuads(nil, b.g, feed)); err != nil {
			w.Close()
			return stats, fmt.Errorf("write catalog.nq: %w", err)
		}
	}
	if err := w.Close(); err != nil {
		return stats, fmt.Errorf("close catalog.nq: %w", err)
	}
	return stats, nil
}
