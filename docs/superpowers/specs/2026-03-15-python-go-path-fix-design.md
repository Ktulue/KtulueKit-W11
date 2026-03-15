# Python / Go PATH Fix Design

**Date:** 2026-03-15
**Status:** Approved
**Branch:** `fix/python-go-path`
**Scope:** Disable Windows App Execution Alias stub for Python; ensure Python and Go resolve correctly on the current machine and in fresh W11 installs

---

## Problem

Two overlapping PATH issues affect `python` and `go` resolution across PowerShell, bash, and Claude Code sessions:

1. **Windows App Execution Alias stub** — Microsoft ships a `python.exe` / `python3.exe` stub under `%LOCALAPPDATA%\Microsoft\WindowsApps\`. When `WindowsApps` appears before the real Python install on PATH (the Windows default), any tool that calls `python` gets the stub, which opens the Microsoft Store instead of running Python. This causes failures in any context where PATH is not fully propagated.

2. **Go `bin` dir occasionally missing** — winget installs Go to `C:\Program Files\Go\` (machine scope) and adds `C:\Program Files\Go\bin` to the system PATH, but this propagates to all processes only after a logout/reboot. In bash and Claude Code sessions that were launched before the winget install completed, `go` commands silently fail.

The W11 installer has no current handling for either issue.

---

## Goals

- Eliminate the Python stub conflict on the current machine immediately
- Ensure `python` and `go` resolve to the real installs in all contexts
- Add idempotent command entries to `ktuluekit.json` so fresh installs are fixed automatically

---

## Non-Goals

- Installing Python or Go (already covered by existing winget entries)
- Fixing PATH for other runtimes (node, rust, etc.) — out of scope for this fix
- System-scope PATH changes — User scope only for PATH injection; alias disable is always HKCU

---

## Install Scope Notes

`ktuluekit.json` sets `default_scope: "machine"`. Python (`Python.Python.3.12`) has no scope override, so it installs machine-scope. Machine-scope Python winget installs to `C:\Program Files\Python312\` and winget adds `C:\Program Files\Python312\` + `\Scripts` to the **system** PATH automatically.

The current machine's Python is at `%LOCALAPPDATA%\Programs\Python\Python312\` — a legacy user-scope install predating the installer. Both paths are valid; the alias disable resolves the stub conflict in either case.

**The alias disable is the primary fix.** PATH injection in the installer entry is a safety net for edge cases where winget fails to update system PATH (e.g., mid-session, or a quirky install). It must use dynamic path detection, not a hardcoded path.

---

## Architecture

Two parallel deliverables:

**A. Immediate current-machine fix**
Run PowerShell commands in this session to disable the stub and verify resolution.

**B. `ktuluekit.json` command entries**
Two new command entries that apply the same fix automatically on fresh installs, inserted at the correct position in the `commands` array.

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

### 2. Verify Python resolution in a fresh process

Open a new `pwsh` invocation (picks up updated User PATH):

```powershell
pwsh -NoProfile -Command "python --version"
```

Expected output: `Python 3.12.x`. If still failing, the real Python dir is not on PATH at all — inject it manually:

```powershell
# Only needed if python still fails after alias disable:
$pyDir = (Get-Command python -ErrorAction SilentlyContinue)?.Source | Split-Path
# If Get-Command still finds the stub, locate the real install:
$pyDir = "$env:LOCALAPPDATA\Programs\Python\Python312"  # user-scope fallback
$cur = [Environment]::GetEnvironmentVariable('PATH', 'User')
if ($cur -notlike "*Python312*") {
    [Environment]::SetEnvironmentVariable('PATH', "$pyDir;$pyDir\Scripts;$cur", 'User')
}
```

### 3. Verify Go resolution

```powershell
pwsh -NoProfile -Command "go version"
```

Expected: `go version go1.x.x windows/amd64`. If failing, inject `C:\Program Files\Go\bin` into User PATH:

```powershell
$goDir = "C:\Program Files\Go\bin"
$cur = [Environment]::GetEnvironmentVariable('PATH', 'User')
if ($cur -notlike "*Go\bin*") {
    [Environment]::SetEnvironmentVariable('PATH', "$goDir;$cur", 'User')
}
```

---

## Deliverable B: `ktuluekit.json` Command Entries

### Phase

Commands in `ktuluekit.json` use phases 3, 4, and 5. All commands that depend on winget packages live in **phase 4**. Both new entries use `phase: 4`.

### Array insertion position

The runner processes commands in JSON array order within a phase. `fix-python-alias` must appear **before** `pip-pipx` in the array so that `python` resolves correctly before pipx commands run. Additionally, `pip-pipx`'s `depends_on` must be updated to include `fix-python-alias`.

### Entry 1: `fix-python-alias`

```json
{
  "id": "fix-python-alias",
  "name": "Fix Python App Execution Alias",
  "phase": 4,
  "depends_on": ["Python.Python.3.12"],
  "check": "cmd /C python --version 2>&1 | findstr /C:\"Python 3.12\"",
  "command": "cmd /C pwsh -NoProfile -Command \"Set-ItemProperty -Path 'HKCU:\\Software\\Microsoft\\Windows\\CurrentVersion\\App Execution Aliases' -Name 'python.exe' -Value 0; Set-ItemProperty -Path 'HKCU:\\Software\\Microsoft\\Windows\\CurrentVersion\\App Execution Aliases' -Name 'python3.exe' -Value 0\""
}
```

**Behavior:**
- `check` runs `python --version | findstr "Python 3.12"`. If it matches (exit 0), the command is skipped.
- The `check` skip-on-success is correct: if `python` already resolves to 3.12, the alias is either already disabled or the real Python is winning the PATH race regardless — in both cases, the alias is not causing problems and the command can safely be skipped.
- If check fails (stub, wrong version, or not found), the command disables both alias registry keys.
- The command intentionally does **not** inject a hardcoded Python path — the machine-scope winget install updates system PATH; if that propagation hasn't happened yet in the current session, the user should restart their shell after the install run.
- `depends_on: ["Python.Python.3.12"]` ensures real Python is installed before this runs.

### Entry 2: `fix-go-path`

```json
{
  "id": "fix-go-path",
  "name": "Fix Go PATH",
  "phase": 4,
  "depends_on": ["GoLang.Go"],
  "check": "cmd /C go version",
  "command": "cmd /C pwsh -NoProfile -Command \"$g = 'C:\\Program Files\\Go\\bin'; $cur = [Environment]::GetEnvironmentVariable('PATH','User'); if ($cur -notlike '*Go\\bin*') { [Environment]::SetEnvironmentVariable('PATH',\\\"$g;$cur\\\",'User') }\""
}
```

**Behavior:**
- `check` runs `go version`. If it exits 0, skip.
- If `go` is not on PATH, injects `C:\Program Files\Go\bin` at the front of User PATH.
- `C:\Program Files\Go\bin` is the winget machine-scope install location for `GoLang.Go`.
- `depends_on: ["GoLang.Go"]` ensures winget has completed the Go install first.

### `pip-pipx` update

Add `fix-python-alias` to `pip-pipx`'s `depends_on` to enforce ordering:

```json
"depends_on": ["Python.Python.3.12", "fix-python-alias"]
```

---

## Idempotency

Both command entries are safe to re-run:
- `check` fields skip the command if the environment is already correct
- Registry writes use `Set-ItemProperty` (overwrite, not create-exclusive — safe if value is already 0)
- PATH injection in `fix-go-path` is guarded by a `notlike` substring check

---

## Testing

No automated tests for these entries — they exercise Windows registry and PATH APIs that cannot be unit-tested without OS mocking. Manual verification (Deliverable A steps 2–3) is the acceptance test.

---

## Sequencing

```
Phase 1 (winget):
  Python.Python.3.12 ─────┐
  GoLang.Go ──────────────┤
  (all other wingets)      │
                           │
Phase 4 (commands):        ▼
  [existing phase-4 entries above insertion point]
  fix-python-alias  (depends_on: Python.Python.3.12)  ← insert here
  fix-go-path       (depends_on: GoLang.Go)            ← insert here
  pip-pipx          (depends_on: Python.Python.3.12, fix-python-alias)  ← updated
  pip-black         (unchanged)
  pip-ruff          (unchanged)
  ...
```

`fix-python-alias` appears before `pip-pipx` in the JSON array AND is listed in `pip-pipx`'s `depends_on` — belt-and-suspenders ordering guarantee.

---

## Success Criteria

- [ ] `python --version` returns `Python 3.12.x` in PowerShell, bash, and Claude Code contexts on current machine
- [ ] `go version` returns successfully in all contexts on current machine
- [ ] `WindowsApps\python.exe` stub no longer intercepts `python` lookups
- [ ] `fix-python-alias` (phase 4) and `fix-go-path` (phase 4) appear in `ktuluekit.json`
- [ ] `fix-python-alias` is inserted before `pip-pipx` in the JSON array
- [ ] `pip-pipx` has `depends_on: ["Python.Python.3.12", "fix-python-alias"]`
- [ ] Both command entries are idempotent (re-running does not duplicate PATH entries or error on already-disabled aliases)
