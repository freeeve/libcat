package config

import "testing"

// TestStoreSelectionDefaults documents that a bare environment (the laptop/demo
// case) leaves the persistent-store knobs empty, so the caller falls back to
// the in-memory and local-directory stores.
func TestStoreSelectionDefaults(t *testing.T) {
	t.Setenv("LCATD_ABUSE_SECRET", "")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DynamoTable != "" || cfg.S3Bucket != "" || cfg.AWSEndpoint != "" {
		t.Fatalf("expected empty store knobs by default, got %+v", cfg)
	}
}

// TestStoreSelectionFromEnv locks the env-var names that opt into the
// persistent stores.
func TestStoreSelectionFromEnv(t *testing.T) {
	t.Setenv("LCATD_DYNAMO_TABLE", "lcat-sidecar")
	t.Setenv("LCATD_S3_BUCKET", "lcat-grains")
	t.Setenv("LCATD_AWS_ENDPOINT", "http://localhost:4566")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DynamoTable != "lcat-sidecar" {
		t.Errorf("DynamoTable = %q", cfg.DynamoTable)
	}
	if cfg.S3Bucket != "lcat-grains" {
		t.Errorf("S3Bucket = %q", cfg.S3Bucket)
	}
	if cfg.AWSEndpoint != "http://localhost:4566" {
		t.Errorf("AWSEndpoint = %q", cfg.AWSEndpoint)
	}
}
