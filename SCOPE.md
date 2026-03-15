# Scope Contract
**Task:** CLI Polish — post-install hooks, --profile flag, --output-format json|md
**Plan:** `docs/superpowers/plans/2026-03-14-cli-polish.md`
**Date:** 2026-03-14
**Status:** CLOSED — 0 scope changes logged

## In Scope

### Files
| File | Change |
|------|--------|
| `internal/config/schema.go` | Add `PostInstall string` to `Package` and `Command` |
| `internal/config/profile.go` | **New file** — `LookupProfile()` |
| `internal/config/profile_test.go` | **New file** — tests for `LookupProfile()` |
| `internal/config/validate_test.go` | Add `TestPostInstallFieldRoundTrip` |
| `internal/reporter/reporter.go` | Add `progressWriter io.Writer`; update `New()` signature; route all stdout through it; add `SummaryJSON()` and `SummaryMD()` |
| `internal/reporter/reporter_test.go` | Add `TestProgressWriter*`, `TestSummaryJSON`, `TestSummaryMD` |
| `internal/installer/command.go` | Add exported `RunHook()` wrapper |
| `internal/runner/runner.go` | Add `runPostInstall()` method; wire after `StatusInstalled`/`StatusUpgraded` |
| `internal/runner/runner_test.go` | Add `TestRunPostInstall_*` tests |
| `cmd/main.go` | Add `outputFormat`, `profileName` vars; flags; `outputFormatError()`, `profileFlagsError()`; update `runInstall()`; update `reporter.New()` call |
| `cmd/helpers.go` | **New file** — `filterItemsByIDs()`, `filterConfigByIDs()` |
| `cmd/status.go` | Wire `--profile` filter before `detector.FlattenItems` |
| `cmd/export.go` | Wire `--profile` filter before `exporter.Export` |
| `cmd/filter_test.go` | Add `TestOutputFormatFlags*`, `TestProfileFlags*` |
| `cmd/status_test.go` | Add `TestFilterItemsByIDs`, `TestFilterConfigByIDs` |

### Features / Acceptance Criteria
- `post_install` field on `Package` and `Command` — JSON round-trip passes
- `progressWriter io.Writer` on `Reporter` — routes all live output; nil defaults to `os.Stdout`
- `Reporter.SummaryJSON()` — returns JSON bytes + logs to file
- `Reporter.SummaryMD()` — returns Markdown string + logs to file
- `--output-format json|md` flag on install — progress to stderr, summary to stdout
- `config.LookupProfile()` — case-sensitive, returns error if not found
- `--profile <name>` flag — resolves to `onlyIDs`; mutually exclusive with `--only`
- `--profile` wired to status and export subcommands
- `installer.RunHook()` exported wrapper
- `runPostInstall()` in runner — warning-only on failure; no-op in dry-run
- All tests pass; `go build ./...` clean

### Explicit Boundaries
- **No parallel install** — out of scope per project constraint
- **Only `reporter.go` stdout writes** rerouted in Task 2; runner/cmd direct `fmt.Printf` calls are NOT touched in that task
- `--profile` uses `PersistentFlags` so status/export inherit it — no changes to subcommand flag registration beyond wiring
- No schema validation changes beyond adding the new field
- No runner refactor beyond the `runPostInstall` method + call sites

## Out of Scope
- Centralizing design tokens to `tokens.css` (noted in CLAUDE.md as future work)
- Any UI/Svelte frontend changes
- Parallel install mode
- Adding `--profile` to any other subcommand not listed above
- Rerouting `fmt.Printf` in runner/cmd for output-format (only the Reporter's internal writes are rerouted)
- Changes to `exporter` package internals

# Scope Change Log
| # | Category | What | Why | Decision | Outcome |
|---|----------|------|-----|----------|---------|

# Follow-up Tasks
_(none yet)_
