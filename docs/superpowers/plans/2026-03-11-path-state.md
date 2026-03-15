# PATH Verification & State File Relocation Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move the state file to `%LOCALAPPDATA%\KtulueKit\state.json` with lazy migration, and add a post-PATH-refresh warning when runtime tools are missing.

**Architecture:** Two independent changes. State relocation is entirely contained in `internal/state/state.go` — the `Load`/`Save`/`Clear` signatures are unchanged so no call sites need touching. PATH verification adds a pure `VerifyRuntimePaths() []string` function in a new `internal/installer/path_check.go`, called from the runner's `pathRefreshed` block wrapped in `if !r.dryRun`.

**Tech Stack:** Go 1.25 standard library (`os/exec.LookPath`, `os.MkdirAll`, `os.Getenv`, `t.Setenv` for test isolation).

---

## Chunk 1: State File Relocation

### Task 1: Relocate state file with lazy migration

**Files:**
- Modify: `internal/state/state.go`

**Context:** `state.go` currently uses `const stateFile = ".ktuluekit-state.json"` (CWD-relative). All reads/writes/deletes use this constant. The three call sites (`cmd/main.go`, `app.go`, `cmd/status.go`) call `state.Load()` and `state.Clear()` — their signatures must not change.

The resolved path logic:
1. `LOCALAPPDATA` empty → use CWD path for everything (legacy behavior)
2. New path exists → load it, done
3. CWD path exists → load it, write to new path, delete CWD path
4. Neither → return fresh `State{}`

All tests must use `t.Setenv("LOCALAPPDATA", t.TempDir())` to avoid writing to the real user profile.

---

- [ ] **Step 1: Write failing tests** — create `internal/state/state_test.go`:

```go
package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// newPath returns the resolved state path under the given LOCALAPPDATA dir.
// Mirrors the internal statePath() logic for test assertions.
func resolvedNewPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(os.Getenv("LOCALAPPDATA"), "KtulueKit", "state.json")
}

func TestLoad_FreshState(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	s, err := Load()
	if err != nil {
		t.Fatalf("Load() on empty dir: %v", err)
	}
	if len(s.Succeeded) != 0 || len(s.Failed) != 0 {
		t.Error("expected empty state on first load")
	}
}

func TestLoad_UsesNewPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dir)

	newPath := resolvedNewPath(t)
	if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(&State{
		Succeeded: map[string]bool{"pkg1": true},
		Failed:    make(map[string]bool),
	})
	os.WriteFile(newPath, data, 0644)

	s, err := Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if !s.Succeeded["pkg1"] {
		t.Error("expected pkg1 in Succeeded")
	}
}

func TestLoad_MigratesLegacy(t *testing.T) {
	// NOTE: must not call t.Parallel() — uses os.Chdir which is process-global.
	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dir)

	// Write legacy CWD file in a temp dir acting as CWD
	cwdDir := t.TempDir()
	legacyPath := filepath.Join(cwdDir, legacyStateFile)
	data, _ := json.Marshal(&State{
		Succeeded: map[string]bool{"legacy-pkg": true},
		Failed:    make(map[string]bool),
	})
	os.WriteFile(legacyPath, data, 0644)

	// Override CWD resolution for this test
	origDir, _ := os.Getwd()
	os.Chdir(cwdDir)
	defer os.Chdir(origDir)

	s, err := Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if !s.Succeeded["legacy-pkg"] {
		t.Error("expected legacy-pkg migrated to Succeeded")
	}

	// New path must exist
	newPath := resolvedNewPath(t)
	if _, err := os.Stat(newPath); os.IsNotExist(err) {
		t.Error("expected new path to be written after migration")
	}

	// Old CWD file must be deleted
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Error("expected legacy CWD file to be deleted after migration")
	}
}

func TestLoad_NewPathTakesPrecedence(t *testing.T) {
	// NOTE: must not call t.Parallel() — uses os.Chdir which is process-global.
	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dir)

	// Write new path file
	newPath := resolvedNewPath(t)
	os.MkdirAll(filepath.Dir(newPath), 0755)
	newData, _ := json.Marshal(&State{Succeeded: map[string]bool{"new-pkg": true}, Failed: make(map[string]bool)})
	os.WriteFile(newPath, newData, 0644)

	// Write legacy CWD file
	cwdDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(cwdDir)
	defer os.Chdir(origDir)
	legacyPath := filepath.Join(cwdDir, legacyStateFile)
	oldData, _ := json.Marshal(&State{Succeeded: map[string]bool{"old-pkg": true}, Failed: make(map[string]bool)})
	os.WriteFile(legacyPath, oldData, 0644)

	s, err := Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if !s.Succeeded["new-pkg"] {
		t.Error("expected new-pkg from new path")
	}
	if s.Succeeded["old-pkg"] {
		t.Error("old-pkg from legacy path should not be loaded when new path exists")
	}
	// Legacy file must be untouched
	if _, err := os.Stat(legacyPath); os.IsNotExist(err) {
		t.Error("legacy file should be untouched when new path takes precedence")
	}
}

func TestLoad_EmptyLOCALAPPDATA(t *testing.T) {
	// NOTE: must not call t.Parallel() — uses os.Chdir which is process-global.
	t.Setenv("LOCALAPPDATA", "")

	cwdDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(cwdDir)
	defer os.Chdir(origDir)

	legacyPath := filepath.Join(cwdDir, legacyStateFile)
	data, _ := json.Marshal(&State{Succeeded: map[string]bool{"cwd-pkg": true}, Failed: make(map[string]bool)})
	os.WriteFile(legacyPath, data, 0644)

	s, err := Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if !s.Succeeded["cwd-pkg"] {
		t.Error("expected CWD fallback when LOCALAPPDATA is empty")
	}
}

func TestStatePath_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dir)

	s := &State{Succeeded: map[string]bool{"x": true}, Failed: make(map[string]bool)}
	if err := s.Save(); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	newPath := resolvedNewPath(t)
	if _, err := os.Stat(newPath); os.IsNotExist(err) {
		t.Error("Save() should create directory and write file")
	}
}
```

- [ ] **Step 2: Run to confirm they fail**

```
cd /f/GDriveClone/Claude_Code/KtulueKit-W11 && go test ./internal/state/... -v
```
Expected: compile errors — `legacyStateFile` undefined, or tests fail because Load/Save still use the old CWD path.

- [ ] **Step 3: Implement the relocation in `state.go`**

Replace the entire contents of `internal/state/state.go` with:

```go
package state

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// legacyStateFile is the old CWD-relative state path, kept for migration.
const legacyStateFile = ".ktuluekit-state.json"

// State tracks which item IDs have completed, so the tool can resume after reboot.
type State struct {
	Succeeded   map[string]bool `json:"succeeded"`
	Failed      map[string]bool `json:"failed"`
	ResumePhase int             `json:"resume_phase,omitempty"`
}

// statePath returns the resolved path for the state file.
// Primary: %LOCALAPPDATA%\KtulueKit\state.json
// Fallback (LOCALAPPDATA empty): CWD-relative .ktuluekit-state.json
func statePath() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		return legacyStateFile
	}
	return filepath.Join(base, "KtulueKit", "state.json")
}

func Load() (*State, error) {
	s := &State{
		Succeeded: make(map[string]bool),
		Failed:    make(map[string]bool),
	}

	base := os.Getenv("LOCALAPPDATA")

	// If LOCALAPPDATA is available, try new path first.
	if base != "" {
		newPath := statePath()
		data, err := os.ReadFile(newPath)
		if err == nil {
			if err := json.Unmarshal(data, s); err != nil {
				return nil, err
			}
			return s, nil
		}
		if !os.IsNotExist(err) {
			return nil, err
		}
	}

	// Try legacy CWD path (also the only path when LOCALAPPDATA is empty).
	data, err := os.ReadFile(legacyStateFile)
	if os.IsNotExist(err) {
		return s, nil // fresh run
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, err
	}

	// Migrate: write to new path and delete legacy file (only when LOCALAPPDATA is set).
	if base != "" {
		if writeErr := s.Save(); writeErr == nil {
			_ = os.Remove(legacyStateFile) // best-effort; orphan is harmless
		}
	}

	return s, nil
}

func (s *State) Save() error {
	path := statePath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (s *State) MarkSucceeded(id string) {
	s.Succeeded[id] = true
	delete(s.Failed, id)
	_ = s.Save()
}

func (s *State) MarkFailed(id string) {
	s.Failed[id] = true
	_ = s.Save()
}

// SaveResumePhase records the next phase to start from and persists state.
func (s *State) SaveResumePhase(phase int) error {
	s.ResumePhase = phase
	return s.Save()
}

// Clear deletes the state file after a clean run completes.
func Clear() error {
	err := os.Remove(statePath())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
```

- [ ] **Step 4: Run the state tests**

```
cd /f/GDriveClone/Claude_Code/KtulueKit-W11 && go test ./internal/state/... -v
```
Expected: all 6 tests PASS.

- [ ] **Step 5: Run the full test suite to check for regressions**

```
go test ./internal/... ./cmd/...
```
Expected: all PASS. (The call sites in `cmd/main.go`, `app.go`, `cmd/status.go` use the same `state.Load()` / `state.Clear()` signatures — no changes needed there.)

- [ ] **Step 6: Commit**

```
git add internal/state/state.go internal/state/state_test.go
git commit -m "feat(state): relocate state file to %LOCALAPPDATA%\\KtulueKit with lazy migration"
```

---

## Chunk 2: PATH Verification

### Task 2: `VerifyRuntimePaths` pure function

**Files:**
- Create: `internal/installer/path_check.go`
- Create: `internal/installer/path_check_test.go`

**Context:** `exec.LookPath(name)` returns an error when the tool is not found on PATH. This is a pure PATH scan — no subprocess is launched. The function returns the names of missing tools (empty slice = all present).

---

- [ ] **Step 1: Write failing tests** — create `internal/installer/path_check_test.go`:

```go
package installer

import (
	"os"
	"testing"
)

func TestVerifyRuntimePaths_AllPresent(t *testing.T) {
	// Point PATH at a temp dir containing stub executables for all required tools.
	dir := t.TempDir()
	tools := []string{"git", "node", "python", "go", "rustup", "pwsh"}
	for _, name := range tools {
		f, err := os.Create(dir + "/" + name + ".exe")
		if err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
	t.Setenv("PATH", dir)

	missing := VerifyRuntimePaths()
	if len(missing) != 0 {
		t.Errorf("expected no missing tools, got: %v", missing)
	}
}

func TestVerifyRuntimePaths_SomeMissing(t *testing.T) {
	// Point PATH at a temp dir that has only git and node.
	dir := t.TempDir()
	for _, name := range []string{"git", "node"} {
		f, _ := os.Create(dir + "/" + name + ".exe")
		f.Close()
	}
	t.Setenv("PATH", dir)

	missing := VerifyRuntimePaths()
	if len(missing) == 0 {
		t.Fatal("expected some missing tools, got none")
	}
	// python, go, rustup, pwsh should be missing
	missingSet := make(map[string]bool)
	for _, m := range missing {
		missingSet[m] = true
	}
	for _, expected := range []string{"python", "go", "rustup", "pwsh"} {
		if !missingSet[expected] {
			t.Errorf("expected %q in missing list, got %v", expected, missing)
		}
	}
}

func TestVerifyRuntimePaths_NonePresent(t *testing.T) {
	dir := t.TempDir() // empty dir — nothing on PATH
	t.Setenv("PATH", dir)

	missing := VerifyRuntimePaths()
	if len(missing) != 6 {
		t.Errorf("expected 6 missing tools, got %d: %v", len(missing), missing)
	}
}
```

- [ ] **Step 2: Run to confirm they fail**

```
cd /f/GDriveClone/Claude_Code/KtulueKit-W11 && go test ./internal/installer/... -run TestVerifyRuntimePaths -v
```
Expected: compile error — `VerifyRuntimePaths undefined`.

- [ ] **Step 3: Implement `VerifyRuntimePaths`** — create `internal/installer/path_check.go`:

```go
package installer

import "os/exec"

// runtimeTools is the fixed list of tools checked after PATH refresh.
// These are the runtimes most likely to require a PATH update after winget install
// and to be depended on by Tier 2 commands.
var runtimeTools = []string{"git", "node", "python", "go", "rustup", "pwsh"}

// VerifyRuntimePaths checks whether each required runtime tool is findable on PATH.
// Returns a slice of tool names that are missing. An empty slice means all are present.
func VerifyRuntimePaths() []string {
	var missing []string
	for _, tool := range runtimeTools {
		if _, err := exec.LookPath(tool); err != nil {
			missing = append(missing, tool)
		}
	}
	return missing
}
```

- [ ] **Step 4: Run the tests**

```
cd /f/GDriveClone/Claude_Code/KtulueKit-W11 && go test ./internal/installer/... -run TestVerifyRuntimePaths -v
```
Expected: all 3 PASS.

- [ ] **Step 5: Commit**

```
git add internal/installer/path_check.go internal/installer/path_check_test.go
git commit -m "feat(installer): add VerifyRuntimePaths for post-refresh PATH check"
```

---

### Task 3: Wire PATH check into runner

**Files:**
- Modify: `internal/installer/path_check.go` (add `RuntimeTools()` export)
- Modify: `internal/runner/runner.go`

**Context:** The `pathRefreshed` block sits inside the phase loop in `Run()` at lines ~250-253:

```go
if !pathRefreshed && phase >= r.firstCommandPhase() {
    installer.RefreshPath()
    pathRefreshed = true
}
```

The runner needs `installer.RuntimeTools()` to compute which tools are present (vs missing). That export must be added to `path_check.go` first, then the runner can call it. `"strings"` is already imported in `runner.go` (used by `strings.Split` / `strings.Join` elsewhere). No new imports needed.

---

- [ ] **Step 1: Add `RuntimeTools()` to `path_check.go`** — replace the full contents of `internal/installer/path_check.go`:

```go
package installer

import "os/exec"

// runtimeTools is the fixed list of tools checked after PATH refresh.
// These are the runtimes most likely to require a PATH update after winget install
// and to be depended on by Tier 2 commands.
var runtimeTools = []string{"git", "node", "python", "go", "rustup", "pwsh"}

// RuntimeTools returns the fixed list of tools checked by VerifyRuntimePaths.
// Exported so callers can compute the present set without re-running LookPath.
func RuntimeTools() []string {
	return append([]string(nil), runtimeTools...)
}

// VerifyRuntimePaths checks whether each required runtime tool is findable on PATH.
// Returns a slice of tool names that are missing. An empty slice means all are present.
func VerifyRuntimePaths() []string {
	var missing []string
	for _, tool := range runtimeTools {
		if _, err := exec.LookPath(tool); err != nil {
			missing = append(missing, tool)
		}
	}
	return missing
}
```

- [ ] **Step 2: Update the `pathRefreshed` block in `runner.go`** — replace:

```go
if !pathRefreshed && phase >= r.firstCommandPhase() {
    installer.RefreshPath()
    pathRefreshed = true
}
```

with:

```go
if !pathRefreshed && phase >= r.firstCommandPhase() {
    installer.RefreshPath()
    pathRefreshed = true
    if !r.dryRun {
        missing := installer.VerifyRuntimePaths()
        missingSet := make(map[string]bool, len(missing))
        for _, m := range missing {
            missingSet[m] = true
        }
        var present []string
        for _, tool := range installer.RuntimeTools() {
            if !missingSet[tool] {
                present = append(present, tool)
            }
        }
        if len(missing) == 0 {
            fmt.Printf("  %s[OK]%s  All runtime tools found on PATH.\n", colorGreen, colorReset)
        } else {
            fmt.Println("  PATH check after refresh:")
            if len(present) > 0 {
                fmt.Printf("    %s[OK]%s    %s\n", colorGreen, colorReset, strings.Join(present, ", "))
            }
            for _, m := range missing {
                fmt.Printf("    %s[WARN]%s  %s — not found on PATH (install may not have completed)\n", colorYellow, colorReset, m)
            }
        }
    }
}
```

- [ ] **Step 3: Build to catch any issues**

```
cd /f/GDriveClone/Claude_Code/KtulueKit-W11 && go build ./...
```
Expected: no errors.

- [ ] **Step 4: Run full test suite**

```
go test ./internal/... ./cmd/...
```
Expected: all PASS.

- [ ] **Step 5: Commit**

```
git add internal/runner/runner.go internal/installer/path_check.go
git commit -m "feat(runner): add PATH verification after RefreshPath"
```

---

## Final: Mark TODO.md items done

- [ ] **Step 1: Update `TODO.md`** — mark both items done:
  - `**PATH verification post-install**`
  - `**State file relocation**`

- [ ] **Step 2: Run final test suite**

```
go test ./internal/... ./cmd/...
```
Expected: all PASS.

- [ ] **Step 3: Commit**

```
git add TODO.md
git commit -m "chore(todo): mark PATH verification and state relocation as done"
```
