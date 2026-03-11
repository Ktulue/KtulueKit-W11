package runner

import (
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

func TestSetSelectedIDsFiltersCount(t *testing.T) {
	cfg := &config.Config{
		Packages: []config.Package{
			{ID: "a", Name: "A", Phase: 1},
			{ID: "b", Name: "B", Phase: 1},
			{ID: "c", Name: "C", Phase: 2},
		},
	}
	rep, _ := reporter.New(t.TempDir())
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
	rep, _ := reporter.New(t.TempDir())
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

func TestConsecutiveFailures_StateAwareSkipNeutral(t *testing.T) {
	cfg := &config.Config{
		Packages: []config.Package{{ID: "already-done", Name: "Already Done", Phase: 1, Scope: "machine", TimeoutSeconds: 300}},
	}
	rep, _ := reporter.New(t.TempDir())
	defer rep.Close()
	s := &state.State{
		Succeeded: map[string]bool{"already-done": true},
		Failed:    make(map[string]bool),
	}
	r := New(cfg, rep, s, false, 1, "", 0)
	r.consecutiveFails = 1 // prime the counter
	var pauseCount int
	r.SetOnPause(func() { pauseCount++ })

	r.runPackagesInPhase(1) // should take the state-aware-skip path, not call trackResult

	if r.consecutiveFails != 1 {
		t.Errorf("state-aware skip should not change consecutiveFails: want 1, got %d", r.consecutiveFails)
	}
	if pauseCount != 0 {
		t.Errorf("state-aware skip should not trigger pause, got %d", pauseCount)
	}
}
