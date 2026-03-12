# CLI Features Phase 2 Design

**Date:** 2026-03-11
**Branch:** feat/cli-features
**Status:** Approved

## Overview

Three CLI improvements to round out the `feat/cli-features` branch:

1. `--phase N` — run only a single phase
2. `--upgrade-only` — sweep already-installed packages
3. Graceful Ctrl+C — clean interrupt handling

---

## Feature 1: `--phase N`

### Flag

`--phase int` on the root install command. Default `0` (disabled = run all phases).

### Behavior

When set, the runner processes only items where `Phase == N`. All other phases are skipped entirely — not logged, not counted. `totalItems` reflects only items in that phase.

### `totalItems` Calculation

In `Run()`, the existing call `r.countItemsFromPhase(r.resumePhase)` (counts items where `Phase >= fromPhase`) is replaced by a conditional:

```
if onlyPhase > 0:
    totalItems = countItemsInPhase(onlyPhase)   // Phase == N only
else:
    totalItems = countItemsFromPhase(resumePhase) // Phase >= resumePhase (existing behavior)
```

`countItemsInPhase(n int) int` is a new helper that uses `==` comparison instead of `>=`, and respects `selectedIDs`.

### Phase Header Display

All runs (not just `--phase` runs) update the phase header to include its position in the full sequence:

```
── Phase 2 | [2 of 4] ──────────────────────────────
```

Where `2` is the 1-based index of the current phase in the sorted phases list, and `4` is the total phase count. `M` comes from the full unfiltered `collectPhases()` result — it always reflects the total number of phases in the config, regardless of `onlyPhase`. The header renders only for phases that actually execute; skipped phases produce no output at all. This applies even when `--phase N` is set — "2 of 4" communicates which phase in the config is being run.

### Validation

Mutually exclusive with `--resume-phase` — returns an error if `--phase > 0 && --resume-phase > 1` (i.e., both set to non-default values). The default for `--resume-phase` is `1`, so this condition is only triggered by an explicit user combination.

### Construction

`onlyPhase` and `upgradeOnly` are set via new setter methods (`SetOnlyPhase(n int)`, `SetUpgradeOnly(b bool)`) following the existing pattern of `SetSelectedIDs`, `SetOnProgress`, etc. `runner.New()` signature is unchanged.

### Files Changed

- `cmd/main.go` — add `--phase` flag; validate mutual exclusion with `--resume-phase`; call `r.SetOnlyPhase(phase)` after constructing runner
- `internal/runner/runner.go` — add `onlyPhase int` field and `SetOnlyPhase` setter; add `countItemsInPhase(n int) int` helper; update `Run()` with the `totalItems` conditional above; in the phase loop, skip phases that don't match `onlyPhase` (when set); add phase header helper that builds the `| [N of M]` suffix using the phase's index in the unfiltered `collectPhases()` result

---

## Feature 2: `--upgrade-only`

### Flag

`--upgrade-only bool` on the root install command.

### Behavior

Before processing each item, calls `detector.CheckItem(item, r.state)` to test install status. The runner already has `r.state`; it must be passed to `CheckItem`. Based on the result:

| Detector result   | Action                                                              |
|-------------------|---------------------------------------------------------------------|
| `StatusInstalled` | Proceed — upgrade path is forced on (see below)                     |
| `StatusMissing`   | Skip silently                                                       |
| `StatusUnknown`   | Skip silently for extensions and commands (neither typically has a reliable check mechanism); skip with a yellow `WARN: no check command` for packages only |

**Item projection:** `detector.CheckItem` takes a `detector.Item`, not a `config.Package`/`config.Command`/`config.Extension`. Each tier loop must project the config struct to a `detector.Item` before calling `CheckItem`. This follows the same pattern used by `detector.FlattenItems` in `printPreRunSummary`.

**Upgrade forcing:** Sets `cfg.Settings.UpgradeIfInstalled = true` in `runInstall` before constructing the runner — same pattern as `--no-upgrade` sets it to `false`. This ensures all installed items are upgraded, not just those with `upgrade_if_installed: true` in config.

**`totalItems` note:** The `[N/Total]` item counter will over-count when `--upgrade-only` is active, because items are dynamically skipped at runtime after `totalItems` is calculated. This is acceptable — the counter reflects how many items were _scheduled_, not how many ran. No pre-scan step is added.

### Composability

- Works freely with `--only` / `--exclude` through the existing `selectedIDs` pipeline
- Works with `--phase N`
- Mutually exclusive with `--no-upgrade` — returns an error if both are set

### Files Changed

- `cmd/main.go` — add `--upgrade-only` flag; validate mutual exclusion with `--no-upgrade`; set `cfg.Settings.UpgradeIfInstalled = true` when flag is set; call `r.SetUpgradeOnly(true)` after constructing runner. Note: `--upgrade-only` is compatible with `--dry-run`; detection runs normally and upgrade commands are dry-run as usual.
- `internal/runner/runner.go` — add `upgradeOnly bool` field and `SetUpgradeOnly` setter; in each tier loop, after the `selectedIDs` check, project the config struct to a `detector.Item` (following the same pattern as `detector.FlattenItems`) and call `detector.CheckItem(item, r.state)`; skip per the table above

---

## Feature 3: Graceful Ctrl+C

### Mechanism

`signal.NotifyContext(context.Background(), os.Interrupt)` in `runInstall`. The resulting context is passed to `r.Run(ctx context.Context)`. `defer stop()` is called to release the signal registration.

### Check Points

At the top of each item iteration in all three tier loops, before any work begins:

```go
if ctx.Err() != nil {
    r.markInterrupted(phase)  // prints message, sets r.interrupted = true
    return                    // caller returns explicitly — markInterrupted does NOT return for the caller
}
```

`phase` is the enclosing phase loop variable. To avoid Go loop variable capture bugs (pre-1.22), assign it to a local copy before the inner loop: `currentPhase := phase`. `markInterrupted` only prints and sets state; the `return` statement is always in the calling loop body.

This does **not** kill in-flight processes. The currently-executing install finishes naturally; the interrupt is honored before the next item starts.

### On Interrupt

When `markInterrupted(phase)` is called:

1. Prints once (guarded by `!r.interrupted`): `"  Interrupted — finishing current item then stopping. Run with --resume-phase=<phase> to continue.\n"`
2. Sets `r.interrupted = true`

The caller then `return`s, propagating up through `Run()`.

The resume hint uses `currentPhase` (not `currentPhase + 1`) because the current phase is incomplete — the user should re-run it from the beginning.

### Post-Run Handling in `runInstall`

After `r.Run(ctx)` returns:

- If `r.WasInterrupted()` is true:
  - Skip `state.Clear()` — preserve state for resume
  - Summary still prints so the user can see what completed before the interrupt
- The `defer rep.Close()` fires normally, flushing the log file

Note: `rep.HasFailures()` is insufficient to guard `state.Clear()` on an interrupted run because a clean-but-incomplete run may have zero failures. `r.WasInterrupted()` is the correct guard.

### GUI Mode

The GUI has no Ctrl+C path; context cancellation is safe because the runner simply exits cleanly if it fires.

### Files Changed

- `cmd/main.go` — create context with `signal.NotifyContext`; `defer stop()`; pass `ctx` to `r.Run(ctx)`; check `r.WasInterrupted()` after run to guard `state.Clear()`
- `internal/runner/runner.go` — update `Run` signature to `Run(ctx context.Context)`; add `interrupted bool` field; add `markInterrupted(phase int)` and `WasInterrupted() bool`; add ctx check before each item in all three tier loops

---

## Testing

- `TestCountItemsInPhase` — counts items where `Phase == N` exactly, respects `selectedIDs`
- `TestPhaseFilter_OnlyPhase` — runner with `onlyPhase=2` skips items in phases 1 and 3; `totalItems` equals only phase-2 items
- `TestPhaseFlag_MutualExclusion` — `--phase > 0` + `--resume-phase > 1` returns error
- `TestUpgradeOnly_SkipsMissing` — item with `StatusMissing` is skipped
- `TestUpgradeOnly_SkipsUnknownPackage` — package/command with `StatusUnknown` is skipped with yellow warning
- `TestUpgradeOnly_SkipsExtensionSilently` — extension with `StatusUnknown` is skipped with no output
- `TestUpgradeOnly_MutualExclusion` — `--upgrade-only` + `--no-upgrade` returns error
- `TestCtrlC_StopsBeforeNextItem` — context cancelled mid-run; `WasInterrupted()` returns true; item after cancellation point is not started
- `TestCtrlC_WasInterrupted` — runner with cancelled context sets `WasInterrupted()` true; verifies the runner does not internally clear state (the `state.Clear()` guard in `runInstall` is covered by integration/manual testing, as it lives in `cmd/main.go` outside the runner package)
