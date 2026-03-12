package main

import (
	"testing"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
)

func TestAllConfigIDs(t *testing.T) {
	cfg := &config.Config{
		Packages:   []config.Package{{ID: "Git.Git"}, {ID: "7zip.7zip"}},
		Commands:   []config.Command{{ID: "wsl2"}},
		Extensions: []config.Extension{{ID: "ext1"}},
	}
	ids := allConfigIDs(cfg)
	for _, want := range []string{"Git.Git", "7zip.7zip", "wsl2", "ext1"} {
		if !ids[want] {
			t.Errorf("want %q in allConfigIDs result", want)
		}
	}
	if len(ids) != 4 {
		t.Errorf("want 4 IDs, got %d", len(ids))
	}
}

func TestAllConfigIDs_Empty(t *testing.T) {
	ids := allConfigIDs(&config.Config{})
	if len(ids) != 0 {
		t.Errorf("want 0 IDs for empty config, got %d", len(ids))
	}
}

func TestFilterFlagsError_MutualExclusion(t *testing.T) {
	if err := filterFlagsError("Git.Git", "Steam"); err == nil {
		t.Fatal("want error when both --only and --exclude are set, got nil")
	}
}

func TestFilterFlagsError_OnlyOneSet(t *testing.T) {
	if err := filterFlagsError("Git.Git", ""); err != nil {
		t.Fatalf("want no error with only --only set, got %v", err)
	}
	if err := filterFlagsError("", "Steam"); err != nil {
		t.Fatalf("want no error with only --exclude set, got %v", err)
	}
	if err := filterFlagsError("", ""); err != nil {
		t.Fatalf("want no error with neither flag set, got %v", err)
	}
}

func TestAllConfigIDs_OnlyFiltering(t *testing.T) {
	cfg := &config.Config{
		Packages: []config.Package{{ID: "Git.Git"}, {ID: "7zip.7zip"}},
		Commands: []config.Command{{ID: "wsl2"}},
	}
	known := allConfigIDs(cfg)
	selected, unknowns := buildOnlySet("Git.Git,wsl2,unknown-id", known)

	if !selected["Git.Git"] {
		t.Error("want Git.Git in selected")
	}
	if !selected["wsl2"] {
		t.Error("want wsl2 in selected")
	}
	if selected["7zip.7zip"] {
		t.Error("7zip.7zip should NOT be in selected")
	}
	if len(unknowns) != 1 || unknowns[0] != "unknown-id" {
		t.Errorf("want [unknown-id] in unknowns, got %v", unknowns)
	}
}

func TestPhaseFlagsError_BothSet(t *testing.T) {
	if err := phaseFlagsError(2, 2); err == nil {
		t.Fatal("expected error when --phase=2 and --resume-phase=2, got nil")
	}
}

func TestPhaseFlagsError_PhaseOnly(t *testing.T) {
	if err := phaseFlagsError(2, 1); err != nil {
		t.Fatalf("expected no error when --resume-phase is default (1), got %v", err)
	}
}

func TestPhaseFlagsError_NeitherSet(t *testing.T) {
	if err := phaseFlagsError(0, 1); err != nil {
		t.Fatalf("expected no error when --phase is default (0), got %v", err)
	}
}

func TestPhaseFlagsError_ResumePhaseOnly(t *testing.T) {
	if err := phaseFlagsError(0, 2); err != nil {
		t.Fatalf("expected no error when only --resume-phase is set, got %v", err)
	}
}

func TestUpgradeOnlyFlagsError_BothSet(t *testing.T) {
	if err := upgradeOnlyFlagsError(true, true); err == nil {
		t.Fatal("expected error when --upgrade-only and --no-upgrade are both set, got nil")
	}
}

func TestUpgradeOnlyFlagsError_UpgradeOnlyOnly(t *testing.T) {
	if err := upgradeOnlyFlagsError(true, false); err != nil {
		t.Fatalf("expected no error when only --upgrade-only is set, got %v", err)
	}
}

func TestUpgradeOnlyFlagsError_NoUpgradeOnly(t *testing.T) {
	if err := upgradeOnlyFlagsError(false, true); err != nil {
		t.Fatalf("expected no error when only --no-upgrade is set, got %v", err)
	}
}

func TestUpgradeOnlyFlagsError_NeitherSet(t *testing.T) {
	if err := upgradeOnlyFlagsError(false, false); err != nil {
		t.Fatalf("expected no error when neither flag is set, got %v", err)
	}
}

func TestAllConfigIDs_ExcludeFiltering(t *testing.T) {
	cfg := &config.Config{
		Packages: []config.Package{{ID: "Git.Git"}, {ID: "7zip.7zip"}},
		Commands: []config.Command{{ID: "wsl2"}},
	}
	remaining, unknowns := buildExcludeSet("7zip.7zip,unknown-id", allConfigIDs(cfg))

	if !remaining["Git.Git"] {
		t.Error("want Git.Git in remaining")
	}
	if !remaining["wsl2"] {
		t.Error("want wsl2 in remaining")
	}
	if remaining["7zip.7zip"] {
		t.Error("7zip.7zip should be excluded")
	}
	if len(unknowns) != 1 || unknowns[0] != "unknown-id" {
		t.Errorf("want [unknown-id] in unknowns, got %v", unknowns)
	}
}
