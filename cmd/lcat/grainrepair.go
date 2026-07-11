package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/freeeve/libcat/bibframe"
)

// runGrainRepair walks every grain under --dir and splits cross-graph fused
// blank nodes -- the residue of the pre-fix write path that let two graphs'
// blank labels collide in one file (subjects absorbing summaries,
// classifications, notes). Clean grains are untouched; repaired grains are
// rewritten canonically. Run once after upgrading past the fix; safe to
// re-run (a repaired grain reports clean).
func runGrainRepair(args []string) error {
	fs2 := flag.NewFlagSet("grain-repair", flag.ExitOnError)
	dir := fs2.String("dir", "", "grain tree root (the blob dir; every *.nq beneath it is checked)")
	dryRun := fs2.Bool("dry-run", false, "report fused grains without writing")
	if err := fs2.Parse(args); err != nil {
		return err
	}
	if *dir == "" {
		return fmt.Errorf("--dir is required")
	}

	var checked, repaired, fusedNodes int
	err := filepath.WalkDir(*dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".nq") || d.Name() == "catalog.nq" {
			return nil
		}
		checked++
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fixed, n, err := bibframe.SplitCrossGraphBlanks(data)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if n == 0 {
			return nil
		}
		repaired++
		fusedNodes += n
		if *dryRun {
			fmt.Printf("would repair %s (%d fused blank nodes)\n", path, n)
			return nil
		}
		if err := os.WriteFile(path, fixed, 0o644); err != nil {
			return err
		}
		fmt.Printf("repaired %s (%d fused blank nodes)\n", path, n)
		return nil
	})
	if err != nil {
		return err
	}
	fmt.Printf("checked %d grains: %d repaired, %d fused nodes split\n", checked, repaired, fusedNodes)
	if repaired > 0 && !*dryRun {
		fmt.Println("rerun serialize/project (or the backend rebuild) so downstream artifacts pick up the repair")
	}
	return nil
}
