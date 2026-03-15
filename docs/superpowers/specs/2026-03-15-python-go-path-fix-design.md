# Python / Go PATH Fix Design

**Date:** 2026-03-15
**Status:** Approved
**Branch:** `fix/python-go-path`
**Scope:** Disable Windows App Execution Alias stub for Python; ensure Python and Go resolve correctly on the current machine and in fresh W11 installs

---

## Problem

Two overlapping PATH issues affect `python` and `go` resolution across PowerShell, bash, and Claude Code sessions:

1. **Windows App Execution Alias stub** — Microsoft ships a `python.exe` / `python3.exe` stub under `%LOCALAPPDATA%\Microsoft\WindowsApps\`. When `WindowsApps` appears before the real Python install on PATH (which is the Windows default), any tool that calls `python` gets the stub, which just opens the Microsoft Store. This causes failures in any context where the full USER PATH is not properly loaded.

2. **Go `bin` dir not on User PATH** — winget installs Go to `C:\Program Files\Go\` but does not always surface `C:\Program Files\Go\bin` on the User PATH in all contexts (bash, Claude Code). `go` commands then silently fail.

The W11 installer has no current handling for either issue.

---

## Goals

- Eliminate the Python stub conflict on the current machine immediately
- Ensure `python`, `python3`, and `go` resolve to the real installs in all contexts
- Add idempotent command entries to `ktuluekit.json` so fresh installs are fixed automatically

---

## Non-Goals

- Installing Python or Go (already covered by existing winget entries)
- Fixing PATH for other runtimes (node, rust, etc.) — out of scope for this fix
- Multi-user / system-scope PATH changes — User scope only

---

## Architecture

Two parallel deliverables:

**A. Immediate current-machine fix**
Run PowerShell commands in the current session to disable the stub and inject real paths now.

**B. `ktuluekit.json` command entries**
Two new `type: command` entries that run the same fix automatically on fresh installs, sequenced after their respective winget packages.

---

## Deliverable A: Current Machine Fix

Three operations, run in sequence via `pwsh`:

### 1. Disable App Execution Aliases

The alias stubs are governed by DWORD values under:
```
HKCU:\Software\Microsoft\Windows\CurrentVersion\App Execution Aliases
```
Values named `python.exe` and `python3.exe` — set to `0` to disable. Setting to `0` is reversible (vs. deleting the key). Does not affect the real Python install.

```powershell
Set-ItemProperty `
  -Path "HKCU:\Software\Microsoft\Windows\CurrentVersion\App Execution Aliases" `
  -Name "python.exe" -Value 0
Set-ItemProperty `
  -Path "HKCU:\Software\Microsoft\Windows\CurrentVersion\App Execution Aliases" `
  -Name "python3.exe" -Value 0
```

### 2. Inject Python dirs into User PATH

Prepend the real Python install dir and its `Scripts` subdir to the front of User PATH. Idempotent: guarded by a `notlike` check.

```powershell
$pyDir = "$env:LOCALAPPDATA\Programs\Python\Python312"
$pyScripts = "$pyDir\Scripts"
$cur = [Environment]::GetEnvironmentVariable('PATH', 'User')
if ($cur -notlike "*Python312*") {
    [Environment]::SetEnvironmentVariable('PATH', "$pyDir;$pyScripts;$cur", 'User')
}
```

### 3. Inject Go bin into User PATH

`C:\Program Files\Go\bin` — machine-scope winget install location. Same idempotent pattern.

```powershell
$goDir = "C:\Program Files\Go\bin"
$cur = [Environment]::GetEnvironmentVariable('PATH', 'User')
if ($cur -notlike "*Go\bin*") {
    [Environment]::SetEnvironmentVariable('PATH', "$goDir;$cur", 'User')
}
```

### 4. Verify

Open a fresh `pwsh` invocation (new process — picks up the updated User PATH) and confirm:
- `python --version` → `Python 3.12.x` (not stub / Store redirect)
- `go version` → `go version go1.x.x windows/amd64`

---

## Deliverable B: `ktuluekit.json` Command Entries

### Entry 1: `fix-python-alias`

```json
{
  "id": "fix-python-alias",
  "name": "Fix Python App Execution Alias + PATH",
  "phase": 2,
  "depends_on": ["Python.Python.3.12"],
  "check": "cmd /C python --version 2>&1 | findstr /C:\"Python 3.12\"",
  "command": "cmd /C pwsh -NoProfile -Command \"Set-ItemProperty -Path 'HKCU:\\Software\\Microsoft\\Windows\\CurrentVersion\\App Execution Aliases' -Name 'python.exe' -Value 0; Set-ItemProperty -Path 'HKCU:\\Software\\Microsoft\\Windows\\CurrentVersion\\App Execution Aliases' -Name 'python3.exe' -Value 0; $p = \\\"$env:LOCALAPPDATA\\Programs\\Python\\Python312\\\"; $s = \\\"$p\\Scripts\\\"; $cur = [Environment]::GetEnvironmentVariable('PATH','User'); if ($cur -notlike '*Python312*') { [Environment]::SetEnvironmentVariable('PATH',\\\"$p;$s;$cur\\\",'User') }\""
}
```

**Behavior:**
- `check` runs `python --version` and greps for `Python 3.12`. If it matches, the command is skipped (idempotent).
- If check fails (stub returns nothing, or version mismatch), the command runs: disables both alias keys, then injects Python dirs into User PATH.
- `depends_on: ["Python.Python.3.12"]` ensures real Python is installed before this runs.
- `phase: 2` — runs in the command phase, after all T1 winget packages.

### Entry 2: `fix-go-path`

```json
{
  "id": "fix-go-path",
  "name": "Fix Go PATH",
  "phase": 2,
  "depends_on": ["GoLang.Go"],
  "check": "cmd /C go version",
  "command": "cmd /C pwsh -NoProfile -Command \"$g = 'C:\\Program Files\\Go\\bin'; $cur = [Environment]::GetEnvironmentVariable('PATH','User'); if ($cur -notlike '*Go\\bin*') { [Environment]::SetEnvironmentVariable('PATH',\\\"$g;$cur\\\",'User') }\""
}
```

**Behavior:**
- `check` runs `go version`. If it exits 0, skip.
- If `go` is not on PATH, injects `C:\Program Files\Go\bin` at the front of User PATH.
- `depends_on: ["GoLang.Go"]` ensures winget has completed the Go install first.

---

## Idempotency

Both command entries and the current-machine fix are safe to re-run:
- Registry writes are `Set-ItemProperty` (overwrite, not create-exclusive)
- PATH injections are guarded by `notlike` substring checks
- `check` fields cause the runner to skip entirely if the environment is already correct

---

## Testing

No automated tests are written for these entries — they exercise Windows registry and PATH APIs that cannot be unit-tested in isolation without mocking the OS. Manual verification (Deliverable A step 4) is the acceptance test.

---

## Sequencing

```
Phase 1 (winget):
  Python.Python.3.12 ─────┐
  GoLang.Go ──────────────┤
                          │
Phase 2 (commands):       ▼
  fix-python-alias  (depends_on: Python.Python.3.12)
  fix-go-path       (depends_on: GoLang.Go)
  pip-pipx          (depends_on: Python.Python.3.12)  ← existing, unchanged
```

`fix-python-alias` runs before `pip-pipx` within phase 2 — ensuring `python` resolves correctly before pipx commands execute.

---

## Success Criteria

- [ ] `python --version` returns `Python 3.12.x` in PowerShell, bash, and Claude Code contexts
- [ ] `go version` returns successfully in all contexts
- [ ] `WindowsApps\python.exe` stub no longer intercepts `python` lookups
- [ ] `fix-python-alias` and `fix-go-path` appear in `ktuluekit.json` with correct `depends_on` and `check` fields
- [ ] Both command entries are idempotent (re-running does not duplicate PATH entries or error on already-disabled aliases)
