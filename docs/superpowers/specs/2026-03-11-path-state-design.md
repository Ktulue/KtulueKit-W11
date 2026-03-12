# PATH Verification & State File Relocation Design

**Date:** 2026-03-11
**Branch:** to be created
**Status:** Draft

## Overview

Two small, independent improvements:

1. **PATH verification** — after `RefreshPath()` fires, scan for key runtimes and warn on gaps
2. **State file relocation** — move state from CWD-relative `.ktuluekit-state.json` to `%LOCALAPPDATA%\KtulueKit\state.json` with lazy migration

---

## Feature 1: PATH Verification

### Trigger

Runs immediately after `installer.RefreshPath()` in `runner.go` (the existing `pathRefreshed` block). Skipped in dry-run mode — no PATH mutations happen during dry-run, so the check would be misleading.

The insertion point in `runner.go` is inside `if !pathRefreshed && phase >= r.firstCommandPhase()`, wrapped in `if !r.dryRun`:

```go
if !pathRefreshed && phase >= r.firstCommandPhase() {
    installer.RefreshPath()
    pathRefreshed = true
    if !r.dryRun {
        missing := installer.VerifyRuntimePaths()
        // format and print missing list using colorYellow/colorGreen/colorReset
    }
}
```

### Runtimes Checked

Fixed list: `git`, `node`, `python`, `go`, `rustup`, `pwsh`

These are the runtimes most likely to require a PATH refresh after winget install and to be depended on by Tier 2 commands. Tools like `npm`, `cargo`, and `wsl` are installed as side-effects of their parent runtime and are not independently checked. The list is not user-configurable.

Detection via `exec.LookPath` — no subprocess spawning, pure PATH scan.

### Output

Uses existing ANSI constants from `runner.go` (`colorGreen`, `colorYellow`, `colorReset`). No new constants needed.

If all found:
```
  [OK]  All runtime tools found on PATH.
```

If any missing:
```
  PATH check after refresh:
    [OK]    git, node, go, pwsh
    [WARN]  python — not found on PATH (install may not have completed)
    [WARN]  rustup — not found on PATH (install may not have completed)
```

`[OK]` rendered in `colorGreen`, `[WARN]` in `colorYellow`. Run continues regardless.

### New Code

`VerifyRuntimePaths() []string` in `internal/installer` — returns the list of missing tool names. Pure function; no side effects. Runner calls it and formats the output inline using the existing ANSI constants.

### Files Changed

- `internal/installer/path_check.go` — new file; `VerifyRuntimePaths() []string`
- `internal/runner/runner.go` — call `VerifyRuntimePaths()` after `RefreshPath()` inside `if !r.dryRun` guard, print results using `colorGreen`/`colorYellow`/`colorReset`

---

## Feature 2: State File Relocation

### New Path

`%LOCALAPPDATA%\KtulueKit\state.json`

Resolved at runtime via `os.Getenv("LOCALAPPDATA")`. If `LOCALAPPDATA` is empty (CI environments, service accounts, SYSTEM context), fall back to CWD-relative `.ktuluekit-state.json` — same behavior as before the relocation. Directory created on first write if it doesn't exist (`os.MkdirAll`).

### Load Logic (Lazy Migration)

```
1. Resolve newPath = %LOCALAPPDATA%\KtulueKit\state.json
   (if LOCALAPPDATA is empty, skip steps 1-2 and go straight to CWD fallback)
2. Try newPath  → found: load and use
3. Try .ktuluekit-state.json (CWD)  → found: load, save to newPath, delete old file
4. Neither found  → return fresh State{} (existing behavior)
```

Migration is silent — no user-facing output. Old file is deleted only after the new file is successfully written. If the process is killed between the write and the delete, the orphaned CWD file will be harmlessly ignored on subsequent runs because step 2 (new path) takes precedence.

### All Writes

All writes (`Save()`, `MarkSucceeded`, `MarkFailed`, `SaveResumePhase`) go to the new path. `Clear()` deletes the new path file.

### Write / Clear

All writes (`Save()`, `MarkSucceeded`, `MarkFailed`, `SaveResumePhase`) go to the new path. `Clear()` deletes the new path file.

### Call Sites

`state.Load()` and `state.Clear()` signatures are unchanged. The three call sites (`cmd/main.go`, `app.go`, `cmd/status.go`) require no modification.

### Files Changed

- `internal/state/state.go` — update `stateFile` constant to a resolved function; add migration logic in `Load()`

---

## Testing

### PATH Verification
- `TestVerifyRuntimePaths_AllPresent` — mock PATH contains all tools; returns empty slice
- `TestVerifyRuntimePaths_SomeMissing` — mock PATH missing `python`, `rustup`; returns `["python", "rustup"]`

### State File Relocation

All state tests use `t.Setenv("LOCALAPPDATA", t.TempDir())` to isolate file I/O from the developer's real user profile and to allow CI execution without side effects.

- `TestLoad_UsesNewPath` — new path file exists; loads from it; does not touch CWD
- `TestLoad_MigratesLegacy` — only CWD file exists; loads it, writes new path, deletes CWD file; asserts CWD file no longer exists after load
- `TestLoad_NewPathTakesPrecedence` — both files exist; loads from new path; CWD file untouched
- `TestLoad_FreshState` — neither file exists; returns empty state
- `TestLoad_EmptyLOCALAPPDATA` — `LOCALAPPDATA` env var is empty; falls back to CWD behavior
- `TestStatePath_CreatesDirectory` — `%LOCALAPPDATA%\KtulueKit` does not exist; `Save()` creates it via `os.MkdirAll`
