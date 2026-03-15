# Python / Go PATH Fix Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Disable the Windows App Execution Alias stub for Python and ensure `python`/`go` resolve correctly on the current machine and in fresh W11 installs.

**Architecture:** Two deliverables — run PowerShell commands in the current session to fix the current machine immediately, then edit `ktuluekit.json` to add two new command entries (`fix-python-alias`, `fix-go-path`) that apply the same fix on fresh installs. No new Go files. No automated tests (these entries exercise Windows registry and PATH APIs that cannot be unit-tested without OS mocking).

**Tech Stack:** PowerShell (`pwsh`), Windows registry (`HKCU:\Software\Microsoft\Windows\CurrentVersion\App Execution Aliases`), JSON config editing.

**Spec:** `docs/superpowers/specs/2026-03-15-python-go-path-fix-design.md`

**Branch:** `fix/python-go-path`

---

## Chunk 1: Current Machine Fix + Config Edits

### Task 1: Fix the current machine

No files to modify — run PowerShell commands directly.

- [ ] **Step 0: Ensure you are on the correct branch**

```bash
git checkout fix/python-go-path
```

If the branch does not exist yet:

```bash
git checkout -b fix/python-go-path
```

- [ ] **Step 1: Disable the Python App Execution Alias registry keys**

Run in a PowerShell or bash terminal:

```powershell
pwsh -NoProfile -Command "Set-ItemProperty -Path 'HKCU:\Software\Microsoft\Windows\CurrentVersion\App Execution Aliases' -Name 'python.exe' -Value 0; Set-ItemProperty -Path 'HKCU:\Software\Microsoft\Windows\CurrentVersion\App Execution Aliases' -Name 'python3.exe' -Value 0"
```

Expected: no output, no error. If the key doesn't exist (clean machine with no Store stub), `Set-ItemProperty` will error — that is fine; the alias is already absent.

- [ ] **Step 2: Verify Python resolves in a fresh process**

```powershell
pwsh -NoProfile -Command "python --version"
```

Expected: `Python 3.12.x`

If it still fails (real Python dir is not on PATH at all), inject the user-scope path manually:

```powershell
pwsh -NoProfile -Command "
  $pyDir = '$env:LOCALAPPDATA\Programs\Python\Python312';
  $cur = [Environment]::GetEnvironmentVariable('PATH','User');
  if ($cur -notlike '*Python312*') {
    [Environment]::SetEnvironmentVariable('PATH', ($pyDir + ';' + $pyDir + '\Scripts;' + $cur), 'User')
  }
"
```

Then open a **new terminal** and re-run `python --version` to pick up the updated PATH.

- [ ] **Step 3: Verify Go resolves in a fresh process**

```powershell
pwsh -NoProfile -Command "go version"
```

Expected: `go version go1.x.x windows/amd64`

If it fails, inject the Go bin dir into User PATH:

```powershell
pwsh -NoProfile -Command "
  $g = 'C:\Program Files\Go\bin';
  $cur = [Environment]::GetEnvironmentVariable('PATH','User');
  if ($cur -notlike '*Go\bin*') {
    [Environment]::SetEnvironmentVariable('PATH', ($g + ';' + $cur), 'User')
  }
"
```

Open a **new terminal** and re-run `go version`.

---

### Task 2: Add `fix-python-alias` and `fix-go-path` to `ktuluekit.json`

**Files:**
- Modify: `ktuluekit.json`

The runner processes commands in JSON array order within a phase. The two new entries must be inserted **before** the `pip-pipx` entry.

- [ ] **Step 1: Insert the two new command entries before `pip-pipx`**

In `ktuluekit.json`, find the `pip-pipx` entry — search for `"id": "pip-pipx"`. Insert the following two JSON objects immediately before that entry. Both new objects need trailing commas (the existing `pip-pipx` object already has correct comma placement after it):

```json
    {
      "id": "fix-python-alias",
      "name": "Fix Python App Execution Alias",
      "phase": 4,
      "category": "Dev Tools",
      "description": "Disables the Windows App Execution Alias stub for python.exe and python3.exe so the real Python install is found instead of the Microsoft Store redirect.",
      "check": "python --version 2>&1 | findstr /C:\"Python 3.12\"",
      "command": "pwsh -NoProfile -Command \"Set-ItemProperty -Path 'HKCU:\\Software\\Microsoft\\Windows\\CurrentVersion\\App Execution Aliases' -Name 'python.exe' -Value 0; Set-ItemProperty -Path 'HKCU:\\Software\\Microsoft\\Windows\\CurrentVersion\\App Execution Aliases' -Name 'python3.exe' -Value 0\"",
      "depends_on": ["Python.Python.3.12"]
    },
    {
      "id": "fix-go-path",
      "name": "Fix Go PATH",
      "phase": 4,
      "category": "Dev Tools",
      "description": "Injects C:\\Program Files\\Go\\bin into the User PATH registry entry so go commands are available in shells opened after the installer exits. Note: this command will typically be skipped during the install run itself because RefreshPath() already surfaces the system PATH entry — it targets post-run user shells.",
      "check": "go version",
      "command": "pwsh -NoProfile -Command \"$g = 'C:\\Program Files\\Go\\bin'; $cur = [Environment]::GetEnvironmentVariable('PATH','User'); if ($cur -notlike '*Go\\bin*') { [Environment]::SetEnvironmentVariable('PATH', ($g + ';' + $cur), 'User') }\"",
      "depends_on": ["GoLang.Go"]
    },
```

After insertion the order in the `commands` array must be:

```
... (existing phase-4 entries above) ...
fix-python-alias   ← new
fix-go-path        ← new
pip-pipx           ← existing
pip-black          ← existing
pip-ruff           ← existing
...
```

- [ ] **Step 2: Update `pip-pipx`'s `depends_on`**

Find the `pip-pipx` entry (just below the insertion point). Change its `depends_on` from:

```json
      "depends_on": ["Python.Python.3.12"],
```

to:

```json
      "depends_on": ["Python.Python.3.12", "fix-python-alias"],
```

- [ ] **Step 3: Update the Full Setup profile**

Find the Full Setup profile `ids` array — search for `"name": "Full Setup"`. Locate the line containing:

```
        "npm-typescript", "npm-prettier", "pip-pipx", "pip-black", "pip-ruff",
```

Change it to:

```
        "npm-typescript", "npm-prettier", "fix-python-alias", "fix-go-path", "pip-pipx", "pip-black", "pip-ruff",
```

- [ ] **Step 4: Update the Dev Only profile**

Find the Dev Only profile `ids` array — search for `"name": "Dev Only"`. Locate the line containing:

```
        "npm-typescript", "npm-prettier", "pip-pipx", "pip-black", "pip-ruff",
```

Change it to:

```
        "npm-typescript", "npm-prettier", "fix-python-alias", "fix-go-path", "pip-pipx", "pip-black", "pip-ruff",
```

- [ ] **Step 5: Validate the JSON is well-formed**

Use `pwsh` for validation — do NOT use `python` or `python3` directly as the Store stub may still intercept it in the current shell session:

```bash
pwsh -NoProfile -Command "Get-Content 'ktuluekit.json' -Raw | ConvertFrom-Json; Write-Host 'JSON valid'"
```

Expected: `JSON valid`

If it throws, the error message will point to the approximate location. Fix the syntax error and re-run until it passes.

- [ ] **Step 6: Run the Go test suite to confirm no regressions**

```bash
go test ./...
```

Expected: all tests pass (`ok` or `PASS` on each package, no `FAIL` lines). Note: `go test` does not load `ktuluekit.json`, so it catches Go-level regressions only — the JSON validation in Step 5 covers config correctness.

- [ ] **Step 7: Commit**

```bash
git add ktuluekit.json
git commit -m "fix: disable Python App Execution Alias stub + ensure Go PATH in installer"
```

- [ ] **Step 8: Push and open PR**

```bash
git push -u origin fix/python-go-path
gh pr create \
  --title "fix: disable Python App Execution Alias stub + ensure Go PATH in installer" \
  --body "$(cat <<'EOF'
## Summary
- Adds `fix-python-alias` command entry (phase 4) that disables the Windows App Execution Alias stubs for `python.exe` and `python3.exe`, preventing the Microsoft Store from intercepting `python` lookups
- Adds `fix-go-path` command entry (phase 4) that injects `C:\Program Files\Go\bin` into the User PATH registry entry for post-run shell sessions
- Updates `pip-pipx` `depends_on` to include `fix-python-alias` (belt-and-suspenders ordering)
- Adds both new IDs to Full Setup and Dev Only profiles

## Test plan
- [ ] JSON validates via `pwsh ConvertFrom-Json`
- [ ] `go test ./...` passes clean
- [ ] `python --version` returns `Python 3.12.x` in a fresh `pwsh` process after alias disable
- [ ] `go version` returns successfully in a fresh `pwsh` process

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

Expected: PR URL printed. Stop here — do not merge. Wait for user approval before merging.
