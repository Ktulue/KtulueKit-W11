# Auto-Resume via Scheduled Task — Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** After triggering a reboot, automatically register a one-shot Windows Scheduled Task that re-launches KtulueKit at next logon with the correct `--resume-phase` flag, then self-deletes at the start of the resumed run.

**Architecture:** New `internal/scheduler/` package with two exported functions (`CreateResumeTask`, `DeleteResumeTask`) plus no-op stubs for non-Windows builds. `cmd/main.go` calls `DeleteResumeTask()` as its very first action. `internal/runner/runner.go`'s `promptReboot()` calls `CreateResumeTask()` after saving state, and `DeleteResumeTask()` if the user cancels the reboot.

**Tech Stack:** Go stdlib (`os/exec`, `fmt`, `strings`), Windows `powershell` + `schtasks` CLI tools.

**Spec:** `docs/superpowers/specs/2026-03-10-auto-resume-scheduled-task-design.md`

---

## Chunk 1: `internal/scheduler/` Package

### Files
- **Create:** `internal/scheduler/scheduler_windows.go`
- **Create:** `internal/scheduler/scheduler_stub.go`
- **Create:** `internal/scheduler/scheduler_windows_test.go`

---

### Task 1: Stub file for non-Windows builds

**Files:**
- Create: `internal/scheduler/scheduler_stub.go`

> **Note on project convention:** Other packages (`restore`, `desktop`, `installer`) use `_windows.go` filenames with no stubs — they're Windows-only with no cross-compile support. This package deliberately adds a stub to keep `go build ./...` working on non-Windows (useful for CI lint passes). The `//go:build !windows` tag is necessary since the stub filename itself doesn't indicate platform.

- [ ] **Step 1: Create the stub**

```go
//go:build !windows

package scheduler

// CreateResumeTask is a no-op on non-Windows platforms.
func CreateResumeTask(binaryPath, configPath, workDir string, resumePhase int, dryRun bool) error {
	return nil
}

// DeleteResumeTask is a no-op on non-Windows platforms.
func DeleteResumeTask() error {
	return nil
}
```

- [ ] **Step 2: Verify it compiles on Windows (current platform)**

```bash
cd F:/GDriveClone/Claude_Code/KtulueKit-W11
go build ./internal/scheduler/...
```

Expected: no output, exit 0.

- [ ] **Step 3: Verify the stub compiles on non-Windows**

```bash
cd F:/GDriveClone/Claude_Code/KtulueKit-W11
GOOS=linux go build ./internal/scheduler/...
```

Expected: no output, exit 0. (This actually compiles `scheduler_stub.go` — the Windows file is excluded by filename suffix.)

- [ ] **Step 4: Commit**

```bash
git add internal/scheduler/scheduler_stub.go
git commit -m "feat(scheduler): add no-op stubs for non-Windows builds"
```

---

### Task 2: Write failing tests for `buildTaskScript`

**Files:**
- Create: `internal/scheduler/scheduler_windows_test.go`

> **Note:** The `_windows` suffix means Go only compiles this file on Windows — same as `scheduler_windows.go`. These tests will not run on non-Windows CI. That's acceptable for this Windows-only tool. The red/green TDD cycle below only works on Windows.
>
> **Commit timing:** The test file is committed in Task 3 alongside the implementation (once both are green). Do NOT commit the test file alone after Step 2 — a file with a compile error doesn't belong in history.

- [ ] **Step 1: Write the failing tests**

```go
package scheduler

import (
	"strings"
	"testing"
)

func TestBuildTaskScript_ContainsAllParts(t *testing.T) {
	script := buildTaskScript(
		`C:\tools\ktuluekit.exe`,
		`C:\config\ktuluekit.json`,
		`C:\tools`,
		4,
	)

	checks := []struct {
		label string
		want  string
	}{
		{"binary path", `C:\tools\ktuluekit.exe`},
		{"config path", `C:\config\ktuluekit.json`},
		{"working dir", `C:\tools`},
		{"resume phase", `--resume-phase=4`},
		{"task name", `KtulueKit-Resume`},
		{"run level", `Highest`},
		{"logon type", `Interactive`},
		{"force flag", `-Force`},
	}

	for _, c := range checks {
		if !strings.Contains(script, c.want) {
			t.Errorf("script missing %s: expected to find %q", c.label, c.want)
		}
	}
}

func TestBuildTaskScript_EscapesSingleQuotes(t *testing.T) {
	// Windows paths won't normally have single quotes, but we must handle it.
	script := buildTaskScript(
		`C:\it's\ktuluekit.exe`,
		`C:\config\ktuluekit.json`,
		`C:\tools`,
		1,
	)

	if strings.Contains(script, `it's`) {
		t.Error("unescaped single quote found in script — PowerShell will break")
	}
	if !strings.Contains(script, `it''s`) {
		t.Error("expected single quote to be doubled (it''s) for PowerShell string escaping")
	}
}

func TestBuildTaskScript_QuotesBinaryPathWithSpaces(t *testing.T) {
	// -Execute must double-quote the binary path so Windows CreateProcess handles
	// paths containing spaces correctly (e.g. C:\Program Files\ktuluekit.exe).
	script := buildTaskScript(
		`C:\Program Files\ktuluekit.exe`,
		`C:\config\ktuluekit.json`,
		`C:\tools`,
		2,
	)

	// The binary path must be wrapped in double-quotes inside the PS string.
	if !strings.Contains(script, `"C:\Program Files\ktuluekit.exe"`) {
		t.Error("binary path with spaces is not double-quoted in -Execute — will fail at runtime")
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
cd F:/GDriveClone/Claude_Code/KtulueKit-W11
go test ./internal/scheduler/... -v -run TestBuildTaskScript
```

Expected: compile error — `buildTaskScript` and `scheduler_windows.go` don't exist yet.

---

### Task 3: Implement `scheduler_windows.go`

**Files:**
- Create: `internal/scheduler/scheduler_windows.go`

- [ ] **Step 1: Create the implementation**

```go
package scheduler

import (
	"fmt"
	"os/exec"
	"strings"
)

const taskName = "KtulueKit-Resume"

// CreateResumeTask registers a one-shot Windows Scheduled Task that re-launches
// KtulueKit at the next interactive logon with the given resume phase.
// The task runs with HIGHEST privileges under the current user.
// If dryRun is true, prints intent and returns without executing.
func CreateResumeTask(binaryPath, configPath, workDir string, resumePhase int, dryRun bool) error {
	script := buildTaskScript(binaryPath, configPath, workDir, resumePhase)

	if dryRun {
		fmt.Printf("  [dry-run] Would register Scheduled Task '%s' via PowerShell:\n", taskName)
		fmt.Printf("    %s\n", script)
		return nil
	}

	cmd := exec.Command("powershell", "-NonInteractive", "-NoProfile", "-Command", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not register scheduled task: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// DeleteResumeTask removes the KtulueKit-Resume scheduled task if it exists.
// Returns nil regardless — absence of the task is not an error.
func DeleteResumeTask() error {
	_ = exec.Command("schtasks", "/delete", "/tn", taskName, "/f").Run()
	return nil
}

// buildTaskScript returns the PowerShell one-liner that creates (or replaces)
// the KtulueKit-Resume scheduled task with the correct action, trigger, and principal.
func buildTaskScript(binaryPath, configPath, workDir string, resumePhase int) string {
	bin := escapeSingleQuote(binaryPath)
	cfg := escapeSingleQuote(configPath)
	wd := escapeSingleQuote(workDir)

	// NOTE: -Execute uses '"%s"' (double-quotes inside single-quoted PS string) so that
	// Windows CreateProcess correctly handles paths containing spaces. WorkingDirectory
	// uses single-quotes only — it is passed as a directory path, not a command token.
	return fmt.Sprintf(
		`$a = New-ScheduledTaskAction -Execute '"%s"' -Argument '--config "%s" --resume-phase=%d' -WorkingDirectory '%s'; `+
			`$t = New-ScheduledTaskTrigger -AtLogOn; `+
			`$s = New-ScheduledTaskSettingsSet -ExecutionTimeLimit (New-TimeSpan -Hours 4) -MultipleInstances IgnoreNew; `+
			`$p = New-ScheduledTaskPrincipal -UserId $env:USERNAME -RunLevel Highest -LogonType Interactive; `+
			`Register-ScheduledTask -TaskName '%s' -Action $a -Trigger $t -Settings $s -Principal $p -Force`,
		bin, cfg, resumePhase, wd, taskName,
	)
}

// escapeSingleQuote doubles any single quotes in s so it can be safely embedded
// inside a PowerShell single-quoted string.
func escapeSingleQuote(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
```

- [ ] **Step 2: Run the tests**

```bash
cd F:/GDriveClone/Claude_Code/KtulueKit-W11
go test ./internal/scheduler/... -v -run TestBuildTaskScript
```

Expected output:
```
=== RUN   TestBuildTaskScript_ContainsAllParts
--- PASS: TestBuildTaskScript_ContainsAllParts
=== RUN   TestBuildTaskScript_EscapesSingleQuotes
--- PASS: TestBuildTaskScript_EscapesSingleQuotes
=== RUN   TestBuildTaskScript_QuotesBinaryPathWithSpaces
--- PASS: TestBuildTaskScript_QuotesBinaryPathWithSpaces
PASS
```

- [ ] **Step 3: Verify full build still passes**

```bash
cd F:/GDriveClone/Claude_Code/KtulueKit-W11
go build ./...
```

Expected: no output, exit 0.

- [ ] **Step 4: Commit**

```bash
git add internal/scheduler/scheduler_windows.go internal/scheduler/scheduler_windows_test.go
git commit -m "feat(scheduler): implement CreateResumeTask and DeleteResumeTask for Windows"
```

---

## Chunk 2: Integration

### Files
- **Modify:** `cmd/main.go` — add import + `DeleteResumeTask()` call at top of `runInstall`
- **Modify:** `internal/runner/runner.go` — add import + `CreateResumeTask()` + cancel-path `DeleteResumeTask()` in `promptReboot()`

---

### Task 4: Wire `DeleteResumeTask()` into `cmd/main.go`

**Files:**
- Modify: `cmd/main.go:46-50`

The call must be the absolute first thing in `runInstall` — before the admin check, before config load. This guarantees no orphaned task survives a crash mid-run.

- [ ] **Step 1: Add the import**

In `cmd/main.go`, the import block currently ends at line 14. Add the scheduler package:

```go
import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
	"github.com/Ktulue/KtulueKit-W11/internal/desktop"
	"github.com/Ktulue/KtulueKit-W11/internal/reporter"
	"github.com/Ktulue/KtulueKit-W11/internal/runner"
	"github.com/Ktulue/KtulueKit-W11/internal/scheduler"
	"github.com/Ktulue/KtulueKit-W11/internal/state"
)
```

- [ ] **Step 2: Add `DeleteResumeTask()` as the first line of `runInstall`**

Current `runInstall` starts at line 46. Insert before the `isAdmin()` check:

```go
func runInstall(cmd *cobra.Command, args []string) error {
	// Always delete the resume task first — cleans up after a previous reboot run.
	// No-op (and error ignored) if the task doesn't exist.
	_ = scheduler.DeleteResumeTask()

	if !dryRun && !isAdmin() {
```

- [ ] **Step 3: Build to confirm no compile errors**

```bash
cd F:/GDriveClone/Claude_Code/KtulueKit-W11
go build ./...
```

Expected: no output, exit 0.

- [ ] **Step 4: Commit**

```bash
git add cmd/main.go
git commit -m "feat(main): delete resume scheduled task at startup before any other action"
```

---

### Task 5: Wire `CreateResumeTask()` into `runner.go`'s `promptReboot()`

**Files:**
- Modify: `internal/runner/runner.go:244-287`

Two changes to `promptReboot()`:
1. Call `CreateResumeTask()` after `SaveResumePhase()`, update the banner to show task status.
2. Call `DeleteResumeTask()` after `shutdown /a` (cancel path).

- [ ] **Step 1: Add the scheduler import to runner.go**

The import block in `runner.go` currently ends at line 18. Add:

```go
import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
	"github.com/Ktulue/KtulueKit-W11/internal/desktop"
	"github.com/Ktulue/KtulueKit-W11/internal/installer"
	"github.com/Ktulue/KtulueKit-W11/internal/reporter"
	"github.com/Ktulue/KtulueKit-W11/internal/restore"
	"github.com/Ktulue/KtulueKit-W11/internal/scheduler"
	"github.com/Ktulue/KtulueKit-W11/internal/state"
)
```

- [ ] **Step 2: Replace `promptReboot()` with the updated version**

Replace the entire function (lines 244–287) with:

```go
// promptReboot saves state, registers an auto-resume Scheduled Task, logs the
// resume command, then triggers a 30-second Windows reboot countdown.
// The user can press Enter to cancel the reboot and continue installing.
func (r *Runner) promptReboot(itemName string, currentPhase int) {
	nextPhase := currentPhase + 1
	resumeCmd := fmt.Sprintf("ktuluekit --config %q --resume-phase=%d", r.configPath, nextPhase)

	// Persist before doing anything else so state survives the reboot.
	if err := r.state.SaveResumePhase(nextPhase); err != nil {
		fmt.Printf("  [warning] Could not save resume phase to state: %v\n", err)
	}

	// Register the auto-resume Scheduled Task.
	binaryPath, _ := os.Executable()
	absConfig, _ := filepath.Abs(r.configPath)
	cwd, _ := os.Getwd()

	taskRegistered := false
	if err := scheduler.CreateResumeTask(binaryPath, absConfig, cwd, nextPhase, r.dryRun); err != nil {
		fmt.Printf("  [warning] Could not register auto-resume task: %v\n", err)
	} else {
		taskRegistered = true
	}

	// Build and print the reboot banner.
	sep := strings.Repeat("─", 56)
	var taskLine string
	if r.dryRun {
		taskLine = "  [dry-run] Auto-resume task would be registered.\n"
	} else if taskRegistered {
		taskLine = "  ✅ Auto-resume task registered — will run automatically after login.\n" +
			"  To cancel task: schtasks /delete /tn KtulueKit-Resume /f\n"
	} else {
		taskLine = "  ⚠️  Auto-resume task NOT registered. Run manually after reboot:\n" +
			"    " + resumeCmd + "\n"
	}

	banner := fmt.Sprintf(`
  🔄  %s requires a reboot.
  %s
%s  Log file: %s
  %s
  Rebooting in 30 seconds. Press Enter to CANCEL and continue without rebooting.
  (To cancel from another terminal: shutdown /a)
`, itemName, sep, taskLine, r.rep.LogPath(), sep)

	fmt.Print(banner)

	// Always write resume command to log — recoverable regardless of task status.
	r.rep.LogLine(fmt.Sprintf("\n[REBOOT REQUIRED — %s]", itemName))
	r.rep.LogLine("  Resume command: " + resumeCmd)
	r.rep.LogLine("")

	// Kick off the OS-level reboot countdown.
	shutdownMsg := fmt.Sprintf("KtulueKit: %s requires restart. After reboot run: %s", itemName, resumeCmd)
	_ = exec.Command("shutdown", "/r", "/t", "30", "/c", shutdownMsg).Run()

	// Block on stdin — if the user presses Enter we cancel the countdown.
	reader := bufio.NewReader(os.Stdin)
	reader.ReadString('\n')

	// Cancel the scheduled reboot and continue the run.
	_ = exec.Command("shutdown", "/a").Run()

	// Remove the resume task — the user chose to continue without rebooting,
	// so we don't want it firing at the next unrelated logon.
	_ = scheduler.DeleteResumeTask()

	fmt.Println("  Reboot cancelled. Continuing installation...")
}
```

- [ ] **Step 3: Build to confirm no compile errors**

```bash
cd F:/GDriveClone/Claude_Code/KtulueKit-W11
go build ./...
```

Expected: no output, exit 0.

- [ ] **Step 4: Run all tests**

```bash
cd F:/GDriveClone/Claude_Code/KtulueKit-W11
go test ./...
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/runner/runner.go
git commit -m "feat(runner): register/delete auto-resume Scheduled Task in promptReboot"
```

---

## Manual Verification Checklist

After implementation, verify end-to-end on a real Windows machine:

1. **Dry-run smoke test** — Run with `--dry-run` on a config that has a `reboot_after: true` package. Confirm banner shows `[dry-run] Auto-resume task would be registered.` No task appears in Task Scheduler.

2. **Task creation** — Run for real against a `reboot_after: true` package. Confirm:
   - Banner shows `✅ Auto-resume task registered`
   - Task Scheduler (`taskschd.msc`) shows `KtulueKit-Resume` under Task Scheduler Library
   - Action path, arguments, working directory, and Run Level (Highest) are correct

3. **Reboot and resume** — Let the reboot proceed. After logging back in:
   - Console window opens automatically
   - Correct phase runs (skips completed phases)
   - Task is gone from Task Scheduler after run completes

4. **Cancel path** — Trigger a reboot, then press Enter to cancel. Confirm:
   - "Reboot cancelled. Continuing installation..." prints
   - `KtulueKit-Resume` is absent from Task Scheduler

5. **Orphan cleanup** — Manually create a task named `KtulueKit-Resume` via Task Scheduler. Run ktuluekit normally. Confirm the task is deleted before the admin check fires.
