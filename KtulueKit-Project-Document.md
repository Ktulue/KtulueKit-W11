# KtulueKit-W11 — Project Document

This file serves as the CLAUDE.md context for Claude Code. Drop it in the repo root.

GitHub repo: github.com/Ktulue/KtulueKit-W11

---

## Context

I'm Josh (Ktulue / "The Water Father"), a software engineer mid-migration from W10 to W11. I've already compiled a comprehensive W11 Software Suite document (markdown) listing ~40+ applications I need across dev tools, streaming/content creation, 3D printing, game dev, productivity, networking, hobbies, and browser extensions. That doc is organized by domain with install order phases.

I want to build a **custom automated installer tool** that handles my entire software stack in one shot — similar in spirit to Chris Titus Tech's WinUtil (github.com/ChrisTitusTech/winutil), but scoped specifically to MY needs rather than being a general-purpose Windows utility.

## What I've Already Installed (manually, during migration)

These were installed manually before this tool existed. Winget natively detects already-installed software and skips it (returns "Already installed" / "No available upgrade found"), so these do NOT need special skip logic — they should be included in the full config alongside everything else. The installer is declarative: it defines the desired state of the machine, not just the diff.

- Notepad++ 8.8.8
- Firefox
- LibreOffice 26.2.1
- SourceTree 3.4.26
- VS Code 1.110.1
- Stream Deck 7.3.1
- Streamer.bot 1.0.4
- DaVinci Resolve 20.3.2
- Steam
- Discord
- Spotify
- OBS Studio 32.0.4
- Brave Browser

## What Still Needs Installing

### Tier 1 — Winget-native (has winget IDs)
These can be installed via `winget install --id=<ID> -e` (already-installed apps will be harmlessly skipped by winget):

| Software | Winget ID | Status |
|---|---|---|
| Notepad++ | Notepad++.Notepad++ | ✅ Already installed |
| Firefox | Mozilla.Firefox | ✅ Already installed |
| LibreOffice | TheDocumentFoundation.LibreOffice | ✅ Already installed |
| SourceTree | Atlassian.Sourcetree | ✅ Already installed |
| VS Code | Microsoft.VisualStudioCode | ✅ Already installed |
| Steam | Valve.Steam | ✅ Already installed |
| Discord | Discord.Discord | ✅ Already installed |
| Spotify | Spotify.Spotify | ✅ Already installed |
| OBS Studio | OBSProject.OBSStudio | ✅ Already installed |
| Brave Browser | Brave.Brave | ✅ Already installed |
| Git for Windows | Git.Git | ⬜ Needed |
| PowerShell 7 | Microsoft.PowerShell | ⬜ Needed |
| .NET 8 SDK | Microsoft.DotNet.SDK.8 | ⬜ Needed |
| Visual Studio 2022 Community | Microsoft.VisualStudio.2022.Community | ⬜ Needed |
| Node.js LTS | OpenJS.NodeJS.LTS | ⬜ Needed |
| Python 3.12+ | Python.Python.3.12 | ⬜ Needed |
| Rust (rustup) | Rustlang.Rustup | ⬜ Needed |
| Go | GoLang.Go | ⬜ Needed |
| 7-Zip | 7zip.7zip | ⬜ Needed |
| Everything (voidtools) | voidtools.Everything | ⬜ Needed |
| PowerToys | Microsoft.PowerToys | ⬜ Needed |
| ShareX | ShareX.ShareX | ⬜ Needed |
| KeePassXC | KeePassXCTeam.KeePassXC | ⬜ Needed |
| Oh My Posh | JanDeDobbeleer.OhMyPosh | ⬜ Needed |
| GIMP | GIMP.GIMP | ⬜ Needed |
| Inkscape | Inkscape.Inkscape | ⬜ Needed |
| Krita | KDE.Krita | ⬜ Needed |
| Audacity | Audacity.Audacity | ⬜ Needed |
| Handbrake | HandBrake.HandBrake | ⬜ Needed |
| VLC | VideoLAN.VLC | ⬜ Needed |
| Kdenlive | KDE.Kdenlive | ⬜ Needed |
| RustDesk | RustDesk.RustDesk | ⬜ Needed |
| WireGuard | WireGuard.WireGuard | ⬜ Needed |
| DBeaver Community | dbeaver.dbeaver | ⬜ Needed |
| Bambu Studio | Bambulab.Bambustudio | ⬜ Needed |
| FreeCAD | FreeCAD.FreeCAD | ⬜ Needed |
| Blender | BlenderFoundation.Blender | ⬜ Needed |
| GnuCash | GnuCash.GnuCash | ⬜ Needed |
| Plex Desktop | Plex.Plex | ⬜ Needed |
| Calibre | calibre.calibre | ⬜ Needed |
| BleachBit | BleachBit.BleachBit | ⬜ Needed |

### Intentionally excluded from Tier 1
FileZilla, WinSCP, and PuTTY were evaluated and intentionally removed from the config. They are not included in `ktuluekit.json`.

### Tier 2 — Non-winget (needs custom install logic)
These require npm commands, direct downloads, or other install methods:

| Software | Install Method |
|---|---|
| Claude Code | `npm install -g @anthropic-ai/claude-code` (requires Node.js from Tier 1) |
| Nerd Fonts (CaskaydiaCove) | `oh-my-posh font install CascadiaCode` (requires Oh My Posh from Tier 1) |
| WSL2 (Ubuntu) | `wsl --install -d Ubuntu` from admin terminal |
| DragonRuby GTK | Manual download (licensed, dragonruby.org) — just open URL |
| DaVinci Resolve | Already installed, but no winget package exists for future reference |
| Streamer.bot | Already installed, no winget package |
| Stream Deck | Already installed, no winget package |
| MeshMixer | Direct download from meshmixer.com |
| Aseprite | One-time purchase or compile from source (github.com/aseprite/aseprite) |
| Plexamp | Direct download from plex.tv/plexamp |
| Claude Ruby Marketplace | Open github.com/hoblin/claude-ruby-marketplace — manual install |
| Peon Ping | Open github.com/PeonPing/peon-ping — manual install |
| DragonRuby Control | Open github.com/peterkarman1/dragonruby-control — manual install |

### Tier 3 — Browser Extensions (Brave / Chromium)
Force-install via registry policy (`HKLM\Software\Policies\BraveSoftware\Brave-Browser\ExtensionInstallForcelist`) or open Chrome Web Store URLs for manual click:

| Extension | Chrome Web Store ID |
|---|---|
| Hype Control | (my extension — get ID from Chrome Web Store listing) |
| uBlock Origin | cjpalhdlnbpafiamejdnhcphjbkeiagm |
| Dark Reader | eimadpbcbfnmbkopoojfekhnkhdbieeh |
| KeePassXC-Browser | oboonakemofpalcgghocfoadofidjkkk |
| React Developer Tools | fmkadmapgofadopljbjfkapdkoienihi |

## What I Want to Build

A CLI tool or PowerShell script that:

1. **Reads a JSON config file** defining my complete software stack (winget IDs, npm commands, direct URLs, extension IDs) — this is a declarative "desired state" for the machine
2. **Installs in dependency order** (runtimes first → tools that depend on them → specialty items)
3. **Handles three install methods:**
   - Winget packages (Tier 1) — winget natively skips already-installed apps, so no detection logic needed here
   - Shell commands like npm/pip/wsl (Tier 2) — these DO need "already installed?" checks before running
   - Browser extension registry entries or URL opening (Tier 3) — check if registry key already exists before writing
4. **Logs results in real-time** — stream output to the terminal as packages install
5. **Generates a summary report at the end** — a clean, categorized breakdown that answers "what do I need to deal with?" without scrolling through terminal history. The report should include:
   - ✅ **Installed successfully** — packages that were freshly installed this run
   - ⏭️ **Already installed (skipped)** — packages winget detected as present
   - ❌ **Failed** — packages that errored, with the exit code and reason if available
   - ⚠️ **Skipped (dependency missing)** — Tier 2 commands that couldn't run because their dependency failed or wasn't installed
   - 🔄 **Reboot required** — packages that flagged a reboot need
   - The report should be displayed in the terminal AND saved to a timestamped log file (e.g., `KtulueKit_2026-03-09_results.log`) so you can reference it after a reboot
6. **Supports dry-run mode** — show what WOULD be installed without doing it
7. **Can be re-run safely** after a failure (idempotent — winget handles this natively for Tier 1, tool handles it for Tiers 2 and 3)

## Known Gotchas & Design Constraints

These are real issues discovered from WinUtil bug reports, winget-cli GitHub issues, and community feedback. The tool MUST account for all of these.

### Winget-Specific

1. **10-package bulk limit.** Winget has a hard cap of 10 packages when passed in a single `winget install pkg1 pkg2 ... pkg11` command. The 11th package is silently dropped. **Solution:** Install packages sequentially in a loop with individual `winget install` calls, NOT in bulk.

2. **Always use exact match.** Running `winget install steam` can return "Multiple packages found matching input criteria" because the name matches multiple packages (e.g., Steam AND Git Extensions). **Solution:** Always use `winget install -e --id Package.ExactID` with the `-e` (exact) flag and the full `--id`. The config already stores exact IDs, so this is handled by design.

3. **Suppress interactive prompts.** Some packages prompt for license agreement acceptance, which blocks the script. **Solution:** Always pass `--accept-package-agreements --accept-source-agreements --disable-interactivity` flags on every install call.

4. **PowerShell can kill itself.** Installing PowerShell 7 via winget while running in PowerShell can terminate the session — and with it, the entire script. **Solution:** Install PowerShell 7 early in the sequence. Consider detecting if the script is running in the PowerShell version being upgraded, and if so, warn or defer that specific install.

5. **Spotify user-scope issue.** Spotify's installer only works at user level, not as admin. Running `winget install` as admin can cause Spotify to fail silently. **Solution:** Certain packages may need `--scope user` instead of the default. The config schema should support a per-package scope override.

6. **Some installers pop UI windows.** Not all installers respect silent mode — they may pop up visible windows, steal focus, or freeze waiting for user input. **Solution:** Log which packages are currently installing so the user knows what to look for if something hangs. Include a configurable timeout per package.

7. **Winget import bails on ambiguity.** The native `winget import` command stops processing the entire list if it hits one ambiguous package. **Solution:** Don't use `winget import` at all. Run individual installs with error handling per package — if one fails, log it and continue to the next.

8. **Exit code handling.** Winget returns exit code 0 for success and various non-zero codes for failures. The script must capture the exit code after EACH install and categorize the result (success, already installed, failed, needs reboot).

### Reboot Considerations

9. **Some installs require a reboot.** WSL2 notably requires a reboot before it's usable. Visual Studio may also request one. **.NET SDK** and **Visual Studio** can behave unpredictably if installed back-to-back without a reboot between. **Solution:** Flag packages in the config that are known to require/request reboots. After those installs, pause and prompt the user: "Reboot now and re-run to continue, or skip and continue?" The tool should track progress so it can resume where it left off after a reboot.

### Tier 2 (Shell Commands) Gotchas

10. **Dependency ordering matters.** Claude Code requires Node.js. Nerd Fonts require Oh My Posh. The config must express these dependencies, and the tool must resolve them before executing. If a dependency wasn't installed (e.g., Node.js failed), skip the dependent command rather than erroring out.

11. **PATH not updated mid-session.** After installing Node.js or Go via winget, the current terminal session may not have the updated PATH. Running `npm install -g` immediately after installing Node.js can fail with "npm not found." **Solution:** After installing runtimes, refresh the PATH in the current session (`$env:Path = [System.Environment]::GetEnvironmentVariable("Path","Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path","User")`) before running dependent commands.

### Tier 3 (Browser Extensions) Gotchas

12. **"Managed by your organization" label.** Force-installing extensions via registry policy causes the browser to show "Managed by your organization" in settings and a small label on force-installed extensions. This is cosmetic but can be surprising. **Solution:** Document this clearly. Offer an alternative mode that simply opens the Chrome Web Store URLs for manual one-click install instead of registry force-install.

13. **Brave vs Chrome registry paths.** Brave uses `HKLM\Software\Policies\BraveSoftware\Brave-Browser\ExtensionInstallForcelist` NOT the Chrome path. Firefox uses an entirely different mechanism (policies.json or the Mozilla registry path). **Solution:** The config should specify which browser each extension targets, and the tool should write to the correct registry/policy path accordingly.

### General

14. **Admin vs user scope.** The script needs to run as admin for winget system-scope installs and registry writes, but some operations (Spotify, npm globals) work better at user scope. **Solution:** Run the script as admin but explicitly set `--scope user` for packages that need it.

15. **Network failures.** A flaky internet connection mid-batch can cause downloads to fail. **Solution:** On failure, retry once before logging as failed. The re-run capability (idempotency) handles the rest — user can just run again later.

## Language Decision Needed

I need help deciding which language to build this in. The options:

- **PowerShell** — most natural for Windows system operations, closest to how WinUtil works, zero dependencies needed on a fresh W11 install. But doesn't serve the career pivot portfolio as strongly.
- **Go** — compiles to a single binary, good CLI story, would be a portfolio piece for startup targeting. But need Go installed first (chicken-and-egg, though a bootstrap script could handle this).
- **Rust** — same benefits as Go but stronger "systems-level" signal on resume. Longer learning curve. Same chicken-and-egg issue.
- **Hybrid** — thin PowerShell bootstrap that installs Go/Rust, then the main tool takes over. Best of both worlds?

## My Background

- 10+ years SWE, primarily C#/.NET/SQL Server
- Currently building an AI-augmented career pivot targeting startups
- My Hype Control Chrome extension (github.com/Ktulue/HypeControl) is TypeScript (76.9%) — built primarily with Claude Code
- I'm a visual learner who prefers dark mode everything
- I value open source / one-time-purchase software
- This tool should be practical first, portfolio piece second
- I use AI (Claude Code) heavily to write code — my strength is reading/reviewing/directing, not memorizing syntax

## What I Need From You

1. Help me decide on the language/architecture
2. Scaffold the project structure
3. Build the JSON config schema
4. Implement the installer logic incrementally
5. Make it something I could realistically ship on GitHub as a public project (useful to others who want to create their own curated install configs)

Let's start by discussing the architecture and language choice, then move into building.
