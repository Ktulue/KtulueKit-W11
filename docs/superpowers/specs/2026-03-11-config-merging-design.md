# Config Merging Design Spec

**Date:** 2026-03-11
**Feature:** `--config base.json --config extras.json` layered config support
**Status:** Approved

---

## Goal

Allow multiple config files to be layered together via repeated `--config` flags. Separates a shared base config from per-machine overrides without duplicating the full file.

## Architecture

### CLI Change

`--config` changes from `StringVarP` (single string) to `StringArrayVarP` (repeatable flag). It is a `PersistentFlag` on the root command, so all subcommands (`status`, `validate`, etc.) automatically receive the full list.

```
ktuluekit --config base.json --config machine.json
ktuluekit -c base.json -c machine.json
ktuluekit status --config base.json --config machine.json
```

Single-flag usage continues to work identically. Default remains `["ktuluekit.json"]` when no flag is provided.

### New Function: `config.LoadAll(paths []string)`

```go
func LoadAll(paths []string) (*Config, error)
```

- Reads each file as raw JSON (hard error on missing file or invalid JSON)
- Individual files are **not** required to be complete configs — only the merged result must pass validation
- Merges left-to-right (later configs override earlier) using the semantics below
- Runs `validate()` and `applyDefaults()` **exactly once** on the merged result
- Returns a single `*Config` ready for use

`Load(path string)` becomes a thin wrapper: `return LoadAll([]string{path})`. It must **not** call `validate()` or `applyDefaults()` independently — these run only inside `LoadAll` on the final merged result.

### Merge Semantics (last-wins per field/ID)

**Settings** — each Settings field is merged independently. A non-zero/non-empty value in a later config overwrites the earlier value. A zero or empty value in a later config is indistinguishable from an absent field and leaves the earlier value intact. **Intentional clearing (setting a field back to zero/empty via extras.json) is not supported.**

**Packages / Commands / Extensions** — three separate ordered maps keyed by `ID`. Overrides are same-tier only: a Package ID in extras cannot override a Command ID in base, and vice versa. Override preserves the first-seen position in the slice (the later config's data replaces the entry in-place, not at the end). Items with new IDs are appended after all base items.

**Profiles** — merged by `Name` (last-wins on name collision). Combined list appended in order.

**Metadata** — first config's metadata is authoritative. Later configs' metadata fields are ignored.

**Schema/Version** — first config's values are used. Later configs' schema/version fields are ignored.

### Error Cases

| Condition | Behaviour |
|---|---|
| No `--config` flags | Use default `["ktuluekit.json"]` |
| File not found | Hard error naming the missing file |
| Invalid JSON in any file | Hard error before merge begins |
| Cross-tier ID collision (Package ID same as Command ID, across any files) | Caught by `validate()` on merged result — hard error |
| Merged result fails validation (missing required fields, bad values) | Hard error with field-level message |
| Duplicate ID within the same tier across files | Last-wins (no error) |

## File Structure

| File | Change |
|---|---|
| `internal/config/loader.go` | Add `LoadAll()`, refactor `Load()` to thin wrapper |
| `internal/config/loader_test.go` | New file: merge behaviour tests |
| `cmd/main.go` | `StringVarP` → `StringArrayVarP`; pass slice to `LoadAll` |
| `cmd/status.go` | Update `runStatus` to call `LoadAll` with config slice (receives it via persistent flag) |

No other files change. Runner, reporter, installer, and GUI all consume `*Config` — the merge is transparent to them.

## Testing

- Single path: output identical to current `Load()`
- Two configs, no overlap: all items from both present in original order
- Two configs, Package ID overlap: later config's Package version wins, position preserved
- Two configs, Settings field overlap: later config's non-zero value wins; zero value in extras leaves base value
- Two configs, Profile name overlap: later config's profile definition wins
- Two configs, cross-tier ID collision (Package vs Command): merged result fails validation, error returned
- Three configs, middle config overridden by third: final value is from third config
- Missing file in list: error names the specific missing path
- Invalid JSON in second file: error before any merge occurs
- `status` subcommand with two `--config` flags: receives merged config correctly

## Out of Scope

- Config from URL (`--config https://...`) — separate feature
- Intentional field clearing via extras (zero/empty overwrites base) — not supported; not needed for the base+override use case
- Merge strategy flags — last-wins is sufficient
