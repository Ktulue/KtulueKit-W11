package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestResolveConfigPaths_LocalPassThrough verifies that plain file paths are
// returned unchanged.
func TestResolveConfigPaths_LocalPassThrough(t *testing.T) {
	input := []string{"base.json", "/abs/path/extras.json"}
	resolved, cleanup, err := resolveConfigPaths(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cleanup()

	if len(resolved) != 2 {
		t.Fatalf("expected 2 resolved paths, got %d", len(resolved))
	}
	if resolved[0] != "base.json" {
		t.Errorf("expected resolved[0] = %q, got %q", "base.json", resolved[0])
	}
	if resolved[1] != "/abs/path/extras.json" {
		t.Errorf("expected resolved[1] = %q, got %q", "/abs/path/extras.json", resolved[1])
	}
}

// TestResolveConfigPaths_EmptySlice verifies a nil/empty input returns an empty
// resolved list without error.
func TestResolveConfigPaths_EmptySlice(t *testing.T) {
	resolved, cleanup, err := resolveConfigPaths(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cleanup()
	if len(resolved) != 0 {
		t.Errorf("expected 0 resolved paths, got %d", len(resolved))
	}
}

// TestResolveConfigPaths_HTTPRejected verifies that http:// URLs are rejected
// immediately with a clear error.
func TestResolveConfigPaths_HTTPRejected(t *testing.T) {
	input := []string{"http://example.com/config.json"}
	_, cleanup, err := resolveConfigPaths(input)
	if cleanup != nil {
		defer cleanup()
	}
	if err == nil {
		t.Fatal("expected error for http:// URL, got nil")
	}
	if !containsAll(err.Error(), "http://example.com/config.json", "https://") {
		t.Errorf("error message should mention the rejected URL and suggest https://, got: %v", err)
	}
}

// TestResolveConfigPaths_HTTPRejectedMidList verifies that http:// rejection fires
// even when valid local paths precede the bad URL (fail-fast, no partial state).
func TestResolveConfigPaths_HTTPRejectedMidList(t *testing.T) {
	input := []string{"base.json", "http://example.com/bad.json", "extras.json"}
	_, cleanup, err := resolveConfigPaths(input)
	if cleanup != nil {
		defer cleanup()
	}
	if err == nil {
		t.Fatal("expected error for http:// URL in middle of list, got nil")
	}
}

// TestResolveConfigPaths_MixedLocalPaths verifies multiple local paths (relative
// and absolute) are all passed through in order.
func TestResolveConfigPaths_MixedLocalPaths(t *testing.T) {
	dir := t.TempDir()
	abs := filepath.Join(dir, "extra.json")
	if err := os.WriteFile(abs, []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	input := []string{"base.json", abs}
	resolved, cleanup, err := resolveConfigPaths(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cleanup()

	if len(resolved) != 2 {
		t.Fatalf("expected 2, got %d", len(resolved))
	}
	if resolved[1] != abs {
		t.Errorf("expected resolved[1] = %q, got %q", abs, resolved[1])
	}
}

// containsAll is a test helper that checks all substrings appear in s.
func containsAll(s string, substrings ...string) bool {
	for _, sub := range substrings {
		found := false
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
