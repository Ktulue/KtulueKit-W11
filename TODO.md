# KtulueKit-W11 — TODO

Organized from least extensive to most extensive changes. Each section groups items by scope of impact.

> **Note:** Completed items (`[x]`) can always be revisited and verified at a later date.

---

## Toolset Overview

| Tool | Scope | Purpose |
|---|---|---|
| **KtulueKit-W11** | Installation | Installs software, runtimes, binaries, VS Code extensions, and applies machine-level configuration (registry tweaks, shell profile, font config) to a fresh Windows 11 machine. Declarative, idempotent, resumable. Does NOT handle personal state or identity. |
| **KtulueKit-Migration** *(planned)* | Migration | Migrates personal state from an old machine to a new one. Handles: git identity (user.name/email), browser extensions (Brave/Chrome/Firefox — uBlock Origin, Dark Reader, KeePassXC-Browser, React DevTools, etc.), app configs, database backups (.kdbx), backup/restore/verification workflows. |

**Boundary rule:** If it puts software on the machine, it belongs in KtulueKit-W11. If it restores personal state, preferences, or identity from a previous machine, it belongs in KtulueKit-Migration.

---

## Config-Only Changes (no code changes)

- [x] **Add missing "already installed" entries** — Stream Deck, Streamer.bot, and DaVinci Resolve added as phase 3 Tier 2 commands with `"check": "echo skip"` and `"command": "echo Already installed — no automated installer"`.

- [x] **Add VS Code extension installs** — Added Go, Python, ESLint, Prettier, GitLens, C# extensions as phase 4 commands with `code --list-extensions | findstr` checks.

- [x] **Add npm global packages** — Added TypeScript and Prettier as phase 4 commands (depends on Node.js).

- [x] **Add pip packages** — Added pipx, black, ruff as phase 4 commands (pipx first, black/ruff depend on pipx).

- [x] **Add Git config commands** — Added core.editor and credential.helper as phase 4 commands. user.name and user.email moved to KtulueKit-Migration.

- [x] **Add PowerShell profile setup** — Added `ps-profile-omp` as a phase 4 command. Idempotent — checks for oh-my-posh in $PROFILE before appending.

- [x] **Add Windows Terminal font config** — Added `wt-font-config` as a phase 4 command. Updates defaults font in WT Store settings.json via PowerShell.

- [x] **Add Windows settings tweaks** — Added show-extensions, show-hidden, developer-mode, dark-mode as phase 4 reg commands.

- [x] **Add Postman** — Added `Postman.Postman` as a phase 3 winget package.

- [x] **Move Brave extensions to KtulueKit-Migration** — Removed uBlock Origin, Dark Reader, KeePassXC-Browser, and React DevTools from `extensions[]`. Browser extension state is migration territory, not bare installation. The `extensions[]` array and code infrastructure remain for future use if needed.

---

## Schema / Validation Updates (tiny code changes)

- [x] **Update JSON schema** — Added `check` field for packages and `upgrade_if_installed` for settings to `schema/ktuluekit.schema.json`. (`StatusUpgraded` is a runtime value, not a config field — no schema entry needed.) VS Code autocomplete and validation now work for these fields.

- [x] **Version pinning** — Added optional `version` field to `Package` struct (`config/schema.go`) and schema. `buildWingetArgs` now appends `--version <ver>` when set (`installer/winget.go`).

---

## One-Liner Code Changes

- [x] **`winget source update` at startup** — Run `winget source update` before the first install to ensure the package database is current. One call in `runner.Run()` before the phase loop. ~5 lines.

- [x] **Progress counter** — Add `[14/42]` before each install name. Requires counting total items across all phases and passing an index through the run functions. ~15 lines across runner.go.

- [x] **Elapsed time per package** — Wrap each install call with `time.Now()` / `time.Since()` and print duration. Add total elapsed time at the end of the summary. ~20 lines.

- [x] **Completion notification** — Play a system beep or Windows toast notification when the full run finishes. A single `exec.Command("powershell", "-Command", "[console]::beep(800,300)")` at the end of `Run()`, or use PowerShell `New-BurntToastNotification` for a proper toast.

---

## Small Feature Additions (< 50 lines each)

- [x] **Winget availability pre-flight check** — Verify `winget --version` succeeds before starting. Fail fast with a clear error message if winget is missing or broken.

- [ ] **Internet connectivity check** — Quick DNS or HTTP check before starting installs. Could piggyback on `winget source update` (if it fails, no internet).

- [x] **State-aware pre-check skip** — If `state.Succeeded["Git.Git"]` is true from a previous run, skip even the pre-check command on resume. Saves 40+ `winget list` calls when resuming after phase 4.

- [ ] **Backup shortcuts instead of deleting** — Move `.lnk` files to `$DESKTOP/.ktuluekit-shortcuts-backup/` instead of permanent deletion. Reversible.

- [ ] **`--no-upgrade` flag** — Override `upgrade_if_installed: true` from the config via CLI. For quick "just install missing stuff" runs.

- [x] **Color output (ANSI)** — Add ANSI color codes alongside emoji. Green for success, red for failure, yellow for warnings. Makes scanning long output much faster.

---

## Medium Feature Additions (50-150 lines each)

- [ ] **`validate` subcommand** — `ktuluekit validate` parses and validates the config, reports errors, and exits. No installs. Useful when editing JSON by hand.

- [ ] **`list` subcommand** — `ktuluekit list` dumps all items grouped by phase/tier. Quick reference without dry-run simulation.

- [ ] **`--only <ids>` flag** — Install/upgrade only specific package IDs. Filter the config before handing to the runner. Comma-separated list.

- [ ] **`--exclude <ids>` flag** — Skip specific packages during a run. Same filtering approach as `--only`.

- [ ] **`--phase N` (single phase)** — Run only phase N, not N through end. Different from `--resume-phase` which runs from N onward.

- [ ] **`--upgrade-only` flag** — Run only the upgrade path for already-installed packages. Skip anything not yet installed. Useful as a regular maintenance sweep.

- [ ] **Graceful Ctrl+C handling** — Trap SIGINT, save state for the current item, print a clean exit message. Prevents state corruption on interrupt.

- [ ] **PATH verification post-install** — After all runtimes install, scan for `git`, `node`, `python`, `go`, `rustup`, `pwsh` on PATH. Report gaps explicitly before entering command phases.

- [ ] **Post-install hooks** — Add optional `post_install` command field to Package struct. Runs after successful install (e.g., set a default registry key after an app installs, run a one-time setup command). Not for migrating user data — that's KtulueKit-Migration.

- [ ] **State file relocation** — Move `.ktuluekit-state.json` from CWD to `%LOCALAPPDATA%\KtulueKit\state.json` for reliable discovery regardless of working directory.

---

## Larger Feature Additions (150-500 lines)

- [x] **`status` subcommand** — `ktuluekit status` runs all check commands against the current machine and displays a table showing installed / missing / outdated for every item. No installs.

- [x] **Auto-resume via Scheduled Task** — After triggering a reboot, create a one-shot Windows Scheduled Task (`schtasks /create /sc onlogon /tn KtulueKit-Resume ...`) that runs the resume command at next logon. Delete the task after it runs. Zero-friction reboots.

- [ ] **Unit tests** — Tests for pure functions: `classifyWingetExit`, `buildWingetArgs`, `validate`, `applyDefaults`, `dependenciesMet`, `isAlreadyInstalled`. No OS interaction needed. Catches regressions.

- [x] **Bootstrap script** — `setup.ps1` installs Go via winget, builds the binary, and launches it with arg passthrough. README updated to reference it.

- [ ] **Config from URL** — `ktuluekit --config https://raw.githubusercontent.com/.../ktuluekit.json` fetches and parses a remote config. Makes it trivial to share configs or pull yours on a fresh machine without cloning.

- [ ] **Summary export formats** — `--output-format json` or `--output-format md` for the summary report. Useful for CI/automation or pasting into a GitHub issue.

- [ ] **Export/scan mode** — `ktuluekit export` scans the machine via `winget list` and generates a `ktuluekit.json` from what's currently installed. Great for bootstrapping a config from an existing machine.

---

## Major Features (500+ lines / new packages)

- [x] **Web UI / Desktop GUI** — Wails v2 + Svelte 4 desktop app (`ktuluekit-gui.exe`). Category accordion with checkboxes, profile presets, live progress feed, reboot dialog, summary screen.

- [ ] **TUI (interactive terminal)** *(nice-to-have — lower priority given the Wails GUI)* — `ktuluekit --interactive` or `-i` flag using `charmbracelet/bubbletea` + `lipgloss`. Checkbox selection screen grouped by phase, arrow keys to navigate, space to toggle, enter to confirm. Hands filtered config to existing runner.

- [x] **Profile system** — Named profiles in the config (`"profiles": [{"name": "Dev Only", "ids": [...]}]`) with profile presets in the GUI. CLI `--profile` flag still pending.

- [ ] **Config merging** — `--config base.json --config extras.json` layers multiple configs. Separates "everyone's base" from per-machine overrides.

- [ ] **Parallel installs** — Run independent packages within the same phase concurrently. Requires careful stdout multiplexing and state locking. Biggest risk: winget itself may not handle concurrent installs gracefully.
