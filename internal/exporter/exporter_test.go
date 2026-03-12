package exporter_test

import (
	"testing"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
	"github.com/Ktulue/KtulueKit-W11/internal/exporter"
	"github.com/Ktulue/KtulueKit-W11/internal/state"
)

func checkFnAlwaysInstalled(_ string) exporter.CheckResult { return exporter.CheckInstalled }
func checkFnAlwaysAbsent(_ string) exporter.CheckResult    { return exporter.CheckAbsent }
func checkFnAlwaysTimeout(_ string) exporter.CheckResult   { return exporter.CheckTimedOut }

func baseOpts(fn func(string) exporter.CheckResult) exporter.Options {
	return exporter.Options{
		Fast: false, SourceConfig: "/path/to/ktuluekit.json",
		ToolVersion: "1.0.0", Machine: "TESTMACHINE", CheckFn: fn,
	}
}

func makeCfg(pkgs []config.Package, cmds []config.Command, exts []config.Extension, profiles []config.Profile) *config.Config {
	return &config.Config{Version: "1.0", Packages: pkgs, Commands: cmds, Extensions: exts, Profiles: profiles}
}

func makeFastOpts(s *state.State) exporter.Options {
	return exporter.Options{
		Fast: true, SourceConfig: "/path/to/ktuluekit.json",
		ToolVersion: "1.0.0", Machine: "TESTMACHINE", State: s,
	}
}

// --- Check mode ---

func TestExport_CheckMode_AllInstalled(t *testing.T) {
	cfg := makeCfg(
		[]config.Package{{ID: "Git.Git", Name: "Git", Phase: 1, Check: "check"}},
		[]config.Command{{ID: "claude-code", Name: "Claude Code", Phase: 4, Check: "check"}},
		nil, nil,
	)
	res, err := exporter.Export(cfg, baseOpts(checkFnAlwaysInstalled))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Packages) != 1 {
		t.Errorf("expected 1 package, got %d", len(res.Packages))
	}
	if len(res.Commands) != 1 {
		t.Errorf("expected 1 command, got %d", len(res.Commands))
	}
	if res.Included != 2 {
		t.Errorf("expected Included=2, got %d", res.Included)
	}
	if res.Checked != 2 {
		t.Errorf("expected Checked=2, got %d", res.Checked)
	}
}

func TestExport_CheckMode_NoneInstalled(t *testing.T) {
	cfg := makeCfg(
		[]config.Package{{ID: "Git.Git", Name: "Git", Phase: 1, Check: "check"}},
		[]config.Command{{ID: "claude-code", Name: "Claude Code", Phase: 4, Check: "check"}},
		nil, nil,
	)
	res, err := exporter.Export(cfg, baseOpts(checkFnAlwaysAbsent))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Packages) != 0 {
		t.Errorf("expected 0 packages, got %d", len(res.Packages))
	}
	if res.Included != 0 {
		t.Errorf("expected Included=0, got %d", res.Included)
	}
}

func TestExport_CheckMode_MixedWithAbsent(t *testing.T) {
	callCount := 0
	results := []exporter.CheckResult{exporter.CheckInstalled, exporter.CheckAbsent}
	fn := func(_ string) exporter.CheckResult {
		r := results[callCount%len(results)]
		callCount++
		return r
	}
	cfg := makeCfg(
		[]config.Package{
			{ID: "pkg-a", Name: "Pkg A", Phase: 1, Check: "check-a"},
			{ID: "pkg-b", Name: "Pkg B", Phase: 1, Check: "check-b"},
		},
		nil, nil, nil,
	)
	res, err := exporter.Export(cfg, exporter.Options{Fast: false, SourceConfig: "/x", ToolVersion: "1.0", Machine: "M", CheckFn: fn})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Packages) != 1 {
		t.Errorf("expected 1 package, got %d", len(res.Packages))
	}
	if res.Packages[0].ID != "pkg-a" {
		t.Errorf("expected pkg-a, got %q", res.Packages[0].ID)
	}
}

func TestExport_CheckMode_EmptyConfig(t *testing.T) {
	res, err := exporter.Export(makeCfg(nil, nil, nil, nil), baseOpts(checkFnAlwaysInstalled))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Checked != 0 || res.Included != 0 {
		t.Errorf("expected Checked=0, Included=0; got %d, %d", res.Checked, res.Included)
	}
}

func TestExport_CheckMode_ExtensionsAlwaysOmitted(t *testing.T) {
	cfg := makeCfg(nil, nil, []config.Extension{{ID: "ublock", Name: "uBlock", Phase: 5}}, nil)
	res, err := exporter.Export(cfg, baseOpts(checkFnAlwaysInstalled))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Extensions) != 0 {
		t.Errorf("expected 0 extensions in check mode, got %d", len(res.Extensions))
	}
}

func TestExport_CheckMode_NoCheckCmd_Skipped(t *testing.T) {
	called := false
	fn := func(_ string) exporter.CheckResult { called = true; return exporter.CheckInstalled }
	cfg := makeCfg(
		[]config.Package{
			{ID: "pkg-no-check", Name: "No Check", Phase: 1, Check: ""},
			{ID: "pkg-echo-skip", Name: "Echo Skip", Phase: 1, Check: "echo skip"},
		},
		nil, nil, nil,
	)
	res, err := exporter.Export(cfg, exporter.Options{Fast: false, SourceConfig: "/x", ToolVersion: "1.0", Machine: "M", CheckFn: fn})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("CheckFn should not be called for items with no check command or 'echo skip'")
	}
	if len(res.Packages) != 0 {
		t.Errorf("expected 0 packages, got %d", len(res.Packages))
	}
}

func TestExport_CheckMode_TimeoutTreatedAsAbsent(t *testing.T) {
	cfg := makeCfg([]config.Package{{ID: "Git.Git", Name: "Git", Phase: 1, Check: "check"}}, nil, nil, nil)
	res, err := exporter.Export(cfg, baseOpts(checkFnAlwaysTimeout))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Packages) != 0 {
		t.Errorf("expected 0 packages after timeout, got %d", len(res.Packages))
	}
}

func TestExport_CheckMode_SnapshotMetadata(t *testing.T) {
	res, err := exporter.Export(makeCfg(nil, nil, nil, nil), exporter.Options{
		Fast: false, SourceConfig: "/abs/path.json", ToolVersion: "2.0.0", Machine: "MYMACHINE", CheckFn: checkFnAlwaysInstalled,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Snapshot.SourceConfig != "/abs/path.json" {
		t.Errorf("expected SourceConfig '/abs/path.json', got %q", res.Snapshot.SourceConfig)
	}
	if res.Snapshot.ToolVersion != "2.0.0" {
		t.Errorf("expected ToolVersion '2.0.0', got %q", res.Snapshot.ToolVersion)
	}
	if res.Snapshot.Machine != "MYMACHINE" {
		t.Errorf("expected Machine 'MYMACHINE', got %q", res.Snapshot.Machine)
	}
	if res.Snapshot.Mode != "check" {
		t.Errorf("expected Mode 'check', got %q", res.Snapshot.Mode)
	}
	if res.Snapshot.GeneratedAt == "" {
		t.Error("GeneratedAt should not be empty")
	}
}

func TestExport_CheckMode_ToolVersionDefaultsToDev(t *testing.T) {
	res, err := exporter.Export(makeCfg(nil, nil, nil, nil), exporter.Options{
		Fast: false, SourceConfig: "/x", ToolVersion: "", Machine: "M", CheckFn: checkFnAlwaysInstalled,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Snapshot.ToolVersion != "dev" {
		t.Errorf("expected ToolVersion 'dev', got %q", res.Snapshot.ToolVersion)
	}
}

// --- Profile filtering ---

func TestExport_CheckMode_ProfileFullyFiltered_Omitted(t *testing.T) {
	cfg := makeCfg(
		[]config.Package{{ID: "Git.Git", Name: "Git", Phase: 1, Check: "check"}},
		nil, nil,
		[]config.Profile{{Name: "Dev Only", IDs: []string{"Git.Git"}}},
	)
	res, err := exporter.Export(cfg, baseOpts(checkFnAlwaysAbsent))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Profiles) != 0 {
		t.Errorf("expected 0 profiles, got %d", len(res.Profiles))
	}
}

func TestExport_CheckMode_ProfilePartiallyFiltered_Emitted(t *testing.T) {
	callCount := 0
	results := []exporter.CheckResult{exporter.CheckInstalled, exporter.CheckAbsent}
	fn := func(_ string) exporter.CheckResult {
		r := results[callCount%len(results)]
		callCount++
		return r
	}
	cfg := makeCfg(
		[]config.Package{
			{ID: "pkg-a", Name: "Pkg A", Phase: 1, Check: "check-a"},
			{ID: "pkg-b", Name: "Pkg B", Phase: 1, Check: "check-b"},
		},
		nil, nil,
		[]config.Profile{{Name: "Both", IDs: []string{"pkg-a", "pkg-b"}}},
	)
	res, err := exporter.Export(cfg, exporter.Options{Fast: false, SourceConfig: "/x", ToolVersion: "1.0", Machine: "M", CheckFn: fn})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Profiles) != 1 {
		t.Errorf("expected 1 profile, got %d", len(res.Profiles))
	}
	if len(res.Profiles[0].IDs) != 1 || res.Profiles[0].IDs[0] != "pkg-a" {
		t.Errorf("expected profile with [pkg-a], got %v", res.Profiles[0].IDs)
	}
}

// --- Fast mode ---

func TestExport_FastMode_IncludesSucceededItems(t *testing.T) {
	s := &state.State{Succeeded: map[string]bool{"Git.Git": true, "claude-code": true}, Failed: map[string]bool{}}
	cfg := makeCfg(
		[]config.Package{{ID: "Git.Git", Name: "Git", Phase: 1}},
		[]config.Command{{ID: "claude-code", Name: "Claude Code", Phase: 4}},
		nil, nil,
	)
	res, err := exporter.Export(cfg, makeFastOpts(s))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Packages) != 1 {
		t.Errorf("expected 1 package, got %d", len(res.Packages))
	}
	if len(res.Commands) != 1 {
		t.Errorf("expected 1 command, got %d", len(res.Commands))
	}
	if res.Snapshot.Mode != "fast" {
		t.Errorf("expected Mode 'fast', got %q", res.Snapshot.Mode)
	}
	if res.Checked != 0 {
		t.Errorf("expected Checked=0 in fast mode, got %d", res.Checked)
	}
}

func TestExport_FastMode_ExcludesFailedItems(t *testing.T) {
	s := &state.State{Succeeded: map[string]bool{}, Failed: map[string]bool{"Git.Git": true}}
	cfg := makeCfg([]config.Package{{ID: "Git.Git", Name: "Git", Phase: 1}}, nil, nil, nil)
	res, err := exporter.Export(cfg, makeFastOpts(s))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Packages) != 0 {
		t.Errorf("expected 0 packages, got %d", len(res.Packages))
	}
}

func TestExport_FastMode_IncludesExtensions(t *testing.T) {
	s := &state.State{Succeeded: map[string]bool{"ublock": true}, Failed: map[string]bool{}}
	cfg := makeCfg(nil, nil, []config.Extension{{ID: "ublock", Name: "uBlock Origin", Phase: 5}}, nil)
	res, err := exporter.Export(cfg, makeFastOpts(s))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Extensions) != 1 {
		t.Errorf("expected 1 extension, got %d", len(res.Extensions))
	}
}
