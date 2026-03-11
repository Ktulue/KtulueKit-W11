# Status, Detection & Setup — Design Spec

**Date:** 2026-03-11
**Goal:** Make KtulueKit usable on a fresh or previously-run machine — correctly identify what's installed, what's needed, and install it.

---

## Scope

Four pieces of work:

1. `internal/detector/` — shared detection package
2. `ktuluekit status` — read-only scan subcommand
3. Pre-run summary in `runner.go`
4. `setup.ps1` — hands-off bootstrap script

---

## 1. `internal/detector/` Package

**File:** `internal/detector/detector.go`

### Types

```go
type Status int

const (
    StatusInstalled Status = iota
    StatusMissing
    StatusUnknown  // check timed out or errored ambiguously
)

type Result struct {
    ID     string
    Name   string
    Status Status
}
```

### Functions

**`CheckItem(item config.Item, state *state.State) Result`**

Logic:
1. If `state.Succeeded[item.ID]` is true → return `StatusInstalled` immediately (state-aware skip, no check command run)
2. If item has no check command → return `StatusUnknown`
3. Run check command with 15s timeout
4. Exit code 0 → `StatusInstalled`, non-zero → `StatusMissing`, timeout/exec error → `StatusUnknown`

**`CheckAll(items []config.Item, state *state.State) []Result`**

Convenience wrapper — calls `CheckItem` for each item in the slice. Used by both `status` and the runner's pre-run summary.

### Constraints
- No output, no side effects — pure detection
- Caller decides how to display or act on results
- 15s timeout matches existing runner check command behavior

---

## 2. `ktuluekit status` Subcommand

**Invocation:** `ktuluekit status`
**Behavior:** Read-only. No installs, no state changes.

### Flow
1. Load config + state file
2. Print header: `KtulueKit Status — <timestamp>`
3. Call `detector.CheckAll()` across all phases
4. Display results grouped by phase, one line per item
5. Print summary footer with counts

### Output Format

```
KtulueKit Status — 2026-03-11 14:32:01

Phase 1 — Runtimes
  [OK]      Git.Git               Git for Windows
  [OK]      GoLang.Go             Go
  [MISSING] Microsoft.PowerShell  PowerShell 7
  [?]       Rustlang.Rustup       Rust (rustup)

Phase 2 — ...

─────────────────────────────────────────────────
Installed: 28   Missing: 9   Unknown: 3
```

### ANSI Color
- `[OK]` — green
- `[MISSING]` — red
- `[?]` — yellow

Same color scheme used throughout the app (pre-run summary, install output, final summary report). No emojis anywhere in the application.

---

## 3. Pre-Run Summary in `runner.go`

Before the phase loop, the runner calls `detector.CheckAll()` and prints:

```
Pre-run scan complete.

  To install:  9
  Already OK:  28
  Unknown:     3

Starting installation...
─────────────────────────────────────────────────
```

If `To install: 0` and `Unknown: 0`, print that and exit cleanly without running the phase loop.

**State-aware skip:** The runner calls `detector.CheckItem()` instead of running check commands inline. The state-aware skip in the detector handles already-succeeded items automatically — no redundant `winget list` calls on resume.

---

## 4. `setup.ps1`

**Location:** repo root
**Run from:** admin PowerShell, from the repo root directory
**Purpose:** Get KtulueKit running on a fresh or existing machine in one step.

### Flow
1. Check admin — warn and exit if not elevated
2. Check if `go` is on PATH — skip Go install if already present
3. If not: `winget install -e --id GoLang.Go --accept-package-agreements --accept-source-agreements`
4. Refresh PATH in the current session
5. Re-check `go` is available — fail with clear message if not
6. Check if `ktuluekit.exe` exists in current directory — if not, run `go build -o ktuluekit.exe ./cmd/`
7. Launch `.\ktuluekit.exe`, passing through any arguments given to `setup.ps1`

### Assumptions
- Script is run from the repo root (where `go.mod` lives)
- Repo is already cloned — setup does not handle cloning
- Args pass-through: `.\setup.ps1 --dry-run` → `.\ktuluekit.exe --dry-run`

---

## Shared Conventions

- No emojis anywhere in the application
- Text status markers: `[OK]`, `[MISSING]`, `[?]`
- ANSI color: green/red/yellow for the three states
- 15s timeout for all check commands
