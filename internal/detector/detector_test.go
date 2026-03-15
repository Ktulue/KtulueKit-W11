package detector_test

import (
	"os"
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
	if result.Item.ID != item.ID {
		t.Errorf("expected result.Item.ID %q, got %q", item.ID, result.Item.ID)
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

func TestFlattenItems_OrderIsPackagesThenCommandsThenExtensions(t *testing.T) {
	cfg := &config.Config{
		Packages: []config.Package{
			{ID: "pkg-a", Name: "Package A", Phase: 1},
		},
		Commands: []config.Command{
			{ID: "cmd-b", Name: "Command B", Phase: 2},
		},
		Extensions: []config.Extension{
			{ID: "ext-c", Name: "Extension C", Phase: 3},
		},
	}

	items := detector.FlattenItems(cfg)

	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	if items[0].Tier != "winget" {
		t.Errorf("expected items[0].Tier 'winget' (packages first), got %q", items[0].Tier)
	}
	if items[1].Tier != "command" {
		t.Errorf("expected items[1].Tier 'command', got %q", items[1].Tier)
	}
	if items[2].Tier != "extension" {
		t.Errorf("expected items[2].Tier 'extension', got %q", items[2].Tier)
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

func TestFlattenItems_CommandFieldsCorrect(t *testing.T) {
	cfg := &config.Config{
		Commands: []config.Command{
			{ID: "claude-code", Name: "Claude Code", Phase: 4, Check: "claude --version"},
		},
	}

	items := detector.FlattenItems(cfg)

	if items[0].ID != "claude-code" {
		t.Errorf("expected ID 'claude-code', got %q", items[0].ID)
	}
	if items[0].Tier != "command" {
		t.Errorf("expected Tier 'command', got %q", items[0].Tier)
	}
	if items[0].CheckCmd != "claude --version" {
		t.Errorf("expected CheckCmd 'claude --version', got %q", items[0].CheckCmd)
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

func TestCheckAll_AppliesStateAwareSkipPerItem(t *testing.T) {
	// One item with state.Succeeded=true → StatusInstalled
	// One item with no state and no check cmd → StatusUnknown
	s := &state.State{
		Succeeded: map[string]bool{"known-good": true},
		Failed:    map[string]bool{},
	}
	items := []detector.Item{
		{ID: "known-good", Name: "Known Good", Tier: "winget"},
		{ID: "unknown", Name: "Unknown", Tier: "extension"},
	}

	results := detector.CheckAll(items, s)

	if results[0].Status != detector.StatusInstalled {
		t.Errorf("expected results[0].Status StatusInstalled, got %v", results[0].Status)
	}
	if results[1].Status != detector.StatusUnknown {
		t.Errorf("expected results[1].Status StatusUnknown, got %v", results[1].Status)
	}
}

// --- RunCheckDetailed tests ---
// These run real shell commands on Windows cmd.exe.

func TestRunCheckDetailed_ExitZero_ReturnsInstalled(t *testing.T) {
	installed, timedOut := detector.RunCheckDetailed("echo hi")
	if !installed {
		t.Error("expected installed=true for exit-0 command")
	}
	if timedOut {
		t.Error("expected timedOut=false for fast command")
	}
}

func TestRunCheckDetailed_ExitNonZero_ReturnsAbsent(t *testing.T) {
	installed, timedOut := detector.RunCheckDetailed("exit 1")
	if installed {
		t.Error("expected installed=false for non-zero exit")
	}
	if timedOut {
		t.Error("expected timedOut=false — non-zero exit is not a timeout")
	}
}

// TestRunCheckDetailed_Timeout verifies that a command that exceeds the
// 15-second timeout returns timedOut=true and installed=false.
//
// This test sleeps for longer than the detector's checkTimeoutSeconds (15s),
// so it is guarded by TEST_SLOW=1 to avoid slowing the default test run.
// Run with: TEST_SLOW=1 go test ./internal/detector/... -v -run TestRunCheckDetailed_Timeout -timeout 30s
func TestRunCheckDetailed_Timeout(t *testing.T) {
	if os.Getenv("TEST_SLOW") == "" {
		t.Skip("skipping slow timeout test; set TEST_SLOW=1 to run")
	}

	// "timeout /T 20 /NOBREAK" sleeps for 20 seconds in both interactive and
	// non-interactive Windows cmd.exe contexts, exceeding the 15s detector timeout.
	installed, timedOut := detector.RunCheckDetailed("timeout /T 20 /NOBREAK")

	if installed {
		t.Error("expected installed=false for timed-out command")
	}
	if !timedOut {
		t.Error("expected timedOut=true for command that exceeds timeout")
	}
}
