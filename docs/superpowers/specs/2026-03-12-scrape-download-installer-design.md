# Scrape-Download Installer Type

**Date:** 2026-03-12
**Status:** Approved

## Summary

Add a new installer type to KtulueKit-W11 that can automatically discover and download the latest version of a tool by scraping its download page — no hardcoded version numbers. Three tools qualify: Frame0 (new), Plexamp (upgrade from `start` command), and Streamer.bot (upgrade from `echo Already installed`).

## Problem

Several tools in the KtulueKit stack are not available on winget and have no stable direct-download URL that is version-independent. The current workarounds are:

- `start <download-page>` — opens a browser tab, requires manual download and install
- `echo Already installed` — tracked in desired state only, never actually automated

Both are second-class citizens in the install flow. The scrape-download type makes these tools fully automated and version-agnostic.

## Qualifying Tools

| Tool | Current State | Download Page | URL Regex Pattern |
|---|---|---|---|
| Frame0 | Not yet in config | `https://frame0.app/download` | `https://files\.frame0\.app/releases/win32/x64/Frame0-[\d\.]+ Setup\.exe` |
| Plexamp | `start https://www.plex.tv/plexamp` | `https://www.plex.tv/media-server-downloads/` | `https://plexamp\.plex\.tv/plexamp\.plex\.tv/desktop/Plexamp%20Setup%20[\d\.]+\.exe` |
| Streamer.bot | `echo Already installed` | `https://streamer.bot` | `https://streamer\.bot/api/releases/streamer\.bot/[\d\.]+/download` |

Non-qualifying tools (remain unchanged):
- **Stream Deck** — JS-triggered download, no `href` in HTML
- **Meshmixer** — Autodesk OAuth2 gate, requires login before download

## Architecture

### Schema & Config Struct — New Fields on `Command`

Three new optional fields added to the `Command` definition:

```json
"scrape_url":   { "type": "string", "description": "Page URL to fetch HTML from to find the download link." },
"url_pattern":  { "type": "string", "description": "Regex pattern to extract the download URL from the page HTML." },
"install_args": { "type": "string", "description": "Optional CLI flags passed to the installer executable (e.g. /S for silent NSIS). Space-separated simple flags only — no quoted tokens with spaces." }
```

Corresponding Go struct fields added to `config.Command`:

```go
ScrapeURL   string `json:"scrape_url"`
URLPattern  string `json:"url_pattern"`
InstallArgs string `json:"install_args"`
```

**`install_args` splitting:** The value is split into individual args using `strings.Fields()`. This handles simple flags like `/S`, `--silent`, `--no-sandbox`. Quoted tokens with internal spaces are not supported — the field description documents this constraint.

**Validation rule:** The `command` field is removed from the JSON schema `required` array. In `internal/config/validate.go`, the existing unconditional `c.Cmd == ""` error check is **replaced** (not supplemented) with the following XOR rule:

- If `ScrapeURL == ""` and `URLPattern == ""`: `Cmd` must be non-empty (standard command entry)
- If `ScrapeURL != ""` or `URLPattern != ""`: both `ScrapeURL` and `URLPattern` must be non-empty, and `Cmd` must be empty
- Any other combination is rejected with a descriptive error: `command %q: must have either 'command' or both 'scrape_url' and 'url_pattern' (not both)`

The `check` field remains required for all `Command` entries (both standard and scrape-type). All three new scrape-type entries include a `check` field; it is consumed by `isAlreadyInstalled()` inside `ScrapeAndInstall`, after the dry-run guard (since the scrape branch fires before the `isAlreadyInstalled` call in `RunCommand`).

### New File: `internal/installer/scrape.go`

A `ScrapeAndInstall(cmd config.Command, dryRun bool) reporter.Result` function:

1. **Dry-run guard** — if `dryRun`, print what would be fetched/downloaded/executed and return `StatusDryRun`. No network calls.
2. **Already-installed check** — call `isAlreadyInstalled(cmd.Check)`. If it returns true, return `StatusAlready` immediately. No network calls. (Note: this is performed here rather than in `RunCommand` because the scrape branch fires before the `isAlreadyInstalled` call in `RunCommand`.)
3. **Fetch page** — HTTP GET `cmd.ScrapeURL` with a 30-second timeout. Return `StatusFailed` on network error or non-200 response.
3. **Match URL** — compile `cmd.URLPattern` as a regex, find first match in response body. Return `StatusFailed` with detail `"no download URL found matching pattern"` if no match.
4. **Download** — HTTP GET the matched URL, following redirects (Go's default `http.Client` behaviour). Stream response body to `filepath.Join(os.TempDir(), cmd.ID+"-setup.exe")` opened with `os.Create` (truncate semantics — overwrites any leftover partial file from a prior run). No explicit timeout on the download step; the OS and Go's HTTP client handle connection-level timeouts.
5. **Execute** — run the downloaded `.exe` using `exec.CommandContext` with a context derived from `cmd.TimeoutSeconds` (same timeout mechanism as `runShellWithTimeout`), passing `strings.Fields(cmd.InstallArgs)` as args. Wait for exit.
6. **Cleanup** — delete the temp file via `defer os.Remove(...)` — runs regardless of outcome.
7. **Return** — `StatusInstalled` on exit 0, `StatusFailed` on non-zero exit or any step error. Error detail includes the step that failed and the underlying error string.

**Retry behaviour:** Scrape-type entries bypass the retry loop in `RunCommand()` (see branch placement below). This is intentional — each attempt re-fetches the page and re-downloads the installer, making retry-on-failure expensive and rarely useful for installer execution failures. Retry is omitted by design.

### `internal/installer/command.go` — Branch Placement in `RunCommand()`

The scrape branch is inserted **before** the `if dryRun` early-return block (before line 45 in the current file), so that `ScrapeAndInstall` handles its own dry-run logic:

```go
func RunCommand(cmd config.Command, dryRun bool, retryCount int, s *state.State) reporter.Result {
    // Scrape-download path — handles dry-run internally.
    if cmd.ScrapeURL != "" {
        return ScrapeAndInstall(cmd, dryRun)
    }

    // Existing path below — unchanged.
    res := reporter.Result{ ... }
    if dryRun { ... }
    ...
}
```

Placing the branch before the dry-run block ensures scrape entries print meaningful dry-run output (what they would fetch/download/exec) rather than falling through to `res.Detail = cmd.Cmd` which would be empty for these entries.

### `internal/config/validate.go` — Replace Existing Check

The current unconditional error:
```go
if c.Cmd == "" {
    add(c.ID, "required field 'command' is missing")
}
```

Is replaced with:
```go
hasScrape := c.ScrapeURL != "" || c.URLPattern != ""
hasCmd := c.Cmd != ""
switch {
case !hasScrape && !hasCmd:
    add(c.ID, "must have either 'command' or both 'scrape_url' and 'url_pattern'")
case hasScrape && hasCmd:
    add(c.ID, "cannot have both 'command' and 'scrape_url'/'url_pattern'")
case hasScrape && (c.ScrapeURL == "" || c.URLPattern == ""):
    add(c.ID, "scrape-type entries must have both 'scrape_url' and 'url_pattern'")
}
```

### `schema/ktuluekit.schema.json` — Updates

1. Remove `"command"` from `Command.required` array
2. Add `scrape_url`, `url_pattern`, `install_args` to `Command.properties` (required by `additionalProperties: false`)
3. Add an `if/then` constraint capturing the XOR rule for editor-level feedback (primary enforcement remains in the Go loader)

### `ktuluekit.json` — Config Entries

**Frame0** (new entry in `commands[]`):
```json
{
  "id": "frame0",
  "name": "Frame0",
  "phase": 5,
  "category": "Dev Tools",
  "description": "UI template design and wireframing tool.",
  "check": "winget list --name \"Frame0\" -e --accept-source-agreements",
  "scrape_url": "https://frame0.app/download",
  "url_pattern": "https://files\\.frame0\\.app/releases/win32/x64/Frame0-[\\d\\.]+ Setup\\.exe",
  "install_args": "/S",
  "notes": "Not on winget. Scraped from frame0.app/download. NSIS installer — /S for silent."
}
```

**Plexamp** (replace existing `commands[]` entry):
```json
{
  "id": "plexamp",
  "name": "Plexamp",
  "phase": 5,
  "category": "Media & Music",
  "description": "Beautiful music player for your Plex library.",
  "check": "winget list --name \"Plexamp\" -e --accept-source-agreements",
  "scrape_url": "https://www.plex.tv/media-server-downloads/",
  "url_pattern": "https://plexamp\\.plex\\.tv/plexamp\\.plex\\.tv/desktop/Plexamp%20Setup%20[\\d\\.]+\\.exe",
  "install_args": "--silent",
  "notes": "Not on winget. Scraped from plex.tv/media-server-downloads. Electron/Squirrel installer — --silent flag may need verification on first run. Uses --name check (not --id) because Plexamp is not in winget; the name check correctly returns non-zero when not installed."
}
```

**Streamer.bot** (replace existing `commands[]` entry):
```json
{
  "id": "streamerbot",
  "name": "Streamer.bot",
  "phase": 3,
  "category": "Streaming",
  "description": "Automation software for streamers. Integrates with OBS, Twitch, and more.",
  "check": "winget list --name \"Streamer.bot\" -e --accept-source-agreements",
  "scrape_url": "https://streamer.bot",
  "url_pattern": "https://streamer\\.bot/api/releases/streamer\\.bot/[\\d\\.]+/download",
  "install_args": "",
  "notes": "Not on winget. Scraped from streamer.bot. Silent install flag unknown — leave install_args empty initially; verify and update without a Go rebuild."
}
```

### Profile Updates

- **`Full Setup`** profile: add `"frame0"` to `ids`
- **`Dev Only`** profile: add `"frame0"` to `ids` (Frame0 is categorised as Dev Tools)
- Plexamp and Streamer.bot IDs are unchanged; their existing profile memberships are preserved automatically

## Silent Install Flags — Confidence Levels

| Tool | Likely Flag | Confidence | Action |
|---|---|---|---|
| Frame0 | `/S` | High — NSIS standard | Ship as-is |
| Plexamp | `--silent` | Medium — Electron/Squirrel common | Verify on first run |
| Streamer.bot | Unknown | Low | Leave `install_args: ""` initially; update in config after testing |

`install_args` lives in the JSON config, so flags can be corrected without a Go rebuild.

## Testing

- Unit test `ScrapeAndInstall` with a mock HTTP server serving canned HTML — verifies page fetch, regex match, download stream, exec invocation, and temp file cleanup
- Unit test the replaced validation rule in `internal/config/validate.go` — covers: missing-both, scrape-only-one-field, has-both-cmd-and-scrape, valid-command, valid-scrape
- Dry-run test: confirm scrape entries return `StatusDryRun` and print fetch/download/exec details without any network calls
- Timeout test: confirm exec step respects `cmd.TimeoutSeconds`

## Out of Scope

- Stream Deck and Meshmixer remain unchanged (JS gate / OAuth2 gate)
- No GUI changes — scrape-download entries appear in the install list identically to other commands
- No version pinning — this feature is explicitly designed to always fetch latest
- No retry loop for scrape entries — intentional; re-downloading on failure is expensive and unlikely to help
