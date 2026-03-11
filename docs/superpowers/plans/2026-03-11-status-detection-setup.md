# Status, Detection & Setup — Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `status` subcommand, pre-run detection summary, shared detector package, and `setup.ps1` bootstrap so KtulueKit works correctly on both fresh and previously-run machines.

**Architecture:** New `internal/detector/` package provides `CheckItem()` and `CheckAll()` used by both the `status` subcommand and the install runner. The detector checks state first (state-aware skip — no shell command needed if already succeeded), then runs the item's check command silently via an internal `runCheckSilent()` function. The detector does NOT import the installer package — it has its own silent exec logic (no stdout side effects). The `status` subcommand prints a grouped, ANSI-colored table. The runner prints a pre-run summary before the phase loop and skips already-succeeded items. `setup.ps1` installs Go if missing, builds the binary, and launches it.

**Tech Stack:** Go stdlib (`os/exec`, `context`, `time`, `fmt`), Cobra (already used for CLI), PowerShell 5.1+ (for setup.ps1).

**Spec:** `docs/superpowers/specs/2026-03-11-status-detection-bootstrap-design.md`

> **Note on spec divergence:** The spec defines `CheckItem(item config.Item, ...)` using the raw config types. The plan uses a unified `detector.Item` type instead. This is intentional — the three config types (Package, Command, Extension) have different fields; `detector.Item` is a minimal projection containing only what detection needs (ID, Name, Phase, Tier, CheckCmd). `FlattenItems()` handles the conversion.

---

## File Map

**New files:**
- `internal/detector/detector.go` — `Status`, `Item`, `Result`, `FlattenItems()`, `CheckItem()`, `CheckAll()`, `runCheckSilent()`
- `internal/detector/detector_test.go` — unit tests (pure logic, no OS calls)
- `cmd/status.go` — `runStatus()` and display helpers
- `setup.ps1` — hands-off bootstrap script

**Modified files:**
- `cmd/main.go` — wire `status` subcommand
- `internal/runner/runner.go` — add pre-run summary + state-aware skip in `runPackagesInPhase` / `runCommandsInPhase`

---

## Chunk 1: Detector Package

### Task 1: Create `internal/detector/detector.go`

The detector is self-contained — it does not import the installer package. It has its own `runCheckSilent()` that suppresses all output (pure detection, no side effects).

**Files:**
- Create: `internal/detector/detector.go`

- [ ] **Step 1: Create the file**

```go
package detector

import (
	"context"
	"os/exec"
	"time"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
	"github.com/Ktulue/KtulueKit-W11/internal/state"
)

// checkTimeoutSeconds is the timeout for detection check commands.
// Matches the installer package's check timeout.
const checkTimeoutSeconds = 15

// Status represents the detected install state of a single item.
type Status int

const (
	StatusInstalled Status = iota // check command returned exit 0 (or state says succeeded)
	StatusMissing                 // check command returned non-zero
	StatusUnknown                 // no check command, or check timed out / errored
)

// Item is a unified projection of any installable item (package, command, or extension)
// containing only the fields needed for detection.
// FlattenItems converts the three config types into this form.
type Item struct {
	ID       string
	Name     string
	Phase    int
	Tier     string // "winget" | "command" | "extension"
	CheckCmd string // empty = no check available → StatusUnknown
}

// Result is the detected state of a single Item.
type Result struct {
	Item   Item
	Status Status
}

// FlattenItems converts a Config into a single slice of Items across all tiers.
// Extensions have no check command and will always detect as StatusUnknown.
func FlattenItems(cfg *config.Config) []Item {
	items := make([]Item, 0, len(cfg.Packages)+len(cfg.Commands)+len(cfg.Extensions))

	for _, p := range cfg.Packages {
		items = append(items, Item{
			ID:       p.ID,
			Name:     p.Name,
			Phase:    p.Phase,
			Tier:     "winget",
			CheckCmd: p.Check,
		})
	}

	for _, c := range cfg.Commands {
		items = append(items, Item{
			ID:       c.ID,
			Name:     c.Name,
			Phase:    c.Phase,
			Tier:     "command",
			CheckCmd: c.Check,
		})
	}

	for _, e := range cfg.Extensions {
		items = append(items, Item{
			ID:    e.ID,
			Name:  e.Name,
			Phase: e.Phase,
			Tier:  "extension",
			// No check command for extensions — they show as Unknown
		})
	}

	return items
}

// CheckItem detects the install state of a single item.
//
// Logic:
//  1. If state.Succeeded[item.ID] is true → StatusInstalled (state-aware skip, no shell command run)
//  2. If item has no check command (or "echo skip") → StatusUnknown
//  3. Run check command silently (no output to terminal)
//     - exit 0 → StatusInstalled
//     - non-zero or timeout → StatusMissing
func CheckItem(item Item, s *state.State) Result {
	// State-aware skip: if a previous run already succeeded, trust it.
	if s != nil && s.Succeeded[item.ID] {
		return Result{Item: item, Status: StatusInstalled}
	}

	// No check command available.
	if item.CheckCmd == "" || item.CheckCmd == "echo skip" {
		return Result{Item: item, Status: StatusUnknown}
	}

	// Run the check command silently.
	if runCheckSilent(item.CheckCmd) {
		return Result{Item: item, Status: StatusInstalled}
	}
	return Result{Item: item, Status: StatusMissing}
}

// CheckAll runs CheckItem for every item in the slice and returns results in the same order.
func CheckAll(items []Item, s *state.State) []Result {
	results := make([]Result, len(items))
	for i, item := range items {
		results[i] = CheckItem(item, s)
	}
	return results
}

// runCheckSilent runs a check command and returns true if exit code is 0.
// All output is suppressed — this is purely detection, not installation.
func runCheckSilent(checkCmd string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeoutSeconds*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "cmd", "/C", checkCmd)
	err := cmd.Run()

	if ctx.Err() == context.DeadlineExceeded {
		return false // treat timeout as not-installed
	}
	return err == nil
}
```

- [ ] **Step 2: Build to confirm no compile errors**

```bash
cd F:/GDriveClone/Claude_Code/KtulueKit-W11
go build ./...
```
Expected: no output (success)

---

### Task 2: Write and run detector tests

All tests exercise pure logic only (no OS calls). The state-aware skip and no-check-command paths are testable without executing any shell commands.

**Files:**
- Create: `internal/detector/detector_test.go`

- [ ] **Step 1: Write the tests**

```go
package detector_test

import (
	"testing"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
	"github.com/Ktulue/KtulueKit-W11/internal/detector"
	"github.com/Ktulue/KtulueKit-W11/internal/state"
)

// --- CheckItem tests ---

func TestCheckItem_StateAwareSkip(t *testing.T) {
	// If state says succeeded, return StatusInstalled without running any shell command.
	s := &state.State{
		Succeeded: map[string]bool{"Git.Git": true},
		Failed:    map[string]bool{},
	}
	item := detector.Item{
		ID:       "Git.Git",
		Name:     "Git for Windows",
		Phase:    1,
		Tier:     "winget",
		CheckCmd: "", // no check command — proves state skip fires before check logic
	}

	result := detector.CheckItem(item, s)

	if result.Status != detector.StatusInstalled {
		t.Errorf("expected StatusInstalled from state-aware skip, got %v", result.Status)
	}
}

func TestCheckItem_NoCheckCmd_ReturnsUnknown(t *testing.T) {
	s := &state.State{
		Succeeded: map[string]bool{},
		Failed:    map[string]bool{},
	}
	item := detector.Item{
		ID:    "some-extension",
		Name:  "Some Extension",
		Phase: 5,
		Tier:  "extension",
		// No CheckCmd
	}

	result := detector.CheckItem(item, s)

	if result.Status != detector.StatusUnknown {
		t.Errorf("expected StatusUnknown for item with no check command, got %v", result.Status)
	}
}

func TestCheckItem_EchoSkip_ReturnsUnknown(t *testing.T) {
	// "echo skip" is a sentinel used in the config for items with no real check.
	s := &state.State{
		Succeeded: map[string]bool{},
		Failed:    map[string]bool{},
	}
	item := detector.Item{
		ID:       "manual-item",
		Name:     "Manual Item",
		Phase:    3,
		Tier:     "command",
		CheckCmd: "echo skip",
	}

	result := detector.CheckItem(item, s)

	if result.Status != detector.StatusUnknown {
		t.Errorf("expected StatusUnknown for 'echo skip' check, got %v", result.Status)
	}
}

func TestCheckItem_NilState_DoesNotPanic(t *testing.T) {
	// nil state is safe — treated as no succeeded items.
	item := detector.Item{
		ID:    "some-id",
		Name:  "Some Item",
		Phase: 1,
		Tier:  "winget",
		// No CheckCmd — avoids running a real shell command
	}

	// Should not panic
	result := detector.CheckItem(item, nil)

	if result.Status != detector.StatusUnknown {
		t.Errorf("expected StatusUnknown for nil state + no check cmd, got %v", result.Status)
	}
}

// --- FlattenItems tests ---

func TestFlattenItems_IncludesAllTiers(t *testing.T) {
	cfg := &config.Config{
		Packages: []config.Package{
			{ID: "Git.Git", Name: "Git", Phase: 1, Check: "git --version"},
		},
		Commands: []config.Command{
			{ID: "claude-code", Name: "Claude Code", Phase: 4, Check: "claude --version"},
		},
		Extensions: []config.Extension{
			{ID: "ublock", Name: "uBlock Origin", Phase: 5},
		},
	}

	items := detector.FlattenItems(cfg)

	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
}

func TestFlattenItems_PackageFieldsCorrect(t *testing.T) {
	cfg := &config.Config{
		Packages: []config.Package{
			{ID: "Git.Git", Name: "Git for Windows", Phase: 1, Check: "git --version"},
		},
	}

	items := detector.FlattenItems(cfg)

	if items[0].ID != "Git.Git" {
		t.Errorf("expected ID 'Git.Git', got %q", items[0].ID)
	}
	if items[0].Tier != "winget" {
		t.Errorf("expected Tier 'winget', got %q", items[0].Tier)
	}
	if items[0].CheckCmd != "git --version" {
		t.Errorf("expected CheckCmd 'git --version', got %q", items[0].CheckCmd)
	}
}

func TestFlattenItems_ExtensionHasNoCheckCmd(t *testing.T) {
	cfg := &config.Config{
		Extensions: []config.Extension{
			{ID: "ublock", Name: "uBlock Origin", Phase: 5},
		},
	}

	items := detector.FlattenItems(cfg)

	if items[0].CheckCmd != "" {
		t.Errorf("expected empty CheckCmd for extension, got %q", items[0].CheckCmd)
	}
	if items[0].Tier != "extension" {
		t.Errorf("expected Tier 'extension', got %q", items[0].Tier)
	}
}

func TestFlattenItems_EmptyConfig_ReturnsEmpty(t *testing.T) {
	cfg := &config.Config{}

	items := detector.FlattenItems(cfg)

	if len(items) != 0 {
		t.Errorf("expected 0 items for empty config, got %d", len(items))
	}
}

// --- CheckAll tests ---

func TestCheckAll_LengthMatchesInput(t *testing.T) {
	s := &state.State{
		Succeeded: map[string]bool{},
		Failed:    map[string]bool{},
	}
	items := []detector.Item{
		{ID: "a", Name: "A", Tier: "winget"},
		{ID: "b", Name: "B", Tier: "command"},
		{ID: "c", Name: "C", Tier: "extension"},
	}

	results := detector.CheckAll(items, s)

	if len(results) != len(items) {
		t.Errorf("expected %d results, got %d", len(items), len(results))
	}
}

func TestCheckAll_PreservesOrder(t *testing.T) {
	s := &state.State{
		Succeeded: map[string]bool{},
		Failed:    map[string]bool{},
	}
	items := []detector.Item{
		{ID: "first", Name: "First", Tier: "winget"},
		{ID: "second", Name: "Second", Tier: "command"},
	}

	results := detector.CheckAll(items, s)

	if results[0].Item.ID != "first" {
		t.Errorf("expected first result ID 'first', got %q", results[0].Item.ID)
	}
	if results[1].Item.ID != "second" {
		t.Errorf("expected second result ID 'second', got %q", results[1].Item.ID)
	}
}
```

- [ ] **Step 2: Run tests to verify they pass**

```bash
go test ./internal/detector/... -v
```
Expected: all tests PASS

- [ ] **Step 3: Commit**

```bash
git add internal/detector/
git commit -m "feat(detector): add CheckItem, CheckAll, FlattenItems with tests"
```

---

## Chunk 2: `status` Subcommand

### Task 3: Create `cmd/status.go`

`status.go` lives in `package main` alongside `main.go` and `admin_windows.go`. It uses the `configPath` variable declared in `main.go`.

**Files:**
- Create: `cmd/status.go`

- [ ] **Step 1: Create the file with correct imports**

```go
package main

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
	"github.com/Ktulue/KtulueKit-W11/internal/detector"
	"github.com/Ktulue/KtulueKit-W11/internal/state"
)

// ANSI color codes for status output.
const (
	colorGreen  = "\033[32m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorReset  = "\033[0m"
)

func runStatus(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	s, err := state.Load()
	if err != nil {
		return fmt.Errorf("state error: %w", err)
	}

	fmt.Printf("KtulueKit Status — %s\n\n", time.Now().Format("2006-01-02 15:04:05"))

	items := detector.FlattenItems(cfg)
	results := detector.CheckAll(items, s)

	printStatusTable(results)
	return nil
}

// printStatusTable groups results by phase and prints a formatted table.
func printStatusTable(results []detector.Result) {
	// Group by phase.
	byPhase := make(map[int][]detector.Result)
	for _, r := range results {
		byPhase[r.Item.Phase] = append(byPhase[r.Item.Phase], r)
	}

	// Sort phases.
	phases := make([]int, 0, len(byPhase))
	for phase := range byPhase {
		phases = append(phases, phase)
	}
	sort.Ints(phases)

	var totalInstalled, totalMissing, totalUnknown int

	for _, phase := range phases {
		fmt.Printf("Phase %d\n", phase)
		for _, r := range byPhase[phase] {
			label, color := statusLabel(r.Status)
			fmt.Printf("  %s%-9s%s  %-40s  %s\n",
				color, label, colorReset,
				r.Item.ID,
				r.Item.Name,
			)
			switch r.Status {
			case detector.StatusInstalled:
				totalInstalled++
			case detector.StatusMissing:
				totalMissing++
			case detector.StatusUnknown:
				totalUnknown++
			}
		}
		fmt.Println()
	}

	fmt.Println("─────────────────────────────────────────────────")
	fmt.Printf("Installed: %d   Missing: %d   Unknown: %d\n",
		totalInstalled, totalMissing, totalUnknown)
}

// statusLabel returns the display text and ANSI color for a status.
func statusLabel(s detector.Status) (label, color string) {
	switch s {
	case detector.StatusInstalled:
		return "[OK]", colorGreen
	case detector.StatusMissing:
		return "[MISSING]", colorRed
	default:
		return "[?]", colorYellow
	}
}
```

- [ ] **Step 2: Build to confirm no compile errors**

```bash
go build ./...
```
Expected: no output (success)

---

### Task 4: Wire `status` subcommand into `cmd/main.go`

**Files:**
- Modify: `cmd/main.go`

- [ ] **Step 1: Add the status subcommand in `main()`**

In `cmd/main.go`, inside `main()`, after the block of `root.PersistentFlags()` calls (after line 39) and before `if err := root.Execute()`, insert:

```go
statusCmd := &cobra.Command{
    Use:   "status",
    Short: "Scan machine and show install status for all configured items",
    RunE:  runStatus,
}
root.AddCommand(statusCmd)
```

> **Note:** Do NOT add a `--config` flag to `statusCmd`. The `--config` / `-c` flag is already registered as a `PersistentFlag` on `root` (line 36) and is automatically inherited by all subcommands. Adding it again will cause a Cobra panic at runtime ("flag redefined: config").

- [ ] **Step 2: Build**

```bash
go build ./...
```
Expected: no output (success)

- [ ] **Step 3: Smoke test the subcommand is registered**

```bash
./ktuluekit.exe --help
```
Expected: output includes `status` in the list of available commands.

- [ ] **Step 4: Commit**

```bash
git add cmd/status.go cmd/main.go
git commit -m "feat(status): add status subcommand with grouped phase table"
```

---

## Chunk 3: Pre-Run Summary + State-Aware Skip in Runner

### Task 5: Add pre-run summary to `runner.go`

Before the phase loop in `Runner.Run()`, detect all items and print a summary.

**Files:**
- Modify: `internal/runner/runner.go`

- [ ] **Step 1: Add detector import to runner.go**

In `internal/runner/runner.go`, add to the import block:
```go
"github.com/Ktulue/KtulueKit-W11/internal/detector"
```

- [ ] **Step 2: Add color constants near the top of runner.go**

After the package declaration and imports (before the `Runner` struct), add:
```go
// ANSI color codes for terminal output.
const (
	colorGreen  = "\033[32m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorReset  = "\033[0m"
)
```

- [ ] **Step 3: Add `printPreRunSummary` method to runner.go**

Add this method after the `Run()` function:
```go
// printPreRunSummary scans all config items and prints counts before the install loop starts.
// Returns true if nothing needs installing (caller should skip the phase loop).
// Dry-run mode always returns false — it proceeds to show what would be done.
func (r *Runner) printPreRunSummary() (nothingToDo bool) {
	if r.dryRun {
		return false
	}

	fmt.Println("Scanning machine...")
	items := detector.FlattenItems(r.cfg)
	results := detector.CheckAll(items, r.state)

	var installed, missing, unknown int
	for _, res := range results {
		switch res.Status {
		case detector.StatusInstalled:
			installed++
		case detector.StatusMissing:
			missing++
		case detector.StatusUnknown:
			unknown++
		}
	}

	fmt.Println()
	fmt.Printf("  %s[OK]%s      Already installed: %d\n", colorGreen, colorReset, installed)
	fmt.Printf("  %s[MISSING]%s To install:        %d\n", colorRed, colorReset, missing)
	fmt.Printf("  %s[?]%s       Unknown:           %d\n", colorYellow, colorReset, unknown)
	fmt.Println()

	if missing == 0 && unknown == 0 {
		fmt.Println("Nothing to install. Everything is already present.")
		return true
	}

	fmt.Println("Starting installation...")
	fmt.Println("─────────────────────────────────────────────────")
	return false
}
```

- [ ] **Step 4: Call `printPreRunSummary` inside `Run()`**

In `Runner.Run()`, after the System Restore point block and before `phases := r.collectPhases()`, add:

```go
if r.printPreRunSummary() {
    return
}
```

The relevant section of `Run()` should look like this after the edit:
```go
func (r *Runner) Run() {
	if r.resumePhase <= 1 {
		restore.CreateRestorePoint(r.dryRun)
	}

	if r.printPreRunSummary() {
		return
	}

	phases := r.collectPhases()
	pathRefreshed := false
	// ... rest unchanged
```

- [ ] **Step 5: Build**

```bash
go build ./...
```
Expected: no output (success)

- [ ] **Step 6: Commit**

```bash
git add internal/runner/runner.go
git commit -m "feat(runner): add pre-run detection summary before install loop"
```

---

### Task 6: Add state-aware skip in runner item loops

When a previous run already succeeded for an item, skip it entirely — no re-check, no re-install.

**Files:**
- Modify: `internal/runner/runner.go`

- [ ] **Step 1: Add state-aware skip to `runPackagesInPhase`**

In `runPackagesInPhase`, after `if pkg.Phase != phase { continue }`, add:

```go
// State-aware skip: if a previous run already succeeded, don't re-check or re-install.
if r.state.Succeeded[pkg.ID] {
    fmt.Printf("\n  Skipping (already succeeded): %s\n", pkg.Name)
    r.rep.Add(reporter.Result{
        ID:     pkg.ID,
        Name:   pkg.Name,
        Tier:   "winget",
        Status: reporter.StatusAlready,
        Detail: "already succeeded in a previous run",
    })
    continue
}
```

- [ ] **Step 2: Add state-aware skip to `runCommandsInPhase`**

In `runCommandsInPhase`, after `if cmd.Phase != phase { continue }`, add:

```go
// State-aware skip: if a previous run already succeeded, don't re-check or re-run.
if r.state.Succeeded[cmd.ID] {
    fmt.Printf("\n  Skipping (already succeeded): %s\n", cmd.Name)
    r.rep.Add(reporter.Result{
        ID:     cmd.ID,
        Name:   cmd.Name,
        Tier:   "command",
        Status: reporter.StatusAlready,
        Detail: "already succeeded in a previous run",
    })
    continue
}
```

- [ ] **Step 3: Build**

```bash
go build ./...
```
Expected: no output (success)

- [ ] **Step 4: Commit**

```bash
git add internal/runner/runner.go
git commit -m "feat(runner): skip already-succeeded items on resume without re-checking"
```

---

## Chunk 4: `setup.ps1` Bootstrap Script

### Task 7: Create `setup.ps1`

**Files:**
- Create: `setup.ps1`

- [ ] **Step 1: Create the file at the repo root**

```powershell
#Requires -Version 5.1
<#
.SYNOPSIS
    KtulueKit-W11 setup script. Installs Go if needed, builds the binary, and launches it.

.DESCRIPTION
    Run this from an admin PowerShell at the repo root to get KtulueKit running
    on a fresh or existing machine in one step.

    Usage:
        .\setup.ps1 [args passed through to ktuluekit.exe]

    Examples:
        .\setup.ps1
        .\setup.ps1 --dry-run
        .\setup.ps1 status

.NOTES
    Requirements:
      - Run as Administrator
      - Run from the repo root (where go.mod lives)
      - The repo must already be cloned
      - winget must be available (comes with Windows 11)
#>

param(
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$KtulueKitArgs
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

# ── Helper functions ────────────────────────────────────────────────────────

function Write-Step {
    param([string]$Message)
    Write-Host ""
    Write-Host "  $Message" -ForegroundColor Cyan
}

function Write-Success {
    param([string]$Message)
    Write-Host "  [OK] $Message" -ForegroundColor Green
}

function Write-Fail {
    param([string]$Message)
    Write-Host ""
    Write-Host "  [ERROR] $Message" -ForegroundColor Red
    Write-Host ""
    exit 1
}

# ── Step 1: Admin check ──────────────────────────────────────────────────────

Write-Host ""
Write-Host "KtulueKit-W11 Setup" -ForegroundColor White
Write-Host "──────────────────────────────────────────────────────" -ForegroundColor DarkGray

$principal = [Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Write-Fail "This script must be run as Administrator.`n  Right-click PowerShell and select 'Run as administrator', then try again."
}
Write-Success "Running as Administrator"

# ── Step 2: Verify repo root ─────────────────────────────────────────────────

if (-not (Test-Path "go.mod")) {
    Write-Fail "go.mod not found. Run this script from the KtulueKit-W11 repo root."
}
Write-Success "Repo root confirmed (go.mod found)"

# ── Step 3: Check / install Go ──────────────────────────────────────────────

Write-Step "Checking for Go..."

$goCmd = Get-Command go -ErrorAction SilentlyContinue
if ($goCmd) {
    $goVersion = & go version
    Write-Success "Go already installed: $goVersion"
} else {
    Write-Step "Go not found. Installing via winget..."

    $wingetCmd = Get-Command winget -ErrorAction SilentlyContinue
    if (-not $wingetCmd) {
        Write-Fail "winget not found. Install the App Installer from the Microsoft Store, then re-run."
    }

    winget install -e --id GoLang.Go --accept-package-agreements --accept-source-agreements
    if ($LASTEXITCODE -ne 0) {
        Write-Fail "winget failed to install Go (exit code $LASTEXITCODE). Check the output above."
    }

    # Refresh PATH in the current session so 'go' is immediately available.
    Write-Step "Refreshing PATH..."
    $machinePath = [System.Environment]::GetEnvironmentVariable("Path", "Machine")
    $userPath    = [System.Environment]::GetEnvironmentVariable("Path", "User")
    $env:Path    = "$machinePath;$userPath"

    # Verify Go is now on PATH.
    $goCmd = Get-Command go -ErrorAction SilentlyContinue
    if (-not $goCmd) {
        Write-Fail "Go was installed but is not on PATH. Close and reopen your terminal, then re-run setup.ps1."
    }

    $goVersion = & go version
    Write-Success "Go installed: $goVersion"
}

# ── Step 4: Build ktuluekit.exe ──────────────────────────────────────────────

Write-Step "Building ktuluekit.exe..."

if (Test-Path "ktuluekit.exe") {
    Write-Success "ktuluekit.exe already exists — skipping build."
    Write-Host "  (Delete ktuluekit.exe and re-run to force a rebuild.)" -ForegroundColor DarkGray
} else {
    & go build -o ktuluekit.exe ./cmd/
    if ($LASTEXITCODE -ne 0) {
        Write-Fail "Build failed (exit code $LASTEXITCODE). Check the output above."
    }
    Write-Success "Build succeeded: ktuluekit.exe"
}

# ── Step 5: Launch ───────────────────────────────────────────────────────────

Write-Host ""
Write-Host "──────────────────────────────────────────────────────" -ForegroundColor DarkGray

if ($KtulueKitArgs.Count -gt 0) {
    Write-Host "  Launching: ktuluekit.exe $KtulueKitArgs" -ForegroundColor White
} else {
    Write-Host "  Launching: ktuluekit.exe" -ForegroundColor White
}
Write-Host "──────────────────────────────────────────────────────" -ForegroundColor DarkGray
Write-Host ""

& .\ktuluekit.exe @KtulueKitArgs
exit $LASTEXITCODE
```

- [ ] **Step 2: Verify the file exists**

```bash
ls setup.ps1
```
Expected: file listed

- [ ] **Step 3: Smoke test the help flag passes through setup.ps1**

From an admin PowerShell at the repo root:
```powershell
.\setup.ps1 --help
```
Expected: KtulueKit help output (Go already installed → skips to build check → launches with `--help`)

- [ ] **Step 4: Commit**

```bash
git add setup.ps1
git commit -m "feat: add setup.ps1 bootstrap script (installs Go, builds, launches)"
```

---

## Final Verification

- [ ] **Run the full test suite**

```bash
go test ./... -v
```
Expected: all tests pass, including `internal/detector/...` and `internal/scheduler/...`

- [ ] **Run `ktuluekit status` as a smoke test**

From an admin terminal at the repo root:
```
.\ktuluekit.exe status
```
Expected: header with timestamp, items grouped by phase, `[OK]`/`[MISSING]`/`[?]` labels in ANSI color, summary footer with counts.

- [ ] **Run `ktuluekit --dry-run` and confirm dry-run proceeds without a pre-run scan**

```
.\ktuluekit.exe --dry-run
```
Expected: "DRY RUN — no changes will be made." header, then phase output directly. The pre-run scan is intentionally skipped in dry-run mode (no "Scanning machine..." message).

- [ ] **Run a second time with no state and confirm "Nothing to install" path**

Temporarily set all items to already-installed state by running the tool on a machine where everything is installed, then re-run. Or: manually create a `.ktuluekit-state.json` with all IDs marked succeeded.
Expected: "Nothing to install. Everything is already present." and clean exit.

- [ ] **Final commit if any loose ends**

```bash
git status
```
If any unstaged changes remain, stage and commit them.
