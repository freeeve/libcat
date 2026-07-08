package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/freeeve/libcat/project"
	codex "github.com/freeeve/libcodex"
	"github.com/freeeve/libcodex/iso2709"
)

// writeMARCFixture encodes two minimal monograph records to a .mrc file.
func writeMARCFixture(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "in.mrc")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	w := iso2709.NewWriter(f)
	for i, title := range []string{"First Book", "Second Book"} {
		r := codex.NewRecord()
		r.SetLeader(codex.Leader([]byte("00000nam a2200000 a 4500")))
		r.AddField(codex.NewControlField("001", fmt.Sprintf("c%d", i+1)))
		r.AddField(codex.NewDataField("020", ' ', ' ', codex.NewSubfield('a', fmt.Sprintf("978000000001%d", i+1))))
		r.AddField(codex.NewDataField("100", '1', ' ', codex.NewSubfield('a', "Author, Ada")))
		r.AddField(codex.NewDataField("245", '1', '0', codex.NewSubfield('a', title)))
		if err := w.Write(r); err != nil {
			t.Fatal(err)
		}
	}
	f.Close()
	return path
}

// TestBuildPipeline runs the whole config-driven pipeline (ingest ->
// serialize -> project -> export -> index) over a MARC fixture and checks
// each step's artifact landed.
func TestBuildPipeline(t *testing.T) {
	dir := t.TempDir()
	mrc := writeMARCFixture(t, dir)
	cfgPath := filepath.Join(dir, "lcat.toml")
	cfg := fmt.Sprintf(`out = %q

[[source]]
provider = "marc"
source = %q

[project]
out = %q

[export]
out = %q

[index]
out = %q
`, filepath.Join(dir, "out"), mrc, filepath.Join(dir, "assets"),
		filepath.Join(dir, "downloads"), filepath.Join(dir, "search"))
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runBuild([]string{"--config", cfgPath}); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(filepath.Join(dir, "assets", "catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cat project.Catalog
	if err := json.Unmarshal(b, &cat); err != nil {
		t.Fatal(err)
	}
	if len(cat.Works) != 2 {
		t.Fatalf("projected works = %d, want 2", len(cat.Works))
	}
	for _, name := range []string{"catalog.nq.gz", "catalog.mrc.gz", "catalog.xml.gz", "downloads.json"} {
		if _, err := os.Stat(filepath.Join(dir, "downloads", name)); err != nil {
			t.Fatalf("export artifact %s: %v", name, err)
		}
	}
	entries, err := os.ReadDir(filepath.Join(dir, "search"))
	if err != nil || len(entries) == 0 {
		t.Fatalf("index artifacts: %v (%d entries)", err, len(entries))
	}

	// --only narrows the run: re-projecting alone must succeed and touch
	// nothing else.
	if err := os.RemoveAll(filepath.Join(dir, "downloads")); err != nil {
		t.Fatal(err)
	}
	if err := runBuild([]string{"--config", cfgPath, "--only", "project"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "downloads")); !os.IsNotExist(err) {
		t.Fatalf("--only project ran the export step")
	}
}

// TestBuildConfigValidation checks the loader rejects incomplete configs.
func TestBuildConfigValidation(t *testing.T) {
	dir := t.TempDir()
	bad := map[string]string{
		"missing out": `[[source]]
provider = "marc"`,
		"source without provider": `out = "x"
[[source]]
source = "y"`,
		"project without out": `out = "x"
[project]
providers = ["marc"]`,
		"hugo without dir": `out = "x"
[hugo]
command = ["hugo"]`,
	}
	for name, body := range bad {
		path := filepath.Join(dir, name+".toml")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := loadBuildConfig(path); err == nil {
			t.Errorf("%s: config accepted, want error", name)
		}
	}
}

// TestBuildFeedsDefault checks projection defaults to each source's feed in
// config order, deduped, feed overrides honored.
func TestBuildFeedsDefault(t *testing.T) {
	cfg := &buildConfig{Sources: []buildSource{
		{Provider: "coll"},
		{Provider: "nquads", Feed: "collnq"},
		{Provider: "csv", Feed: "collnq"},
	}}
	got := cfg.feeds()
	if len(got) != 2 || got[0] != "coll" || got[1] != "collnq" {
		t.Fatalf("feeds = %v", got)
	}
}
