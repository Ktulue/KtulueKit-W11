package detector_test

import (
	"testing"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
	"github.com/Ktulue/KtulueKit-W11/internal/detector"
	"github.com/Ktulue/KtulueKit-W11/internal/state"
)

// --- CheckItem tests ---

func TestCheckItem_StateAwareSkip(t *testing.T) {
	// If state says succeeded, return StatusInstalled without running any shell command.
	s := &state.State{
		Succeeded: map[string]bool{"Git.Git": true},
		Failed:    map[string]bool{},
	}
	item := detector.Item{
		ID:       "Git.Git",
		Name:     "Git for Windows",
		Phase:    1,
		Tier:     "winget",
		CheckCmd: "", // no check command — proves state skip fires before check logic
	}

	result := detector.CheckItem(item, s)

	if result.Status != detector.StatusInstalled {
		t.Errorf("expected StatusInstalled from state-aware skip, got %v", result.Status)
	}
}

func TestCheckItem_NoCheckCmd_ReturnsUnknown(t *testing.T) {
	s := &state.State{
		Succeeded: map[string]bool{},
		Failed:    map[string]bool{},
	}
	item := detector.Item{
		ID:    "some-extension",
		Name:  "Some Extension",
		Phase: 5,
		Tier:  "extension",
		// No CheckCmd
	}

	result := detector.CheckItem(item, s)

	if result.Status != detector.StatusUnknown {
		t.Errorf("expected StatusUnknown for item with no check command, got %v", result.Status)
	}
}

func TestCheckItem_EchoSkip_ReturnsUnknown(t *testing.T) {
	// "echo skip" is a sentinel used in the config for items with no real check.
	s := &state.State{
		Succeeded: map[string]bool{},
		Failed:    map[string]bool{},
	}
	item := detector.Item{
		ID:       "manual-item",
		Name:     "Manual Item",
		Phase:    3,
		Tier:     "command",
		CheckCmd: "echo skip",
	}

	result := detector.CheckItem(item, s)

	if result.Status != detector.StatusUnknown {
		t.Errorf("expected StatusUnknown for 'echo skip' check, got %v", result.Status)
	}
}

func TestCheckItem_NilState_DoesNotPanic(t *testing.T) {
	// nil state is safe — treated as no succeeded items.
	item := detector.Item{
		ID:    "some-id",
		Name:  "Some Item",
		Phase: 1,
		Tier:  "winget",
		// No CheckCmd — avoids running a real shell command
	}

	// Should not panic
	result := detector.CheckItem(item, nil)

	if result.Status != detector.StatusUnknown {
		t.Errorf("expected StatusUnknown for nil state + no check cmd, got %v", result.Status)
	}
}

// --- FlattenItems tests ---

func TestFlattenItems_IncludesAllTiers(t *testing.T) {
	cfg := &config.Config{
		Packages: []config.Package{
			{ID: "Git.Git", Name: "Git", Phase: 1, Check: "git --version"},
		},
		Commands: []config.Command{
			{ID: "claude-code", Name: "Claude Code", Phase: 4, Check: "claude --version"},
		},
		Extensions: []config.Extension{
			{ID: "ublock", Name: "uBlock Origin", Phase: 5},
		},
	}

	items := detector.FlattenItems(cfg)

	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
}

func TestFlattenItems_PackageFieldsCorrect(t *testing.T) {
	cfg := &config.Config{
		Packages: []config.Package{
			{ID: "Git.Git", Name: "Git for Windows", Phase: 1, Check: "git --version"},
		},
	}

	items := detector.FlattenItems(cfg)

	if items[0].ID != "Git.Git" {
		t.Errorf("expected ID 'Git.Git', got %q", items[0].ID)
	}
	if items[0].Tier != "winget" {
		t.Errorf("expected Tier 'winget', got %q", items[0].Tier)
	}
	if items[0].CheckCmd != "git --version" {
		t.Errorf("expected CheckCmd 'git --version', got %q", items[0].CheckCmd)
	}
}

func TestFlattenItems_ExtensionHasNoCheckCmd(t *testing.T) {
	cfg := &config.Config{
		Extensions: []config.Extension{
			{ID: "ublock", Name: "uBlock Origin", Phase: 5},
		},
	}

	items := detector.FlattenItems(cfg)

	if items[0].CheckCmd != "" {
		t.Errorf("expected empty CheckCmd for extension, got %q", items[0].CheckCmd)
	}
	if items[0].Tier != "extension" {
		t.Errorf("expected Tier 'extension', got %q", items[0].Tier)
	}
}

func TestFlattenItems_EmptyConfig_ReturnsEmpty(t *testing.T) {
	cfg := &config.Config{}

	items := detector.FlattenItems(cfg)

	if len(items) != 0 {
		t.Errorf("expected 0 items for empty config, got %d", len(items))
	}
}

// --- CheckAll tests ---

func TestCheckAll_LengthMatchesInput(t *testing.T) {
	s := &state.State{
		Succeeded: map[string]bool{},
		Failed:    map[string]bool{},
	}
	items := []detector.Item{
		{ID: "a", Name: "A", Tier: "winget"},
		{ID: "b", Name: "B", Tier: "command"},
		{ID: "c", Name: "C", Tier: "extension"},
	}

	results := detector.CheckAll(items, s)

	if len(results) != len(items) {
		t.Errorf("expected %d results, got %d", len(items), len(results))
	}
}

func TestCheckAll_PreservesOrder(t *testing.T) {
	s := &state.State{
		Succeeded: map[string]bool{},
		Failed:    map[string]bool{},
	}
	items := []detector.Item{
		{ID: "first", Name: "First", Tier: "winget"},
		{ID: "second", Name: "Second", Tier: "command"},
	}

	results := detector.CheckAll(items, s)

	if results[0].Item.ID != "first" {
		t.Errorf("expected first result ID 'first', got %q", results[0].Item.ID)
	}
	if results[1].Item.ID != "second" {
		t.Errorf("expected second result ID 'second', got %q", results[1].Item.ID)
	}
}
