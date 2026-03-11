# Auto-Resume via Scheduled Task — Design Spec

**Date:** 2026-03-10
**Feature:** Zero-friction reboot resume using Windows Scheduled Task
**Status:** Approved

---

## Problem

When KtulueKit triggers a reboot (`reboot_after: true`), the user must manually re-run the tool with the correct `--resume-phase=N` flag after logging back in. This is friction — the user has to remember or find the resume command from the log file.

## Goal

After triggering a reboot, automatically register a one-shot Windows Scheduled Task that re-launches KtulueKit at next logon with the correct resume flags. The task self-deletes at the start of the resumed run. Zero user action required after the initial launch.

---

## Decisions

| Question | Decision |
|---|---|
| Elevation at resume | HIGHEST run level — auto-elevates, no UAC prompt |
| Window behavior | Console window opens (same as manual run) |
| Binary/config paths | Captured at task-creation time via `os.Executable()` + `filepath.Abs()` |
| Multiple reboots | Task recreated each time `promptReboot()` is called (`-Force` overwrites) |
| Self-deletion timing | At the very start of `main.go`, before admin check or anything else |
| Task creation failure | Warn user, display manual resume command, still reboot (graceful degradation) |
| User notification | Display in reboot banner + cancellation command |
| Reboot cancel path | `DeleteResumeTask()` called immediately after `shutdown /a` |

---

## Architecture

### New Package: `internal/scheduler/`

```
internal/scheduler/
  scheduler_windows.go   // CreateResumeTask(), DeleteResumeTask()
  scheduler_stub.go      // no-op stubs for non-Windows builds
```

**`CreateResumeTask(binaryPath, configPath, workDir string, resumePhase int, dryRun bool) error`**
Builds and executes a PowerShell script that registers a Scheduled Task named `KtulueKit-Resume`. On dry-run, prints intent and returns nil without executing.

**`DeleteResumeTask() error`**
Runs `schtasks /delete /tn KtulueKit-Resume /f`. Error is always ignored by callers — non-existence is expected on first run.

---

## PowerShell Task Definition

```powershell
$action = New-ScheduledTaskAction `
    -Execute '"<binaryPath>"' `
    -Argument '--config "<configPath>" --resume-phase=<N>' `
    -WorkingDirectory '<workDir>'

$trigger = New-ScheduledTaskTrigger -AtLogOn

$settings = New-ScheduledTaskSettingsSet `
    -ExecutionTimeLimit (New-TimeSpan -Hours 4) `
    -MultipleInstances IgnoreNew

$principal = New-ScheduledTaskPrincipal `
    -UserId $env:USERNAME `
    -RunLevel Highest `
    -LogonType Interactive

Register-ScheduledTask `
    -TaskName 'KtulueKit-Resume' `
    -Action $action `
    -Trigger $trigger `
    -Settings $settings `
    -Principal $principal `
    -Force
```

Key properties:
- `-Force` — overwrites existing task (handles multi-reboot sequences)
- `-LogonType Interactive` — fires only on desktop logon, not service logon
- `-ExecutionTimeLimit 4 hours` — prevents task kill mid-install
- `-MultipleInstances IgnoreNew` — drops duplicate triggers
- All paths embedded with absolute values and proper quoting

---

## Integration Points

### `cmd/main.go` — startup (line 1 of run function)

```go
_ = scheduler.DeleteResumeTask() // no-op if task doesn't exist
```

Called before admin check, config load, or anything else. Guarantees no orphaned task survives a crash.

### `internal/runner/runner.go` — `promptReboot()`

After `SaveResumePhase()`, before displaying the reboot banner:

```go
binaryPath, _ := os.Executable()
absConfig, _ := filepath.Abs(r.configPath)
cwd, _ := os.Getwd()

taskErr := scheduler.CreateResumeTask(binaryPath, absConfig, cwd, nextPhase, r.dryRun)
if taskErr != nil {
    fmt.Printf("  [warning] Could not register auto-resume task: %v\n", taskErr)
    fmt.Printf("  Auto-resume will NOT happen. Run manually after reboot:\n")
    fmt.Printf("  %s\n", resumeCmd)
} else {
    fmt.Println("  Auto-resume task registered. Will run automatically after login.")
    fmt.Printf("  To cancel: schtasks /delete /tn KtulueKit-Resume /f\n")
}
```

Manual resume command still written to log file regardless of task creation success.

After `shutdown /a` (reboot cancelled by user):

```go
_ = scheduler.DeleteResumeTask()
```

---

## Error Handling

| Scenario | Behavior |
|---|---|
| `schtasks /delete` fails at startup (task doesn't exist) | Error ignored — expected |
| PowerShell unavailable | `CreateResumeTask()` returns error → manual resume fallback |
| Group Policy blocks task creation | Same — error returned, warning shown |
| Binary/config paths contain spaces | Individually quoted before PowerShell embedding |
| Config path is relative | `filepath.Abs()` resolves at task-creation time |
| User cancels reboot | `DeleteResumeTask()` called immediately after `shutdown /a` |
| Machine reboots mid-install unexpectedly | Task fires at next logon, resumes correctly |
| Crash before startup deletion | Next run deletes at line 1 — no double-run |
| Dry-run mode | `CreateResumeTask()` prints intent, returns nil |
| Non-Windows build | Stub returns nil silently |

---

## Testing

### Unit Tests
- Path quoting logic — paths with spaces produce correctly quoted PowerShell
- Dry-run short-circuit — `CreateResumeTask()` returns nil without executing

### Manual Verification Checklist
1. Dry-run with a `reboot_after: true` package — confirm task registration message appears, no task created in Task Scheduler
2. Real run with reboot item — confirm `KtulueKit-Resume` appears in Task Scheduler with correct action, working directory (`C:\...`), and run level (Highest)
3. Reboot — confirm console window opens automatically at next logon
4. Confirm task absent from Task Scheduler after resumed run completes
5. Cancel reboot (press Enter) — confirm task deleted and absent from Task Scheduler
