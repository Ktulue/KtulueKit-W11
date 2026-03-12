# PATH Verification & State File Relocation Design

**Date:** 2026-03-11
**Branch:** to be created
**Status:** Approved

## Overview

Two small, independent improvements:

1. **PATH verification** — after `RefreshPath()` fires, scan for key runtimes and warn on gaps
2. **State file relocation** — move state from CWD-relative `.ktuluekit-state.json` to `%LOCALAPPDATA%\KtulueKit\state.json` with lazy migration

---

## Feature 1: PATH Verification

### Trigger

Runs immediately after `installer.RefreshPath()` in `runner.go` (the existing `pathRefreshed` block). Skipped in dry-run mode — no PATH mutations happen during dry-run, so the check would be misleading.

### Runtimes Checked

Fixed list: `git`, `node`, `python`, `go`, `rustup`, `pwsh`

Detection via `exec.LookPath` — no subprocess spawning, pure PATH scan.

### Output

If all found:
```
  ✅  All runtime tools found on PATH.
```

If any missing:
```
  PATH check after refresh:
    ✅  git, node, go, pwsh
    ⚠️  python — not found on PATH (install may not have completed)
    ⚠️  rustup — not found on PATH (install may not have completed)
```

Yellow ANSI for the warning lines, green for the all-clear. Run continues regardless.

### New Code

`VerifyRuntimePaths() []string` in `internal/installer` — returns the list of missing tool names. Pure function; no side effects. Runner calls it and formats the output inline.

### Files Changed

- `internal/installer/path_check.go` — new file; `VerifyRuntimePaths() []string`
- `internal/runner/runner.go` — call `VerifyRuntimePaths()` after `RefreshPath()`, print results

---

## Feature 2: State File Relocation

### New Path

`%LOCALAPPDATA%\KtulueKit\state.json`

Resolved at runtime via `os.Getenv("LOCALAPPDATA")`. Directory created on first write if it doesn't exist (`os.MkdirAll`).

### Load Logic (Lazy Migration)

```
1. Try %LOCALAPPDATA%\KtulueKit\state.json  → found: load and use
2. Try .ktuluekit-state.json (CWD)          → found: load, save to new path, delete old file
3. Neither found                            → return fresh State{} (existing behavior)
```

Migration is silent — no user-facing output. Old file is deleted only after the new file is successfully written.

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
- `TestLoad_UsesNewPath` — new path file exists; loads from it; does not touch CWD
- `TestLoad_MigratesLegacy` — only CWD file exists; loads it, writes new path, deletes CWD file
- `TestLoad_NewPathTakesPrecedence` — both files exist; loads from new path; CWD file untouched
- `TestLoad_FreshState` — neither file exists; returns empty state
- `TestStatePath_CreatesDirectory` — `%LOCALAPPDATA%\KtulueKit` does not exist; `Save()` creates it
