package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/freeeve/libcatalog/ingest"
)

// runIngestCmd ingests any registered provider into canonical grains under --out:
// `lcat ingest --provider <name> --source <input> --out <dir> [--feed <name>]`.
// Which provider runs is a runtime selection against the built-in registry, so
// enabling a source is a config/flag change, not a code change (ARCHITECTURE §9a,
// tasks/006). The OverDrive `lcat overdrive` command is a convenience alias for
// `--provider overdrive` that also offers the MARC-fixture export.
func runIngestCmd(args []string) error {
	reg := providerRegistry()
	fs := flag.NewFlagSet("ingest", flag.ExitOnError)
	provider := fs.String("provider", "", fmt.Sprintf("registered provider to run %v", reg.Names()))
	source := fs.String("source", "", "provider input (e.g. an OverDrive page-cache directory)")
	out := fs.String("out", "", "output directory for canonical grains and catalog.nq")
	feed := fs.String("feed", "", "provenance graph feed:<name> (default: the provider name)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *provider == "" {
		return fmt.Errorf("--provider is required (registered: %v)", reg.Names())
	}
	if *out == "" {
		return fmt.Errorf("--out is required")
	}
	cfg := ingest.Config{Feed: *feed, Source: *source}
	return runIngest(reg, *provider, cfg, *out)
}

// runIngest constructs the named provider from reg and runs the shared ingest
// pipeline into out, surfacing resolver conflicts on stderr and a run summary on
// stdout. It is shared by `lcat ingest` and the `lcat overdrive` alias.
func runIngest(reg *ingest.Registry, name string, cfg ingest.Config, out string) error {
	prov, err := reg.New(name, cfg)
	if err != nil {
		return err
	}
	res, err := ingest.Run(prov, out)
	if err != nil {
		return err
	}
	for _, c := range res.Conflicts {
		fmt.Fprintln(os.Stderr, "conflict:", c)
	}
	fmt.Printf("built %d works from %d instances under %s (feed:%s); minted %d works, %d instances; retired %d works\n",
		res.Stats.Grains, res.Stats.Records, out, prov.Name(), res.MintedWorks, res.MintedInstances, res.Retired)
	return nil
}
