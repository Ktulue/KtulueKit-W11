package runner

import (
	"context"
	"io"
	"testing"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
	"github.com/Ktulue/KtulueKit-W11/internal/reporter"
	"github.com/Ktulue/KtulueKit-W11/internal/state"
)

func TestCountItemsFromPhase_AllPhases(t *testing.T) {
	r := &Runner{
		cfg: &config.Config{
			Packages:   []config.Package{{ID: "p1", Phase: 1}, {ID: "p2", Phase: 2}},
			Commands:   []config.Command{{ID: "c1", Phase: 2}},
			Extensions: []config.Extension{{ID: "e1", Phase: 3}},
		},
	}

	got := r.countItemsFromPhase(1)

	if got != 4 {
		t.Errorf("countItemsFromPhase(1): expected 4, got %d", got)
	}
}

func TestCountItemsFromPhase_ResumeFiltersEarlierPhases(t *testing.T) {
	r := &Runner{
		cfg: &config.Config{
			Packages: []config.Package{
				{ID: "p1", Phase: 1},
				{ID: "p2", Phase: 2},
				{ID: "p3", Phase: 3},
			},
			Commands:   []config.Command{{ID: "c1", Phase: 2}},
			Extensions: []config.Extension{},
		},
	}

	// fromPhase=2 counts p2, p3, c1 — excludes p1 (phase 1)
	got := r.countItemsFromPhase(2)

	if got != 3 {
		t.Errorf("countItemsFromPhase(2): expected 3, got %d", got)
	}
}

func TestCountItemsFromPhase_EmptyConfig(t *testing.T) {
	r := &Runner{cfg: &config.Config{}}

	got := r.countItemsFromPhase(1)

	if got != 0 {
		t.Errorf("countItemsFromPhase on empty config: expected 0, got %d", got)
	}
}

func TestCountItemsInPhase_ExactMatch(t *testing.T) {
	r := &Runner{
		cfg: &config.Config{
			Packages:   []config.Package{{ID: "p1", Phase: 1}, {ID: "p2", Phase: 2}},
			Commands:   []config.Command{{ID: "c1", Phase: 2}},
			Extensions: []config.Extension{{ID: "e1", Phase: 3}},
		},
	}
	if got := r.countItemsInPhase(2); got != 2 {
		t.Errorf("countItemsInPhase(2): expected 2 (p2+c1), got %d", got)
	}
}

func TestCountItemsInPhase_OtherPhasesExcluded(t *testing.T) {
	r := &Runner{
		cfg: &config.Config{
			Packages: []config.Package{{ID: "p1", Phase: 1}, {ID: "p2", Phase: 3}},
		},
	}
	if got := r.countItemsInPhase(2); got != 0 {
		t.Errorf("countItemsInPhase(2) with no phase-2 items: expected 0, got %d", got)
	}
}

func TestCountItemsInPhase_RespectsSelectedIDs(t *testing.T) {
	r := &Runner{
		cfg: &config.Config{
			Packages: []config.Package{{ID: "p1", Phase: 2}, {ID: "p2", Phase: 2}},
		},
		selectedIDs: map[string]bool{"p1": true},
	}
	if got := r.countItemsInPhase(2); got != 1 {
		t.Errorf("countItemsInPhase with selectedIDs: expected 1, got %d", got)
	}
}

func TestSetSelectedIDsFiltersCount(t *testing.T) {
	cfg := &config.Config{
		Packages: []config.Package{
			{ID: "a", Name: "A", Phase: 1},
			{ID: "b", Name: "B", Phase: 1},
			{ID: "c", Name: "C", Phase: 2},
		},
	}
	rep, _ := reporter.New(t.TempDir(), io.Discard)
	defer rep.Close()
	s := &state.State{Succeeded: make(map[string]bool), Failed: make(map[string]bool)}

	r := New(cfg, rep, s, false, 1, "", 0)
	r.SetSelectedIDs(map[string]bool{"a": true, "c": true})

	total := r.countItemsFromPhase(1)
	if total != 2 {
		t.Errorf("want 2 (a + c selected), got %d", total)
	}
}

func TestSetSelectedIDsNilRunsAll(t *testing.T) {
	cfg := &config.Config{
		Packages: []config.Package{
			{ID: "a", Name: "A", Phase: 1},
			{ID: "b", Name: "B", Phase: 1},
		},
	}
	rep, _ := reporter.New(t.TempDir(), io.Discard)
	defer rep.Close()
	s := &state.State{Succeeded: make(map[string]bool), Failed: make(map[string]bool)}

	r := New(cfg, rep, s, false, 1, "", 0)
	// selectedIDs is nil — run all

	total := r.countItemsFromPhase(1)
	if total != 2 {
		t.Errorf("want 2 (all items), got %d", total)
	}
}

func TestConsecutiveFailures_ThreeFailsTriggerPause(t *testing.T) {
	r := &Runner{}
	var pauseCount int
	r.SetOnPause(func() { pauseCount++ })

	r.trackResult(reporter.StatusFailed)
	r.trackResult(reporter.StatusFailed)
	if pauseCount != 0 {
		t.Error("pause should not fire after only 2 failures")
	}
	r.trackResult(reporter.StatusFailed)
	if pauseCount != 1 {
		t.Errorf("want pause called once after 3 failures, got %d", pauseCount)
	}
	if r.consecutiveFails != 0 {
		t.Errorf("want consecutiveFails reset to 0 after pause, got %d", r.consecutiveFails)
	}
}

func TestConsecutiveFailures_ResetBySuccess(t *testing.T) {
	r := &Runner{}
	var pauseCount int
	r.SetOnPause(func() { pauseCount++ })

	r.trackResult(reporter.StatusFailed)
	r.trackResult(reporter.StatusFailed)
	r.trackResult(reporter.StatusInstalled) // resets counter
	r.trackResult(reporter.StatusFailed)
	r.trackResult(reporter.StatusFailed)

	if pauseCount != 0 {
		t.Errorf("want pause not called (counter reset by success), got %d", pauseCount)
	}
	if r.consecutiveFails != 2 {
		t.Errorf("want consecutiveFails=2 after reset+2 fails, got %d", r.consecutiveFails)
	}
}

func TestConsecutiveFailures_SkipsCountAsFails(t *testing.T) {
	r := &Runner{}
	var pauseCount int
	r.SetOnPause(func() { pauseCount++ })

	r.trackResult(reporter.StatusSkipped)
	r.trackResult(reporter.StatusSkipped)
	r.trackResult(reporter.StatusSkipped)

	if pauseCount != 1 {
		t.Errorf("want pause called once after 3 skips, got %d", pauseCount)
	}
}

func TestPhaseHeader_Format(t *testing.T) {
	got := phaseHeader(2, 2, 4)
	// 26 trailing dashes — must match the format string in phaseHeader exactly.
	want := "\n── Phase 2 | [2 of 4] ──────────────────────────────"
	if got != want {
		t.Errorf("phaseHeader(2,2,4):\n  got:  %q\n  want: %q", got, want)
	}
}

func TestSetOnlyPhase_TotalItemsReflectsOnlyPhase(t *testing.T) {
	cfg := &config.Config{
		Packages: []config.Package{
			{ID: "p1", Name: "P1", Phase: 1},
			{ID: "p2", Name: "P2", Phase: 2},
			{ID: "p3", Name: "P3", Phase: 2},
		},
		Commands:   []config.Command{},
		Extensions: []config.Extension{},
	}
	rep, _ := reporter.New(t.TempDir(), io.Discard)
	defer rep.Close()
	s := &state.State{Succeeded: make(map[string]bool), Failed: make(map[string]bool)}

	r := New(cfg, rep, s, false, 1, "", 0)
	r.SetOnlyPhase(2)
	if r.onlyPhase != 2 {
		t.Errorf("SetOnlyPhase(2): expected r.onlyPhase=2, got %d", r.onlyPhase)
	}

	// countItemsInPhase(2) should return 2 (p2 + p3 only)
	if got := r.countItemsInPhase(2); got != 2 {
		t.Errorf("countItemsInPhase(2) with onlyPhase=2: expected 2, got %d", got)
	}
	// countItemsInPhase(1) should return 1 (only p1)
	if got := r.countItemsInPhase(1); got != 1 {
		t.Errorf("countItemsInPhase(1): expected 1, got %d", got)
	}
}

func TestConsecutiveFailures_StateAwareSkipNeutral(t *testing.T) {
	cfg := &config.Config{
		Packages: []config.Package{{ID: "already-done", Name: "Already Done", Phase: 1, Scope: "machine", TimeoutSeconds: 300}},
	}
	rep, _ := reporter.New(t.TempDir(), io.Discard)
	defer rep.Close()
	s := &state.State{
		Succeeded: map[string]bool{"already-done": true},
		Failed:    make(map[string]bool),
	}
	r := New(cfg, rep, s, false, 1, "", 0)
	r.consecutiveFails = 1 // prime the counter
	var pauseCount int
	r.SetOnPause(func() { pauseCount++ })

	r.runPackagesInPhase(context.Background(), 1) // should take the state-aware-skip path, not call trackResult

	if r.consecutiveFails != 1 {
		t.Errorf("state-aware skip should not change consecutiveFails: want 1, got %d", r.consecutiveFails)
	}
	if pauseCount != 0 {
		t.Errorf("state-aware skip should not trigger pause, got %d", pauseCount)
	}
}

func TestUpgradeOnly_SkipsMissingPackage(t *testing.T) {
	// "cmd /C exit 1" deterministically returns non-zero → detector sees StatusMissing
	cfg := &config.Config{
		Packages: []config.Package{
			{ID: "missing-pkg", Name: "Missing", Phase: 1, Check: "cmd /C exit 1"},
		},
		Commands:   []config.Command{},
		Extensions: []config.Extension{},
	}
	rep, _ := reporter.New(t.TempDir(), io.Discard)
	defer rep.Close()
	s := &state.State{Succeeded: make(map[string]bool), Failed: make(map[string]bool)}

	r := New(cfg, rep, s, true, 1, "", 0) // dryRun=true
	r.SetUpgradeOnly(true)

	r.runPackagesInPhase(context.Background(), 1)

	if r.itemIdx != 0 {
		t.Errorf("missing package should be skipped (itemIdx=0), got %d", r.itemIdx)
	}
}

func TestUpgradeOnly_SkipsUnknownPackage(t *testing.T) {
	// No check command → detector returns StatusUnknown → skip with warning
	cfg := &config.Config{
		Packages: []config.Package{
			{ID: "unknown-pkg", Name: "Unknown", Phase: 1, Check: ""},
		},
		Commands:   []config.Command{},
		Extensions: []config.Extension{},
	}
	rep, _ := reporter.New(t.TempDir(), io.Discard)
	defer rep.Close()
	s := &state.State{Succeeded: make(map[string]bool), Failed: make(map[string]bool)}

	r := New(cfg, rep, s, true, 1, "", 0)
	r.SetUpgradeOnly(true)

	r.runPackagesInPhase(context.Background(), 1)

	if r.itemIdx != 0 {
		t.Errorf("unknown package should be skipped (itemIdx=0), got %d", r.itemIdx)
	}
}

func TestUpgradeOnly_ProceedsWhenInstalled(t *testing.T) {
	// state.Succeeded → detector returns StatusInstalled → proceed (dry-run, no OS call)
	cfg := &config.Config{
		Packages: []config.Package{
			{ID: "installed-pkg", Name: "Installed", Phase: 1, Scope: "machine", TimeoutSeconds: 300},
		},
		Commands:   []config.Command{},
		Extensions: []config.Extension{},
	}
	rep, _ := reporter.New(t.TempDir(), io.Discard)
	defer rep.Close()
	s := &state.State{
		Succeeded: map[string]bool{"installed-pkg": true},
		Failed:    make(map[string]bool),
	}

	r := New(cfg, rep, s, true, 1, "", 0) // dryRun=true
	r.SetUpgradeOnly(true)

	r.runPackagesInPhase(context.Background(), 1)

	if r.itemIdx != 1 {
		t.Errorf("installed package should be processed (itemIdx=1), got %d", r.itemIdx)
	}
}

func TestUpgradeOnly_SkipsExtensionSilently(t *testing.T) {
	// Extensions never have a check command → StatusUnknown → skip silently (no warning)
	cfg := &config.Config{
		Packages:   []config.Package{},
		Commands:   []config.Command{},
		Extensions: []config.Extension{
			{ID: "ext1", Name: "Extension", Phase: 1},
		},
	}
	rep, _ := reporter.New(t.TempDir(), io.Discard)
	defer rep.Close()
	s := &state.State{Succeeded: make(map[string]bool), Failed: make(map[string]bool)}

	r := New(cfg, rep, s, true, 1, "", 0)
	r.SetUpgradeOnly(true)

	r.runExtensionsInPhase(context.Background(), 1)

	if r.itemIdx != 0 {
		t.Errorf("extension should be silently skipped (itemIdx=0), got %d", r.itemIdx)
	}
}

func TestUpgradeOnly_SkipsMissingCommand(t *testing.T) {
	cfg := &config.Config{
		Packages: []config.Package{},
		Commands: []config.Command{
			{ID: "missing-cmd", Name: "Missing Cmd", Phase: 1, Check: "cmd /C exit 1"},
		},
		Extensions: []config.Extension{},
	}
	rep, _ := reporter.New(t.TempDir(), io.Discard)
	defer rep.Close()
	s := &state.State{Succeeded: make(map[string]bool), Failed: make(map[string]bool)}

	r := New(cfg, rep, s, true, 1, "", 0)
	r.SetUpgradeOnly(true)

	r.runCommandsInPhase(context.Background(), 1)

	if r.itemIdx != 0 {
		t.Errorf("missing command should be skipped (itemIdx=0), got %d", r.itemIdx)
	}
}

func TestUpgradeOnly_ProceedsWhenCommandInstalled(t *testing.T) {
	cfg := &config.Config{
		Packages: []config.Package{},
		Commands: []config.Command{
			{ID: "installed-cmd", Name: "Installed Cmd", Phase: 1, Cmd: "echo skip"},
		},
		Extensions: []config.Extension{},
	}
	rep, _ := reporter.New(t.TempDir(), io.Discard)
	defer rep.Close()
	s := &state.State{
		Succeeded: map[string]bool{"installed-cmd": true},
		Failed:    make(map[string]bool),
	}

	r := New(cfg, rep, s, true, 1, "", 0) // dryRun=true
	r.SetUpgradeOnly(true)

	r.runCommandsInPhase(context.Background(), 1)

	if r.itemIdx != 1 {
		t.Errorf("installed command should be processed (itemIdx=1), got %d", r.itemIdx)
	}
}

func TestMarkInterrupted_SetsFlag(t *testing.T) {
	r := &Runner{}
	if r.WasInterrupted() {
		t.Fatal("WasInterrupted should be false before any interrupt")
	}
	r.markInterrupted(2)
	if !r.WasInterrupted() {
		t.Error("WasInterrupted should be true after markInterrupted")
	}
}

func TestMarkInterrupted_IsIdempotent(t *testing.T) {
	r := &Runner{}
	// Second call should be a no-op — interrupted flag remains true, no panic.
	r.markInterrupted(1)
	r.markInterrupted(1)
	if !r.WasInterrupted() {
		t.Error("WasInterrupted should be true")
	}
}

func TestCtrlC_StopsBeforeFirstItem(t *testing.T) {
	// Pre-cancelled context: interrupt fires before any item starts.
	cfg := &config.Config{
		Packages: []config.Package{
			{ID: "p1", Name: "P1", Phase: 1, Scope: "machine", TimeoutSeconds: 300},
		},
		Commands:   []config.Command{},
		Extensions: []config.Extension{},
	}
	rep, _ := reporter.New(t.TempDir(), io.Discard)
	defer rep.Close()
	s := &state.State{Succeeded: make(map[string]bool), Failed: make(map[string]bool)}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel

	r := New(cfg, rep, s, true, 1, "", 0)
	r.runPackagesInPhase(ctx, 1)

	if !r.WasInterrupted() {
		t.Error("WasInterrupted should be true after cancelled-context run")
	}
	if r.itemIdx != 0 {
		t.Errorf("no items should start when ctx is pre-cancelled, got itemIdx=%d", r.itemIdx)
	}
}
