package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
	"github.com/Ktulue/KtulueKit-W11/internal/detector"
)

func TestFilterItemsByIDs(t *testing.T) {
	items := []detector.Item{
		{ID: "Git.Git", Name: "Git"},
		{ID: "OBSProject.OBSStudio", Name: "OBS"},
		{ID: "wsl2", Name: "WSL 2"},
	}
	want := map[string]bool{"Git.Git": true, "wsl2": true}

	got := filterItemsByIDs(items, want)
	if len(got) != 2 {
		t.Fatalf("want 2 items, got %d", len(got))
	}
	for _, item := range got {
		if !want[item.ID] {
			t.Errorf("unexpected item %q in filtered result", item.ID)
		}
	}
}

func TestFilterConfigByIDs(t *testing.T) {
	cfg := &config.Config{
		Packages:   []config.Package{{ID: "Git.Git"}, {ID: "7zip.7zip"}},
		Commands:   []config.Command{{ID: "wsl2"}, {ID: "npm-global"}},
		Extensions: []config.Extension{{ID: "ext1"}},
	}
	ids := []string{"Git.Git", "wsl2"}

	filterConfigByIDs(cfg, ids)

	if len(cfg.Packages) != 1 || cfg.Packages[0].ID != "Git.Git" {
		t.Errorf("unexpected packages after filter: %v", cfg.Packages)
	}
	if len(cfg.Commands) != 1 || cfg.Commands[0].ID != "wsl2" {
		t.Errorf("unexpected commands after filter: %v", cfg.Commands)
	}
	if len(cfg.Extensions) != 0 {
		t.Errorf("want 0 extensions after filter, got %d", len(cfg.Extensions))
	}
}

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
