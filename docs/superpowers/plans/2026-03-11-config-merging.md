# Config Merging Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `LoadAll(paths []string)` to the config package and wire `--config` as a repeatable CLI flag so users can layer multiple config files left-to-right with last-wins override semantics.

**Architecture:** A new `LoadAll` function reads each file, merges them in order (Settings: non-zero fields win; Packages/Commands/Extensions: last-wins by ID within same tier; Profiles: last-wins by name), then runs `validate()` and `applyDefaults()` once on the merged result. `Load` becomes a thin wrapper. The CLI flag changes from `StringVarP` to `StringArrayVarP`; both `runInstall` and `runStatus` call `LoadAll`.

**Tech Stack:** Go stdlib only — no new dependencies.

---

## File Map

| File | Action | What changes |
|---|---|---|
| `internal/config/loader.go` | Modify | Add `LoadAll()`, refactor `Load()` as thin wrapper |
| `internal/config/loader_test.go` | Create | All merge behaviour tests |
| `cmd/main.go` | Modify | `configPath string` → `configPaths []string`, `StringVarP` → `StringArrayVarP`, call `LoadAll` |
| `cmd/status.go` | Modify | Call `LoadAll(configPaths)` instead of `Load(configPath)` |

---

## Chunk 1: LoadAll + tests

### Task 1: Write failing tests for `LoadAll`

**Files:**
- Create: `internal/config/loader_test.go`

> **Context:** The existing `Load(path string)` function in `internal/config/loader.go` reads one JSON file, calls `validate()` and `applyDefaults()`, and returns `*Config`. You are adding `LoadAll(paths []string)` which merges multiple files. Tests go first.
>
> The `Config` struct (in `internal/config/schema.go`) has: `Version string`, `Metadata Metadata`, `Settings Settings`, `Packages []Package`, `Commands []Command`, `Extensions []Extension`, `Profiles []Profile`.
>
> `Settings` fields: `LogDir string`, `RetryCount int`, `DefaultTimeoutSeconds int`, `DefaultScope string`, `ExtensionMode string`, `UpgradeIfInstalled bool`.
>
> `Package` fields: `ID string` (unique key), `Name string`, `Phase int`, `Category string`, `Description string`, `Scope string`, `Check string`, `Version string`, `RebootAfter bool`, `TimeoutSeconds int`, `Notes string`.
>
> `Command` fields: `ID string`, `Name string`, `Phase int`, `Category string`, `Description string`, `Check string`, `Cmd string`, `DependsOn []string`, `RebootAfter bool`, `TimeoutSeconds int`, `OnFailurePrompt string`, `Notes string`.
>
> `Profile` fields: `Name string`, `IDs []string`.

- [ ] **Step 1: Create the test file with all test cases**

Create `internal/config/loader_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
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

// TestLoadAll_CrossTierIDCollision Package ID in one file matches Command ID in another — validation must catch it.
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
	_, err := LoadAll([]string{base, extras})
	if err == nil {
		t.Fatal("expected error for cross-tier ID collision (Package 'foo' vs Command 'foo'), got nil")
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
```

- [ ] **Step 2: Run tests to confirm they fail (function doesn't exist yet)**

```bash
cd /f/GDriveClone/Claude_Code/KtulueKit-W11/.worktrees/feat-config-merge
go test ./internal/config/... -run TestLoadAll -v 2>&1 | head -20
```

Expected: compile error — `LoadAll undefined`

---

### Task 2: Implement `LoadAll` and refactor `Load`

**Files:**
- Modify: `internal/config/loader.go`

- [ ] **Step 1: Add `LoadAll` and refactor `Load` as thin wrapper**

Replace the entire content of `internal/config/loader.go` with:

```go
package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Load reads and parses the single config file at path.
// It is a convenience wrapper around LoadAll.
func Load(path string) (*Config, error) {
	return LoadAll([]string{path})
}

// LoadAll merges one or more config files left-to-right and returns the combined Config.
// Later files override earlier files on ID/name collision (last-wins).
// validate() and applyDefaults() are called exactly once on the merged result.
// If paths is empty, it defaults to ["ktuluekit.json"].
func LoadAll(paths []string) (*Config, error) {
	if len(paths) == 0 {
		paths = []string{"ktuluekit.json"}
	}

	var merged Config

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("cannot read config file %q: %w", path, err)
		}

		var cfg Config
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("cannot parse config file %q: %w", path, err)
		}

		mergeInto(&merged, &cfg)
	}

	if err := validate(&merged); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	applyDefaults(&merged)
	return &merged, nil
}

// mergeInto applies src on top of dst using last-wins semantics.
func mergeInto(dst, src *Config) {
	// Metadata: first config is authoritative.
	if dst.Metadata.Name == "" {
		dst.Metadata = src.Metadata
	}

	// Version/Schema: first config is authoritative.
	if dst.Version == "" {
		dst.Version = src.Version
	}
	if dst.Schema == "" {
		dst.Schema = src.Schema
	}

	// Settings: non-zero src fields overwrite dst fields.
	mergeSettings(&dst.Settings, &src.Settings)

	// Packages: last-wins by ID, preserving first-seen position.
	dst.Packages = mergePackages(dst.Packages, src.Packages)

	// Commands: last-wins by ID, preserving first-seen position.
	dst.Commands = mergeCommands(dst.Commands, src.Commands)

	// Extensions: last-wins by ID, preserving first-seen position.
	dst.Extensions = mergeExtensions(dst.Extensions, src.Extensions)

	// Profiles: last-wins by Name.
	dst.Profiles = mergeProfiles(dst.Profiles, src.Profiles)
}

// mergeSettings overwrites dst fields with src fields where src is non-zero.
func mergeSettings(dst, src *Settings) {
	if src.LogDir != "" {
		dst.LogDir = src.LogDir
	}
	if src.RetryCount != 0 {
		dst.RetryCount = src.RetryCount
	}
	if src.DefaultTimeoutSeconds != 0 {
		dst.DefaultTimeoutSeconds = src.DefaultTimeoutSeconds
	}
	if src.DefaultScope != "" {
		dst.DefaultScope = src.DefaultScope
	}
	if src.ExtensionMode != "" {
		dst.ExtensionMode = src.ExtensionMode
	}
	if src.UpgradeIfInstalled {
		dst.UpgradeIfInstalled = src.UpgradeIfInstalled
	}
}

// mergePackages returns base with src entries merged in (last-wins by ID, position preserved).
func mergePackages(base, src []Package) []Package {
	// index maps ID → position in result slice.
	index := make(map[string]int, len(base))
	result := make([]Package, len(base))
	copy(result, base)
	for i, p := range result {
		index[p.ID] = i
	}
	for _, p := range src {
		if i, exists := index[p.ID]; exists {
			result[i] = p // override in place
		} else {
			index[p.ID] = len(result)
			result = append(result, p)
		}
	}
	return result
}

// mergeCommands returns base with src entries merged in (last-wins by ID, position preserved).
func mergeCommands(base, src []Command) []Command {
	index := make(map[string]int, len(base))
	result := make([]Command, len(base))
	copy(result, base)
	for i, c := range result {
		index[c.ID] = i
	}
	for _, c := range src {
		if i, exists := index[c.ID]; exists {
			result[i] = c
		} else {
			index[c.ID] = len(result)
			result = append(result, c)
		}
	}
	return result
}

// mergeExtensions returns base with src entries merged in (last-wins by ID, position preserved).
func mergeExtensions(base, src []Extension) []Extension {
	index := make(map[string]int, len(base))
	result := make([]Extension, len(base))
	copy(result, base)
	for i, e := range result {
		index[e.ID] = i
	}
	for _, e := range src {
		if i, exists := index[e.ID]; exists {
			result[i] = e
		} else {
			index[e.ID] = len(result)
			result = append(result, e)
		}
	}
	return result
}

// mergeProfiles returns base with src profiles merged in (last-wins by Name).
func mergeProfiles(base, src []Profile) []Profile {
	index := make(map[string]int, len(base))
	result := make([]Profile, len(base))
	copy(result, base)
	for i, p := range result {
		index[p.Name] = i
	}
	for _, p := range src {
		if i, exists := index[p.Name]; exists {
			result[i] = p
		} else {
			index[p.Name] = len(result)
			result = append(result, p)
		}
	}
	return result
}

// validate checks required fields and catches obvious mistakes.
func validate(cfg *Config) error {
	if cfg.Version == "" {
		return fmt.Errorf("missing required field: version")
	}
	if cfg.Metadata.Name == "" {
		return fmt.Errorf("missing required field: metadata.name")
	}

	ids := make(map[string]bool)

	for i, p := range cfg.Packages {
		if p.ID == "" {
			return fmt.Errorf("packages[%d]: missing required field 'id'", i)
		}
		if p.Name == "" {
			return fmt.Errorf("packages[%d] (%s): missing required field 'name'", i, p.ID)
		}
		if p.Phase < 1 {
			return fmt.Errorf("packages[%d] (%s): phase must be >= 1", i, p.ID)
		}
		if ids[p.ID] {
			return fmt.Errorf("packages[%d]: duplicate id %q", i, p.ID)
		}
		ids[p.ID] = true
	}

	for i, c := range cfg.Commands {
		if c.ID == "" {
			return fmt.Errorf("commands[%d]: missing required field 'id'", i)
		}
		if c.Name == "" {
			return fmt.Errorf("commands[%d] (%s): missing required field 'name'", i, c.ID)
		}
		if c.Phase < 1 {
			return fmt.Errorf("commands[%d] (%s): phase must be >= 1", i, c.ID)
		}
		if c.Check == "" {
			return fmt.Errorf("commands[%d] (%s): missing required field 'check'", i, c.ID)
		}
		if c.Cmd == "" {
			return fmt.Errorf("commands[%d] (%s): missing required field 'command'", i, c.ID)
		}
		if ids[c.ID] {
			return fmt.Errorf("commands[%d]: duplicate id %q", i, c.ID)
		}
		ids[c.ID] = true
	}

	for i, e := range cfg.Extensions {
		if e.ID == "" {
			return fmt.Errorf("extensions[%d]: missing required field 'id'", i)
		}
		if e.Name == "" {
			return fmt.Errorf("extensions[%d] (%s): missing required field 'name'", i, e.ID)
		}
		if e.Phase < 1 {
			return fmt.Errorf("extensions[%d] (%s): phase must be >= 1", i, e.ID)
		}
		if e.ExtensionID == "" {
			return fmt.Errorf("extensions[%d] (%s): missing required field 'extension_id'", i, e.ID)
		}
		if len(e.ExtensionID) != 32 {
			return fmt.Errorf("extensions[%d] (%s): extension_id must be 32 characters", i, e.ID)
		}
		if ids[e.ID] {
			return fmt.Errorf("extensions[%d]: duplicate id %q", i, e.ID)
		}
		ids[e.ID] = true
	}

	return nil
}

// applyDefaults fills in zero-value fields from settings.
func applyDefaults(cfg *Config) {
	if cfg.Settings.LogDir == "" {
		cfg.Settings.LogDir = "./logs"
	}
	if cfg.Settings.RetryCount == 0 {
		cfg.Settings.RetryCount = 1
	}
	if cfg.Settings.DefaultTimeoutSeconds == 0 {
		cfg.Settings.DefaultTimeoutSeconds = 300
	}
	if cfg.Settings.DefaultScope == "" {
		cfg.Settings.DefaultScope = "machine"
	}
	if cfg.Settings.ExtensionMode == "" {
		cfg.Settings.ExtensionMode = "url"
	}

	for i := range cfg.Packages {
		if cfg.Packages[i].Scope == "" {
			cfg.Packages[i].Scope = cfg.Settings.DefaultScope
		}
		if cfg.Packages[i].TimeoutSeconds == 0 {
			cfg.Packages[i].TimeoutSeconds = cfg.Settings.DefaultTimeoutSeconds
		}
	}

	for i := range cfg.Commands {
		if cfg.Commands[i].TimeoutSeconds == 0 {
			cfg.Commands[i].TimeoutSeconds = cfg.Settings.DefaultTimeoutSeconds
		}
	}

	for i := range cfg.Extensions {
		if cfg.Extensions[i].Mode == "" {
			cfg.Extensions[i].Mode = cfg.Settings.ExtensionMode
		}
	}
}
```

> **Note on `Schema` field:** `Config` has `Schema string \`json:"$schema"\``. You'll need to verify the exact field name in `internal/config/schema.go` and update the `mergeInto` function if the field name differs.

- [ ] **Step 2: Run tests**

```bash
go test ./internal/config/... -run TestLoadAll -v
```

Expected: all `TestLoadAll_*` tests PASS.

- [ ] **Step 3: Run full config test suite to catch regressions**

```bash
go test ./internal/config/... -v
```

Expected: all tests PASS (no regressions to existing config loading).

- [ ] **Step 4: Commit**

```bash
git add internal/config/loader.go internal/config/loader_test.go
git commit -m "feat(config): add LoadAll for multi-file merging; Load becomes thin wrapper"
```

---

## Chunk 2: CLI wiring

### Task 3: Wire `--config` as repeatable flag and update call sites

**Files:**
- Modify: `cmd/main.go` (lines ~18-37 for var declaration and flag registration; line ~64 for Load call)
- Modify: `cmd/status.go` (line 24 for Load call)

> **Context:** `cmd/main.go` declares `configPath string` as a package-level var and registers it via `root.PersistentFlags().StringVarP(...)`. `runInstall` calls `config.Load(configPath)`. `cmd/status.go`'s `runStatus` also calls `config.Load(configPath)` using the same package-level var (it's shared because it's a persistent flag on the root command).

- [ ] **Step 1: Update `cmd/main.go`**

Change the var block — rename `configPath string` to `configPaths []string`:

```go
var (
	configPaths        []string
	dryRun             bool
	resumePhase        int
	noDesktopShortcuts bool
)
```

Change the flag registration (find `StringVarP` for config and replace):

```go
root.PersistentFlags().StringArrayVarP(&configPaths, "config", "c", nil, "Path to config file (repeatable: --config base.json --config extras.json)")
```

Change the `config.Load` call in `runInstall` (find `cfg, err := config.Load(configPath)` and replace):

```go
cfg, err := config.LoadAll(configPaths)
```

- [ ] **Step 2: Update `cmd/status.go`**

Change `config.Load(configPath)` to `config.LoadAll(configPaths)` (line 24):

```go
cfg, err := config.LoadAll(configPaths)
```

- [ ] **Step 3: Build to catch compile errors**

```bash
go build ./cmd/...
```

Expected: builds cleanly with exit 0.

- [ ] **Step 4: Smoke test — single config (default behaviour unchanged)**

```bash
./ktuluekit --dry-run 2>&1 | head -5
```

Expected: runs exactly as before, shows package count.

- [ ] **Step 5: Smoke test — help text shows new flag description**

```bash
./ktuluekit --help | grep config
```

Expected: shows `--config` (repeatable) description.

- [ ] **Step 6: Smoke test — `status` with two configs**

```bash
./ktuluekit status --config ktuluekit.json --config /tmp/extras-test.json 2>&1 | head -5
```

Expected: runs cleanly, shows status table with merged items. This verifies the persistent flag reaches `runStatus` correctly.

- [ ] **Step 7: Smoke test — two configs (install path)**

Create a minimal extras file, then test:

```bash
cat > /tmp/extras-test.json << 'EOF'
{
  "version": "1.0",
  "metadata": {"name": "Extras"},
  "settings": {"retry_count": 3},
  "packages": []
}
EOF
./ktuluekit --config ktuluekit.json --config /tmp/extras-test.json --dry-run 2>&1 | head -5
```

Expected: runs cleanly, no errors.

- [ ] **Step 8: Run all tests**

```bash
go test ./internal/... ./cmd/... 2>&1
```

Expected: all pass.

- [ ] **Step 9: Commit**

```bash
git add cmd/main.go cmd/status.go
git commit -m "feat(cli): make --config repeatable for multi-file merging"
```

- [ ] **Step 10: Update TODO**

In `TODO.md`, mark the config merging item as done:

```
- [x] **Config merging** — `--config base.json --config extras.json` layers multiple configs. Last-wins by ID/name. `LoadAll` in config package; `--config` is now a repeatable flag.
```

```bash
git add TODO.md
git commit -m "chore: mark config merging as done in TODO"
```
