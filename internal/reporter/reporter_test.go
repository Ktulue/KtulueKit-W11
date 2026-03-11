package reporter

import "testing"

func TestNamesBy(t *testing.T) {
	r := &Reporter{}
	r.results = []Result{
		{Name: "Go", Status: StatusInstalled},
		{Name: "Node", Status: StatusInstalled},
		{Name: "Python", Status: StatusFailed},
	}

	names := r.NamesBy(StatusInstalled)
	if len(names) != 2 {
		t.Fatalf("want 2 installed names, got %d", len(names))
	}
	if names[0] != "Go" || names[1] != "Node" {
		t.Errorf("unexpected names: %v", names)
	}

	failed := r.NamesBy(StatusFailed)
	if len(failed) != 1 || failed[0] != "Python" {
		t.Errorf("unexpected failed names: %v", failed)
	}

	empty := r.NamesBy(StatusSkipped)
	if len(empty) != 0 {
		t.Errorf("want empty slice, got %v", empty)
	}
}
