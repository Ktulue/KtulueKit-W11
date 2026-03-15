package reporter

import (
	"bytes"
	"encoding/json"
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

func TestSummaryJSON(t *testing.T) {
	r := &Reporter{}
	r.results = []Result{
		{ID: "Git.Git", Name: "Git", Tier: "winget", Status: StatusInstalled},
		{ID: "wsl2", Name: "WSL 2", Tier: "command", Status: StatusFailed, Detail: "exit code 1"},
	}

	data, err := r.SummaryJSON()
	if err != nil {
		t.Fatalf("SummaryJSON() returned error: %v", err)
	}

	var out struct {
		Results []Result `json:"results"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("SummaryJSON() output is not valid JSON: %v", err)
	}
	if len(out.Results) != 2 {
		t.Errorf("want 2 results, got %d", len(out.Results))
	}
	if out.Results[0].ID != "Git.Git" {
		t.Errorf("want first result ID 'Git.Git', got %q", out.Results[0].ID)
	}
	if out.Results[1].Status != StatusFailed {
		t.Errorf("want second result status 'failed', got %q", out.Results[1].Status)
	}
}

func TestSummaryMD(t *testing.T) {
	r := &Reporter{}
	r.results = []Result{
		{ID: "Git.Git", Name: "Git", Tier: "winget", Status: StatusInstalled},
		{ID: "wsl2", Name: "WSL 2", Tier: "command", Status: StatusFailed, Detail: "exit code 1"},
	}

	md := r.SummaryMD()
	if !strings.Contains(md, "# KtulueKit Install Summary") {
		t.Error("SummaryMD() output missing expected H1 heading")
	}
	if !strings.Contains(md, "Git") {
		t.Error("SummaryMD() output missing 'Git'")
	}
	if !strings.Contains(md, "Installed successfully") {
		t.Error("SummaryMD() output missing installed section heading")
	}
	if !strings.Contains(md, "Failed") {
		t.Error("SummaryMD() output missing failed section heading")
	}
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
