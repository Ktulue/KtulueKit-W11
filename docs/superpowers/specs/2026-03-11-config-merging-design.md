# Config Merging Design Spec

**Date:** 2026-03-11
**Feature:** `--config base.json --config extras.json` layered config support
**Status:** Approved

---

## Goal

Allow multiple config files to be layered together via repeated `--config` flags. Separates a shared base config from per-machine overrides without duplicating the full file.

## Architecture

### CLI Change

`--config` changes from `StringVarP` (single string) to `StringArrayVarP` (repeatable flag).

```
ktuluekit --config base.json --config machine.json
ktuluekit -c base.json -c machine.json
```

Single-flag usage continues to work identically (`--config ktuluekit.json`). Default remains `ktuluekit.json` when no flag is provided.

### New Function: `config.LoadAll(paths []string)`

```go
func LoadAll(paths []string) (*Config, error)
```

- Loads each file in order
- Merges left-to-right (later configs override earlier)
- Runs existing `validate()` and `applyDefaults()` on the merged result
- Returns a single `*Config` ready for use

`Load(path string)` becomes a thin wrapper around `LoadAll([]string{path})` — all existing call sites unchanged.

### Merge Semantics (last-wins per field/ID)

**Settings** — later config fields overwrite earlier fields only where the value is non-zero. A field absent (or zero-valued) in `extras.json` leaves the base config value intact.

**Packages / Commands / Extensions** — merged into an ordered map keyed by `ID`. Later configs overwrite same-ID entries. Final slice order: first-seen position is preserved; an override replaces in-place (does not move to end).

**Profiles** — merged by `Name` (last-wins on name collision). Combined list is available in both CLI dry-run output and GUI profile dropdown.

**Metadata** — first config is authoritative. Later configs' metadata is ignored.

### Error Cases

| Condition | Behaviour |
|---|---|
| No `--config` flags | Use default `ktuluekit.json` |
| File not found | Hard error: "config file not found: extras.json" |
| Duplicate ID across configs | Last-wins (no error) |
| Invalid JSON in any file | Hard error before merge begins |
| Merged result fails validation | Hard error with field-level message |

## File Structure

| File | Change |
|---|---|
| `internal/config/loader.go` | Add `LoadAll()`, refactor `Load()` to call it |
| `cmd/main.go` | Change `StringVarP` → `StringArrayVarP`; pass slice to `LoadAll` |
| `internal/config/loader_test.go` | New file: tests for merge behaviour |

No other files change. Runner, reporter, installer, GUI, and status subcommand all consume `*Config` — the merge is transparent to them.

## Testing

- Single path: identical output to current `Load()`
- Two configs, no overlap: all items from both present in correct order
- Two configs, overlapping ID: later config's version wins
- Two configs, overlapping Settings field: later config's value wins; unset fields preserve base value
- Two configs, overlapping Profile name: later config's profile wins
- Missing file: returns error naming the missing path
- Invalid JSON in second file: returns error before any merge
- Merged result with duplicate ID introduced by bad extras: validated and caught

## Out of Scope

- Config from URL (`--config https://...`) — separate feature
- More than two configs (supported naturally by the slice approach, no special casing needed)
- Merge strategy flags (`--merge-strategy`) — last-wins is sufficient for the stated use case
