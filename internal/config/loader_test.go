package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeJSON writes content to a temp file and returns the path.
func writeJSON(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "*.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return f.Name()
}

const baseConfig = `{
  "version": "1.0",
  "metadata": {"name": "Base"},
  "settings": {"log_dir": "./base-logs", "retry_count": 2},
  "packages": [
    {"id": "Git.Git", "name": "Git", "phase": 1},
    {"id": "Mozilla.Firefox", "name": "Firefox", "phase": 2}
  ],
  "commands": [
    {"id": "npm-globals", "name": "NPM Globals", "phase": 3, "check": "npm -v", "command": "npm install -g typescript"}
  ],
  "profiles": [
    {"name": "Full", "ids": ["Git.Git", "Mozilla.Firefox"]}
  ]
}`

const extrasConfig = `{
  "version": "1.0",
  "metadata": {"name": "Extras"},
  "settings": {"retry_count": 5},
  "packages": [
    {"id": "Mozilla.Firefox", "name": "Firefox ESR", "phase": 3},
    {"id": "Spotify.Spotify", "name": "Spotify", "phase": 3, "scope": "user"}
  ],
  "profiles": [
    {"name": "Full", "ids": ["Git.Git", "Mozilla.Firefox", "Spotify.Spotify"]}
  ]
}`

// TestLoadAll_SinglePath behaves identically to Load().
func TestLoadAll_SinglePath(t *testing.T) {
	path := writeJSON(t, baseConfig)
	cfg, err := LoadAll([]string{path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Metadata.Name != "Base" {
		t.Errorf("metadata.name = %q, want %q", cfg.Metadata.Name, "Base")
	}
	if len(cfg.Packages) != 2 {
		t.Errorf("len(packages) = %d, want 2", len(cfg.Packages))
	}
}

// TestLoadAll_NoOverlap merges two disjoint configs — all items present.
func TestLoadAll_NoOverlap(t *testing.T) {
	base := writeJSON(t, `{
		"version": "1.0",
		"metadata": {"name": "Base"},
		"packages": [{"id": "Git.Git", "name": "Git", "phase": 1}]
	}`)
	extras := writeJSON(t, `{
		"version": "1.0",
		"metadata": {"name": "Extras"},
		"packages": [{"id": "Mozilla.Firefox", "name": "Firefox", "phase": 2}]
	}`)
	cfg, err := LoadAll([]string{base, extras})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Packages) != 2 {
		t.Errorf("len(packages) = %d, want 2", len(cfg.Packages))
	}
}

// TestLoadAll_PackageIDOverride later config wins on same Package ID.
func TestLoadAll_PackageIDOverride(t *testing.T) {
	base := writeJSON(t, baseConfig)
	extras := writeJSON(t, extrasConfig)
	cfg, err := LoadAll([]string{base, extras})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Firefox should be overridden by extras (name "Firefox ESR", phase 3)
	var firefox *Package
	for i := range cfg.Packages {
		if cfg.Packages[i].ID == "Mozilla.Firefox" {
			firefox = &cfg.Packages[i]
		}
	}
	if firefox == nil {
		t.Fatal("Mozilla.Firefox not found in merged packages")
	}
	if firefox.Name != "Firefox ESR" {
		t.Errorf("Firefox name = %q, want %q", firefox.Name, "Firefox ESR")
	}
	if firefox.Phase != 3 {
		t.Errorf("Firefox phase = %d, want 3", firefox.Phase)
	}
	// Spotify from extras should also be present
	if len(cfg.Packages) != 3 {
		t.Errorf("len(packages) = %d, want 3 (Git, Firefox, Spotify)", len(cfg.Packages))
	}
}

// TestLoadAll_PackageIDOverride_PositionPreserved override keeps original position.
func TestLoadAll_PackageIDOverride_PositionPreserved(t *testing.T) {
	base := writeJSON(t, baseConfig)
	extras := writeJSON(t, extrasConfig)
	cfg, err := LoadAll([]string{base, extras})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Git.Git is first in base and not overridden — should still be first.
	// Mozilla.Firefox is second in base — override should keep it second.
	// Spotify.Spotify is new from extras — should be third.
	ids := make([]string, len(cfg.Packages))
	for i, p := range cfg.Packages {
		ids[i] = p.ID
	}
	want := []string{"Git.Git", "Mozilla.Firefox", "Spotify.Spotify"}
	for i, id := range want {
		if i >= len(ids) || ids[i] != id {
			t.Errorf("packages[%d] = %q, want %q (full order: %v)", i, ids[i], id, ids)
		}
	}
}

// TestLoadAll_SettingsNonZeroWins later non-zero settings field wins.
func TestLoadAll_SettingsNonZeroWins(t *testing.T) {
	base := writeJSON(t, baseConfig)   // log_dir="./base-logs", retry_count=2
	extras := writeJSON(t, extrasConfig) // retry_count=5, log_dir absent
	cfg, err := LoadAll([]string{base, extras})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// extras overrides retry_count
	if cfg.Settings.RetryCount != 5 {
		t.Errorf("retry_count = %d, want 5", cfg.Settings.RetryCount)
	}
	// log_dir absent in extras — base value preserved
	if cfg.Settings.LogDir != "./base-logs" {
		t.Errorf("log_dir = %q, want %q", cfg.Settings.LogDir, "./base-logs")
	}
}

// TestLoadAll_ProfileNameOverride later config wins on same Profile name.
func TestLoadAll_ProfileNameOverride(t *testing.T) {
	base := writeJSON(t, baseConfig)
	extras := writeJSON(t, extrasConfig)
	cfg, err := LoadAll([]string{base, extras})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var full *Profile
	for i := range cfg.Profiles {
		if cfg.Profiles[i].Name == "Full" {
			full = &cfg.Profiles[i]
		}
	}
	if full == nil {
		t.Fatal("profile 'Full' not found")
	}
	// extras version has 3 IDs
	if len(full.IDs) != 3 {
		t.Errorf("Full profile IDs = %v, want 3 items", full.IDs)
	}
}

// TestLoadAll_MetadataFirstWins metadata comes from first config.
func TestLoadAll_MetadataFirstWins(t *testing.T) {
	base := writeJSON(t, baseConfig)
	extras := writeJSON(t, extrasConfig)
	cfg, err := LoadAll([]string{base, extras})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Metadata.Name != "Base" {
		t.Errorf("metadata.name = %q, want %q (first config is authoritative)", cfg.Metadata.Name, "Base")
	}
}

// TestLoadAll_ThreeConfigs middle value is overridden by third config.
func TestLoadAll_ThreeConfigs(t *testing.T) {
	base := writeJSON(t, `{
		"version": "1.0",
		"metadata": {"name": "Base"},
		"settings": {"retry_count": 1}
	}`)
	middle := writeJSON(t, `{
		"version": "1.0",
		"metadata": {"name": "Middle"},
		"settings": {"retry_count": 3}
	}`)
	third := writeJSON(t, `{
		"version": "1.0",
		"metadata": {"name": "Third"},
		"settings": {"retry_count": 7}
	}`)
	cfg, err := LoadAll([]string{base, middle, third})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Settings.RetryCount != 7 {
		t.Errorf("retry_count = %d, want 7 (third config wins)", cfg.Settings.RetryCount)
	}
}

// TestLoadAll_MissingFile returns an error naming the missing path.
func TestLoadAll_MissingFile(t *testing.T) {
	base := writeJSON(t, baseConfig)
	missing := filepath.Join(t.TempDir(), "nonexistent.json")
	_, err := LoadAll([]string{base, missing})
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	if !strings.Contains(err.Error(), missing) {
		t.Errorf("error %q does not mention missing path %q", err.Error(), missing)
	}
}

// TestLoadAll_InvalidJSON returns an error before merge begins.
func TestLoadAll_InvalidJSON(t *testing.T) {
	base := writeJSON(t, baseConfig)
	bad := writeJSON(t, `{not valid json`)
	_, err := LoadAll([]string{base, bad})
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

// TestLoadAll_CrossTierIDCollision Package ID in one file matches Command ID in another — Validate must catch it.
func TestLoadAll_CrossTierIDCollision(t *testing.T) {
	base := writeJSON(t, `{
		"version": "1.0",
		"metadata": {"name": "Base"},
		"packages": [{"id": "foo", "name": "Foo Package", "phase": 1}]
	}`)
	extras := writeJSON(t, `{
		"version": "1.0",
		"metadata": {"name": "Extras"},
		"commands": [{"id": "foo", "name": "Foo Command", "phase": 2, "check": "foo -v", "command": "install foo"}]
	}`)
	cfg, err := LoadAll([]string{base, extras})
	if err != nil {
		t.Fatalf("LoadAll should not validate: unexpected error %v", err)
	}
	errs := Validate(cfg)
	if len(errs) == 0 {
		t.Fatal("expected Validate to catch cross-tier ID collision, got no errors")
	}
}

// TestLoadAll_EmptyPaths defaults to ktuluekit.json (file won't exist in test — expect error).
func TestLoadAll_EmptyPaths(t *testing.T) {
	// Change to a temp dir that has no ktuluekit.json
	orig, _ := os.Getwd()
	defer os.Chdir(orig)
	os.Chdir(t.TempDir())

	_, err := LoadAll(nil)
	if err == nil {
		t.Fatal("expected error when default ktuluekit.json is absent, got nil")
	}
}
