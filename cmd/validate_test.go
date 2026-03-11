package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
)

func TestValidateCmd_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "k.json")
	os.WriteFile(f, []byte(`{
		"version": "1.0",
		"metadata": {"name": "Test"},
		"packages": [{"id": "Git.Git", "name": "Git", "phase": 1}],
		"settings": {}
	}`), 0644)

	cfg, err := config.LoadAll([]string{f})
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	errs := config.Validate(cfg)
	if len(errs) != 0 {
		t.Errorf("want 0 errors for valid config, got %+v", errs)
	}
}

func TestValidateCmd_InvalidConfig(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "k.json")
	// version missing, duplicate ID
	os.WriteFile(f, []byte(`{
		"metadata": {"name": "Test"},
		"packages": [
			{"id": "dup", "name": "A", "phase": 1},
			{"id": "dup", "name": "B", "phase": 1}
		],
		"settings": {}
	}`), 0644)

	cfg, err := config.LoadAll([]string{f})
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	errs := config.Validate(cfg)
	if len(errs) == 0 {
		t.Fatal("want errors for invalid config, got none")
	}
}
