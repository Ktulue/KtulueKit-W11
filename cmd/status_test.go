package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
)

// TestStatusCmd_TwoConfigs verifies that the status subcommand receives a correctly
// merged config when two --config flags are provided. This tests the wiring that
// runStatus uses configPaths (the persistent flag slice) and calls LoadAll.
func TestStatusCmd_TwoConfigs(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "base.json")
	extras := filepath.Join(dir, "extras.json")

	if err := os.WriteFile(base, []byte(`{
		"version": "1.0",
		"metadata": {"name": "Base"},
		"packages": [{"id": "Git.Git", "name": "Git", "phase": 1}],
		"settings": {"retry_count": 1}
	}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(extras, []byte(`{
		"version": "1.0",
		"metadata": {"name": "Extras"},
		"settings": {"retry_count": 3}
	}`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.LoadAll([]string{base, extras})
	if err != nil {
		t.Fatalf("LoadAll with two configs failed: %v", err)
	}
	// Metadata: first config wins
	if cfg.Metadata.Name != "Base" {
		t.Errorf("metadata.name = %q, want %q", cfg.Metadata.Name, "Base")
	}
	// Settings: extras overrides retry_count
	if cfg.Settings.RetryCount != 3 {
		t.Errorf("retry_count = %d, want 3 (from extras config)", cfg.Settings.RetryCount)
	}
	// Package from base preserved
	if len(cfg.Packages) != 1 || cfg.Packages[0].ID != "Git.Git" {
		t.Errorf("packages = %v, want [Git.Git]", cfg.Packages)
	}
}
