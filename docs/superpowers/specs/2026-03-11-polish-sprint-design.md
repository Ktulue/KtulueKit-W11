# Polish Sprint — Design Spec

**Date:** 2026-03-11
**Branch:** `maint/polish-sprint`
**Goal:** Implement 5 small UX improvements from TODO.md in a single feature branch.

---

## Scope

Five changes + TODO.md housekeeping:

1. Winget pre-flight check
2. Winget source update at startup
3. Progress counter `[N/Total]`
4. Elapsed time per item + total run elapsed
5. Completion notification (system beep)

Also: mark 2 already-shipped TODO items as `[x]` (state-aware skip, ANSI color).

---

## Files Changed

- `internal/installer/winget.go` — 2 new exported functions (~25 lines)
- `internal/runner/runner.go` — ~50 lines across 5 touch points
- `TODO.md` — mark 7 items `[x]`

---

## 1. Winget Pre-Flight Check

**File:** `internal/installer/winget.go`

New exported function:

```go
func CheckWingetAvailable() error
```

Runs `winget --version` with a 5-second timeout. Returns an error if winget is missing, not on PATH, or times out.

**Called from:** `runner.Run()` as the very first action — before restore point, before pre-run scan.

**Behavior:**
- If error: print clear message and return from `Run()` immediately.
- Skipped in dry-run (winget not required to simulate).

---

## 2. Winget Source Update

**File:** `internal/installer/winget.go`

New exported function:

```go
func UpdateSources() error
```

Runs `winget source update` with stdout/stderr streamed to console. Returns error if the command fails.

**Called from:** `runner.Run()` after the pre-run summary confirms there is actual work to do, before the phase loop. Not called if `printPreRunSummary()` returns `nothingToDo = true`. Skipped in dry-run.

**Print before call:** `Updating winget sources...`

---

## 3. Progress Counter

**File:** `internal/runner/runner.go`

Add two fields to `Runner`:

```go
totalItems int
itemIdx    int
```

At the start of `Run()`, compute `totalItems` by counting all packages + commands + extensions in phases `>= r.resumePhase`. A resume from phase 3 shows `[1/12]` not `[1/42]`.

Each `run*InPhase` function increments `r.itemIdx` and prefixes the item line:

```
  [3/42] Installing: Go
```

Applies to all three tiers (packages, commands, extensions). Items that are state-skipped still count and show `[N/Total] Skipping (already succeeded): ...`.

---

## 4. Elapsed Time

**File:** `internal/runner/runner.go`

### Per-item

In each `run*InPhase`:
```go
start := time.Now()
res := installer.InstallPackage(...)
r.rep.Add(res)
fmt.Printf("      elapsed: %s\n", time.Since(start).Round(time.Second))
```

Printed on its own line after the reporter output. Applies to packages, commands, and extensions.

### Total run

At start of `Run()`:
```go
runStart := time.Now()
```

After `r.rep.Summary()` at end of `Run()`:
```go
fmt.Printf("Total elapsed: %s\n", time.Since(runStart).Round(time.Second))
```

---

## 5. Completion Notification

**File:** `internal/runner/runner.go`

At the very end of `Run()`, after total elapsed:

```go
exec.Command("powershell", "-Command", "[console]::beep(800,300)").Run()
```

Skipped in dry-run. Error is ignored (non-critical UX feature).

---

## Run() Flow After Changes

```
Run():
  1. if !dryRun: CheckWingetAvailable() — fail fast if missing
  2. runStart := time.Now()
  3. totalItems = countItemsFromPhase(resumePhase)
  4. if resumePhase <= 1: CreateRestorePoint()
  5. if printPreRunSummary() returns nothingToDo: return
  6. if !dryRun: UpdateSources()
  7. phase loop (with [N/Total] counters and per-item timing)
  8. rep.Summary()
  9. print Total elapsed
  10. if !dryRun: beep
```

---

## Output Shape

```
  [3/42] Installing: Go
    │ winget output...
  ✅  Go                                           [winget]
      elapsed: 1m 23s

  [4/42] Installing: Node.js LTS
    ...
```

```
============================================================
SUMMARY
============================================================

✅ Installed successfully (12)
    • Go
    • Node.js LTS
    ...

Total elapsed: 14m 37s
```

---

## Constraints

- No emojis in new code
- Dry-run skips: pre-flight check, source update, beep
- Error from beep is silently ignored
- `time.Since(...).Round(time.Second)` for human-readable output
