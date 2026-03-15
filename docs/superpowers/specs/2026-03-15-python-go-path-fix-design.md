# Python / Go PATH Fix Design

**Date:** 2026-03-15
**Status:** Approved
**Branch:** `fix/python-go-path`
**Scope:** Disable Windows App Execution Alias stub for Python; ensure Python and Go resolve correctly on the current machine and in fresh W11 installs

---

## Problem

Two overlapping PATH issues affect `python` and `go` resolution across PowerShell, bash, and Claude Code sessions:

1. **Windows App Execution Alias stub** — Microsoft ships a `python.exe` / `python3.exe` stub under `%LOCALAPPDATA%\Microsoft\WindowsApps\`. When `WindowsApps` appears before the real Python install on PATH (the Windows default), any tool that calls `python` gets the stub, which opens the Microsoft Store instead of running Python. The stub exits non-zero when invoked non-interactively (e.g., via `cmd /C python --version`), so it does not print a version string.

2. **Go `bin` dir not yet propagated to user shells** — winget installs Go to `C:\Program Files\Go\` (machine scope) and adds `C:\Program Files\Go\bin` to the system PATH registry entry. This is visible to processes launched after the install, but bash and Claude Code sessions opened before the winget run completes do not inherit it.

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

## Install Scope and RefreshPath Notes

`ktuluekit.json` sets `default_scope: "machine"`. Python (`Python.Python.3.12`) has no scope override, so it installs machine-scope. Machine-scope Python winget installs to `C:\Program Files\Python312\` and winget adds that dir and `\Scripts` to the **system** PATH registry entry automatically.

The runner calls `installer.RefreshPath()` before phase 4. This reads both Machine and User PATH from the registry and injects them into the runner's in-process environment. All child processes the runner spawns (via `cmd /C`) inherit that environment. This means:

- After phase 1 winget installs complete and `RefreshPath()` fires, `python` and `go` are already resolvable in child processes spawned by the runner for phase 4 commands.
- The alias disable in `fix-python-alias` is still needed: `RefreshPath()` makes the real Python findable, but if `WindowsApps` appears first in the refreshed PATH, the stub still wins.
- The User PATH injection in `fix-go-path` targets user shell sessions opened after the installer run — not the installer's own execution. During the installer run, `RefreshPath()` already surfaces `C:\Program Files\Go\bin` via the system PATH.

**The alias disable is the primary fix for Python.** The Go PATH injection is a convenience for post-run shells.

The current machine's Python is at `%LOCALAPPDATA%\Programs\Python\Python312\` — a legacy user-scope install predating the installer. The alias disable resolves the stub conflict regardless of whether the install is user-scope or machine-scope.

---

## Architecture

Two parallel deliverables:

**A. Immediate current-machine fix**
Run PowerShell commands in this session to disable the stub and verify resolution.

**B. `ktuluekit.json` command entries**
Two new command entries that apply the same fix automatically on fresh installs, inserted at the correct position in the `commands` array, plus a profiles update.

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

Expected output: `Python 3.12.x`. If still failing, the real Python dir is not on PATH at all — inject it manually (current machine is user-scope, so use the user-scope path):

```powershell
$pyDir = "$env:LOCALAPPDATA\Programs\Python\Python312"
$cur = [Environment]::GetEnvironmentVariable('PATH', 'User')
if ($cur -notlike "*Python312*") {
    [Environment]::SetEnvironmentVariable('PATH', "$pyDir;$pyDir\Scripts;$cur", 'User')
}
```

Then open a new shell and re-verify.

### 3. Verify Go resolution

```powershell
pwsh -NoProfile -Command "go version"
```

Expected: `go version go1.x.x windows/amd64`. If failing (bash/Claude Code session predates the install), inject `C:\Program Files\Go\bin` into User PATH so new shells pick it up:

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

### `depends_on` cross-tier note

`depends_on` entries may reference IDs from either the `packages` or `commands` arrays — the runner resolves both against the same succeeded state map. Using a package ID like `"Python.Python.3.12"` in a command's `depends_on` is supported and correct.

### Array insertion position

The runner processes commands in JSON array order within a phase. `fix-python-alias` must appear **before** `pip-pipx` in the array. Additionally, `pip-pipx`'s `depends_on` must be updated to include `fix-python-alias`.

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
- `check` runs `cmd /C python --version | findstr "Python 3.12"`. The Microsoft Store stub exits non-zero when invoked non-interactively, so if the stub is active, the check fails and the command runs. If `python --version` exits 0 and matches `Python 3.12`, the stub is either already disabled or not intercepting the lookup in this context — either way, the fix is not needed and the command is skipped.
- The command disables both `python.exe` and `python3.exe` alias keys.
- The command does not inject a Python PATH entry. After `RefreshPath()`, the machine-scope winget install is already on the child process PATH. The alias disable is sufficient.
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
- `check` runs `go version`. If `go` resolves from any location (exit 0), the command is skipped. The goal is "go is resolvable," not "exactly this path is on User PATH."
- During the installer run, `RefreshPath()` will have already surfaced `C:\Program Files\Go\bin` via the machine system PATH, so the check will typically pass and this command will be skipped. The command exists as a convenience for post-run user shell sessions where the system PATH registry update hasn't propagated yet.
- The path `C:\Program Files\Go\bin` is hardcoded. This is acceptable for Go: winget machine-scope `GoLang.Go` always installs to this location, there is no user-scope variant, and the path is not machine-specific.
- `depends_on: ["GoLang.Go"]` ensures winget has completed the Go install first.

### `pip-pipx` update

Add `fix-python-alias` to `pip-pipx`'s `depends_on`:

```json
"depends_on": ["Python.Python.3.12", "fix-python-alias"]
```

### Profiles update

`ktuluekit.json` contains named profiles (Full Setup, Dev Only, etc.). The runner only runs commands whose IDs appear in the active profile's `ids` list. Any profile that includes `pip-pipx` must also include `fix-python-alias` — otherwise `fix-python-alias` never succeeds, and `pip-pipx`'s `depends_on` check fails, causing `pip-pipx` to be skipped.

Add `fix-python-alias` and `fix-go-path` to every profile that currently includes `pip-pipx`, `pip-black`, or `pip-ruff`. Based on the existing config this means:

- **Full Setup** — add both `fix-python-alias` and `fix-go-path`
- **Dev Only** — add both `fix-python-alias` and `fix-go-path`

If a profile does not include any Python or Go commands, no change is needed.

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
  [RefreshPath() fires]    ▼

Phase 4 (commands):
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
- [ ] `fix-python-alias` and `fix-go-path` added to Full Setup and Dev Only profiles
- [ ] Both command entries are idempotent (re-running does not duplicate PATH entries or error on already-disabled aliases)
