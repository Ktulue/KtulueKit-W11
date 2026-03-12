# Export / Scan Mode Design

**Date:** 2026-03-12
**Status:** Approved
**Tool:** KtulueKit-W11

---

## Overview

`ktuluekit export` scans the current machine against a reference config, collects every item whose `check` command passes, and writes a replay-ready `ktuluekit-snapshot.json`. The snapshot is valid as a standalone W11 config (replay on a new machine) and serves as the handoff artifact that KtulueKit-Migration reads to determine which apps are present and therefore which migration items are relevant.

---

## Command Interface

```
ktuluekit export [--config <path>] [--output <path>] [--fast]
```

| Flag | Default | Description |
|---|---|---|
| `--config` | `ktuluekit.json` (beside binary) | Reference config to scan against |
| `--output` | `ktuluekit-snapshot.json` (beside binary) | Output path for the snapshot |
| `--fast` | off | Skip check commands; derive installed list from state file only |

### Behaviour
1. Load and validate the reference config.
2. For each item in `packages[]`, `commands[]`, and `extensions[]`: run its `check` command.
3. Items whose check exits 0 → **installed**; all others → **absent** (silently omitted from output).
4. Write `ktuluekit-snapshot.json` with only installed items, plus a `snapshot` metadata block.
5. Print a summary table: how many items checked, how many included, output path.

`--fast` replaces step 2–3 with a state file read (`%LOCALAPPDATA%\KtulueKit\state.json`, `Succeeded` map). Items not in the state file are omitted. Faster but only reflects what KtulueKit itself installed.

---

## Output Format

The snapshot is a valid `ktuluekit.json` with one additional top-level key: `snapshot`.

```json
{
  "$schema": "./schema/ktuluekit.schema.json",
  "version": "1.0",
  "snapshot": {
    "generated_at": "2026-03-12T14:30:00Z",
    "machine": "KLUTESSTREAMRIG",
    "source_config": "ktuluekit.json",
    "tool_version": "1.0",
    "mode": "check"
  },
  "metadata": { ... },
  "settings": { ... },
  "packages": [ ...installed only... ],
  "commands": [ ...installed only... ],
  "extensions": [ ...installed only... ],
  "profiles": [ ...filtered to installed IDs only... ]
}
```

`snapshot.mode` is `"check"` for a full scan or `"fast"` for a state-file-only export.

Profile `ids` arrays are filtered to only include IDs present in the emitted packages/commands/extensions. Empty profiles are omitted.

---

## Architecture

### New: `cmd/export.go`

`exportCmd` wired into the existing Cobra command tree (alongside `validate`, `list`, `status`).

Responsibilities:
- Parse `--config`, `--output`, `--fast` flags.
- Delegate to `internal/exporter/exporter.go`.
- Print summary and output path.

### New: `internal/exporter/exporter.go`

```go
type Result struct {
    Packages   []config.Package
    Commands   []config.Command
    Extensions []config.Extension
    Profiles   []config.Profile
    Snapshot   SnapshotMeta
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

`Export` is pure logic — no file I/O, no flag parsing. The cmd layer handles reading/writing files. This keeps the exporter unit-testable.

### Check execution

Reuses `internal/installer.RunCheck(cmd string) bool` (already exists for pre-flight checks). No new subprocess logic needed.

### Fast mode

Reads `internal/state.Load()` → `state.Succeeded` map. Items with `Succeeded[id] == true` are included.

---

## Migration Contract

KtulueKit-Migration reads `ktuluekit-snapshot.json` → `packages[]` + `commands[]` to build a set of installed app IDs. It uses this set to pre-filter its own migration items: if an app's winget ID or command ID is absent from the snapshot, its migration items are skipped (or shown as optional/greyed-out in the GUI).

No structural changes to the Migration schema are required. Migration treats the snapshot as an optional input — if absent, all migration items are shown.

---

## Error Handling

| Scenario | Behaviour |
|---|---|
| Reference config not found | Fatal error with path hint |
| Reference config invalid | Fatal error, list validation failures |
| `check` command times out | Item treated as absent; warning printed |
| State file missing (`--fast`) | Fatal: state file not found — run without `--fast` |
| Output path not writable | Fatal error with path shown |

Check commands run with a 10-second timeout per item (not the full `default_timeout_seconds` from settings — checks are lightweight).

---

## Testing

- Unit tests in `internal/exporter/exporter_test.go`.
- Test cases: all installed, none installed, mixed, empty config, fast mode with state file, fast mode missing state file.
- No OS subprocess calls in tests — `RunCheck` is injected as a function parameter (`checkFn func(string) bool`).

---

## Out of Scope

- Generating a config from `winget list` without a reference (no reference-free scan mode).
- Modifying the Migration schema.
- GUI integration (export is CLI-only for now).
- `--profile` flag on export (filter export to a named profile) — deferred.
