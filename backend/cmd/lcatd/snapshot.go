package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/freeeve/libcatalog/backend/workindex"
	"github.com/freeeve/libcatalog/storage/blob"
)

// runWorkindexSnapshot builds the work-index snapshot from a grain store off the
// running server -- the offline seed a Lambda deployment needs so its first cold
// start loads the projection instead of scanning the corpus (tasks/155). It
// scans the store once and writes the snapshot blob back into it.
//
//	lcatd workindex-snapshot --blob-dir <dir> [--out data/workindex.snapshot]
func runWorkindexSnapshot(args []string) error {
	fs := flag.NewFlagSet("workindex-snapshot", flag.ExitOnError)
	dir := fs.String("blob-dir", "", "grain store directory (holds data/works/*.nq)")
	out := fs.String("out", workindex.DefaultSnapshotPath, "snapshot path within the store")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *dir == "" {
		return fmt.Errorf("workindex-snapshot: --blob-dir is required")
	}
	ctx := context.Background()
	ix := workindex.New(blob.NewDir(*dir), "data/works/")
	ix.SetSnapshotPath(*out)
	if err := ix.RefreshNow(ctx); err != nil {
		return fmt.Errorf("scan grains: %w", err)
	}
	if err := ix.Save(ctx); err != nil {
		return fmt.Errorf("write snapshot: %w", err)
	}
	fmt.Printf("wrote %s\n", *out)
	return nil
}
