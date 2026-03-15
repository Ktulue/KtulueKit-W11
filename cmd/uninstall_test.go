package main

import (
	"testing"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
)

func makeUninstallTestConfig() *config.Config {
	return &config.Config{
		Packages: []config.Package{
			{ID: "Git.Git", Name: "Git"},
			{ID: "Mozilla.Firefox", Name: "Firefox"},
		},
		Commands: []config.Command{
			{ID: "wsl", Name: "WSL"},
		},
	}
}

func TestBuildUninstallList_AllItems(t *testing.T) {
	cfg := makeUninstallTestConfig()
	names := buildUninstallList(cfg, nil, nil)
	if len(names) != 3 {
		t.Errorf("expected 3 items, got %d: %v", len(names), names)
	}
}

func TestBuildUninstallList_FilterApplied(t *testing.T) {
	cfg := makeUninstallTestConfig()
	filter := map[string]bool{"Git.Git": true}
	names := buildUninstallList(cfg, filter, nil)
	if len(names) != 1 || names[0] != "Git" {
		t.Errorf("expected only Git, got: %v", names)
	}
}

func TestBuildUninstallList_ExcludeApplied(t *testing.T) {
	cfg := makeUninstallTestConfig()
	exclude := map[string]bool{"Git.Git": true}
	names := buildUninstallList(cfg, nil, exclude)
	for _, n := range names {
		if n == "Git" {
			t.Error("excluded 'Git' should not appear")
		}
	}
}

func TestBuildUninstallList_EmptyWhenFilterMatchesNone(t *testing.T) {
	cfg := makeUninstallTestConfig()
	filter := map[string]bool{"NonExistent.ID": true}
	names := buildUninstallList(cfg, filter, nil)
	if len(names) != 0 {
		t.Errorf("expected empty list, got: %v", names)
	}
}
