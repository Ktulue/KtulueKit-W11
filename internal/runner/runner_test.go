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
	r.SetSelectedIDs([]string{"a", "c"})

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
