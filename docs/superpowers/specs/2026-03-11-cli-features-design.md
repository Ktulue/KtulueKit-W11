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

### Architecture

Validation logic lives in a new `internal/config/validate.go` as a pure function:

```go
func Validate(cfg *Config) []ValidationError
```

`ValidationError` has `Field string` and `Message string`. The function collects **all** errors and returns them — it does not fail fast.

The subcommand is registered in `cmd/main.go` and calls `config.LoadAll` then `config.Validate`. It prints all errors and exits 1 if any are found, exits 0 if clean.

### Checks

1. **Required fields** — every Package, Command, and Extension must have non-empty `id`, `name`, and non-zero `phase`.
2. **Duplicate IDs** — IDs are shared across all three tiers. Any duplicate is an error.
3. **`depends_on` references** — every ID listed in a `depends_on` array must exist in the config (packages or commands).
4. **Profile `ids` references** — every ID in a profile's `ids` array must exist in the config (packages, commands, or extensions).

### Output

```
Validating config: ktuluekit.json
  ERROR  packages[3].id        duplicate ID "Git.Git"
  ERROR  commands[7].depends_on  unknown ID "nvm" (not in packages or commands)
  ERROR  profiles[0].ids       unknown ID "does-not-exist"

3 error(s) found. Fix the above before running.
```

On success:
```
Validating config: ktuluekit.json
  OK — no errors found (87 items validated)
```

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

Columns: tier (padded), ID (padded), name. Uses existing ANSI phase separator style for consistency.

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
- Composable with `--resume-phase` and `--phase` (future) — phase filter applies first, then ID filter.

### Architecture

Two new flags (`only`, `exclude`) declared in `cmd/main.go`. After config load and before constructing the runner:

- `--only`: split by comma → call `runner.SetSelectedIDs(ids)` directly (the method already exists for GUI mode)
- `--exclude`: build full ID set from all packages + commands + extensions, remove excluded IDs, call `runner.SetSelectedIDs(remaining)`

Unknown IDs in `--only` or `--exclude` print a yellow warning but do not abort (the ID may be intentionally absent from the current config subset).

---

## 4. Consecutive-Failure Pause

### Purpose

When 3 or more installs fail back-to-back, pause the run and prompt the user to investigate before continuing. Prevents blindly hammering through a broken environment.

### Architecture

Add `consecutiveFails int` field to `Runner`. After each install result across all three tiers (packages, commands, extensions):

- Non-failure result (installed, upgraded, already, skipped) → reset `consecutiveFails` to 0
- `StatusFailed` → increment `consecutiveFails`
- When `consecutiveFails == 3` → call `r.promptConsecutiveFailures()`

`promptConsecutiveFailures` in CLI mode prints a yellow warning banner and blocks on stdin (Enter to continue, Ctrl+C to abort). In GUI mode it emits a `ProgressEvent{Status: "paused"}` and blocks on a new `pauseResponse chan bool` — same channel pattern as the reboot response.

The threshold (3) is hardcoded. No config field needed.

### CLI output

```
  ⚠️  3 consecutive failures. Something may be wrong.
  ──────────────────────────────────────────────────
  Press Enter to continue, or Ctrl+C to abort and investigate.
```

After the user presses Enter, `consecutiveFails` resets to 0 and the run continues.

---

## Files Changed

| File | Change |
|---|---|
| `internal/config/validate.go` | New — `Validate()` and `ValidationError` |
| `internal/runner/runner.go` | Add `consecutiveFails`, `pauseResponse`, `promptConsecutiveFailures()` |
| `cmd/main.go` | Add `validate` subcommand, `list` subcommand, `--only`/`--exclude` flags |

---

## Testing

- `internal/config/validate.go` — unit tests for each check: missing fields, duplicate IDs, bad `depends_on`, bad profile IDs, clean config returns no errors
- `--only` / `--exclude` — tested via existing runner test infrastructure with filtered ID sets
- `validate` and `list` subcommands — tested via `cmd` package tests (already have test coverage)
- Consecutive-failure pause — unit test: mock 3 failing results in sequence, verify `promptConsecutiveFailures` is called
