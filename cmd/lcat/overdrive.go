package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/freeeve/libcatalog/ingest"
	"github.com/freeeve/libcatalog/ingest/overdrive"
	"github.com/freeeve/libcodex/iso2709"
)

// runOverdrive ingests a cached OverDrive scan (page-*.json). With --out it maps the
// Thunder JSON directly to canonical BIBFRAME grains with stable, minted two-tier
// ids (ARCHITECTURE §4/§9) via the OverDrive ingest provider and the shared
// ingest.Run pipeline: any grains already under --out seed the resolver, so
// re-ingest reuses ids and clusters editions into one Work. With --marc it also
// exports an ISO 2709 fixture for the MARC-import ramp (tasks/007). It is a
// convenience alias for `lcat ingest --provider overdrive`.
func runOverdrive(args []string) error {
	fs := flag.NewFlagSet("overdrive", flag.ExitOnError)
	cache := fs.String("cache", "", "OverDrive page-cache directory (contains page-*.json)")
	out := fs.String("out", "", "output directory for canonical grains (direct JSON->BIBFRAME)")
	marcOut := fs.String("marc", "", "optional MARC (.mrc) fixture output (the MARC-import ramp)")
	provider := fs.String("provider", overdrive.ProviderName, "provenance graph feed:<provider> for the records")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *cache == "" {
		return fmt.Errorf("--cache is required")
	}
	if *out == "" && *marcOut == "" {
		return fmt.Errorf("one of --out (grains) or --marc (fixture) is required")
	}

	if *marcOut != "" {
		items, err := overdrive.ReadCache(*cache)
		if err != nil {
			return err
		}
		if err := writeOverdriveMARC(items, *marcOut); err != nil {
			return err
		}
	}
	if *out != "" {
		cfg := ingest.Config{Feed: *provider, Source: *cache}
		if err := runIngest(providerRegistry(), overdrive.ProviderName, cfg, *out); err != nil {
			return err
		}
	}
	return nil
}

// writeOverdriveMARC exports the cached items as an ISO 2709 MARC file.
func writeOverdriveMARC(items []overdrive.Item, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := iso2709.NewWriter(f)
	for _, rec := range overdrive.Records(items) {
		if err := w.Write(rec); err != nil {
			return err
		}
	}
	fmt.Printf("wrote %d records to %s\n", len(items), path)
	return nil
}
