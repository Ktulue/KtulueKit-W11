# Scope Contract
**Task:** Config URL — remote config fetch via --config https://
**Plan:** `docs/superpowers/plans/2026-03-14-config-url.md`
**Date:** 2026-03-14
**Status:** ACTIVE

## In Scope

### Files
| File | Change |
|------|--------|
| `cmd/main.go` | Add `resolveConfigPaths()`, `fetchToTemp()`, `httpClient` var, constants; wire into `runInstall()` |
| `cmd/status.go` | Wire `resolveConfigPaths` before `config.LoadAll` |
| `cmd/validate.go` | Wire `resolveConfigPaths` before `config.LoadAll` |
| `cmd/list.go` | Wire `resolveConfigPaths` before `config.LoadAll` |
| `cmd/export.go` | Wire `resolveConfigPaths` before `config.LoadAll` (uses local `paths` var) |
| `cmd/resolve_config_test.go` | **New file** — all tests for `resolveConfigPaths` and `fetchToTemp` |
| `TODO.md` | Mark remote config URL item done |

### Features / Acceptance Criteria
- `resolveConfigPaths(paths []string) (resolved []string, cleanup func(), err error)` in `cmd/main.go`
- `http://` URLs rejected immediately with clear error mentioning `https://`
- Local paths passed through unchanged; empty/nil input returns empty list
- `fetchToTemp(url string) (string, error)` — 15s timeout, 1 MiB cap, non-200 rejected
- `var httpClient = &http.Client{}` — tests can swap to trust test TLS cert
- `fetchMaxBytes` and `fetchTimeout` constants
- Cleanup func always non-nil; removes all temp files
- Wired into all 5 subcommand handlers; display paths remain `configPaths` (not resolved) for readability
- `config` package unchanged — no `net/http` dependency added there
- All tests pass; `go build ./...` clean

### Explicit Boundaries
- `config` package is **not touched** — URL handling lives entirely in `cmd` layer
- No host allowlist, no checksum verification — deliberate for personal tool
- No `http://` → redirect-to-https behaviour — explicit rejection only
- Display paths (for headers/metadata) use original `configPaths`, not resolved temp paths
- `app.go` (Wails GUI entry) is **not touched** — GUI doesn't expose `--config` flags

## Out of Scope
- Any changes to `internal/config` package
- `app.go` / GUI changes
- Checksum or host allowlist
- Redirect support for http:// URLs
- Any other subcommand or runner changes

# Scope Change Log
| # | Category | What | Why | Decision | Outcome |
|---|----------|------|-----|----------|---------|

# Follow-up Tasks
_(none yet)_
