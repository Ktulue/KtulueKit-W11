# Uninstall Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add config-scoped uninstall capability — CLI `ktuluekit uninstall` and GUI Install/Uninstall tabs — covering all four install tiers plus a latent bug fix.

**Architecture:** Schema gains `UninstallCmd` on `Command`. State gains `DeleteSucceeded`. Each installer file gains an uninstall function. Runner gains `RunUninstall`. App gains `StartUninstall` and the latent `SetPauseResponse` bug fix wired into both `StartInstall` and `StartUninstall`. CLI gains the `uninstall` subcommand. GUI adds Install/Uninstall tab bar to `SelectionScreen.svelte`.

**Tech Stack:** Go 1.25, Cobra CLI, Wails v2, Svelte 4, `golang.org/x/sys/windows/registry`.

---

## Branch Setup

```bash
git checkout main && git pull
git checkout -b feat/uninstall
```

---

## File Map

| File | Change |
|---|---|
| `internal/config/schema.go` | Add `UninstallCmd string` to `Command` |
| `internal/state/state.go` | Add `DeleteSucceeded(id string)` |
| `internal/installer/winget.go` | Add `UninstallPackage()` |
| `internal/installer/command.go` | Add `RunUninstallCommand()` |
| `internal/installer/extension.go` | Add `UninstallExtension()` (force-mode registry removal + renumber) |
| `internal/runner/runner.go` | Add `RunUninstall()` method |
| `cmd/uninstall.go` | New file — `uninstall` cobra subcommand |
| `cmd/main.go` | No changes needed — `rootCmd.AddCommand(uninstallCmd)` is called in `cmd/uninstall.go`'s `init()` which Cobra picks up automatically |
| `app.go` | Add `StartUninstall()` + `GetInstalledItems()`; fix `SetPauseResponse` latent bug in `StartInstall` |
| `frontend/src/screens/SelectionScreen.svelte` | Add Install/Uninstall tab bar; uninstall scan + UI |
| `frontend/src/components/ItemRow.svelte` | Add `mode` prop for install/uninstall accent color |

---

## Chunk 1: Backend — Schema, State, Installer Functions

### Task 1: Add `UninstallCmd` to `config.Command` schema

**Files:**
- Modify: `internal/config/schema.go`
- Test: `internal/config/schema_test.go` (create if absent)

- [ ] **Step 1: Write failing test** — create `internal/config/schema_test.go`:

```go
package config

import (
	"encoding/json"
	"testing"
)

func TestCommandUninstallCmdFieldRoundTrip(t *testing.T) {
	input := `{"id":"x","name":"X","command":"install.exe","uninstall_cmd":"uninstall.exe"}`
	var cmd Command
	if err := json.Unmarshal([]byte(input), &cmd); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if cmd.UninstallCmd != "uninstall.exe" {
		t.Errorf("UninstallCmd = %q, want %q", cmd.UninstallCmd, "uninstall.exe")
	}
	out, err := json.Marshal(cmd)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var cmd2 Command
	if err := json.Unmarshal(out, &cmd2); err != nil {
		t.Fatalf("re-unmarshal error: %v", err)
	}
	if cmd2.UninstallCmd != "uninstall.exe" {
		t.Errorf("after round-trip UninstallCmd = %q, want %q", cmd2.UninstallCmd, "uninstall.exe")
	}
}

func TestCommandUninstallCmdOmitted(t *testing.T) {
	input := `{"id":"x","name":"X","command":"install.exe"}`
	var cmd Command
	if err := json.Unmarshal([]byte(input), &cmd); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if cmd.UninstallCmd != "" {
		t.Errorf("UninstallCmd should be empty when omitted, got %q", cmd.UninstallCmd)
	}
}
```

- [ ] **Step 2: Run to confirm it fails**

```
cd /f/GDriveClone/Claude_Code/KtulueKit-W11 && go test ./internal/config/... -run "TestCommandUninstallCmd" -v
```

Expected: compile error — `UninstallCmd` field does not exist.

- [ ] **Step 3: Add `UninstallCmd` field to `Command` struct** in `internal/config/schema.go`, directly after the `Cmd` field:

```go
Cmd             string   `json:"command"`
UninstallCmd    string   `json:"uninstall_cmd"`
DependsOn       []string `json:"depends_on"`
```

- [ ] **Step 4: Run to confirm tests pass**

```
cd /f/GDriveClone/Claude_Code/KtulueKit-W11 && go test ./internal/config/... -run "TestCommandUninstallCmd" -v
```

Expected: both PASS.

- [ ] **Step 5: Build check**

```
cd /f/GDriveClone/Claude_Code/KtulueKit-W11 && go build ./...
```

- [ ] **Step 6: Commit**

```
git add internal/config/schema.go internal/config/schema_test.go
git commit -m "feat(config): add uninstall_cmd field to Command struct"
```

---

### Task 2: Add `DeleteSucceeded` to `internal/state/state.go`

**Files:**
- Modify: `internal/state/state.go`
- Test: `internal/state/state_test.go` (create if absent)

**Context:** `DeleteSucceeded` removes an ID from `Succeeded` map and persists state. Does NOT touch `Failed`.

- [ ] **Step 1: Write failing test** — create `internal/state/state_test.go`:

```go
package state

import (
	"testing"
)

func TestDeleteSucceeded_RemovesFromSucceededMap(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dir)

	s := &State{
		Succeeded: map[string]bool{"Git.Git": true, "Steam.Steam": true},
		Failed:    map[string]bool{"BadPkg": true},
	}
	if err := s.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	s.DeleteSucceeded("Git.Git")

	if s.Succeeded["Git.Git"] {
		t.Error("Git.Git should have been removed from Succeeded")
	}
	if !s.Succeeded["Steam.Steam"] {
		t.Error("Steam.Steam should still be in Succeeded")
	}
	if !s.Failed["BadPkg"] {
		t.Error("Failed map should be unchanged")
	}

	s2, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if s2.Succeeded["Git.Git"] {
		t.Error("Git.Git should not appear in reloaded state")
	}
	if !s2.Succeeded["Steam.Steam"] {
		t.Error("Steam.Steam should persist in reloaded state")
	}
}

func TestDeleteSucceeded_NoopForUnknownID(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dir)

	s := &State{
		Succeeded: map[string]bool{"A": true},
		Failed:    map[string]bool{},
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	s.DeleteSucceeded("DoesNotExist")
	if !s.Succeeded["A"] {
		t.Error("existing entry A should be unaffected")
	}
}
```

- [ ] **Step 2: Run to confirm it fails**

```
cd /f/GDriveClone/Claude_Code/KtulueKit-W11 && go test ./internal/state/... -run "TestDeleteSucceeded" -v
```

Expected: compile error — `DeleteSucceeded` undefined.

- [ ] **Step 3: Implement `DeleteSucceeded`** — add to `internal/state/state.go` after `MarkFailed`:

```go
// DeleteSucceeded removes id from the Succeeded map and persists state.
// Used after a successful uninstall. No-op if id is not present.
func (s *State) DeleteSucceeded(id string) {
	delete(s.Succeeded, id)
	_ = s.Save()
}
```

- [ ] **Step 4: Run to confirm tests pass**

```
cd /f/GDriveClone/Claude_Code/KtulueKit-W11 && go test ./internal/state/... -v
```

- [ ] **Step 5: Commit**

```
git add internal/state/state.go internal/state/state_test.go
git commit -m "feat(state): add DeleteSucceeded method for uninstall tracking"
```

---

### Task 3: `UninstallPackage` in `internal/installer/winget.go`

**Files:**
- Modify: `internal/installer/winget.go`
- Test: `internal/installer/winget_test.go` (create)

**Context:** Runs `winget uninstall -e --id <ID>` via existing `runWithTimeout`. If `pkg.Check` exits non-zero (not installed) → `StatusSkipped`. Dry-run → `StatusDryRun`. Success → `StatusInstalled` (reused as "Removed"; runner relabels in events).

Add after `InstallPackage`:

```go
// UninstallPackage runs winget uninstall for a Tier 1 package.
// If pkg.Check exits non-zero (not installed), returns StatusSkipped.
// If pkg.Check is empty, runs unconditionally (winget handles "not found").
func UninstallPackage(pkg config.Package, dryRun bool) reporter.Result {
	res := reporter.Result{ID: pkg.ID, Name: pkg.Name, Tier: "winget"}

	if dryRun {
		res.Status = reporter.StatusDryRun
		res.Detail = fmt.Sprintf("winget uninstall -e --id %s", pkg.ID)
		return res
	}

	if pkg.Check != "" && !isAlreadyInstalled(pkg.Check) {
		res.Status = reporter.StatusSkipped
		res.Detail = "not detected as installed — skipping"
		return res
	}

	args := []string{
		"uninstall", "-e", "--id", pkg.ID,
		"--accept-source-agreements", "--disable-interactivity",
	}

	exitCode, err := runWithTimeout(args, pkg.TimeoutSeconds)
	res.ExitCode = exitCode
	if exitCode == 0 && err == nil {
		res.Status = reporter.StatusInstalled
	} else {
		res.Status = reporter.StatusFailed
		res.Detail = fmt.Sprintf("exit code %d", exitCode)
		if err != nil {
			res.Detail += fmt.Sprintf(": %s", err.Error())
		}
	}
	return res
}
```

- [ ] **Step 1: Write failing tests** — create `internal/installer/winget_test.go`:

```go
package installer

import (
	"testing"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
	"github.com/Ktulue/KtulueKit-W11/internal/reporter"
)

func TestUninstallPackage_DryRun(t *testing.T) {
	pkg := config.Package{ID: "Git.Git", Name: "Git", TimeoutSeconds: 60}
	res := UninstallPackage(pkg, true)
	if res.Status != reporter.StatusDryRun {
		t.Errorf("status = %q, want %q", res.Status, reporter.StatusDryRun)
	}
	if res.Detail == "" {
		t.Error("Detail should contain the winget uninstall command")
	}
}

func TestUninstallPackage_SkippedWhenCheckFails(t *testing.T) {
	pkg := config.Package{
		ID:             "NotInstalled.Package",
		Name:           "Not Installed",
		Check:          "cmd /C exit 1",
		TimeoutSeconds: 15,
	}
	res := UninstallPackage(pkg, false)
	if res.Status != reporter.StatusSkipped {
		t.Errorf("status = %q, want %q", res.Status, reporter.StatusSkipped)
	}
}
```

- [ ] **Step 2: Run to confirm it fails**

```
cd /f/GDriveClone/Claude_Code/KtulueKit-W11 && go test ./internal/installer/... -run "TestUninstallPackage" -v
```

- [ ] **Step 3: Implement `UninstallPackage`** — add to `internal/installer/winget.go`.

- [ ] **Step 4: Run to confirm tests pass**

```
cd /f/GDriveClone/Claude_Code/KtulueKit-W11 && go test ./internal/installer/... -run "TestUninstallPackage" -v
```

- [ ] **Step 5: Commit**

```
git add internal/installer/winget.go internal/installer/winget_test.go
git commit -m "feat(installer): add UninstallPackage for Tier 1 winget uninstall"
```

---

### Task 4: `RunUninstallCommand` in `internal/installer/command.go`

**Files:**
- Modify: `internal/installer/command.go`
- Test: `internal/installer/command_test.go` (create)

**Context:**
- `ScrapeURL != ""` → `StatusSkipped` (T4 always skipped; this check takes precedence over `UninstallCmd`)
- `UninstallCmd == ""` → `StatusSkipped`
- Dry-run → `StatusDryRun` with `[DRY RUN] Would run uninstall_cmd: <cmd>` in Detail
- Otherwise: run via `runShellWithTimeout`; use `cmd.TimeoutSeconds` or fall back to `checkTimeoutSeconds`
- Exit 0 → `StatusInstalled` (reused as "Removed")

Add to `internal/installer/command.go`:

```go
// RunUninstallCommand handles Tier 2 command uninstall.
// ScrapeURL (T4) items are always skipped. Items without UninstallCmd are skipped.
func RunUninstallCommand(cmd config.Command, dryRun bool) reporter.Result {
	res := reporter.Result{ID: cmd.ID, Name: cmd.Name, Tier: "command"}

	if cmd.ScrapeURL != "" {
		res.Status = reporter.StatusSkipped
		res.Detail = "scrape-download items cannot be uninstalled automatically — remove via Windows Settings"
		return res
	}
	if cmd.UninstallCmd == "" {
		res.Status = reporter.StatusSkipped
		res.Detail = "no uninstall_cmd defined for this item"
		return res
	}
	if dryRun {
		res.Status = reporter.StatusDryRun
		res.Detail = fmt.Sprintf("[DRY RUN] Would run uninstall_cmd: %s", cmd.UninstallCmd)
		return res
	}

	timeout := cmd.TimeoutSeconds
	if timeout == 0 {
		timeout = checkTimeoutSeconds
	}

	exitCode, err := runShellWithTimeout(cmd.UninstallCmd, timeout)
	res.ExitCode = exitCode
	if exitCode == 0 && err == nil {
		res.Status = reporter.StatusInstalled
	} else {
		res.Status = reporter.StatusFailed
		res.Detail = fmt.Sprintf("exit code %d", exitCode)
		if err != nil {
			res.Detail += fmt.Sprintf(": %s", err.Error())
		}
	}
	return res
}
```

- [ ] **Step 1: Write failing tests** — create `internal/installer/command_test.go`:

```go
package installer

import (
	"testing"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
	"github.com/Ktulue/KtulueKit-W11/internal/reporter"
)

func TestRunUninstallCommand_ScrapeURLSkipped(t *testing.T) {
	cmd := config.Command{
		ID: "app", Name: "App",
		ScrapeURL:    "https://example.com/download",
		UninstallCmd: "uninstall.exe", // ScrapeURL takes precedence
	}
	res := RunUninstallCommand(cmd, false)
	if res.Status != reporter.StatusSkipped {
		t.Errorf("status = %q, want StatusSkipped (T4 must skip)", res.Status)
	}
}

func TestRunUninstallCommand_NoUninstallCmdSkipped(t *testing.T) {
	cmd := config.Command{ID: "npm-tool", Name: "npm tool", Cmd: "npm install -g something"}
	res := RunUninstallCommand(cmd, false)
	if res.Status != reporter.StatusSkipped {
		t.Errorf("status = %q, want StatusSkipped", res.Status)
	}
}

func TestRunUninstallCommand_DryRun(t *testing.T) {
	cmd := config.Command{
		ID: "npm-tool", Name: "npm tool",
		UninstallCmd: "npm uninstall -g something",
	}
	res := RunUninstallCommand(cmd, true)
	if res.Status != reporter.StatusDryRun {
		t.Errorf("status = %q, want StatusDryRun", res.Status)
	}
	if res.Detail == "" {
		t.Error("Detail should contain the uninstall command preview")
	}
}
```

- [ ] **Step 2: Run to confirm it fails**

```
cd /f/GDriveClone/Claude_Code/KtulueKit-W11 && go test ./internal/installer/... -run "TestRunUninstallCommand" -v
```

- [ ] **Step 3: Implement `RunUninstallCommand`** — add to `internal/installer/command.go`.

- [ ] **Step 4: Run to confirm tests pass**

```
cd /f/GDriveClone/Claude_Code/KtulueKit-W11 && go test ./internal/installer/... -run "TestRunUninstallCommand" -v
```

- [ ] **Step 5: Commit**

```
git add internal/installer/command.go internal/installer/command_test.go
git commit -m "feat(installer): add RunUninstallCommand for Tier 2 shell command uninstall"
```

---

### Task 5: `UninstallExtension` in `internal/installer/extension.go`

**Files:**
- Modify: `internal/installer/extension.go`
- Test: `internal/installer/extension_test.go` (create)

**Context:**
- `mode != "force"` → `StatusSkipped`
- Unknown browser → `StatusFailed`
- Dry-run → `StatusDryRun`
- Real run: open registry key with `ALL_ACCESS`, find value beginning with `ext.ExtensionID`, delete it, renumber remaining numeric values starting at 1 (non-atomic, best-effort)
- No matching value → `StatusSkipped`
- Registry error → `StatusFailed`
- Success → `StatusInstalled` (reused)
- Add `"sort"` to import block of `extension.go`

```go
// UninstallExtension handles Tier 3 browser extension uninstall.
// url-mode: skipped (not installed programmatically).
// force-mode: removes registry value matching ext.ExtensionID and renumbers remaining.
// Non-atomic; best-effort for personal tool.
func UninstallExtension(ext config.Extension, dryRun bool) reporter.Result {
	res := reporter.Result{ID: ext.ID, Name: ext.Name, Tier: "extension"}

	if ext.Mode != "force" {
		res.Status = reporter.StatusSkipped
		res.Detail = "url-mode extensions are not installed programmatically — uninstall via browser"
		return res
	}

	path, ok := browserPolicyPaths[ext.Browser]
	if !ok {
		res.Status = reporter.StatusFailed
		res.Detail = fmt.Sprintf("unsupported browser: %s", ext.Browser)
		return res
	}

	if dryRun {
		res.Status = reporter.StatusDryRun
		res.Detail = fmt.Sprintf("HKLM\\%s — remove value matching %s and renumber", path, ext.ExtensionID)
		return res
	}

	k, err := registry.OpenKey(registry.LOCAL_MACHINE, path, registry.ALL_ACCESS)
	if err != nil {
		res.Status = reporter.StatusFailed
		res.Detail = fmt.Sprintf("cannot open registry key: %s", err)
		return res
	}
	defer k.Close()

	names, _ := k.ReadValueNames(-1)
	var targetName string
	for _, name := range names {
		val, _, _ := k.GetStringValue(name)
		if strings.HasPrefix(val, ext.ExtensionID) {
			targetName = name
			break
		}
	}
	if targetName == "" {
		res.Status = reporter.StatusSkipped
		res.Detail = "extension value not found in registry — may already be removed"
		return res
	}

	if err := k.DeleteValue(targetName); err != nil {
		res.Status = reporter.StatusFailed
		res.Detail = fmt.Sprintf("cannot delete registry value %q: %s", targetName, err)
		return res
	}

	// Renumber remaining numeric values contiguously starting at 1.
	remaining := make(map[string]string)
	names, _ = k.ReadValueNames(-1)
	for _, name := range names {
		var n int
		if _, err := fmt.Sscanf(name, "%d", &n); err == nil {
			val, _, _ := k.GetStringValue(name)
			remaining[name] = val
		}
	}
	sortedNames := make([]string, 0, len(remaining))
	for n := range remaining {
		sortedNames = append(sortedNames, n)
	}
	sort.Slice(sortedNames, func(i, j int) bool {
		var ni, nj int
		fmt.Sscanf(sortedNames[i], "%d", &ni)
		fmt.Sscanf(sortedNames[j], "%d", &nj)
		return ni < nj
	})
	for _, name := range sortedNames {
		_ = k.DeleteValue(name)
	}
	for i, name := range sortedNames {
		_ = k.SetStringValue(fmt.Sprintf("%d", i+1), remaining[name])
	}

	res.Status = reporter.StatusInstalled
	res.Detail = "registry policy value removed — browser restart required"
	return res
}
```

- [ ] **Step 1: Write failing tests** — create `internal/installer/extension_test.go`:

```go
package installer

import (
	"testing"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
	"github.com/Ktulue/KtulueKit-W11/internal/reporter"
)

func TestUninstallExtension_URLModeSkipped(t *testing.T) {
	ext := config.Extension{
		ID: "darkreader", Name: "Dark Reader",
		ExtensionID: "eimadpbcbfnmbkopoojfekhnkhdbieeh",
		Browser: "brave", Mode: "url",
	}
	res := UninstallExtension(ext, false)
	if res.Status != reporter.StatusSkipped {
		t.Errorf("status = %q, want StatusSkipped for url mode", res.Status)
	}
}

func TestUninstallExtension_ForceDryRun(t *testing.T) {
	ext := config.Extension{
		ID: "darkreader", Name: "Dark Reader",
		ExtensionID: "eimadpbcbfnmbkopoojfekhnkhdbieeh",
		Browser: "brave", Mode: "force",
	}
	res := UninstallExtension(ext, true)
	if res.Status != reporter.StatusDryRun {
		t.Errorf("status = %q, want StatusDryRun", res.Status)
	}
	if res.Detail == "" {
		t.Error("Detail should describe the registry operation")
	}
}
```

- [ ] **Step 2: Run to confirm it fails**

```
cd /f/GDriveClone/Claude_Code/KtulueKit-W11 && go test ./internal/installer/... -run "TestUninstallExtension" -v
```

- [ ] **Step 3: Implement `UninstallExtension`** — add to `internal/installer/extension.go`. Add `"sort"` to the import block.

- [ ] **Step 4: Run to confirm tests pass**

```
cd /f/GDriveClone/Claude_Code/KtulueKit-W11 && go test ./internal/installer/... -run "TestUninstallExtension" -v
```

- [ ] **Step 5: Run full installer suite**

```
cd /f/GDriveClone/Claude_Code/KtulueKit-W11 && go test ./internal/installer/... -v
```

- [ ] **Step 6: Commit**

```
git add internal/installer/extension.go internal/installer/extension_test.go
git commit -m "feat(installer): add UninstallExtension for Tier 3 force-mode registry removal"
```

---

## Chunk 2: Runner, App Latent Bug Fix, CLI Subcommand

### Task 6: `RunUninstall` in `internal/runner/runner.go`

**Files:**
- Modify: `internal/runner/runner.go`

**IMPORTANT: Read `runner.go` fully before implementing.** The helper function names for item flattening and progress emission are not listed in this plan — you must read the file and use the actual names. Common patterns to look for: a method that returns a `[]flatItem` or equivalent, and a progress emit pattern using `r.onProgress`.

`RunUninstall` must:
1. Build flat item list filtered by `r.selectedIDs`
2. For each item, emit a `"uninstalling"` progress event (or print to stdout in CLI mode)
3. Call the appropriate per-tier uninstall function
4. Add the result to `r.rep`
5. If `res.Status == reporter.StatusInstalled` (success signal): call `r.state.DeleteSucceeded(item.ID)`, emit status `"uninstalled"` in the progress event
6. Otherwise emit the result's status as-is
7. Does NOT use scheduler, resume, reboot, ShortcutMode, or consecutiveFails

- [ ] **Step 1: Read `internal/runner/runner.go` fully** to understand: flat item helpers, progress emission, `ProgressEvent` struct fields, how `r.onProgress` and `r.dryRun` are used.

- [ ] **Step 2: Implement `RunUninstall`** based on the patterns you find. Here is the logical skeleton — adjust helper names to match what exists in runner.go:

```go
func (r *Runner) RunUninstall(ctx context.Context) {
    // 1. Build filtered item list (use the existing flat-item helper, e.g. r.flatItems() or similar)
    items := /* existing flat-item helper, filtered by r.selectedIDs */

    // 2. Set total item count for progress events (so GUI shows [N/Total] correctly).
    //    Do NOT use countItemsFromPhase — that is phase-based for install.
    //    Simply count the flat items you will iterate:
    r.totalItems = len(items)

    // 3. For each item:
    for _, item := range items {
        r.itemIdx++ // REQUIRED: increment per item so progress events show correct index

        //    a. Check ctx.Done()
        //    b. Emit "uninstalling" progress event
        //    c. Call installer.UninstallPackage / installer.RunUninstallCommand / installer.UninstallExtension
        //    d. r.rep.Add(res)
        //    e. If res.Status == reporter.StatusInstalled:
        //          eventStatus = "uninstalled"
        //          if !r.dryRun { r.state.DeleteSucceeded(item.ID) }
        //       Else: eventStatus = res.Status
        //    f. Emit completion progress event with eventStatus
    }
}
```

**Note:** `r.itemIdx` starts at 0 on a fresh Runner. Increment it at the top of each loop iteration (before emitting the first progress event) so the event's `Index` field is 1-based and matches the `Total` field.

- [ ] **Step 3: Build check**

```
cd /f/GDriveClone/Claude_Code/KtulueKit-W11 && go build ./...
```

- [ ] **Step 4: Commit**

```
git add internal/runner/runner.go
git commit -m "feat(runner): add RunUninstall method for config-scoped uninstall"
```

---

### Task 7: Fix latent `SetPauseResponse` bug + add `StartUninstall` and `GetInstalledItems` to `app.go`

**Files:**
- Modify: `app.go`

**Context:** `StartInstall` creates a runner but never calls `r.SetPauseResponse(...)`. The `pauseResponse` channel is nil. When three consecutive failures occur, the runner blocks forever. Fix: add `SetPauseResponse` in both `StartInstall` and `StartUninstall`.

Read `app.go` before editing to find the exact location of `r.SetRebootResponse(rebootCh)`.

- [ ] **Step 1: Fix `StartInstall`** — add after `r.SetRebootResponse(rebootCh)`:

```go
pauseCh := make(chan bool, 1)
r.SetPauseResponse(pauseCh)
```

- [ ] **Step 2: Add `StartUninstall`** after `CancelReboot`:

```go
func (a *App) StartUninstall(ids []string) string {
	if len(ids) == 0 {
		return "No items selected."
	}
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return "An operation is already in progress."
	}
	a.running = true
	a.mu.Unlock()

	go func() {
		defer func() {
			a.mu.Lock()
			a.running = false
			a.mu.Unlock()
		}()

		cfg, err := config.Load(a.configPath)
		if err != nil {
			runtime.EventsEmit(a.ctx, "uninstall_complete", SummaryResult{
				Failed: []string{fmt.Sprintf("Failed to load config: %v", err)},
			})
			return
		}
		rep, err := reporter.New(cfg.Settings.LogDir)
		if err != nil {
			runtime.EventsEmit(a.ctx, "uninstall_complete", SummaryResult{
				Failed: []string{fmt.Sprintf("Failed to create log: %v", err)},
			})
			return
		}
		defer rep.Close()
		s, err := state.Load()
		if err != nil {
			runtime.EventsEmit(a.ctx, "uninstall_complete", SummaryResult{
				Failed:  []string{fmt.Sprintf("Failed to load state: %v", err)},
				LogPath: rep.LogPath(),
			})
			return
		}

		r := runner.New(cfg, rep, s, false, 1, a.configPath, desktop.ShortcutRemove)
		selectedMap := make(map[string]bool, len(ids))
		for _, id := range ids {
			selectedMap[id] = true
		}
		r.SetSelectedIDs(selectedMap)
		pauseCh := make(chan bool, 1)
		r.SetPauseResponse(pauseCh)
		r.SetOnProgress(func(e runner.ProgressEvent) {
			runtime.EventsEmit(a.ctx, "progress", e)
		})

		runStart := time.Now()
		r.RunUninstall(context.Background())
		elapsed := time.Since(runStart).Round(time.Second).String()

		summary := SummaryResult{
			Installed:    rep.NamesBy("installed"),
			Failed:       rep.NamesBy("failed"),
			Skipped:      rep.NamesBy("skipped"),
			TotalElapsed: elapsed,
			LogPath:      rep.LogPath(),
		}
		runtime.EventsEmit(a.ctx, "uninstall_complete", summary)
	}()

	return ""
}
```

- [ ] **Step 3: Add `GetInstalledItems`** after `StartUninstall`. Also add `"github.com/Ktulue/KtulueKit-W11/internal/detector"` to the import block:

```go
// GetInstalledItems runs detector checks for the given IDs and returns those
// that are currently installed (detector check exits 0).
func (a *App) GetInstalledItems(ids []string) []string {
	cfg, err := config.Load(a.configPath)
	if err != nil {
		return nil
	}
	checkCmds := make(map[string]string)
	for _, p := range cfg.Packages {
		checkCmds[p.ID] = p.Check
	}
	for _, c := range cfg.Commands {
		checkCmds[c.ID] = c.Check
	}
	// NOTE: config.Extension has no Check field — extensions are excluded here.
	// The GUI scan will simply not show extensions in the uninstall list,
	// which matches the spec (extensions have no reliable installed-state check command).

	var installed []string
	for _, id := range ids {
		check := checkCmds[id]
		if check == "" {
			continue
		}
		if isInstalled, _ := detector.RunCheckDetailed(check); isInstalled {
			installed = append(installed, id)
		}
	}
	return installed
}
```

- [ ] **Step 4: Build check**

```
cd /f/GDriveClone/Claude_Code/KtulueKit-W11 && go build ./...
```

- [ ] **Step 5: Commit**

```
git add app.go
git commit -m "fix(app): wire SetPauseResponse in StartInstall; add StartUninstall and GetInstalledItems"
```

---

### Task 8: `uninstall` CLI subcommand — `cmd/uninstall.go`

**Files:**
- Create: `cmd/uninstall.go`
- Test: `cmd/uninstall_test.go`

**Context:**
- Flags: `--only`, `--profile`, `--exclude`, `--dry-run` (same package-level vars as other subcommands)
- `--profile` and `--only` mutually exclusive (error)
- `--profile` and `--exclude` compatible
- Confirmation gate: print item list, read stdin, accept `yes`/`Yes`/`YES` via `strings.EqualFold`; anything else cancels; dry-run bypasses gate
- Non-TTY (`golang.org/x/term`): auto-approve pause channel when stdin is not a TTY
- Pre-flight: `installer.CheckWingetAvailable()`
- `filterFlagsError(only, exclude string) error` is in `cmd/filter.go` (existing package-level function — already present before this branch).
- `resolveFilter`, `parseIDList`, `buildSelectedMap` come from `cmd/helpers.go` (feat/cli-polish). If not yet merged, implement minimal inline versions. `profileFlag` is a package-level var from feat/cli-polish — if absent, declare it here.
- Register the command by calling `rootCmd.AddCommand(uninstallCmd)` in `init()`

- [ ] **Step 1: Write failing tests** — create `cmd/uninstall_test.go`:

```go
package main

import (
	"testing"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
)

func makeUninstallTestConfig() *config.Config {
	return &config.Config{
		Packages: []config.Package{
			{ID: "Git.Git", Name: "Git"},
			{ID: "Mozilla.Firefox", Name: "Firefox"},
		},
		Commands: []config.Command{
			{ID: "wsl", Name: "WSL"},
		},
	}
}

func TestBuildUninstallList_AllItems(t *testing.T) {
	cfg := makeUninstallTestConfig()
	names := buildUninstallList(cfg, nil, nil)
	if len(names) != 3 {
		t.Errorf("expected 3 items, got %d: %v", len(names), names)
	}
}

func TestBuildUninstallList_FilterApplied(t *testing.T) {
	cfg := makeUninstallTestConfig()
	filter := map[string]bool{"Git.Git": true}
	names := buildUninstallList(cfg, filter, nil)
	if len(names) != 1 || names[0] != "Git" {
		t.Errorf("expected only Git, got: %v", names)
	}
}

func TestBuildUninstallList_ExcludeApplied(t *testing.T) {
	cfg := makeUninstallTestConfig()
	exclude := map[string]bool{"Git.Git": true}
	names := buildUninstallList(cfg, nil, exclude)
	for _, n := range names {
		if n == "Git" {
			t.Error("excluded 'Git' should not appear")
		}
	}
}

func TestBuildUninstallList_EmptyWhenFilterMatchesNone(t *testing.T) {
	cfg := makeUninstallTestConfig()
	filter := map[string]bool{"NonExistent.ID": true}
	names := buildUninstallList(cfg, filter, nil)
	if len(names) != 0 {
		t.Errorf("expected empty list, got: %v", names)
	}
}
```

- [ ] **Step 2: Run to confirm it fails**

```
cd /f/GDriveClone/Claude_Code/KtulueKit-W11 && go test ./cmd/... -run "TestBuildUninstallList" -v
```

Expected: compile error — `buildUninstallList` undefined.

- [ ] **Step 3: Create `cmd/uninstall.go`**:

```go
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
	"github.com/Ktulue/KtulueKit-W11/internal/desktop"
	"github.com/Ktulue/KtulueKit-W11/internal/installer"
	"github.com/Ktulue/KtulueKit-W11/internal/reporter"
	"github.com/Ktulue/KtulueKit-W11/internal/runner"
	"github.com/Ktulue/KtulueKit-W11/internal/state"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall config-listed items from this machine",
	RunE:  runUninstall,
}

func init() {
	rootCmd.AddCommand(uninstallCmd)
	uninstallCmd.Flags().StringVar(&onlyIDs, "only", "", "comma-separated list of IDs to uninstall")
	uninstallCmd.Flags().StringVar(&excludeIDs, "exclude", "", "comma-separated list of IDs to skip")
	uninstallCmd.Flags().StringVar(&profileFlag, "profile", "", "named profile from config")
	uninstallCmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview without making changes")
}

func runUninstall(cmd *cobra.Command, args []string) error {
	if err := filterFlagsError(onlyIDs, excludeIDs); err != nil {
		return err
	}
	if profileFlag != "" && onlyIDs != "" {
		return fmt.Errorf("--profile and --only are mutually exclusive")
	}

	resolved, cleanup, err := resolveConfigPaths(configPaths)
	if err != nil {
		return err
	}
	defer cleanup()

	cfg, err := config.LoadAll(resolved)
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	filter, err := resolveFilter(cfg, profileFlag, onlyIDs)
	if err != nil {
		return err
	}
	excludeSet := parseIDList(excludeIDs)

	items := buildUninstallList(cfg, filter, excludeSet)
	if len(items) == 0 {
		fmt.Println("No items to uninstall.")
		return nil
	}

	if !dryRun {
		fmt.Println("Items to be removed:")
		for _, name := range items {
			fmt.Printf("  - %s\n", name)
		}
		fmt.Println()
		fmt.Print("Type 'yes' to confirm: ")
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		answer := strings.TrimSpace(scanner.Text())
		if !strings.EqualFold(answer, "yes") {
			fmt.Println("Uninstall cancelled.")
			return nil
		}
	}

	if err := installer.CheckWingetAvailable(); err != nil {
		return fmt.Errorf("winget not available: %w", err)
	}

	s, err := state.Load()
	if err != nil {
		return fmt.Errorf("state load: %w", err)
	}

	rep, err := reporter.New(cfg.Settings.LogDir)
	if err != nil {
		return fmt.Errorf("reporter: %w", err)
	}
	defer rep.Close()

	selectedMap := buildSelectedMap(cfg, filter, excludeSet)
	firstPath := ""
	if len(configPaths) > 0 {
		firstPath = configPaths[0]
	}
	r := runner.New(cfg, rep, s, dryRun, 1, firstPath, desktop.ShortcutRemove)
	r.SetSelectedIDs(selectedMap)

	pauseCh := make(chan bool, 1)
	r.SetPauseResponse(pauseCh)
	isNonTTY := !term.IsTerminal(int(os.Stdin.Fd()))
	if isNonTTY {
		go func() {
			for {
				pauseCh <- true
			}
		}()
	}

	r.RunUninstall(cmd.Context())
	rep.PrintSummary()
	return nil
}

// buildUninstallList returns display names of items matching filter and not excluded.
// filter nil = all items. exclude nil = no exclusions.
func buildUninstallList(cfg *config.Config, filter map[string]bool, exclude map[string]bool) []string {
	var names []string
	for _, p := range cfg.Packages {
		if (filter == nil || filter[p.ID]) && !exclude[p.ID] {
			names = append(names, p.Name)
		}
	}
	for _, c := range cfg.Commands {
		if (filter == nil || filter[c.ID]) && !exclude[c.ID] {
			names = append(names, c.Name)
		}
	}
	for _, e := range cfg.Extensions {
		if (filter == nil || filter[e.ID]) && !exclude[e.ID] {
			names = append(names, e.Name)
		}
	}
	return names
}
```

**Note on `resolveFilter` and `buildSelectedMap`:** These are helpers added by `feat/cli-polish` in `cmd/helpers.go`. If that branch is not yet merged, implement minimal inline versions:

```go
// resolveFilter returns the ID set for the given profile or --only list.
// Returns nil if neither is set (meaning "all items").
func resolveFilter(cfg *config.Config, profile, only string) (map[string]bool, error) {
    if profile != "" {
        // Look up profile in cfg.Profiles by name (case-sensitive).
        for _, p := range cfg.Profiles {
            if p.Name == profile {
                m := make(map[string]bool, len(p.IDs))
                for _, id := range p.IDs {
                    m[id] = true
                }
                return m, nil
            }
        }
        return nil, fmt.Errorf("profile %q not found", profile)
    }
    if only != "" {
        return parseIDSet(only), nil
    }
    return nil, nil
}

func parseIDSet(csv string) map[string]bool {
    m := make(map[string]bool)
    for _, id := range strings.Split(csv, ",") {
        id = strings.TrimSpace(id)
        if id != "" {
            m[id] = true
        }
    }
    return m
}

// buildSelectedMap builds the map[string]bool passed to r.SetSelectedIDs.
func buildSelectedMap(cfg *config.Config, filter map[string]bool, exclude map[string]bool) map[string]bool {
    m := make(map[string]bool)
    add := func(id string) {
        if (filter == nil || filter[id]) && !exclude[id] {
            m[id] = true
        }
    }
    for _, p := range cfg.Packages { add(p.ID) }
    for _, c := range cfg.Commands { add(c.ID) }
    for _, e := range cfg.Extensions { add(e.ID) }
    return m
}
```

If `parseIDList` already exists in `cmd/main.go` or `cmd/helpers.go`, use that instead of `parseIDSet`.

If `profileFlag` is already declared as a package-level var by `feat/cli-polish`, do not redeclare it. If it doesn't exist yet, add `var profileFlag string` to `cmd/uninstall.go`.

- [ ] **Step 4: Build check**

```
cd /f/GDriveClone/Claude_Code/KtulueKit-W11 && go build ./...
```

- [ ] **Step 5: Run tests**

```
cd /f/GDriveClone/Claude_Code/KtulueKit-W11 && go test ./cmd/... -run "TestBuildUninstallList" -v
```

Expected: all four PASS.

- [ ] **Step 6: Commit**

```
git add cmd/uninstall.go cmd/uninstall_test.go
git commit -m "feat(cmd): add uninstall subcommand with confirmation gate and non-TTY support"
```

---

## Chunk 3: GUI — Install/Uninstall Tab Bar

### Task 9: Add Install/Uninstall tab bar to `SelectionScreen.svelte`

**Files:**
- Modify: `frontend/src/screens/SelectionScreen.svelte`
- Modify: `frontend/src/components/ItemRow.svelte`

**IMPORTANT: Read both Svelte files in full before editing.** Understand existing store references, component imports, event handlers, and layout structure. The steps below describe what to add — the exact insertion point depends on what you find.

**Wails bindings used:**
- `GetInstalledItems(ids: string[]): Promise<string[]>` — added in Task 7
- `StartUninstall(ids: string[]): Promise<string>` — added in Task 7

**What to add to `SelectionScreen.svelte`:**

1. **Script additions** — tab state, scan logic, uninstall trigger:

```js
let activeTab = 'install';
let scanLoading = false;
let installedIDs = [];
let scanDone = false;
let uninstallSelected = {};

$: hasUninstallSelection = Object.values(uninstallSelected).some(Boolean);

function getAllItemIDs() {
  let ids = [];
  for (const cat of categories) {
    for (const item of cat.items) ids.push(item.id);
  }
  return ids;
}

async function switchTab(tab) {
  if ($running) return;
  activeTab = tab;
  if (tab === 'uninstall' && !scanDone) {
    scanLoading = true;
    installedIDs = await GetInstalledItems(getAllItemIDs());
    scanLoading = false;
    scanDone = true;
  }
}

async function startUninstall() {
  const ids = Object.entries(uninstallSelected)
    .filter(([, v]) => v)
    .map(([id]) => id);
  if (!ids.length) return;
  const err = await StartUninstall(ids);
  if (err) console.error('StartUninstall error:', err);
}
```

2. **Tab bar markup** — add immediately before the category accordion list:

```svelte
<div class="tab-bar">
  <button
    class="tab"
    class:tab-active={activeTab === 'install'}
    class:tab-disabled={$running}
    on:click={() => switchTab('install')}
    disabled={$running}
  >Install</button>
  <button
    class="tab"
    class:tab-active={activeTab === 'uninstall'}
    class:tab-uninstall-active={activeTab === 'uninstall'}
    class:tab-disabled={$running}
    on:click={() => switchTab('uninstall')}
    disabled={$running}
  >Uninstall</button>
</div>
```

3. **Conditional content** — wrap existing accordion + action button in install branch; add uninstall branch:

```svelte
{#if activeTab === 'install'}
  <!-- existing category accordion list and install action button -->
{:else}
  {#if scanLoading}
    <div class="scan-state">Scanning installed items...</div>
  {:else if installedIDs.length === 0}
    <div class="scan-state">No installed items detected.</div>
  {:else}
    {#each categories as cat}
      {#if cat.items.some(i => installedIDs.includes(i.id))}
        <CategoryAccordion name={cat.name}>
          {#each cat.items.filter(i => installedIDs.includes(i.id)) as item}
            <ItemRow {item} mode="uninstall" bind:selected={uninstallSelected[item.id]} />
          {/each}
        </CategoryAccordion>
      {/if}
    {/each}
    <button
      class="action-btn uninstall-action-btn"
      disabled={$running || !hasUninstallSelection}
      on:click={startUninstall}
    >Uninstall Selected</button>
  {/if}
{/if}
```

4. **Style additions**:

```css
.tab-bar {
  display: flex;
  border-bottom: 1px solid #333;
  margin-bottom: 8px;
}
.tab {
  padding: 8px 20px;
  background: none;
  border: none;
  border-bottom: 2px solid transparent;
  cursor: pointer;
  color: #888;
  font-size: 15px;
  transition: color 100ms ease;
}
.tab.tab-active { color: #0e7fd4; border-bottom-color: #0e7fd4; }
.tab.tab-active.tab-uninstall-active { color: #ff6b6b; border-bottom-color: #ff6b6b; }
.tab.tab-disabled { opacity: 0.4; cursor: not-allowed; }
.scan-state { padding: 32px; text-align: center; color: #888; }
.uninstall-action-btn { background: #c0392b; }
.uninstall-action-btn:hover:not(:disabled) { background: #e74c3c; }
```

**What to add to `ItemRow.svelte`:** Read the file, then add a `mode` prop and use it to switch checkbox accent color:

```svelte
<script>
  export let mode = 'install'; // 'install' | 'uninstall'
  // ... existing props
</script>
```

Apply the accent color via a CSS custom property or class:

```css
/* Install mode (default) */
input[type='checkbox']:checked { accent-color: #0e7fd4; }

/* Uninstall mode */
.uninstall-mode input[type='checkbox']:checked { accent-color: #ff6b6b; }
```

Or bind `data-mode={mode}` on the root element and target with CSS attribute selectors. Choose the approach that best fits the existing component structure.

- [ ] **Step 1: Read `frontend/src/screens/SelectionScreen.svelte` and `frontend/src/components/ItemRow.svelte` fully.**

- [ ] **Step 2: Add tab state and handlers to SelectionScreen `<script>` block.**

- [ ] **Step 3: Add tab bar markup before the accordion list.**

- [ ] **Step 4: Wrap install content in `{#if activeTab === 'install'}` and add uninstall tab branch.**

- [ ] **Step 5: Add styles to `<style>` block.**

- [ ] **Step 6: Add `mode` prop to `ItemRow.svelte` and apply accent color switching.**

- [ ] **Step 7: Test in Wails dev server**

```
cd /f/GDriveClone/Claude_Code/KtulueKit-W11 && wails dev
```

Check:
- Install tab shows existing accordion list.
- Uninstall tab: "Scanning installed items..." → then shows installed items with red checkboxes.
- Both tabs disabled (muted) during active run.
- "Uninstall Selected" triggers `StartUninstall` and the progress screen appears.

- [ ] **Step 8: Commit**

```
git add frontend/src/screens/SelectionScreen.svelte frontend/src/components/ItemRow.svelte
git commit -m "feat(gui): add Install/Uninstall tab bar with scan-on-open and installed item list"
```

---

## Final Verification

- [ ] **Step 1: Update `TODO.md`** — mark all uninstall-related items done.

- [ ] **Step 2: Full test suite**

```
cd /f/GDriveClone/Claude_Code/KtulueKit-W11 && go test ./... && go build ./...
```

Expected: all PASS, clean build.

- [ ] **Step 3: Commit TODO update**

```
git add TODO.md
git commit -m "chore(todo): mark uninstall track as done"
```

- [ ] **Step 4: Push and open PR**

```
git push -u origin feat/uninstall
gh pr create --title "feat: add config-scoped uninstall (CLI + GUI tabs)" --body "$(cat <<'EOF'
## Summary

- Adds `uninstall_cmd` field to `config.Command` for Tier 2 custom uninstall commands.
- Adds `state.DeleteSucceeded()` to remove items from state after successful uninstall.
- Adds per-tier uninstall: `UninstallPackage` (winget), `RunUninstallCommand` (shell), `UninstallExtension` (registry value removal + renumber).
- Adds `runner.RunUninstall()` routing to tier functions and clearing state on success.
- Fixes latent bug: `SetPauseResponse` now wired in both `StartInstall` and new `StartUninstall`.
- Adds `ktuluekit uninstall` CLI subcommand with confirmation gate, `--dry-run`, `--profile`, `--only`, `--exclude`, piped-stdin support, non-TTY auto-continue.
- Adds `App.GetInstalledItems()` Wails binding for GUI detection scan.
- Adds `App.StartUninstall()` Wails binding.
- Adds Install/Uninstall tab bar to `SelectionScreen.svelte` with goroutine scan on first Uninstall tab open.
- T4 (scrape-download) items always skipped. Extension url-mode skipped. Non-atomic registry renumber accepted.

## Test plan

- [ ] `go test ./internal/config/... -run TestCommandUninstallCmd` — schema round-trip
- [ ] `go test ./internal/state/... -run TestDeleteSucceeded` — state removal + persistence
- [ ] `go test ./internal/installer/... -run "TestUninstallPackage|TestRunUninstallCommand|TestUninstallExtension"` — per-tier dry-run and skip cases
- [ ] `go test ./cmd/... -run TestBuildUninstallList` — filter and exclude
- [ ] `go test ./...` — full suite passes
- [ ] `go build ./...` — clean build
- [ ] Manual: `ktuluekit uninstall --dry-run` previews without confirmation gate
- [ ] Manual: `echo yes | ktuluekit uninstall --only SomeItem.ID` works with piped stdin
- [ ] Manual: GUI Uninstall tab scans and shows installed items with red checkboxes

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```
