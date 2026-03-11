package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
)

func TestListCmd_LoadsConfig(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "k.json")
	if err := os.WriteFile(f, []byte(`{
		"version": "1.0",
		"metadata": {"name": "Test"},
		"packages": [{"id": "Git.Git", "name": "Git", "phase": 1}],
		"commands": [{"id": "npm-ts", "name": "TypeScript", "phase": 4, "check": "tsc --version", "command": "npm i -g typescript"}],
		"settings": {}
	}`), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := config.LoadAll([]string{f})
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(cfg.Packages) != 1 {
		t.Errorf("want 1 package, got %d", len(cfg.Packages))
	}
	if len(cfg.Commands) != 1 {
		t.Errorf("want 1 command, got %d", len(cfg.Commands))
	}
}
