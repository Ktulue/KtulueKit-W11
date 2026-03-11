# CLI Features Design — validate, list, --only/--exclude, consecutive-failure pause

**Date:** 2026-03-11
**Status:** Approved

---

## Overview

Four additions to the KtulueKit CLI:

1. `validate` subcommand — full config cross-reference validation
2. `list` subcommand — phase/tier grouped item table
3. `--only` / `--exclude` flags — targeted install filtering
4. Consecutive-failure pause — interactive pause after 3 back-to-back failures in the runner

---

## 1. `validate` Subcommand

### Purpose

Parse and fully validate the config without running any installs. Useful when editing `ktuluekit.json` by hand or in CI.

### Relationship to existing loader validation

`loader.go` currently has a private `validate()` called inside `LoadAll` that does fail-fast checks. **The private `validate()` is deleted.** `Validate()` replaces it.

**Critical architecture point:** `LoadAll` does **not** call `Validate()` internally. `LoadAll`'s job is JSON parsing, merging, and applying defaults only. The install command calls `LoadAll` then calls `Validate()` itself, returning `errs[0]` if any errors are found (preserving fail-fast behavior for the install path). The `validate` subcommand calls `LoadAll` then calls `Validate()` directly and reports all errors — this is the only way the all-errors reporting mode is reachable.

### Architecture

New file `internal/config/validate.go` with:

```go
func Validate(cfg *Config) []ValidationError
```

`ValidationError` has `Field string` and `Message string`. The function collects **all** errors and returns them — it does not fail fast.

The subcommand registered in `cmd/main.go`:
1. Calls `config.LoadAll` (parse + defaults only, no validation)
2. Calls `config.Validate(cfg)` to get all errors
3. Prints all errors and exits 1 if any; exits 0 if clean

The root install command (`runInstall`):
1. Calls `config.LoadAll`
2. Calls `config.Validate(cfg)`; if `len(errs) > 0`, returns `errs[0]` (fail-fast, same behavior as before)

### Checks (complete list — replaces all checks previously in private validate)

1. **Top-level required fields** — `cfg.Version` must be non-empty; `cfg.Metadata.Name` must be non-empty.
2. **Required fields — all item tiers** — every Package, Command, and Extension must have non-empty `id`, `name`, and `phase >= 1`.
3. **Required fields — Commands** — `check` and `command` must be non-empty.
4. **Required fields — Extensions** — `extension_id` must be non-empty and exactly 32 characters.
5. **Duplicate IDs** — IDs are shared across all three tiers. Any duplicate is an error.
6. **`depends_on` references** — Only `Command` has a `DependsOn []string` field; `Package` and `Extension` do not. For each Command, every ID listed in `depends_on` must exist in the config (packages or commands). This is a config-only check; runtime state (`state.Succeeded`) is not consulted.
7. **Profile `ids` references** — every ID in a profile's `ids` array must exist in the config (packages, commands, or extensions).

### Output

```
Validating config: ktuluekit.json
  ERROR  [top-level]             version is required
  ERROR  packages[3].id          duplicate ID "Git.Git"
  ERROR  commands[7].depends_on  unknown ID "nvm" (not in packages or commands)
  ERROR  profiles[0].ids         unknown ID "does-not-exist"

4 error(s) found. Fix the above before running.
```

On success:
```
Validating config: ktuluekit.json
  OK — no errors found (52 packages + 31 commands + 4 extensions = 87 items validated)
```

Item count is `len(Packages) + len(Commands) + len(Extensions)`. Profiles are not counted as items.

### Error handling

If `config.LoadAll` fails (malformed JSON), the parse error is printed and the command exits 1 before reaching `Validate`.

---

## 2. `list` Subcommand

### Purpose

Dump all configured items grouped by phase and tier. Quick reference without simulating a dry-run.

### Architecture

Subcommand registered in `cmd/main.go`. Calls `config.LoadAll` then formats the output. No runner, no state, no OS interaction.

Respects the existing `--config` persistent flag.

### Output format

```
── Phase 1 ──────────────────────────────────────
  [winget]     Git.Git                     Git
  [winget]     Microsoft.WindowsTerminal   Windows Terminal

── Phase 3 ──────────────────────────────────────
  [winget]     Valve.Steam                 Steam
  [command]    wsl2-ubuntu                 WSL2 + Ubuntu

── Phase 4 ──────────────────────────────────────
  [command]    claude-code                 Claude Code CLI
  [extension]  react-devtools              React DevTools

Total: 52 winget  |  31 commands  |  4 extensions
```

Columns: tier (padded), ID (padded), name. The phase separator format string (`"── Phase %d ──..."`) is duplicated locally in the `list` handler — it is a simple constant and does not warrant a shared package. The `list` subcommand does not import `runner`.

---

## 3. `--only` and `--exclude` Flags

### Purpose

- `--only <ids>` — install only the listed items, skip everything else
- `--exclude <ids>` — install everything except the listed items

### Syntax

Comma-separated IDs: `--only Git.Git,nodejs,claude-code`

### Constraints

- Mutually exclusive. If both are provided, the command returns an error before any installs run.
- Applied to the root install command only (not `validate` or `list`).
- Composable with `--resume-phase` — phase filter applies first, then ID filter. `SetSelectedIDs` already integrates with `countItemsFromPhase` and the phase-skip logic in `Run()`, so this is correct with no extra handling.

### Unknown ID warnings

Unknown IDs in `--only` or `--exclude` print a yellow warning but do not abort. The check is symmetric: both flags scan their ID lists against the full config before calling `SetSelectedIDs`, and warn on any ID not found. For `--exclude`, an unknown ID is a no-op subtraction but the warning is still emitted.

### Pre-run summary interaction

`printPreRunSummary` currently has three early-return guards (dry-run, resume phase > 1, GUI mode). A fourth guard is added: `r.selectedIDs != nil`. Guard order in `printPreRunSummary`:

1. `r.dryRun` → return false
2. `r.resumePhase > 1` → return false
3. `r.onProgress != nil` → return false
4. `r.selectedIDs != nil` → return false *(new)*

This avoids showing misleading totals for the full config when only a subset will be installed.

### Architecture

Two new flags (`only`, `exclude`) declared in `cmd/main.go`. After config load and before constructing the runner:

- `--only`: split by comma, warn on unknowns, call `runner.SetSelectedIDs(ids)` directly
- `--exclude`: build full ID set from all packages + commands + extensions, warn on unknown excluded IDs, remove excluded IDs, call `runner.SetSelectedIDs(remaining)`

---

## 4. Consecutive-Failure Pause

### Purpose

When 3 or more installs fail or are dependency-skipped back-to-back, pause the run and prompt the user to investigate before continuing.

### What counts as a failure

Both `StatusFailed` and `StatusSkipped` increment the counter. A dependency cascade — where one upstream failure causes multiple downstream items to be skipped — is exactly the scenario this feature is meant to surface.

**State-aware skips** (items already succeeded in a previous run, handled by the `r.state.Succeeded[id]` early-continue before any install attempt) do **not** affect the counter. They are treated as neutral — neither incrementing nor resetting. The counter only tracks freshly-attempted items.

Any other fresh-attempt result (installed, upgraded, already) resets the counter to 0.

### Counter scope

`consecutiveFails int` is a field on `Runner`. It persists across all three tier-processing calls (`runPackagesInPhase`, `runCommandsInPhase`, `runExtensionsInPhase`) and across all phases. It is only reset on a non-failure fresh-attempt result or after a pause.

### Pause behavior — CLI mode

Prints a yellow warning banner and blocks on stdin. After the user presses Enter, `consecutiveFails` resets to 0 and the run continues. Ctrl+C aborts the process (standard signal behavior).

```
  ⚠️  3 consecutive failures. Something may be wrong.
  ──────────────────────────────────────────────────
  Press Enter to continue, or Ctrl+C to abort and investigate.
```

### Pause behavior — GUI mode

Emits `ProgressEvent{Status: "paused"}` and blocks on `pauseResponse chan bool`.

**Channel lifecycle:** `pauseResponse` is a field on `Runner` set by a new `SetPauseResponse(ch chan bool)` method, called by the app layer before `Run()` — same pattern as `SetRebootResponse`. Unlike the reboot channel (one-shot, set to nil after use), `pauseResponse` is **not** set to nil after each use because pauses can occur multiple times in a run. The app sends `true` on the channel each time the user confirms "continue." If `pauseResponse` is nil when a pause fires (e.g., no GUI wired), the runner falls through to CLI behavior — matching the nil-check pattern used by `rebootResponse` and `onProgress`.

The threshold (3) is hardcoded. No config field needed.

### Testability

`promptConsecutiveFailures` uses `os.Stdin` in CLI mode (consistent with `promptManualInstall` and `promptReboot`). To test the counter logic without hanging, `Runner` gets an optional `onPause func()` field set via `SetOnPause(fn func())`. When non-nil, `promptConsecutiveFailures` calls `onPause()` and returns immediately instead of blocking on stdin. Production code leaves it nil. Tests in `runner_test.go` (same package: `package runner`) set this hook directly on the struct field.

---

## Files Changed

| File | Change |
|---|---|
| `internal/config/validate.go` | New — public `Validate()` and `ValidationError` |
| `internal/config/loader.go` | Remove private `validate()`; callers of `LoadAll` now call `Validate()` explicitly |
| `internal/runner/runner.go` | Add `consecutiveFails`, `pauseResponse chan bool`, `onPause func()`, `SetPauseResponse()`, `SetOnPause()`, `promptConsecutiveFailures()` |
| `cmd/main.go` | Add `validate` subcommand, `list` subcommand, `--only`/`--exclude` flags; `runInstall` calls `Validate()` after `LoadAll`; add `selectedIDs != nil` guard to pre-run summary |

---

## Testing

**`internal/config/validate_test.go`** — unit tests for each check:
- Missing `version`, `metadata.name`
- Missing `id`/`name`/`phase` on each tier
- Missing `check`/`command` on Commands
- Short/missing `extension_id` on Extensions
- Duplicate IDs across tiers
- Unknown `depends_on` ID
- Unknown profile `ids` entry
- Clean config returns empty slice

**`internal/runner/runner_test.go`** — consecutive-failure counter tests:
- Set `onPause` hook; feed 3 `StatusFailed` results → verify hook called once, counter resets to 0
- Feed 2 `StatusFailed` + 1 `StatusInstalled` → verify hook not called
- Feed 3 `StatusSkipped` → verify hook called (skips count)
- State-aware-skip path fires → verify counter unchanged (neither increment nor reset)

**`cmd` package tests** — `validate` and `list` subcommands tested by calling the underlying config functions directly (consistent with `cmd/status_test.go`, which calls `config.LoadAll` rather than invoking cobra). Cobra wiring (flag parsing, command registration) is not separately tested.

**`--only` / `--exclude`** — tested via cmd package: verify filtered ID sets reach the runner, mutual exclusion error, unknown ID warning.
