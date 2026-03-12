# Export / Scan Mode Design

**Date:** 2026-03-12
**Status:** Approved
**Tool:** KtulueKit-W11

---

## Overview

`ktuluekit export` scans the current machine against a reference config, collects every item whose `check` command passes, and writes a replay-ready `ktuluekit-snapshot.json`. The snapshot is a valid standalone W11 config (replay on a new machine) and the handoff artifact that KtulueKit-Migration reads to determine which apps are present and therefore which migration items are relevant.

---

## Command Interface

```
ktuluekit export [--config <path>] [--output <path>] [--fast]
```

| Flag | Default | Description |
|---|---|---|
| `--config` | `ktuluekit.json` in cwd | Reference config to scan against |
| `--output` | `ktuluekit-snapshot.json` in cwd | Output path for the snapshot |
| `--fast` | off | Skip check commands; derive installed list from state file only |

Both defaults are relative to the current working directory, not the binary location, to avoid permission issues when the binary is in a system PATH location.

### Behaviour

1. Load and validate the reference config.
2. For each item in `packages[]` and `commands[]`: run its `check` command.
   - Exit 0 → **installed** (included in output).
   - Exit non-zero → **absent** (silently omitted).
   - Timeout after 10 seconds → **absent** + warning: `[warn] <name>: check timed out, treated as absent`.
3. `extensions[]` have no `check` field and cannot be probed in check mode. All extensions are omitted from check-mode output.
4. Write the snapshot JSON to `--output`.
5. Print summary: items checked, items included, output path.

`--fast` replaces steps 2–3: `cmd/export.go` checks whether the state file exists at `%LOCALAPPDATA%\KtulueKit\state.json` (via `os.Stat`) before calling `state.Load()`. If the file is absent, export exits with a fatal error. If the file exists, `state.Load()` is called and items where `state.Succeeded[id] == true` are included — packages, commands, and extensions alike. (Extensions are written to `state.Succeeded` by the runner on successful install, so they appear here if KtulueKit installed them.)

The `os.Stat` pre-check in `cmd/export.go` is necessary because `state.Load()` silently returns an empty state on file-not-found rather than erroring — this is correct behavior for install runs but must not silently produce an empty snapshot in fast mode.

---

## Output Format

The snapshot follows the `ktuluekit.json` structure with one additional top-level key: `snapshot`. The `$schema` field is omitted to avoid broken relative schema references when `--output` points outside the project directory.

```json
{
  "version": "1.0",
  "snapshot": {
    "generated_at": "2026-03-12T14:30:00Z",
    "machine": "KLUTESSTREAMRIG",
    "source_config": "C:\\Users\\Ktulue\\KtulueKit-W11\\ktuluekit.json",
    "tool_version": "1.2.0",
    "mode": "check"
  },
  "metadata": { ... },
  "settings": { ... },
  "packages": [ ...installed only... ],
  "commands": [ ...installed only... ],
  "extensions": [ ...fast mode only, if in state... ],
  "profiles": [ ...filtered, see below... ]
}
```

- `snapshot.source_config` — absolute path to the reference config used (not a basename or relative path).
- `snapshot.tool_version` — populated via ldflags at build time (`-X main.Version=...`); falls back to `"dev"` if not set. Same mechanism used by the existing `version` subcommand.
- `snapshot.mode` — `"check"` or `"fast"`.

### Profile filtering

Each profile's `ids` array is filtered to retain only IDs present in the emitted `packages[]`, `commands[]`, and `extensions[]` arrays. State keys match the item `id` field: winget package ID for packages (e.g. `Git.Git`), slug for commands (e.g. `claude-code`), slug for extensions. Profiles with at least one remaining ID after filtering are emitted; profiles where all IDs were filtered out are omitted entirely. Partially populated profiles are valid and emitted as-is.

### Schema compatibility

The `snapshot` key is not in the current `ktuluekit.schema.json`. `schema/ktuluekit.schema.json` must be updated to add `snapshot` as an optional root-level property (with its sub-fields typed). This ensures a snapshot file does not fail `ktuluekit validate` if someone passes it. Note: `validate` is designed for config files; snapshot files are not a required input to `validate`, but the schema must not reject them.

---

## Architecture

### New: `cmd/export.go`

Cobra command wired into the existing command tree alongside `validate`, `list`, `status`.

Responsibilities:
1. Parse `--config`, `--output`, `--fast` flags; resolve cwd-relative defaults.
2. Load reference config via `config.LoadAll`.
3. If `--fast`: `os.Stat` the state file path; fatal if absent or corrupt. Call `state.Load()`.
4. Build `exporter.Options`, call `exporter.Export(cfg, opts)`.
5. Marshal `Result` to JSON and write to `--output`.
6. Print summary line.

`cmd/export.go` is not unit-tested directly; it is covered by integration tests that invoke the binary with known fixture configs.

### New: `internal/exporter/exporter.go`

```go
// CheckResult is returned by CheckFn to distinguish outcomes.
type CheckResult int

const (
    CheckInstalled CheckResult = iota
    CheckAbsent
    CheckTimedOut
)

type Options struct {
    Fast        bool
    SourceConfig string            // absolute path, written into snapshot metadata
    ToolVersion  string            // from ldflags; "dev" if empty
    Machine      string            // os.Hostname()
    State        *state.State      // non-nil only in fast mode; nil in check mode
    CheckFn      func(cmd string) CheckResult // injected; nil only in fast mode
}

type Result struct {
    Packages   []config.Package
    Commands   []config.Command
    Extensions []config.Extension
    Profiles   []config.Profile
    Snapshot   SnapshotMeta
    Checked    int  // total items probed (0 in fast mode)
    Included   int  // total items in output
}

type SnapshotMeta struct {
    GeneratedAt  string
    Machine      string
    SourceConfig string
    ToolVersion  string
    Mode         string // "check" | "fast"
}

func Export(cfg *config.Config, opts Options) (Result, error)
```

`Export` contains no file I/O and no flag parsing. `Result.Checked` and `Result.Included` carry the summary counts back to `cmd/export.go` for printing.

In production, `opts.CheckFn` wraps `internal/installer.RunCheck` with a 10-second `context.WithTimeout`. In tests, it is a mock. In fast mode, `opts.CheckFn` is nil and `opts.State` is populated.

### Check execution

The `detector` package already contains `runCheckSilent` (unexported, returns `bool`) and the `checkTimeoutSeconds = 15` constant. However, `runCheckSilent` cannot distinguish a timeout from a non-zero exit, so the exporter's timeout warning cannot be implemented against it directly.

As part of this feature, a new exported function is added to `internal/detector`:

```go
// RunCheckDetailed runs a check command and returns whether it was installed
// and whether it timed out (15-second timeout, matching checkTimeoutSeconds).
// timedOut implies installed==false.
func RunCheckDetailed(checkCmd string) (installed, timedOut bool)
```

This wraps the same `exec.CommandContext` logic as `runCheckSilent` but surfaces the `context.DeadlineExceeded` outcome as the second return value. The exporter's production `CheckFn` calls `detector.RunCheckDetailed` and maps the pair to `CheckInstalled`, `CheckAbsent`, or `CheckTimedOut`.

The timeout is 15 seconds per item, matching the existing `checkTimeoutSeconds` constant in `detector.go`.

### Fast mode

Uses `opts.State.Succeeded` map. Key is item `id` field — consistent with how `runner.go` calls `state.MarkSucceeded(pkg.ID)`, `state.MarkSucceeded(cmd.ID)`, and `state.MarkSucceeded(ext.ID)`.

---

## Migration Contract

KtulueKit-Migration reads `ktuluekit-snapshot.json` → `packages[]` + `commands[]` to build a set of installed app IDs. It uses this set to pre-filter migration items: if an app's ID is absent from the snapshot, its migration items are skipped or shown as optional in the GUI. Migration treats the snapshot as optional input — if absent, all migration items are shown.

No structural changes to the Migration schema are required.

---

## Error Handling

| Scenario | Behaviour |
|---|---|
| Reference config not found | Fatal: path shown, hint to pass `--config` |
| Reference config invalid | Fatal: list all validation failures |
| `check` exits non-zero | Item silently omitted |
| `check` times out (10s) | Item omitted + `[warn] <name>: check timed out, treated as absent` |
| State file missing (`--fast`) | Fatal: state file path shown; suggest running without `--fast` |
| State file corrupt (`--fast`) | Fatal: parse error shown; suggest running without `--fast` |
| Output path not writable | Fatal: path shown |

---

## Testing

Unit tests in `internal/exporter/exporter_test.go`. `CheckFn` is injected — no subprocesses spawned.

| Test case | Description |
|---|---|
| All installed | All items return CheckInstalled → all in output, all profiles retained |
| None installed | All items return CheckAbsent → empty packages/commands, all profiles omitted |
| Mixed | Some pass, some fail → correct subset; profiles filtered accordingly |
| Empty config | No packages, no commands → valid empty snapshot, Checked=0, Included=0 |
| Timeout warning | CheckFn returns CheckTimedOut → item absent, warning emitted |
| Fast mode — state present | Items in Succeeded → included; extensions included if in state |
| Fast mode — state missing | cmd returns fatal before Export() is called |
| Fast mode — state corrupt | cmd returns fatal before Export() is called |
| Profile — partial filter | Profile with 3 IDs, 1 passes → profile with 1 ID emitted |
| Profile — full filter | Profile with 2 IDs, 0 pass → profile omitted |

`cmd/export.go` summary output (items checked / included / path) is verified via integration tests using fixture configs.

---

## Out of Scope

- Generating a config from `winget list` without a reference config.
- Modifying the Migration schema.
- GUI integration (export is CLI-only).
- `--profile` flag on export (filter export to a named profile) — deferred.
- Auto-discovery of reference config outside cwd.
