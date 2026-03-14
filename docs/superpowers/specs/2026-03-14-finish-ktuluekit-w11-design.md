# KtulueKit-W11: Finishing Sprint Design

**Date:** 2026-03-14
**Status:** Approved
**Scope:** All remaining features to reach v1.0 and hand off to KtulueKit-Migration

---

## Overview

KtulueKit-W11 is a mature Windows 11 software installer with three install tiers (winget, shell commands, browser extensions), a Wails/Svelte desktop GUI, and a full CLI. The core feature set is complete. This spec covers the five remaining tracks needed to call the project done.

**Out of scope:** Parallel installs (too risky), desired-state enforcement / machine-wide uninstall scanning (outside project scope). Only items defined in the user's JSON config are ever touched.

**Definition of done:** All five tracks merged to main, `/security-review` run before each PR, TODO.md fully cleared, project in a clean state for KtulueKit-Migration handoff.

---

## Architecture

No fundamental changes to the system. All work extends existing packages and patterns.

```
┌─────────────────────────────────────────────────────┐
│  CLI (cmd/)                  GUI (frontend/src/)     │
│  + uninstall subcommand      + Install/Uninstall tabs│
│  + --profile flag            + Mode-aware ItemRow    │
│  + --output-format flag                              │
├─────────────────────────────────────────────────────┤
│  Runner (internal/runner/)                           │
│  + uninstall action routing                          │
│  + post_install execution                            │
├─────────────────────────────────────────────────────┤
│  Installer (internal/installer/)                     │
│  + winget.go: UninstallPackage()                     │
│  + command.go: uninstall_cmd + post_install support  │
│  + extension.go: registry policy value removal       │
├─────────────────────────────────────────────────────┤
│  Config (internal/config/)                           │
│  + schema: uninstall_cmd, post_install fields        │
│  + loader: unchanged (URL fetch handled in cmd layer)│
├─────────────────────────────────────────────────────┤
│  Reporter (internal/reporter/)                       │
│  + progressWriter io.Writer injection                │
│  + --output-format json|md serializers               │
└─────────────────────────────────────────────────────┘
```

---

## Track 1: `feat/cli-polish`

Three small features, one branch. Backend only — no UI changes.

### Post-Install Hooks

**Schema changes:**
- Add `PostInstall string` field to both `config.Package` and `config.Command` structs in `schema.go`
- No validation rule required beyond confirming the field is present — empty string is valid (hook is skipped if empty)

```json
{
  "id": "nodejs",
  "name": "Node.js",
  "post_install": "node --version"
}
```

**Trigger rules (explicit):**
- Hook fires on `StatusInstalled` (fresh install) or `StatusUpgraded` (upgraded)
- Hook does NOT fire on `StatusAlready`, `StatusFailed`, `StatusSkipped`, or `StatusReboot`
- `StatusReboot` is not a completion state — the package is not yet done; the hook will run after the reboot resume completes the install if the post-reboot result reaches `StatusInstalled`

**Execution:** The `post_install` string is passed to `cmd /C` via the existing `runShellWithTimeout` executor. The hook uses the item's `TimeoutSeconds` if set, otherwise the global `DefaultTimeoutSeconds`.

**Failure behavior:** Hook failure is logged as a warning. It does not change the item's result status or block subsequent items.

**Dry-run:** Prints `[DRY RUN] Would run post_install: <cmd>` and skips execution.

**Single string only** — no array support; covers all current use cases.

### CLI `--profile` Flag

**Background:** Profiles are named subsets of item IDs in the JSON config:
```json
{ "profiles": [{ "name": "Gaming", "ids": ["steam", "discord", "obs"] }] }
```

**Usage:**
```
ktuluekit install --profile Gaming
ktuluekit status --profile Gaming
ktuluekit export --profile Gaming
```

**Behavior per subcommand:**
- `install`: resolves profile to its ID list, passes as `--only` filter internally
- `status`: same — resolves to `--only` filter; only profile items appear in the status table, in normal order and grouping
- `export`: pre-filters `cfg.Packages`, `cfg.Commands`, and `cfg.Extensions` slices to the profile's IDs before calling `exporter.Export`. Tier 4 (scrape-download) items in the profile are silently omitted from the snapshot — they have no check command and the existing exporter has no unknown-status concept; maintaining current exporter behavior is correct here.

**Flag conflicts:**
- `--profile` and `--only`: mutually exclusive — exit with error
- `--profile` and `--exclude`: compatible — excludes apply within resolved ID list
- Case-sensitive name matching; error if profile not found

### Summary Export Formats

**Placement:** `--output-format` on `install` subcommand only. Does not affect `export`.

```
ktuluekit install --output-format json > results.json
ktuluekit install --output-format md > results.md
```

**Formats:** `json` (machine-readable) and `md` (GitHub-flavored Markdown table), both to stdout.

**Implementation — writer injection:** The `Reporter` struct gains a `progressWriter io.Writer` field. `reporter.New()` gains an optional `progressWriter` parameter (default `os.Stdout`). All live progress writes in `Reporter.Add()` and in the runner's phase-loop `fmt.Printf` calls are replaced with writes to this writer. When `--output-format` is set, `cmd/main.go` passes `os.Stderr` as `progressWriter`; the final formatted summary is written to `os.Stdout`. This avoids swapping file descriptors at process level and keeps the change self-contained to reporter and runner.

**Existing stderr output (errors, warnings):** Unchanged and unsuppressed. Consumers piping stdout receive only the clean formatted summary; stderr carries progress and any errors — acceptable for this personal-use tool.

**Default (no flag):** Existing ANSI output to stdout, unchanged.

---

## Track 2: `feat/config-url`

Remote config fetch via `--config https://...`

**Fetch location: cmd layer.** URL detection and fetching happens in `cmd/main.go` before calling `config.LoadAll`. For each `--config` value that starts with `https://`, the cmd layer fetches the content, writes it to a temp file, and substitutes the temp file path in the argument list passed to `LoadAll`. This keeps the `config` package free of `net/http` dependency. Temp files are cleaned up after `LoadAll` returns.

**Guardrails (enforced in cmd layer before writing temp file):**
1. HTTPS only — `http://` rejected with clear error
2. 1MB size cap — response body over 1MB rejected
3. 15-second fetch timeout

**Merge order:** Argument position determines priority — last-listed wins on conflicts, whether source is local or remote. No special-casing for remote vs local.

**UpgradeIfInstalled ratchet:** Remote configs processed via the same `mergeSettings` call as local files — existing one-way ratchet applies automatically.

**Trust model:** HTTPS + size cap only. No checksum or host allowlist. Deliberate scope decision for personal tool. Revisit if ever distributed for multi-user use.

**Dry-run:** Remote config fetched, written to temp file, parsed; no installs execute.

---

## Track 3: `feat/uninstall`

Config-scoped only. CLI first, then GUI.

### Schema Addition

Optional `uninstall_cmd` string on `config.Command`:

```json
{
  "id": "npm-globals",
  "cmd": "npm install -g eslint prettier typescript",
  "uninstall_cmd": "npm uninstall -g eslint prettier typescript"
}
```

**Scrape-download precedence:** `ScrapeURL != ""` check routes to Tier 4 skip path regardless of `uninstall_cmd` presence.

### Uninstall Behavior Per Tier

| Tier | Mechanism | If unavailable |
|---|---|---|
| Winget (T1) | `winget uninstall -e --id <ID>` | Skip if not detected as installed |
| Commands (T2) | `uninstall_cmd` via `runShellWithTimeout`; item's `TimeoutSeconds` or global default | Skip with notice if absent |
| Extensions (T3, force) | Remove specific numbered value matching `ExtensionID`; renumber remaining values — non-atomic, best-effort (see note) | Skip with notice if value not found |
| Extensions (T3, url) | No-op — nothing was installed programmatically | Skip with notice |
| Scrape-download (T4) | Always skipped | Notice logged; user removes manually via Windows Settings |

**Extension registry renumbering:** Non-atomic. Delete the matching value, then read remaining values and rewrite them as 1, 2, 3... The gap between delete and renumber is accepted for this personal tool. No locking is added. Parent key is never deleted.

**State:** Successful uninstall removes item from `state.Succeeded`. Failed uninstall leaves it in `state.Succeeded`.

### Latent Bug Fix (wired in this track)

The existing `App.StartInstall` does not call `r.SetPauseResponse`, leaving the runner's consecutive-failure prompt path to block on nil channel in GUI mode. This track fixes the bug for both install and uninstall by wiring `SetPauseResponse` in both `StartInstall` and the new `StartUninstall` App binding.

### CLI Subcommand

```
ktuluekit uninstall [--only id1,id2] [--profile Name] [--exclude id1] [--dry-run]
```

Mirrors install flags. Same pre-flight checks (admin, winget available).

**Confirmation gate:**
- Prints summary of items to be removed
- Reads line from stdin; accepts `yes`, `Yes`, `YES` (case-insensitive exact match)
- Anything else — including empty Enter — cancels with no action
- Piped stdin supported: `echo yes | ktuluekit uninstall` works correctly
- When stdin is piped (non-TTY), consecutive-failure prompts that would normally block for user input are bypassed — uninstall continues automatically. This is the correct behavior for scripted invocations.
- `--dry-run` bypasses confirmation gate entirely
- No `--force` flag

### GUI — Install / Uninstall Tabs

**Tab bar** at top of `SelectionScreen.svelte`:
```
[ Install ] │ [ Uninstall ]
```

**Tab locking:** Both tabs non-clickable and visually muted during any active operation.

**Uninstall tab — status scan:**
- Goroutine on first open; UI shows loading state ("Scanning installed items...") during scan
- Scans all config items via existing `detector` package
- Each check command uses item's `TimeoutSeconds` or global default — same as `status` subcommand
- Timed-out items excluded from uninstall list (treated as unknown)
- Shows only `StatusInstalled` items with checkboxes and red-tinted row accents

**Action:** "Uninstall Selected". Confirmation dialog before any run.

**Shared screens:** Progress and Summary reused. Uninstall context relabels: Removed / Not Found / Skipped / Failed.

**Styling:** Tab bar blue accent throughout. Red tint on item rows in uninstall mode only.

---

## Track 4: `feat/unit-tests`

**Responsibility:**
- Tracks 1–3 each ship with tests for their own new code in the same feature branch
- Track 4 covers **pre-sprint existing code gaps only** — runs in parallel with Tracks 1–3

### Coverage Targets (pre-sprint code)

| Package | Specific gaps |
|---|---|
| `internal/config` | Multi-config merge order, profile lookup (found/not found), invalid field combinations |
| `internal/installer` | Scrape URL pattern matching edge cases, check command output parsing |
| `internal/detector` | Timeout vs not-found distinction in check command result handling |
| `cmd` | Flag conflict validation edge cases, error exit paths |

**`cmd` tests:** `package main` internal test files, following `cmd/filter_test.go` pattern.

No minimum coverage threshold. Standard Go `testing`, table-driven tests.

---

## Track 5: `feat/impeccable-ui`

Runs after all feature branches (Tracks 1–4) merged to main.

**Skill sequence per screen:** `normalize` → `distill` → `polish` → `colorize` → `animate`

**Targets:** SelectionScreen (tab bar, accordion, rows, buttons), ProgressScreen (feed, elapsed, reboot dialog), SummaryScreen (headers, badges, list), CategoryAccordion, ItemRow (blue/red variants), ProgressItem.

**What doesn't change:** Wails bindings, Go backend, event contracts. Purely visual.

---

## Branch Ship Order

| Order | Branch | Contents | Parallel? |
|---|---|---|---|
| 1 | `feat/cli-polish` | `--profile`, post-install hooks, install summary export formats | No |
| 2 | `feat/config-url` | Remote config fetch (cmd-layer URL detection + temp file) | No |
| 3 | `feat/uninstall` | Uninstall logic + latent bug fix + CLI + GUI tabs | No |
| 4 | `feat/unit-tests` | Pre-sprint existing code coverage | Yes — alongside 1–3 |
| 5 | `feat/impeccable-ui` | Impeccable design pass | No — after all above |

**Before each PR:** `/security-review`. **Merge:** Squash to main.

---

## Success Criteria

- [ ] All five branches merged to main
- [ ] `PostInstall` field added to both `config.Package` and `config.Command`
- [ ] Post-install hooks fire on `StatusInstalled`/`StatusUpgraded`; skip on all other statuses including `StatusReboot`
- [ ] `--profile` on install/status/export resolves to `--only` filter; T4 items silently omitted from `export --profile`
- [ ] `--output-format json|md` on install only; `progressWriter` injected into Reporter; progress on stderr; existing stderr output unaffected
- [ ] Remote config: HTTPS only, temp-file pattern, 1MB cap, 15s timeout, argument-position merge order
- [ ] `ktuluekit uninstall --dry-run` previews without confirmation or system changes
- [ ] Uninstall confirmation gate: case-insensitive `yes`; piped stdin supported; non-TTY bypasses consecutive-failure prompts
- [ ] Successful uninstall removes item from `state.Succeeded`
- [ ] Extension uninstall removes specific registry value and renumbers remaining (non-atomic, accepted)
- [ ] Scrape-download items skip uninstall even if `uninstall_cmd` present
- [ ] Latent `SetPauseResponse` bug fixed in both `StartInstall` and new `StartUninstall`
- [ ] GUI tabs locked during active operations; uninstall scan in goroutine with loading state
- [ ] Tracks 1–3 each include unit tests for their new code
- [ ] Track 4 covers pre-sprint gaps; `cmd` tests use `package main` internal pattern
- [ ] Impeccable pass on all Svelte screens after all features land
- [ ] `/security-review` passed on each PR
- [ ] TODO.md fully cleared
