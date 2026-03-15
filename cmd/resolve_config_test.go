package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
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

// TestFetchToTemp_Success verifies a valid HTTPS URL is downloaded to a temp file
// whose contents match the server response.
func TestFetchToTemp_Success(t *testing.T) {
	body := `{"packages":[],"commands":[],"extensions":[]}`
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, body)
	}))
	defer srv.Close()

	origClient := httpClient
	httpClient = srv.Client()
	defer func() { httpClient = origClient }()

	tmp, err := fetchToTemp(srv.URL)
	if err != nil {
		t.Fatalf("fetchToTemp() error: %v", err)
	}
	defer os.Remove(tmp)

	got, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", tmp, err)
	}
	if string(got) != body {
		t.Errorf("temp file contents = %q, want %q", got, body)
	}
}

// TestFetchToTemp_SizeCap verifies that a response exceeding 1 MiB is rejected.
func TestFetchToTemp_SizeCap(t *testing.T) {
	bigBody := strings.Repeat("x", fetchMaxBytes+1)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, bigBody)
	}))
	defer srv.Close()

	origClient := httpClient
	httpClient = srv.Client()
	defer func() { httpClient = origClient }()

	_, err := fetchToTemp(srv.URL)
	if err == nil {
		t.Fatal("expected error for oversized response, got nil")
	}
	if !containsAll(err.Error(), "1 MiB") {
		t.Errorf("error should mention 1 MiB limit, got: %v", err)
	}
}

// TestFetchToTemp_Non200 verifies that non-200 HTTP status codes are rejected.
func TestFetchToTemp_Non200(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	origClient := httpClient
	httpClient = srv.Client()
	defer func() { httpClient = origClient }()

	_, err := fetchToTemp(srv.URL)
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
	if !containsAll(err.Error(), "404") {
		t.Errorf("error should mention 404, got: %v", err)
	}
}

// TestResolveConfigPaths_HTTPSFetch verifies the full resolveConfigPaths flow
// with an https:// URL resolves to a temp file containing the expected content.
func TestResolveConfigPaths_HTTPSFetch(t *testing.T) {
	body := `{"packages":[],"commands":[],"extensions":[]}`
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, body)
	}))
	defer srv.Close()

	origClient := httpClient
	httpClient = srv.Client()
	defer func() { httpClient = origClient }()

	input := []string{"local.json", srv.URL}
	resolved, cleanup, err := resolveConfigPaths(input)
	if err != nil {
		t.Fatalf("resolveConfigPaths error: %v", err)
	}
	defer cleanup()

	if len(resolved) != 2 {
		t.Fatalf("expected 2 resolved paths, got %d: %v", len(resolved), resolved)
	}
	if resolved[0] != "local.json" {
		t.Errorf("resolved[0] = %q, want %q", resolved[0], "local.json")
	}

	got, err := os.ReadFile(resolved[1])
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", resolved[1], err)
	}
	if string(got) != body {
		t.Errorf("temp file content = %q, want %q", got, body)
	}
}

// TestResolveConfigPaths_CleanupRemovesTempFiles verifies that calling cleanup()
// removes temp files created for https:// URLs.
func TestResolveConfigPaths_CleanupRemovesTempFiles(t *testing.T) {
	body := `{"packages":[]}`
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, body)
	}))
	defer srv.Close()

	origClient := httpClient
	httpClient = srv.Client()
	defer func() { httpClient = origClient }()

	resolved, cleanup, err := resolveConfigPaths([]string{srv.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resolved) != 1 {
		t.Fatalf("expected 1 resolved path, got %d", len(resolved))
	}
	tmpPath := resolved[0]

	if _, err := os.Stat(tmpPath); err != nil {
		t.Fatalf("temp file should exist before cleanup: %v", err)
	}

	cleanup()

	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("temp file should be removed after cleanup()")
	}
}

// TestRunValidate_WithLocalConfigPath verifies that runValidate correctly resolves
// a local config path through resolveConfigPaths before passing to LoadAll.
func TestRunValidate_WithLocalConfigPath(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ktuluekit.json")
	minimalCfg := `{
		"$schema": "",
		"version": "1",
		"metadata": {"name": "test"},
		"settings": {},
		"packages": [],
		"commands": [],
		"extensions": []
	}`
	if err := os.WriteFile(cfgPath, []byte(minimalCfg), 0644); err != nil {
		t.Fatal(err)
	}

	origPaths := configPaths
	configPaths = []string{cfgPath}
	defer func() { configPaths = origPaths }()

	cmd := &cobra.Command{}
	err := runValidate(cmd, nil)
	if err != nil && containsAll(err.Error(), "not yet implemented") {
		t.Errorf("runValidate returned stub error: %v", err)
	}
	if err != nil && containsAll(err.Error(), "insecure URL") {
		t.Errorf("runValidate rejected a local path as a URL: %v", err)
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
