# Scope Contract
**Task:** Add config-scoped uninstall (CLI + GUI tabs) | **Plan:** `docs/superpowers/plans/2026-03-14-uninstall.md` | **Date:** 2026-03-14 | **Status:** ACTIVE

## In Scope

- **Files:**
  - `internal/config/schema.go` — add `UninstallCmd string` to `Command`
  - `internal/config/schema_test.go` — 2 round-trip tests
  - `internal/state/state.go` — add `DeleteSucceeded(id string)`
  - `internal/state/state_test.go` — 2 tests
  - `internal/installer/winget.go` — add `UninstallPackage()`
  - `internal/installer/winget_test.go` — dry-run and skip tests
  - `internal/installer/command.go` — add `RunUninstallCommand()`
  - `internal/installer/command_test.go` — 3 tests
  - `internal/installer/extension.go` — add `UninstallExtension()`; add `"sort"` import
  - `internal/installer/extension_test.go` — 2 tests
  - `internal/runner/runner.go` — add `RunUninstall()` method
  - `cmd/uninstall.go` — new file, `uninstall` cobra subcommand
  - `cmd/uninstall_test.go` — 4 tests for `buildUninstallList`
  - `cmd/main.go` — add `root.AddCommand(uninstallCmd)` (plan says "no changes" but local `root` var requires it)
  - `cmd/helpers.go` — add `resolveFilter`, `parseIDList`, `buildSelectedMap` helpers (plan provides inline fallbacks; helpers.go is their natural home)
  - `app.go` — fix `SetPauseResponse` latent bug in `StartInstall`; add `StartUninstall()` and `GetInstalledItems()`
  - `frontend/src/screens/SelectionScreen.svelte` — Install/Uninstall tab bar, scan logic, uninstall trigger
  - `frontend/src/components/ItemRow.svelte` — add `mode` prop for accent color switching
  - `TODO.md` — mark uninstall items done

- **Features / Criteria:**
  - `UninstallCmd` schema field on `config.Command`
  - `state.DeleteSucceeded()` removes ID and persists
  - `UninstallPackage`: winget uninstall, dry-run, check-based skip
  - `RunUninstallCommand`: T4/scrape-URL skip, no-uninstall-cmd skip, dry-run, shell run
  - `UninstallExtension`: url-mode skip, force-mode registry removal + renumber
  - `runner.RunUninstall()`: flat iteration (not phase-based), per-tier dispatch, state clearing on success
  - `ktuluekit uninstall` CLI with `--only`, `--exclude`, `--profile`, `--dry-run`, confirmation gate, non-TTY auto-continue
  - `App.StartUninstall()` + `App.GetInstalledItems()` Wails bindings
  - Install/Uninstall tab bar in SelectionScreen

- **Explicit Boundaries (plan corrections pre-approved):**
  - `reporter.New` calls must include `io.Writer` second arg (plan omits it; already fixed in codebase)
  - `rep.Summary()` not `rep.PrintSummary()` (plan typo)
  - Package-level var is `profileName` not `profileFlag`
  - `rootCmd.AddCommand` in `init()` won't work — use `root.AddCommand(uninstallCmd)` in `main()` instead
  - `config.Load` (single file) used in app.go — matches plan's `StartUninstall` code

## Out of Scope
- Parallel uninstall
- Uninstall for T4 (scrape-download) items — always skipped by design
- Atomic registry renumber for extension uninstall — non-atomic/best-effort by design
- Any other CLI subcommands or flags not listed above
- Any refactoring of existing install code

# Scope Change Log
| # | Category | What | Why | Decision | Outcome |
|---|----------|------|-----|----------|---------|
| 1 | user-expansion | Add rustup-init.exe to ktuluekit.json config | User requested during uninstall track | Defer | Follow-up #1 |

# Follow-up Tasks
- [ ] Add `rustup-init.exe` as a scrape-download Command entry in `ktuluekit.json` — download URL from https://rustup.rs/ — scope change #1
