# Polish Sprint Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement five small UX improvements (winget pre-flight, source update, progress counter, per-item elapsed time, completion beep) in a single `maint/polish-sprint` branch.

**Architecture:** All changes live in `internal/installer/winget.go` (two new functions) and `internal/runner/runner.go` (fields + wiring). Total elapsed time is printed in `cmd/main.go` after `rep.Summary()` — `Run()` never calls `rep.Summary()` directly; that's the caller's job. No new packages, no interface changes.

**Tech Stack:** Go stdlib (`os/exec`, `time`, `context`), existing project packages.

**Spec:** `docs/superpowers/specs/2026-03-11-polish-sprint-design.md`

> **Note on spec deviation:** The spec's "Run() Flow" diagram shows `runStart` inside `Run()` and total elapsed printed after `r.rep.Summary()` inside `Run()`. However, `Run()` does not call `rep.Summary()` — that's done in `cmd/main.go:103`. Accordingly this plan places `runStart` and the total elapsed print in `cmd/main.go`'s `runInstall()` function, which is correct.

---

## Chunk 1: Branch + TODO.md housekeeping

### Task 1: Create feature branch and update TODO.md

**Files:**
- Modify: `TODO.md`

- [ ] **Step 1: Create the feature branch**

```bash
git checkout -b maint/polish-sprint
```

Expected: `Switched to a new branch 'maint/polish-sprint'`

- [ ] **Step 2: Mark already-shipped items as done in TODO.md**

In `TODO.md`, find and update these two lines in the **Small Feature Additions** section:

```
- [ ] **State-aware pre-check skip** — If `state.Succeeded["Git.Git"]` is true...
```
→
```
- [x] **State-aware pre-check skip** — If `state.Succeeded["Git.Git"]` is true...
```

And:
```
- [ ] **Color output (ANSI)** — Add ANSI color codes alongside emoji...
```
→
```
- [x] **Color output (ANSI)** — Add ANSI color codes alongside emoji...
```

Both shipped in the previous status/detection feature.

- [ ] **Step 3: Verify the file changed as expected**

```bash
grep -n "\[x\]" TODO.md | tail -5
```

Expected: State-aware pre-check skip and Color output (ANSI) appear in the output.

- [ ] **Step 4: Commit**

```bash
git add TODO.md
git commit -m "chore: mark state-aware skip and ANSI color as shipped in TODO"
```

---

## Chunk 2: Winget pre-flight check

### Task 2: Add `CheckWingetAvailable()` to winget.go and wire into Run()

**Files:**
- Modify: `internal/installer/winget.go` (add function before `InstallPackage`)
- Modify: `internal/runner/runner.go` (add call at start of `Run()`)

**Context:** `winget.go` already imports `context`, `os/exec`, and `time`. No new imports needed.
`runner.go` already imports `"os/exec"` and `"github.com/Ktulue/KtulueKit-W11/internal/installer"`. No new imports needed for this task.

- [ ] **Step 1: Add `CheckWingetAvailable` to `internal/installer/winget.go`**

Add this function **immediately before** the `InstallPackage` function (currently at line 14 in winget.go, right after the import block):

```go
// CheckWingetAvailable verifies that winget is on PATH and functional.
// Returns an error if winget is missing or does not respond within 5 seconds.
func CheckWingetAvailable() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "winget", "--version").Run()
}
```

- [ ] **Step 2: Wire into `runner.Run()`**

In `internal/runner/runner.go`, in `Run()`, add the pre-flight check as the **very first action**, before `restore.CreateRestorePoint`. Find this exact block:

```go
func (r *Runner) Run() {
	// Create a System Restore point before touching anything.
	// Skipped on resume runs (user already has the pre-run snapshot).
	if r.resumePhase <= 1 {
```

Replace with:

```go
func (r *Runner) Run() {
	// Fail fast if winget is missing or broken.
	if !r.dryRun {
		if err := installer.CheckWingetAvailable(); err != nil {
			fmt.Printf("ERROR: winget is not available: %v\n", err)
			fmt.Println("Install App Installer from the Microsoft Store, then re-run.")
			return
		}
	}

	// Create a System Restore point before touching anything.
	// Skipped on resume runs (user already has the pre-run snapshot).
	if r.resumePhase <= 1 {
```

- [ ] **Step 3: Build to verify no compile errors**

```bash
go build ./...
```

Expected: no output (clean build).

- [ ] **Step 4: Commit**

```bash
git add internal/installer/winget.go internal/runner/runner.go
git commit -m "feat(runner): add winget pre-flight check before install loop"
```

---

## Chunk 3: Winget source update

### Task 3: Add `UpdateSources()` to winget.go and wire into Run()

**Files:**
- Modify: `internal/installer/winget.go` (add function after `CheckWingetAvailable`)
- Modify: `internal/runner/runner.go` (call after pre-run summary)

**Context:** `winget.go` needs `"os"` added to its imports for `os.Stdout` and `os.Stderr`. Current imports are: `context`, `fmt`, `os/exec`, `strings`, `time`. Add `"os"`.

- [ ] **Step 1: Add `"os"` to imports in `internal/installer/winget.go`**

Find (the full import block — include the internal packages so the replacement is exact):
```go
import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
	"github.com/Ktulue/KtulueKit-W11/internal/reporter"
)
```

Replace with:
```go
import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
	"github.com/Ktulue/KtulueKit-W11/internal/reporter"
)
```

- [ ] **Step 2: Add `UpdateSources` to `internal/installer/winget.go`**

Add this function immediately after `CheckWingetAvailable` (and before `InstallPackage`):

```go
// UpdateSources runs "winget source update" to refresh the package database.
// Output is streamed to the console. Returns an error if the command fails.
func UpdateSources() error {
	cmd := exec.Command("winget", "source", "update")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
```

- [ ] **Step 3: Wire into `runner.Run()` after the pre-run summary**

In `internal/runner/runner.go`, in `Run()`, find:

```go
	if r.printPreRunSummary() {
		return
	}

	phases := r.collectPhases()
```

Replace with:

```go
	if r.printPreRunSummary() {
		return
	}

	if !r.dryRun {
		fmt.Println("Updating winget sources...")
		if err := installer.UpdateSources(); err != nil {
			fmt.Printf("  [warning] winget source update failed: %v\n", err)
		}
		fmt.Println()
	}

	phases := r.collectPhases()
```

- [ ] **Step 4: Build to verify no compile errors**

```bash
go build ./...
```

Expected: no output.

- [ ] **Step 5: Commit**

```bash
git add internal/installer/winget.go internal/runner/runner.go
git commit -m "feat(runner): run winget source update before install loop"
```

---

## Chunk 4: Progress counter

### Task 4: Add `[N/Total]` counter to all install output lines

**Files:**
- Modify: `internal/runner/runner.go` — add fields to struct, add `countItemsFromPhase` method, update all three `run*InPhase` functions
- Create: `internal/runner/runner_test.go` — test `countItemsFromPhase`

**Context:** This is a white-box test (testing an unexported method on `Runner`), so it uses `package runner` (not `package runner_test`). Look at `internal/scheduler/scheduler_windows_test.go` for the test style (one assertion per test, descriptive function names, no table-driven tests). `runner.go` currently uses `"fmt"`, `"os/exec"`, `"path/filepath"`, `"sort"`, `"strings"`, `"bufio"`, `"os"` and internal packages — no `"time"` import yet.

- [ ] **Step 1: Write the failing test first**

Create `internal/runner/runner_test.go`:

```go
package runner

import (
	"testing"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
)

func TestCountItemsFromPhase_AllPhases(t *testing.T) {
	r := &Runner{
		cfg: &config.Config{
			Packages:   []config.Package{{ID: "p1", Phase: 1}, {ID: "p2", Phase: 2}},
			Commands:   []config.Command{{ID: "c1", Phase: 2}},
			Extensions: []config.Extension{{ID: "e1", Phase: 3}},
		},
	}

	got := r.countItemsFromPhase(1)

	if got != 4 {
		t.Errorf("countItemsFromPhase(1): expected 4, got %d", got)
	}
}

func TestCountItemsFromPhase_ResumeFiltersEarlierPhases(t *testing.T) {
	r := &Runner{
		cfg: &config.Config{
			Packages: []config.Package{
				{ID: "p1", Phase: 1},
				{ID: "p2", Phase: 2},
				{ID: "p3", Phase: 3},
			},
			Commands:   []config.Command{{ID: "c1", Phase: 2}},
			Extensions: []config.Extension{},
		},
	}

	// fromPhase=2 counts p2, p3, c1 — excludes p1 (phase 1)
	got := r.countItemsFromPhase(2)

	if got != 3 {
		t.Errorf("countItemsFromPhase(2): expected 3, got %d", got)
	}
}

func TestCountItemsFromPhase_EmptyConfig(t *testing.T) {
	r := &Runner{cfg: &config.Config{}}

	got := r.countItemsFromPhase(1)

	if got != 0 {
		t.Errorf("countItemsFromPhase on empty config: expected 0, got %d", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/runner/...
```

Expected: compile error — `undefined: Runner.countItemsFromPhase` (or `unknown field plannedIDs` if Runner struct changes haven't been applied yet — both indicate the method doesn't exist, which is correct at this stage).

- [ ] **Step 3: Add fields to `Runner` struct**

In `internal/runner/runner.go`, find the `Runner` struct definition:

```go
type Runner struct {
	cfg          *config.Config
	rep          *reporter.Reporter
	state        *state.State
	dryRun       bool
	resumePhase  int
	configPath   string               // preserved so resume commands can reference the right config file
	shortcutMode desktop.ShortcutMode // how to handle .lnk files dropped by installers
	plannedIDs   map[string]bool      // all IDs declared in config (packages + commands)
}
```

Replace with:

```go
type Runner struct {
	cfg          *config.Config
	rep          *reporter.Reporter
	state        *state.State
	dryRun       bool
	resumePhase  int
	configPath   string               // preserved so resume commands can reference the right config file
	shortcutMode desktop.ShortcutMode // how to handle .lnk files dropped by installers
	plannedIDs   map[string]bool      // all IDs declared in config (packages + commands)
	totalItems   int                  // total items in phases >= resumePhase
	itemIdx      int                  // current item index (1-based, increments each item)
}
```

- [ ] **Step 4: Add `countItemsFromPhase` method**

Find this exact block (the comment that opens `Run()`, used as the insertion anchor):

```go
// Run executes all phases in order.
func (r *Runner) Run() {
```

Replace with (inserting the new method above `Run()`):

```go
// countItemsFromPhase returns the total number of items across all tiers
// in phases >= fromPhase. Used to drive the [N/Total] progress counter.
func (r *Runner) countItemsFromPhase(fromPhase int) int {
	count := 0
	for _, p := range r.cfg.Packages {
		if p.Phase >= fromPhase {
			count++
		}
	}
	for _, c := range r.cfg.Commands {
		if c.Phase >= fromPhase {
			count++
		}
	}
	for _, e := range r.cfg.Extensions {
		if e.Phase >= fromPhase {
			count++
		}
	}
	return count
}

// Run executes all phases in order.
func (r *Runner) Run() {
```

- [ ] **Step 5: Initialize `totalItems` at start of `Run()`**

In `Run()`, add `r.totalItems = r.countItemsFromPhase(r.resumePhase)` as the very first line inside `Run()`, before the winget pre-flight check. Find:

```go
func (r *Runner) Run() {
	// Fail fast if winget is missing or broken.
```

Replace with:

```go
func (r *Runner) Run() {
	r.totalItems = r.countItemsFromPhase(r.resumePhase)

	// Fail fast if winget is missing or broken.
```

- [ ] **Step 6: Update `runPackagesInPhase` — state-skipped path**

In `runPackagesInPhase`, find this exact block (the state-aware skip, unique to this function because it uses `pkg`):

```go
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

Replace with:

```go
		if r.state.Succeeded[pkg.ID] {
			r.itemIdx++
			fmt.Printf("\n  [%d/%d] Skipping (already succeeded): %s\n", r.itemIdx, r.totalItems, pkg.Name)
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

- [ ] **Step 7: Update `runPackagesInPhase` — active install path**

In `runPackagesInPhase`, find:

```go
		fmt.Printf("\n  Installing: %s\n", pkg.Name)
		res := installer.InstallPackage(pkg, r.dryRun, r.cfg.Settings.RetryCount, r.cfg.Settings.UpgradeIfInstalled)
```

Replace with:

```go
		r.itemIdx++
		fmt.Printf("\n  [%d/%d] Installing: %s\n", r.itemIdx, r.totalItems, pkg.Name)
		res := installer.InstallPackage(pkg, r.dryRun, r.cfg.Settings.RetryCount, r.cfg.Settings.UpgradeIfInstalled)
```

- [ ] **Step 8: Update `runCommandsInPhase` — state-skipped path**

In `runCommandsInPhase`, find this exact block (unique because it uses `cmd` and `Tier: "command"`):

```go
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

Replace with:

```go
		if r.state.Succeeded[cmd.ID] {
			r.itemIdx++
			fmt.Printf("\n  [%d/%d] Skipping (already succeeded): %s\n", r.itemIdx, r.totalItems, cmd.Name)
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

- [ ] **Step 9: Update `runCommandsInPhase` — active run path**

In `runCommandsInPhase`, find:

```go
		fmt.Printf("\n  Running: %s\n", cmd.Name)
```

Replace with:

```go
		r.itemIdx++
		fmt.Printf("\n  [%d/%d] Running: %s\n", r.itemIdx, r.totalItems, cmd.Name)
```

- [ ] **Step 10: Update `runExtensionsInPhase` — state-skipped path**

In `runExtensionsInPhase`, find this exact block (unique because it uses `ext` and `Tier: "extension"`):

```go
		if r.state.Succeeded[ext.ID] {
			fmt.Printf("\n  Skipping (already succeeded): %s\n", ext.Name)
			r.rep.Add(reporter.Result{
				ID:     ext.ID,
				Name:   ext.Name,
				Tier:   "extension",
				Status: reporter.StatusAlready,
				Detail: "already succeeded in a previous run",
			})
			continue
		}
```

Replace with:

```go
		if r.state.Succeeded[ext.ID] {
			r.itemIdx++
			fmt.Printf("\n  [%d/%d] Skipping (already succeeded): %s\n", r.itemIdx, r.totalItems, ext.Name)
			r.rep.Add(reporter.Result{
				ID:     ext.ID,
				Name:   ext.Name,
				Tier:   "extension",
				Status: reporter.StatusAlready,
				Detail: "already succeeded in a previous run",
			})
			continue
		}
```

- [ ] **Step 11: Update `runExtensionsInPhase` — active extension path**

In `runExtensionsInPhase`, find:

```go
		fmt.Printf("\n  Extension: %s\n", ext.Name)
		res := installer.InstallExtension(ext, r.dryRun)
```

Replace with:

```go
		r.itemIdx++
		fmt.Printf("\n  [%d/%d] Extension: %s\n", r.itemIdx, r.totalItems, ext.Name)
		res := installer.InstallExtension(ext, r.dryRun)
```

- [ ] **Step 12: Run tests to verify they pass**

```bash
go test ./internal/runner/...
```

Expected: PASS (3 tests pass).

- [ ] **Step 13: Build and run full test suite**

```bash
go build ./...
go test ./...
```

Expected: clean build, all tests pass.

- [ ] **Step 14: Commit**

```bash
git add internal/runner/runner.go internal/runner/runner_test.go
git commit -m "feat(runner): add [N/Total] progress counter to all install output"
```

---

## Chunk 5: Elapsed time

### Task 5: Add per-item elapsed time and total elapsed

**Files:**
- Modify: `internal/runner/runner.go` — add `"time"` import, per-item timing in all three `run*InPhase` functions
- Modify: `cmd/main.go` — add `"time"` import, record `runStart`, print total elapsed after `rep.Summary()`

**Context:** `runner.go` does NOT currently import `"time"` — it must be added. `cmd/main.go` also does not import `"time"`.

The state-skipped paths in `run*InPhase` do NOT get elapsed time — they return in microseconds and timing adds no value. Only the actual install/run call gets a timer.

For `runCommandsInPhase`: the dependency check (`dependenciesMet`) and its early `continue` happen between the counter print and the `RunCommand` call. The timer wraps only `RunCommand`, not the dependency check.

- [ ] **Step 1: Add `"time"` import to `internal/runner/runner.go`**

In `runner.go`, find the import block:

```go
import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
```

Replace with:

```go
import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
```

- [ ] **Step 2: Add per-item timing to `runPackagesInPhase`**

The desktop snapshot block (`desktopBefore`) sits between the progress-counter print and the `InstallPackage` call, so we cannot use a single find block spanning both. Instead, make two targeted find-replaces:

**2a.** Add `start := time.Now()` immediately before the install call. Find (unique in the entire file):

```go
		res := installer.InstallPackage(pkg, r.dryRun, r.cfg.Settings.RetryCount, r.cfg.Settings.UpgradeIfInstalled)
```

Replace with:

```go
		start := time.Now()
		res := installer.InstallPackage(pkg, r.dryRun, r.cfg.Settings.RetryCount, r.cfg.Settings.UpgradeIfInstalled)
```

**2b.** Add the elapsed print immediately after `r.rep.Add(res)` in `runPackagesInPhase`. Find (use the following lines to uniquely identify the correct `r.rep.Add(res)` — the one followed by the state-marking if-block):

```go
		r.rep.Add(res)

		if res.Status == reporter.StatusInstalled || res.Status == reporter.StatusUpgraded || res.Status == reporter.StatusAlready {
```

Replace with:

```go
		r.rep.Add(res)
		fmt.Printf("      elapsed: %s\n", time.Since(start).Round(time.Second))

		if res.Status == reporter.StatusInstalled || res.Status == reporter.StatusUpgraded || res.Status == reporter.StatusAlready {
```

- [ ] **Step 3: Add per-item timing to `runCommandsInPhase`**

In `runCommandsInPhase`, find:

```go
		res := installer.RunCommand(cmd, r.dryRun, r.cfg.Settings.RetryCount, r.state)
		r.rep.Add(res)
```

Replace with:

```go
		start := time.Now()
		res := installer.RunCommand(cmd, r.dryRun, r.cfg.Settings.RetryCount, r.state)
		r.rep.Add(res)
		fmt.Printf("      elapsed: %s\n", time.Since(start).Round(time.Second))
```

- [ ] **Step 4: Add per-item timing to `runExtensionsInPhase`**

In `runExtensionsInPhase`, find (after Chunk 4 Step 11, `start := time.Now()` does NOT yet exist here):

```go
		r.itemIdx++
		fmt.Printf("\n  [%d/%d] Extension: %s\n", r.itemIdx, r.totalItems, ext.Name)
		res := installer.InstallExtension(ext, r.dryRun)
		r.rep.Add(res)
```

Replace with:

```go
		r.itemIdx++
		fmt.Printf("\n  [%d/%d] Extension: %s\n", r.itemIdx, r.totalItems, ext.Name)
		start := time.Now()
		res := installer.InstallExtension(ext, r.dryRun)
		r.rep.Add(res)
		fmt.Printf("      elapsed: %s\n", time.Since(start).Round(time.Second))
```

- [ ] **Step 5: Add `"time"` import to `cmd/main.go`**

In `cmd/main.go`, find:

```go
import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
```

Replace with:

```go
import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
```

- [ ] **Step 6: Record start time and print total elapsed in `runInstall`**

In `cmd/main.go`, in `runInstall()`, find:

```go
	r := runner.New(cfg, rep, s, dryRun, resumePhase, configPath, shortcutMode)
	r.Run()

	rep.Summary()
```

Replace with:

```go
	r := runner.New(cfg, rep, s, dryRun, resumePhase, configPath, shortcutMode)

	runStart := time.Now()
	r.Run()

	rep.Summary()
	fmt.Printf("Total elapsed: %s\n", time.Since(runStart).Round(time.Second))
```

- [ ] **Step 7: Build to verify no compile errors**

```bash
go build ./...
```

Expected: no output.

- [ ] **Step 8: Run full test suite**

```bash
go test ./...
```

Expected: all tests pass.

- [ ] **Step 9: Commit**

```bash
git add internal/runner/runner.go cmd/main.go
git commit -m "feat(runner): add per-item elapsed time and total run elapsed"
```

---

## Chunk 6: Completion beep + TODO housekeeping

### Task 6: Add completion beep and mark TODO items done

**Files:**
- Modify: `internal/runner/runner.go` — add beep at end of `Run()`
- Modify: `TODO.md` — mark 5 implemented items `[x]`

**Context:** `"os/exec"` is already imported in `runner.go` (used in `promptReboot`). No new imports needed for this task.

- [ ] **Step 1: Add beep at end of `Run()`**

In `internal/runner/runner.go`, find the closing brace of `Run()`. The last meaningful line of the phase loop is `r.runExtensionsInPhase(phase)`. Find:

```go
		r.runPackagesInPhase(phase)
		r.runCommandsInPhase(phase)
		r.runExtensionsInPhase(phase)
	}
}
```

Replace with:

```go
		r.runPackagesInPhase(phase)
		r.runCommandsInPhase(phase)
		r.runExtensionsInPhase(phase)
	}

	// Play a completion beep (skipped in dry-run).
	if !r.dryRun {
		_ = exec.Command("powershell", "-NoProfile", "-Command", "[console]::beep(800,300)").Run()
	}
}
```

- [ ] **Step 2: Build to verify no compile errors**

```bash
go build ./...
```

Expected: no output.

- [ ] **Step 3: Run full test suite**

```bash
go test ./...
```

Expected: all tests pass.

- [ ] **Step 4: Update TODO.md — mark 5 items done**

In `TODO.md`, update these items in **One-Liner Code Changes**:

```
- [ ] **`winget source update` at startup**
```
→ `- [x] **`winget source update` at startup**`

```
- [ ] **Progress counter**
```
→ `- [x] **Progress counter**`

```
- [ ] **Elapsed time per package**
```
→ `- [x] **Elapsed time per package**`

```
- [ ] **Completion notification**
```
→ `- [x] **Completion notification**`

And in **Small Feature Additions**:
```
- [ ] **Winget availability pre-flight check**
```
→ `- [x] **Winget availability pre-flight check**`

- [ ] **Step 5: Commit**

```bash
git add internal/runner/runner.go TODO.md
git commit -m "feat(runner): add completion beep; mark polish sprint items done in TODO"
```

---

## Final: Security review and PR

- [ ] Run `/security-review` before pushing
- [ ] Push branch and create PR:

```bash
git push -u origin maint/polish-sprint
```
