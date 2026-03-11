# KtulueKit-W11

A declarative Windows 11 software stack installer. Define your entire machine setup in a single JSON config and install everything in one shot — winget packages, shell commands, and browser extensions — in dependency order with retry, reboot-resume, and full reporting.

Inspired by [Chris Titus Tech's WinUtil](https://github.com/ChrisTitusTech/winutil), but scoped to a personal curated stack rather than being a general-purpose Windows utility. Fork this repo, edit `ktuluekit.json` to match your own software stack, and run it on a fresh Windows 11 install.

## Features

- **Three install tiers** in a single run:
  - **Tier 1** — Winget packages (sequential installs, exact ID matching, automatic skip for already-installed apps)
  - **Tier 2** — Shell commands (npm globals, WSL, fonts, etc.) with dependency checking and pre-run detection
  - **Tier 3** — Browser extensions (Brave, Chrome, Firefox) via registry policy force-install or store URL opening
- **Phased execution** — items are grouped into numbered phases and run in order, so runtimes install before tools that depend on them
- **Dependency resolution** — Tier 2 commands declare `depends_on` references; if a dependency failed, dependents are skipped (not errored)
- **Dry-run mode** — preview every action without touching the system
- **Reboot-resume** — items flagged `reboot_after` prompt the user, and `--resume-phase` picks up where you left off
- **Automatic PATH refresh** — after winget installs runtimes, PATH is refreshed in-session before shell commands run
- **Retry on failure** — configurable retry count per item (default: 1 retry)
- **Per-package scope** — global default `machine` scope, with per-item `user` override (e.g. Spotify)
- **Manual fallback guidance** — `on_failure_prompt` field prints step-by-step instructions and pauses when auto-install fails
- **Timestamped log files** — every run writes a categorized summary to `./logs/`
- **Idempotent** — safe to re-run at any time; winget skips installed packages, commands check before running
- **JSON Schema** — full schema at `schema/ktuluekit.schema.json` for editor autocompletion and validation

## Quick Start

### Prerequisites

- Windows 11 with [winget](https://learn.microsoft.com/en-us/windows/package-manager/winget/) available (ships with modern W11)
- An **administrator** terminal (right-click > Run as administrator)

### Option A: Setup from scratch

If Go is not yet installed, the setup script handles everything:

```powershell
# From an admin PowerShell:
.setup.ps1
```

This installs Go via winget, refreshes PATH, builds `ktuluekit.exe`, and launches it. Pass any flags through directly:

```powershell
.setup.ps1 --dry-run
.setup.ps1 status
```

### Option B: Build manually (Go already installed)

```bash
go build -o ktuluekit.exe ./cmd/
```

### Run

```bash
# Preview what would be installed (no admin required):
.\ktuluekit.exe --dry-run

# Full install (requires admin):
.\ktuluekit.exe

# Resume after a reboot (skip phases 1-3, start at phase 4):
.\ktuluekit.exe --resume-phase=4
```

## CLI Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--config` | `-c` | `ktuluekit.json` | Path to config file |
| `--dry-run` | `-d` | `false` | Show what would be installed without doing it |
| `--resume-phase` | | `1` | Skip all phases before this number (for post-reboot resume) |

## Config Structure

The config file (`ktuluekit.json`) is a declarative desired-state definition with three sections:

```jsonc
{
  "$schema": "./schema/ktuluekit.schema.json",
  "version": "1.0",
  "metadata": { "name": "...", "author": "..." },
  "settings": {
    "log_dir": "./logs",
    "retry_count": 1,
    "default_timeout_seconds": 300,
    "default_scope": "machine",       // "machine" or "user"
    "extension_mode": "url"           // "url" or "force"
  },
  "packages": [
    { "id": "Git.Git", "name": "Git for Windows", "phase": 1 },
    { "id": "Spotify.Spotify", "name": "Spotify", "phase": 3, "scope": "user" }
  ],
  "commands": [
    {
      "id": "claude-code",
      "name": "Claude Code",
      "phase": 4,
      "check": "claude --version",
      "command": "npm install -g @anthropic-ai/claude-code",
      "depends_on": ["OpenJS.NodeJS.LTS"]
    }
  ],
  "extensions": [
    {
      "id": "ublock-origin",
      "name": "uBlock Origin",
      "phase": 6,
      "extension_id": "cjpalhdlnbpafiamejdnhcphjbkeiagm",
      "browser": "brave"
    }
  ]
}
```

See `schema/ktuluekit.schema.json` for the full schema with descriptions and validation rules.

## Summary Report

Every run prints a categorized summary to the terminal and saves it to a timestamped log file:

```
============================================================
SUMMARY
============================================================

✅ Installed successfully (5)
    • Git for Windows
    • Node.js LTS
    • Claude Code

⏭️  Already installed (skipped) (12)
    • Firefox
    • VS Code
    • Discord

❌ Failed (1)
    • Visual Studio 2022 Community: exit code 1603

⚠️  Skipped (dependency missing) (1)
    • Nerd Fonts: dependency not met: JanDeDobbeleer.OhMyPosh

🔄 Reboot required (1)
    • WSL2 (Ubuntu)
```

## Customizing for Your Own Machine

1. Fork this repo
2. Edit `ktuluekit.json` — add/remove packages, commands, and extensions to match your stack
3. Assign phases to control install order (lower phases run first)
4. Use `winget search <name>` to find exact package IDs for Tier 1
5. Build and run

## Project Structure

```
├── setup.ps1                     # One-shot setup: installs Go, builds and launches the binary
├── ktuluekit.json                # Your software stack config (edit this)
├── schema/
│   └── ktuluekit.schema.json     # JSON Schema for editor validation
├── cmd/
│   ├── main.go                   # CLI entry point (cobra)
│   └── admin_windows.go          # Admin privilege check
├── internal/
│   ├── config/                   # Config loading and schema types
│   ├── installer/                # Tier 1/2/3 install logic (winget, commands, extensions)
│   ├── reporter/                 # Result collection and summary output
│   ├── runner/                   # Phase orchestration and dependency resolution
│   └── state/                    # Reboot-resume state persistence
├── logs/                         # Timestamped run logs (gitignored)
└── KtulueKit-Project-Document.md # Full design spec and known gotchas
```

## Known Gotchas

- **PowerShell 7 via winget** can terminate the current PowerShell session — run from `cmd.exe` or Windows Terminal
- **Spotify** requires `--scope user` (the config handles this via the `scope` field)
- **Extension force-install** via registry policy shows "Managed by your organization" in the browser — use `"extension_mode": "url"` to avoid this
- **WSL2** requires a reboot before it's usable — the tool prompts automatically
- **Large installs** like Visual Studio may need extended timeouts (configurable per-item via `timeout_seconds`)

See `KtulueKit-Project-Document.md` for the full list of design constraints and workarounds.

## Tech Stack

- **Go** with [Cobra](https://github.com/spf13/cobra) for the CLI
- **Winget** for Tier 1 package management
- **Windows Registry** APIs via `golang.org/x/sys/windows/registry` for browser extension policies
- **PowerShell setup script** for first-run setup on a bare machine

## License

[MIT](LICENSE)
