# KtulueKit GUI — Design Spec

**Date:** 2026-03-11
**Branch:** `feat/gui`
**Goal:** Add a Wails-based local GUI (WebView2 + Svelte) that wraps the existing installer with a selection screen, live progress view, and summary — matching the WinUtil feel without requiring a browser.

---

## Out of Scope

- **Migration feature** — restoring dotfiles, MCP server configs, Claude settings after install. Separate design session.
- CLI behavior is unchanged. The GUI is an additive entry point.

---

## Approach

Wails framework: purpose-built Go → WebView2 bridge. Produces a single `.exe` that embeds the Svelte frontend. Uses the system-bundled Edge WebView2 (ships with Windows 11 — no extra runtime required for end users).

---

## Architecture

Two binaries from one repo:

```
cmd/
  main.go          ← existing CLI (unchanged)
  gui/
    main.go        ← Wails app entry point
frontend/
  src/             ← Svelte app
internal/
  runner/          ← shared by CLI and GUI
  config/          ← gains category, description, profiles fields
  reporter/        ← gains OnProgress callback hook
```

The Wails app exposes two Go functions to the Svelte frontend:

- `GetConfig() ConfigView` — returns categories, items with descriptions, and profiles parsed from `ktuluekit.json`
- `StartInstall(ids []string)` — validates selection and launches the runner in a goroutine

Live progress flows via `runtime.EventsEmit()` from Go to Svelte. The CLI binary is unaffected — its `OnProgress` stays nil and behavior is identical to today.

---

## Config Changes

### 1. `category` field on each item

Added to packages, commands, and extensions. Display-only — phases still control install order. Items without a category fall into "Other".

```json
{
  "id": "GoLang.Go",
  "name": "Go",
  "phase": 1,
  "category": "Dev Tools",
  "description": "The Go programming language runtime and compiler.",
  "check": "go version"
}
```

### 2. `description` field on each item

Short, user-facing tooltip text: "What does this do?" Distinct from `notes` (which are installer gotchas). Items without a `description` fall back to displaying `notes` in the tooltip.

### 3. Top-level `profiles` array

```json
"profiles": [
  {
    "name": "Full Setup",
    "ids": ["GoLang.Go", "OpenJS.NodeJS.LTS", "..."]
  },
  {
    "name": "Dev Only",
    "ids": ["GoLang.Go", "OpenJS.NodeJS.LTS", "Git.Git", "Microsoft.VisualStudioCode", "claude-code"]
  },
  {
    "name": "Creative",
    "ids": ["GIMP.GIMP", "Inkscape.Inkscape", "KDE.Krita", "BlenderFoundation.Blender", "Audacity.Audacity", "LMMS.LMMS", "KDE.Kdenlive", "davinci-resolve"]
  },
  {
    "name": "Minimal",
    "ids": ["Mozilla.Firefox", "Brave.Brave", "Discord.Discord", "7zip.7zip", "Microsoft.PowerToys", "Bitwarden.Bitwarden"]
  }
]
```

### 4. Bitwarden replaces KeePassXC

KeePassXC is removed from the config. Bitwarden (`Bitwarden.Bitwarden`) is added as the password manager. KeePassXC references in profiles are updated accordingly.

---

## Categories

| Category | Representative items |
|---|---|
| Dev Tools | Go, Node, Python, Rust, .NET, Git, PowerShell, VS 2022, DBeaver, Postman, WSL2, SourceTree, Claude Code, TypeScript, Prettier, pipx/black/ruff |
| Terminal & Shell | Oh My Posh, Nerd Fonts, PowerShell Profile, Windows Terminal font, Git configs |
| Editors & IDEs | VS Code, Notepad++, VS Code extensions (Go, Python, ESLint, Prettier, GitLens, C#) |
| AI Tools | Claude Desktop, Pencil Desktop, Claude Ruby Marketplace |
| Creative | GIMP, Inkscape, Krita, Audacity, LMMS, Kdenlive, Aseprite, DaVinci Resolve, Tiled |
| 3D & Making | Blender, Bambu Studio, FreeCAD, OpenSCAD, MeshMixer |
| Streaming | OBS Studio, Elgato Stream Deck, Streamer.bot, HandBrake, Plexamp |
| Gaming & Game Dev | Steam, DragonRuby GTK, DragonRuby Control |
| Media & Music | VLC, Plex Desktop, Spotify |
| Utilities | 7-Zip, Everything, PowerToys, ShareX, BleachBit, Calibre, LibreOffice, GnuCash, Bitwarden (replaces KeePassXC) |
| Browsers & Social | Firefox, Brave, Discord |
| Networking | RustDesk, WireGuard |
| Windows Config | Show File Extensions, Show Hidden Files, Developer Mode, Dark Mode |

---

## Screen Flow

Single-page Svelte app. Three states, no URL navigation.

### Screen 1: Selection

- Profile dropdown at top ("Full Setup", "Dev Only", "Creative", "Minimal")
- Loading a profile pre-checks those items; user can adjust after
- Categories displayed as collapsible accordion sections
- Each section has a "select all / deselect all" toggle
- Each item: checkbox + name + `?` icon
  - Hovering `?` shows the `description` field (falls back to `notes` if no description)
- Bottom bar: selected count + "Start Install" button (disabled if 0 selected)

### Screen 2: Progress

- Activated when "Start Install" is clicked
- Live feed of install events:
  ```
  [3/42] Installing: Go
    ✅ Go                          [winget]
       elapsed: 12s
       ▶ Raw Output  (collapsible)
  ```
- Each item shows: index/total, name, status icon, elapsed time
- Collapsible "Raw Output" drawer per item shows actual winget/command output
- No back navigation — install is in flight
- Overall progress bar at top

### Screen 3: Summary

- Mirrors the existing CLI summary output
- Sections: Installed / Failed / Skipped / Already present
- Total elapsed time
- Log file path with "Copy" button
- "Close" button

---

## Backend Changes

### Selection filter

The runner gains two new fields set via post-construction setters (not added to `New()` — avoids breaking the existing CLI call site):

```go
func (r *Runner) SetSelectedIDs(ids []string) {
    r.selectedIDs = make(map[string]bool, len(ids))
    for _, id := range ids {
        r.selectedIDs[id] = true
    }
}

func (r *Runner) SetOnProgress(fn func(ProgressEvent)) {
    r.onProgress = fn
}
```

In all three `run*InPhase` functions, items whose ID is not in `selectedIDs` are silently skipped (no output, not counted in `totalItems`). When `selectedIDs` is nil (CLI mode), all items run as today.

`printPreRunSummary()` returns `false` unconditionally when `r.onProgress != nil` (GUI mode). The suppression lives **inside** `printPreRunSummary()` as its first guard — the call site in `Run()` is unchanged:

```go
if r.printPreRunSummary() {
    return
}
```

Because the function returns `false` in GUI mode, the early-return in `Run()` is never triggered. The runner always proceeds to the phase loop. If all selected items are already installed, they all emit `"already"` events and the `"complete"` event fires immediately after — the Summary screen shows them under "Already present."

### Progress event hook

A new `OnProgress func(ProgressEvent)` is added to the `Runner` struct (not the reporter — the reporter only sees completed results, but GUI needs a "installing..." event fired *before* the installer returns).

```go
type ProgressEvent struct {
    Index   int
    Total   int
    ID      string
    Name    string
    Status  string   // see status mapping below
    Detail  string   // raw output line or error message
    Elapsed string   // "1m 23s" — empty for "installing" events
}
```

**Status mapping** (GUI string → existing `reporter.Status*` constant):

| GUI status string | reporter constant(s) |
|---|---|
| `"installing"` | emitted by runner *before* calling installer (no reporter constant) |
| `"installed"` | `StatusInstalled` |
| `"upgraded"` | `StatusUpgraded` |
| `"already"` | `StatusAlready` |
| `"failed"` | `StatusFailed` |
| `"skipped"` | `StatusSkipped` |
| `"reboot"` | `StatusReboot` |
| `"reboot_cancelled"` | emitted by runner after `CancelReboot()` is received — no reporter constant |
| `"shortcut_removed"` | `StatusShortcutRemoved` |

`StatusDryRun` will never appear via `OnProgress` — the GUI does not expose a dry-run mode.

On receiving `"reboot_cancelled"`, the frontend dismisses the reboot modal and appends "Reboot cancelled. Continuing installation..." to the progress feed.

Runner calls `r.onProgress(event)` at two points per item:
1. Before calling the installer — `Status: "installing"`
2. After `r.rep.Add(res)` — `Status` mapped from `res.Status`, `Elapsed` populated

If `OnProgress` is nil (CLI mode), runner continues printing to stdout via `fmt.Printf` as today. Both code paths are in the runner; the reporter is unchanged.

`ProgressEvent.Index` and `ProgressEvent.Total` use `r.itemIdx` and `r.totalItems` — both already present in the `Runner` struct as of the `maint/polish-sprint` branch (confirmed in `internal/runner/runner.go` lines 41–42). No new fields are needed. In GUI mode, `totalItems` is computed at the start of `Run()` by counting only items present in `selectedIDs`. `itemIdx` increments for each item processed, same as CLI.

### Reboot handling in GUI mode

`promptReboot()` blocks on stdin — incompatible with a GUI goroutine. When `OnProgress` is non-nil (GUI mode), runner emits a `"reboot"` status event instead of calling `promptReboot()`. The Svelte frontend shows a modal dialog:

> "**[ItemName] requires a reboot.**
> The auto-resume task has been registered and will run after login.
> Click Reboot Now or Continue Without Rebooting."

After emitting the `"reboot"` event, the runner goroutine blocks on `r.rebootResponse chan bool`. This channel is created by `StartInstall` before calling `r.Run()` and set via a setter:

```go
func (r *Runner) SetRebootResponse(ch chan bool) {
    r.rebootResponse = ch
}
```

In GUI mode, `promptReboot()` is not called at all — the runner checks `if r.onProgress != nil` and takes the GUI path instead. The GUI path does **not** fire `shutdown /r /t 30` automatically; that is delegated entirely to the button bindings:

- `ConfirmReboot()` — sends `true` to `r.rebootResponse`. The **runner goroutine** (not `App`) receives `true`, writes the resume command to the log, calls `exec.Command("shutdown", "/r", "/t", "30")`, emits a partial `"complete"` event with whatever `SummaryResult` has accumulated so far, sets `r.rebootResponse = nil`, and returns. The frontend transitions to the Summary screen showing partial results before Windows reboots.
- `CancelReboot()` — sends `false` to `r.rebootResponse`. The runner goroutine receives `false`, deletes the resume task, sets `r.rebootResponse = nil`, emits a `"reboot_cancelled"` progress event, and continues the run. Does **not** call `shutdown /a` — no countdown was started in GUI mode.

`ConfirmReboot()` and `CancelReboot()` are no-ops if `r.rebootResponse` is nil (no reboot pending — guards against spurious calls after run completion).

### OnFailurePrompt handling in GUI mode

`promptManualInstall()` also blocks on stdin — same incompatibility as `promptReboot()`. When `OnProgress` is non-nil (GUI mode), the runner skips the stdin block and instead emits a `ProgressEvent` with `Status: "failed"` and `Detail` set to the full `OnFailurePrompt` text. The frontend renders this text inside the item's Raw Output drawer, so the user can read the manual instructions without a blocking dialog.

### Shortcut mode in GUI

`PromptRemove()` reads from stdin — incompatible with GUI. In GUI mode, `ShortcutMode` is hardcoded to `ShortcutRemove`. Shortcuts are silently cleaned up; no dialog is shown. This is set in `cmd/gui/main.go` when constructing the runner.

### Admin check

`cmd/gui/main.go` calls `isAdmin()` on startup (same logic as CLI). If not elevated, the GUI window opens but immediately shows a full-screen error state:

> "KtulueKit must be run as Administrator.
> Right-click the .exe and choose 'Run as administrator'."

No install controls are shown until elevation is confirmed.

### Wails bindings (cmd/gui/app.go)

```go
// ConfigView is the Go→Svelte data contract for the selection screen.
type ConfigView struct {
    Categories []CategoryView
    Profiles   []ProfileView
}

type CategoryView struct {
    Name  string
    Items []ItemView  // sorted alphabetically by Name within each category
}

// Categories are returned in this fixed order, hard-coded in app.go:
// var categoryOrder = []string{
//   "Dev Tools", "Terminal & Shell", "Editors & IDEs", "AI Tools",
//   "Creative", "3D & Making", "Streaming", "Gaming & Game Dev",
//   "Media & Music", "Utilities", "Browsers & Social", "Networking", "Windows Config",
// }
// Items whose category is not in this list fall into an appended "Other" category.

type ItemView struct {
    ID          string
    Name        string
    Description string  // user-facing tooltip; falls back to Notes if empty
    Notes       string
}

type ProfileView struct {
    Name string
    IDs  []string
}

func (a *App) GetConfig() ConfigView
func (a *App) StartInstall(ids []string) string  // returns error message or "" on success
func (a *App) ConfirmReboot()
func (a *App) CancelReboot()
```

`StartInstall` performs synchronous validation only: checks that `ids` is non-empty, that no run is currently active, and that all IDs exist in the config. Returns a non-empty error string if validation fails — the frontend shows this as an inline error on Screen 1. If validation passes, returns `""` and launches the runner in a goroutine. All runtime errors during the run come through the `"progress"` event stream, not through the return value.

`App` holds a `sync.Mutex` and a `running bool` flag. `StartInstall` returns an error if `running` is true (prevents double-call). The flag is cleared when the `"complete"` event is emitted. When the run completes, a final `"complete"` event is emitted with a `SummaryResult` payload:

```go
type SummaryResult struct {
    Installed        []string
    Upgraded         []string
    Already          []string
    Failed           []string
    Skipped          []string
    Reboot           []string   // items that required a reboot
    ShortcutsRemoved []string   // shortcut filenames that were cleaned up
    TotalElapsed     string
    LogPath          string
}
```

The Summary screen (Screen 3) renders all non-empty slices as labelled sections, matching the CLI summary output.

The frontend transitions to the Summary screen on receiving this event.

### Schema update

`schema/ktuluekit.schema.json` is updated to include:
- `category` (string, optional) on package/command/extension items
- `description` (string, optional) on package/command/extension items
- Top-level `profiles` array with `name` (string) and `ids` (string array)

---

## Profile Contents (illustrative — finalized during implementation)

```json
"profiles": [
  {
    "name": "Full Setup",
    "ids": [ /* all IDs */ ]
  },
  {
    "name": "Dev Only",
    "ids": [
      "Git.Git", "Microsoft.PowerShell", "Microsoft.DotNet.SDK.8",
      "OpenJS.NodeJS.LTS", "Python.Python.3.12", "Rustlang.Rustup", "GoLang.Go",
      "Microsoft.VisualStudioCode", "Atlassian.Sourcetree", "Postman.Postman",
      "JanDeDobbeleer.OhMyPosh", "Anthropic.Claude", "Pencil.Desktop",
      "claude-code", "nerd-fonts-cascadia", "wsl2-ubuntu",
      "git-config-editor", "git-config-credential", "ps-profile-omp",
      "wt-font-config", "npm-typescript", "npm-prettier",
      "pip-pipx", "pip-black", "pip-ruff",
      "vscode-ext-go", "vscode-ext-python", "vscode-ext-eslint",
      "vscode-ext-prettier", "vscode-ext-gitlens", "vscode-ext-csharp",
      "windows-show-extensions", "windows-show-hidden",
      "windows-developer-mode", "windows-dark-mode"
    ]
  },
  {
    "name": "Creative",
    "ids": [
      "GIMP.GIMP", "Inkscape.Inkscape", "KDE.Krita", "BlenderFoundation.Blender",
      "Audacity.Audacity", "LMMS.LMMS", "KDE.Kdenlive", "davinci-resolve",
      "aseprite", "Tiled.Tiled", "Bambulab.Bambustudio",
      "FreeCAD.FreeCAD", "OpenSCAD.OpenSCAD", "meshmixer",
      "VideoLAN.VLC", "HandBrake.HandBrake"
    ]
  },
  {
    "name": "Minimal",
    "ids": [
      "Mozilla.Firefox", "Brave.Brave", "Discord.Discord",
      "7zip.7zip", "Microsoft.PowerToys", "Bitwarden.Bitwarden",
      "windows-show-extensions", "windows-show-hidden", "windows-dark-mode"
    ]
  }
]
```

---

## Constraints

- No emojis in Go code (existing convention). Svelte frontend may use them for status icons.
- Wails `v2` (stable). Svelte 5 (current stable).
- Single `.exe` output — all frontend assets embedded via `go:embed`.
- WebView2 is bundled with Windows 11 — no installer dependency for end users.
- CLI (`cmd/main.go`) must compile and behave identically after all changes.
- GUI hardcodes `ShortcutMode = ShortcutRemove` (no stdin-based prompt).
- Reboot dialog replaces `promptReboot()` stdin blocking in GUI mode.
