package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/freeeve/libcatalog/project"
)

// runProject reads a catalog.nq dataset and writes the projected catalog.json --
// the derived data the Hugo module's content adapter and the search index consume
// (ARCHITECTURE §7). The graph stays the source of truth; this is a build artifact.
func runProject(args []string) error {
	fs := flag.NewFlagSet("project", flag.ExitOnError)
	catalogNQ := fs.String("catalog", "", "path to a catalog.nq dataset")
	out := fs.String("out", ".", "output directory for catalog.json")
	provider := fs.String("provider", "overdrive", "provenance graph feed:<provider> to project")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *catalogNQ == "" {
		return fmt.Errorf("--catalog is required")
	}

	b, err := os.ReadFile(*catalogNQ)
	if err != nil {
		return err
	}
	cat, err := project.Project(b, *provider)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(*out, "catalog.json"), cat); err != nil {
		return err
	}
	facets := cat.Facets()
	if err := writeJSON(filepath.Join(*out, "facets.json"), facets); err != nil {
		return err
	}
	fmt.Printf("projected %d works to %s (schema v%d); facets: %d languages, %d subjects, %d contributors\n",
		len(cat.Works), *out, project.SchemaVersion,
		len(facets.Languages), len(facets.Subjects), len(facets.Contributors))
	return nil
}

// writeJSON marshals v as indented JSON to path.
func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
