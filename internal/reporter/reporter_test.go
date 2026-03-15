package reporter

import (
	"bytes"
	"strings"
	"testing"
)

func TestProgressWriterCapture(t *testing.T) {
	var buf bytes.Buffer
	r := &Reporter{progressWriter: &buf}
	r.results = []Result{
		{ID: "Git.Git", Name: "Git", Tier: "winget", Status: StatusInstalled},
	}
	r.Add(Result{ID: "Git.Git", Name: "Git", Tier: "winget", Status: StatusInstalled})

	if !strings.Contains(buf.String(), "Git") {
		t.Errorf("expected progress output to contain 'Git', got: %q", buf.String())
	}
}

func TestProgressWriterDefault(t *testing.T) {
	// Reporter with nil progressWriter should not panic on Add.
	r := &Reporter{progressWriter: nil}
	defer func() {
		if p := recover(); p != nil {
			t.Fatalf("Add() panicked with nil progressWriter: %v", p)
		}
	}()
	r.Add(Result{ID: "test", Name: "Test", Tier: "winget", Status: StatusInstalled})
}

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
