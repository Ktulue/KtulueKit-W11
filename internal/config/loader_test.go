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

func TestMergeProfiles(t *testing.T) {
	tests := []struct {
		name         string
		base         []Profile
		src          []Profile
		wantNames    []string
		wantIDCounts map[string]int // profile name -> expected len(IDs)
	}{
		{
			name:      "no overlap — both profiles kept",
			base:      []Profile{{Name: "Full", IDs: []string{"a", "b"}}},
			src:       []Profile{{Name: "Minimal", IDs: []string{"a"}}},
			wantNames: []string{"Full", "Minimal"},
		},
		{
			name: "last-wins on name collision",
			base: []Profile{{Name: "Full", IDs: []string{"a", "b"}}},
			src:  []Profile{{Name: "Full", IDs: []string{"a", "b", "c"}}},
			wantNames:    []string{"Full"},
			wantIDCounts: map[string]int{"Full": 3},
		},
		{
			name: "position preserved — collision stays at base index",
			base: []Profile{
				{Name: "Full", IDs: []string{"a"}},
				{Name: "Minimal", IDs: []string{"b"}},
			},
			src: []Profile{
				{Name: "Full", IDs: []string{"a", "b", "c"}},
			},
			wantNames:    []string{"Full", "Minimal"},
			wantIDCounts: map[string]int{"Full": 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeProfiles(tt.base, tt.src)
			if len(got) != len(tt.wantNames) {
				t.Fatalf("len = %d, want %d (got names: %v)", len(got), len(tt.wantNames), extractProfileNames(got))
			}
			for i, name := range tt.wantNames {
				if got[i].Name != name {
					t.Errorf("[%d] Name = %q, want %q", i, got[i].Name, name)
				}
			}
			for name, wantCount := range tt.wantIDCounts {
				for _, p := range got {
					if p.Name == name && len(p.IDs) != wantCount {
						t.Errorf("profile %q: len(IDs) = %d, want %d", name, len(p.IDs), wantCount)
					}
				}
			}
		})
	}
}

// extractProfileNames is a test helper that extracts profile names for error messages.
func extractProfileNames(profiles []Profile) []string {
	names := make([]string, len(profiles))
	for i, p := range profiles {
		names[i] = p.Name
	}
	return names
}

func TestMergeSettings(t *testing.T) {
	tests := []struct {
		name               string
		dst                Settings
		src                Settings
		wantLogDir         string
		wantRetryCount     int
		wantTimeout        int
		wantScope          string
		wantExtMode        string
		wantUpgradeEnabled bool
	}{
		{
			name:           "src non-zero overwrites dst",
			dst:            Settings{LogDir: "./logs", RetryCount: 2, DefaultTimeoutSeconds: 60, DefaultScope: "machine", ExtensionMode: "url"},
			src:            Settings{LogDir: "./new-logs", RetryCount: 5},
			wantLogDir:     "./new-logs",
			wantRetryCount: 5,
			wantTimeout:    60,
			wantScope:      "machine",
			wantExtMode:    "url",
		},
		{
			name:           "src zero fields do not overwrite dst",
			dst:            Settings{LogDir: "./logs", RetryCount: 3},
			src:            Settings{},
			wantLogDir:     "./logs",
			wantRetryCount: 3,
		},
		{
			name:               "UpgradeIfInstalled one-way ratchet: src true sets dst",
			dst:                Settings{UpgradeIfInstalled: false},
			src:                Settings{UpgradeIfInstalled: true},
			wantUpgradeEnabled: true,
		},
		{
			name:               "UpgradeIfInstalled one-way ratchet: src false cannot clear dst",
			dst:                Settings{UpgradeIfInstalled: true},
			src:                Settings{UpgradeIfInstalled: false},
			wantUpgradeEnabled: true,
		},
		{
			name:               "UpgradeIfInstalled both false stays false",
			dst:                Settings{UpgradeIfInstalled: false},
			src:                Settings{UpgradeIfInstalled: false},
			wantUpgradeEnabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dst := tt.dst
			mergeSettings(&dst, &tt.src)

			if tt.wantLogDir != "" && dst.LogDir != tt.wantLogDir {
				t.Errorf("LogDir = %q, want %q", dst.LogDir, tt.wantLogDir)
			}
			if tt.wantRetryCount != 0 && dst.RetryCount != tt.wantRetryCount {
				t.Errorf("RetryCount = %d, want %d", dst.RetryCount, tt.wantRetryCount)
			}
			if tt.wantTimeout != 0 && dst.DefaultTimeoutSeconds != tt.wantTimeout {
				t.Errorf("DefaultTimeoutSeconds = %d, want %d", dst.DefaultTimeoutSeconds, tt.wantTimeout)
			}
			if tt.wantScope != "" && dst.DefaultScope != tt.wantScope {
				t.Errorf("DefaultScope = %q, want %q", dst.DefaultScope, tt.wantScope)
			}
			if tt.wantExtMode != "" && dst.ExtensionMode != tt.wantExtMode {
				t.Errorf("ExtensionMode = %q, want %q", dst.ExtensionMode, tt.wantExtMode)
			}
			if dst.UpgradeIfInstalled != tt.wantUpgradeEnabled {
				t.Errorf("UpgradeIfInstalled = %v, want %v", dst.UpgradeIfInstalled, tt.wantUpgradeEnabled)
			}
		})
	}
}

func TestMergePackages(t *testing.T) {
	tests := []struct {
		name        string
		base        []Package
		src         []Package
		wantIDs     []string // expected order
		wantUpdated map[string]string // id -> expected Name after merge
	}{
		{
			name:    "empty base and src",
			base:    nil,
			src:     nil,
			wantIDs: []string{},
		},
		{
			name:    "src into empty base",
			base:    nil,
			src:     []Package{{ID: "A", Name: "Alpha"}},
			wantIDs: []string{"A"},
		},
		{
			name:    "base with no overlap",
			base:    []Package{{ID: "A", Name: "Alpha"}},
			src:     []Package{{ID: "B", Name: "Beta"}},
			wantIDs: []string{"A", "B"},
		},
		{
			name:        "last-wins on ID collision",
			base:        []Package{{ID: "A", Name: "Old Name"}},
			src:         []Package{{ID: "A", Name: "New Name"}},
			wantIDs:     []string{"A"},
			wantUpdated: map[string]string{"A": "New Name"},
		},
		{
			name: "position preserved on collision — colliding ID stays at original index",
			base: []Package{
				{ID: "A", Name: "Alpha"},
				{ID: "B", Name: "Beta"},
				{ID: "C", Name: "Gamma"},
			},
			src: []Package{
				{ID: "B", Name: "Beta v2"},
			},
			wantIDs:     []string{"A", "B", "C"},
			wantUpdated: map[string]string{"B": "Beta v2"},
		},
		{
			name: "new src item appended after base",
			base: []Package{
				{ID: "A", Name: "Alpha"},
			},
			src: []Package{
				{ID: "A", Name: "Alpha v2"},
				{ID: "D", Name: "Delta"},
			},
			wantIDs:     []string{"A", "D"},
			wantUpdated: map[string]string{"A": "Alpha v2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergePackages(tt.base, tt.src)

			// Verify order
			if len(tt.wantIDs) == 0 && len(got) != 0 {
				t.Fatalf("want empty result, got %d items", len(got))
			}
			gotIDs := make([]string, len(got))
			for i, p := range got {
				gotIDs[i] = p.ID
			}
			if len(tt.wantIDs) > 0 {
				if len(got) != len(tt.wantIDs) {
					t.Fatalf("len(result) = %d, want %d", len(got), len(tt.wantIDs))
				}
				for i, id := range tt.wantIDs {
					if gotIDs[i] != id {
						t.Errorf("position %d: want ID %q, got order %v", i, id, gotIDs)
					}
				}
			}

			// Verify updated names
			for id, wantName := range tt.wantUpdated {
				var found bool
				for _, p := range got {
					if p.ID == id {
						found = true
						if p.Name != wantName {
							t.Errorf("id %q: Name = %q, want %q", id, p.Name, wantName)
						}
					}
				}
				if !found {
					t.Errorf("id %q not found in result", id)
				}
			}
		})
	}
}

func TestMergeCommands(t *testing.T) {
	tests := []struct {
		name        string
		base        []Command
		src         []Command
		wantIDs     []string
		wantUpdated map[string]string // id -> expected Name
	}{
		{
			name:    "no overlap",
			base:    []Command{{ID: "a", Name: "A"}},
			src:     []Command{{ID: "b", Name: "B"}},
			wantIDs: []string{"a", "b"},
		},
		{
			name:        "last-wins on collision",
			base:        []Command{{ID: "x", Name: "Old"}},
			src:         []Command{{ID: "x", Name: "New"}},
			wantIDs:     []string{"x"},
			wantUpdated: map[string]string{"x": "New"},
		},
		{
			name: "position preserved",
			base: []Command{
				{ID: "a", Name: "A"},
				{ID: "b", Name: "B"},
			},
			src: []Command{
				{ID: "a", Name: "A2"},
			},
			wantIDs:     []string{"a", "b"},
			wantUpdated: map[string]string{"a": "A2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeCommands(tt.base, tt.src)
			if len(got) != len(tt.wantIDs) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.wantIDs))
			}
			for i, id := range tt.wantIDs {
				if got[i].ID != id {
					t.Errorf("[%d] ID = %q, want %q", i, got[i].ID, id)
				}
			}
			for id, wantName := range tt.wantUpdated {
				for _, c := range got {
					if c.ID == id && c.Name != wantName {
						t.Errorf("id %q: Name = %q, want %q", id, c.Name, wantName)
					}
				}
			}
		})
	}
}

func TestMergeExtensions(t *testing.T) {
	tests := []struct {
		name        string
		base        []Extension
		src         []Extension
		wantIDs     []string
		wantUpdated map[string]string // id -> expected Name
	}{
		{
			name:    "no overlap",
			base:    []Extension{{ID: "ext-a", Name: "Ext A"}},
			src:     []Extension{{ID: "ext-b", Name: "Ext B"}},
			wantIDs: []string{"ext-a", "ext-b"},
		},
		{
			name:        "last-wins on collision",
			base:        []Extension{{ID: "ext-a", Name: "Old"}},
			src:         []Extension{{ID: "ext-a", Name: "New"}},
			wantIDs:     []string{"ext-a"},
			wantUpdated: map[string]string{"ext-a": "New"},
		},
		{
			name: "position preserved",
			base: []Extension{
				{ID: "ext-a", Name: "A"},
				{ID: "ext-b", Name: "B"},
			},
			src: []Extension{
				{ID: "ext-a", Name: "A2"},
				{ID: "ext-c", Name: "C"},
			},
			wantIDs:     []string{"ext-a", "ext-b", "ext-c"},
			wantUpdated: map[string]string{"ext-a": "A2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeExtensions(tt.base, tt.src)
			if len(got) != len(tt.wantIDs) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.wantIDs))
			}
			for i, id := range tt.wantIDs {
				if got[i].ID != id {
					t.Errorf("[%d] ID = %q, want %q", i, got[i].ID, id)
				}
			}
			for id, wantName := range tt.wantUpdated {
				for _, e := range got {
					if e.ID == id && e.Name != wantName {
						t.Errorf("id %q: Name = %q, want %q", id, e.Name, wantName)
					}
				}
			}
		})
	}
}
