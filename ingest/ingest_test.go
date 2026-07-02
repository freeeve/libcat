package ingest_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/freeeve/libcatalog/identity"
	"github.com/freeeve/libcatalog/ingest"
	"github.com/freeeve/libcatalog/ingest/overdrive"
	codexbf "github.com/freeeve/libcodex/bibframe"
)

// stubRecord is a minimal ingest.Record: enough identity keys to resolve and enough
// BIBFRAME to serialize. It stands in for a deployment-authored provider's record,
// proving the pipeline is provider-agnostic.
type stubRecord struct {
	id, author, title, lang, isbn string
}

func (r stubRecord) Identity() identity.Record {
	rec := identity.Record{Author: r.author, Title: r.title, Lang: r.lang}
	rec.ProviderKeys = append(rec.ProviderKeys, identity.ProviderKey(identity.SchemeID, r.id))
	if r.isbn != "" {
		rec.ProviderKeys = append(rec.ProviderKeys, identity.ProviderKey(identity.SchemeISBN, r.isbn))
	}
	return rec
}

func (r stubRecord) Work() codexbf.Work {
	return codexbf.Work{
		Class:  "Text",
		Titles: []codexbf.Title{{MainTitle: r.title}},
		Contributions: []codexbf.Contribution{
			{Primary: true, Class: "Person", Label: r.author, Role: "author"},
		},
		Languages: []string{r.lang},
	}
}

func (r stubRecord) Instance() codexbf.Instance {
	inst := codexbf.Instance{Titles: []codexbf.Title{{MainTitle: r.title}}}
	if r.isbn != "" {
		inst.Identifiers = []codexbf.Identifier{{Class: "Isbn", Value: r.isbn}}
	}
	return inst
}

// stubProvider is a deployment-authored provider built from records held in memory.
type stubProvider struct {
	feed string
	role ingest.Role
	recs []ingest.Record
}

func (p stubProvider) Name() string                                     { return p.feed }
func (p stubProvider) Role() ingest.Role                                { return p.role }
func (p stubProvider) Records(context.Context) ([]ingest.Record, error) { return p.recs, nil }

func stubFactory(recs []ingest.Record) ingest.Factory {
	return func(cfg ingest.Config) (ingest.Provider, error) {
		return stubProvider{feed: cfg.Feed, role: ingest.RoleIngest, recs: recs}, nil
	}
}

// TestRegistryComposition covers registration: a first-party factory (OverDrive) and
// a custom stub coexist, keys are sorted, duplicates and unknowns error, and the
// registry key defaults the provenance feed.
func TestRegistryComposition(t *testing.T) {
	reg := ingest.NewRegistry()
	if err := reg.Register(overdrive.ProviderName, overdrive.New); err != nil {
		t.Fatalf("register overdrive: %v", err)
	}
	if err := reg.Register("acme", stubFactory(nil)); err != nil {
		t.Fatalf("register acme: %v", err)
	}

	if got, want := strings.Join(reg.Names(), ","), "acme,overdrive"; got != want {
		t.Errorf("Names() = %q, want %q", got, want)
	}
	if err := reg.Register("acme", stubFactory(nil)); err == nil {
		t.Error("duplicate Register(acme) should error")
	}
	if err := reg.Register("", stubFactory(nil)); err == nil {
		t.Error("Register with empty name should error")
	}
	if err := reg.Register("nilfac", nil); err == nil {
		t.Error("Register with nil factory should error")
	}
	if _, err := reg.New("nope", ingest.Config{}); err == nil {
		t.Error("New for unknown provider should error")
	}

	// An empty Config.Feed defaults the provenance graph to the registry key.
	prov, err := reg.New("acme", ingest.Config{})
	if err != nil {
		t.Fatalf("New(acme): %v", err)
	}
	if prov.Name() != "acme" {
		t.Errorf("default feed = %q, want acme", prov.Name())
	}
	// An explicit Config.Feed overrides it.
	prov, err = reg.New("acme", ingest.Config{Feed: "acme-mirror"})
	if err != nil {
		t.Fatalf("New(acme, feed override): %v", err)
	}
	if prov.Name() != "acme-mirror" {
		t.Errorf("overridden feed = %q, want acme-mirror", prov.Name())
	}
}

// TestRunGraphRouting proves the shared pipeline tags a provider's statements with
// its own feed:<name> graph and never another's -- the provenance contract that
// lets providers coexist (ARCHITECTURE §5/§9).
func TestRunGraphRouting(t *testing.T) {
	recs := []ingest.Record{
		stubRecord{id: "a1", author: "Doe, Jane", title: "Alpha", lang: "eng", isbn: "9780000000001"},
		stubRecord{id: "a2", author: "Roe, Rick", title: "Beta", lang: "eng", isbn: "9780000000002"},
	}
	reg := ingest.NewRegistry()
	if err := reg.Register("acme", stubFactory(recs)); err != nil {
		t.Fatal(err)
	}

	out := t.TempDir()
	prov, err := reg.New("acme", ingest.Config{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := ingest.Run(prov, out)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Stats.Grains != 2 {
		t.Errorf("grains = %d, want 2", res.Stats.Grains)
	}
	if res.MintedWorks != 2 || res.MintedInstances != 2 {
		t.Errorf("minted works/instances = %d/%d, want 2/2", res.MintedWorks, res.MintedInstances)
	}

	nq := readNQuads(t, out)
	if !strings.Contains(nq, "<feed:acme>") {
		t.Errorf("grains missing feed:acme graph:\n%s", nq)
	}
	if strings.Contains(nq, "<feed:overdrive>") {
		t.Errorf("grains leaked a foreign feed graph:\n%s", nq)
	}
	if !strings.Contains(nq, "Alpha") || !strings.Contains(nq, "Beta") {
		t.Errorf("grains missing expected titles:\n%s", nq)
	}
}

// TestRunReingestStable proves the pipeline is derive-from-grains: a second run over
// the same records seeds ids from the committed grains, mints nothing, and rewrites
// byte-identical grains (the tasks/002 no-churn gate, now exercised generically).
func TestRunReingestStable(t *testing.T) {
	recs := []ingest.Record{
		stubRecord{id: "a1", author: "Doe, Jane", title: "Alpha", lang: "eng", isbn: "9780000000001"},
	}
	prov := stubProvider{feed: "acme", role: ingest.RoleIngest, recs: recs}
	out := t.TempDir()

	first, err := ingest.Run(prov, out)
	if err != nil {
		t.Fatalf("first Run: %v", err)
	}
	if first.MintedWorks == 0 {
		t.Fatal("first run minted no works")
	}
	before := readNQuads(t, out)

	second, err := ingest.Run(prov, out)
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if second.MintedWorks != 0 || second.MintedInstances != 0 {
		t.Errorf("re-ingest minted %d works, %d instances; want 0/0",
			second.MintedWorks, second.MintedInstances)
	}
	if after := readNQuads(t, out); after != before {
		t.Errorf("re-ingest changed grains:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// TestRunRejectsEnrichRole confirms Run executes only ingest-role providers today;
// an enrichment provider is refused rather than silently ingested.
func TestRunRejectsEnrichRole(t *testing.T) {
	prov := stubProvider{feed: "authority", role: ingest.RoleEnrich}
	if _, err := ingest.Run(prov, t.TempDir()); err == nil {
		t.Error("Run should reject a non-ingest provider")
	}
}

// TestOverdriveProviderThroughRegistry runs the real first-party provider end-to-end
// through the registry over a minimal page cache, proving the built-in factory plugs
// into the same pipeline and routes to feed:overdrive.
func TestOverdriveProviderThroughRegistry(t *testing.T) {
	cache := t.TempDir()
	page := `{"items":[{"id":"12345","title":"Registry Test","creators":[{"name":"Doe, Jane","role":"Author","sortName":"Doe, Jane"}],"languages":[{"id":"en","name":"English"}],"formats":[{"identifiers":[{"type":"ISBN","value":"9780000000009"}]}]}]}`
	if err := os.WriteFile(filepath.Join(cache, "page-0001.json"), []byte(page), 0o644); err != nil {
		t.Fatal(err)
	}

	reg := ingest.NewRegistry()
	if err := reg.Register(overdrive.ProviderName, overdrive.New); err != nil {
		t.Fatal(err)
	}
	prov, err := reg.New(overdrive.ProviderName, ingest.Config{Source: cache})
	if err != nil {
		t.Fatalf("New(overdrive): %v", err)
	}
	out := t.TempDir()
	res, err := ingest.Run(prov, out)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Stats.Grains != 1 {
		t.Errorf("grains = %d, want 1", res.Stats.Grains)
	}
	nq := readNQuads(t, out)
	if !strings.Contains(nq, "<feed:overdrive>") {
		t.Errorf("overdrive grains missing feed:overdrive graph:\n%s", nq)
	}
	if !strings.Contains(nq, "Registry Test") {
		t.Errorf("overdrive grains missing title:\n%s", nq)
	}
}

// readNQuads returns the concatenated contents of every per-Work grain under dir
// (skipping the bulk catalog.nq), so a test can assert on provenance graphs.
func readNQuads(t *testing.T, dir string) string {
	t.Helper()
	var b strings.Builder
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".nq") || d.Name() == "catalog.nq" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		b.Write(data)
		return nil
	})
	if err != nil {
		t.Fatalf("read grains: %v", err)
	}
	return b.String()
}
