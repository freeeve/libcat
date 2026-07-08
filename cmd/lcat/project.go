package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/freeeve/libcat/project"
)

// runProject reads a catalog.nq dataset and writes the projected catalog.json --
// the derived data the Hugo module's content adapter and the search index consume
// (ARCHITECTURE §7). The graph stays the source of truth; this is a build artifact.
//
// --provider takes a comma-separated feed list (tasks/172): the projector views
// one feed graph at a time, so each feed projects separately and the catalogs
// merge by work id, first-listed feed winning a shared work. After a multi-feed
// ingest run `lcat serialize` first, since each ingest run rewrites catalog.nq
// with only its own run's works.
func runProject(args []string) error {
	fs := flag.NewFlagSet("project", flag.ExitOnError)
	catalogNQ := fs.String("catalog", "", "path to a catalog.nq dataset")
	out := fs.String("out", ".", "output directory for catalog.json")
	provider := fs.String("provider", "overdrive", "provenance graph feed(s) to project, comma-separated, first wins")
	publicSources := fs.String("public-sources", "",
		"comma-separated extra.sources names allowed on the public face; others are stripped (tasks/172). Empty (default) keeps everything.")
	schemeMap := fs.String("subject-scheme", "",
		"extra authority namespace -> scheme entries, comma-separated prefix=code pairs (prepended, so they override the built-in table; tasks/141)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *catalogNQ == "" {
		return fmt.Errorf("--catalog is required")
	}
	if *schemeMap != "" {
		var extra []project.SchemePrefix
		for pair := range strings.SplitSeq(*schemeMap, ",") {
			prefix, code, ok := strings.Cut(strings.TrimSpace(pair), "=")
			if !ok || prefix == "" || code == "" {
				return fmt.Errorf("bad --subject-scheme entry %q (want prefix=code)", pair)
			}
			extra = append(extra, project.SchemePrefix{Prefix: prefix, Scheme: code})
		}
		project.SubjectSchemePrefixes = append(extra, project.SubjectSchemePrefixes...)
	}

	b, err := os.ReadFile(*catalogNQ)
	if err != nil {
		return err
	}
	var cats []*project.Catalog
	for p := range strings.SplitSeq(*provider, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		c, err := project.Project(b, p)
		if err != nil {
			return fmt.Errorf("project feed %q: %w", p, err)
		}
		cats = append(cats, c)
	}
	if len(cats) == 0 {
		return fmt.Errorf("--provider named no feeds")
	}
	cat := project.Merge(cats)
	if *publicSources != "" {
		stripped := project.SanitizeSources(cat, project.SourceSet(*publicSources))
		if stripped > 0 {
			fmt.Fprintf(os.Stderr, "project: stripped %d private source attributions from the public catalog\n", stripped)
		}
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
	redirects, err := project.Redirects(b)
	if err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(*out, "redirects.json"), redirects); err != nil {
		return err
	}
	fmt.Printf("projected %d works to %s (schema v%d); facets: %d languages, %d subjects, %d contributors; %d redirects\n",
		len(cat.Works), *out, project.SchemaVersion,
		len(facets.Languages), len(facets.Subjects), len(facets.Contributors), len(redirects.Redirects))
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
