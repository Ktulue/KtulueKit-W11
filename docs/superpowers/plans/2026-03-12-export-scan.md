# Export / Scan Mode Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `ktuluekit export` — scans installed items against a reference config and writes a replay-ready `ktuluekit-snapshot.json` usable by KtulueKit-Migration.

**Architecture:** Three layers: (1) `detector.RunCheckDetailed` exposes timeout-vs-absent distinction; (2) `internal/exporter` is pure logic — no I/O, fully unit-testable via injected `CheckFn`; (3) `cmd/export.go` handles flags, file I/O, and terminal output. A `state.StatePath()` export enables the `--fast` pre-check without duplicating path logic.

**Tech Stack:** Go 1.21+, Cobra (already used), `encoding/json`, `os/exec`, `context` (all already in use).

---

**Test baseline before starting:**
```
go test ./cmd/... ./internal/config/... ./internal/detector/... ./internal/installer/... ./internal/reporter/... ./internal/runner/... ./internal/scheduler/... ./internal/state/...
```
All packages above must pass before you start. The root package (`github.com/Ktulue/KtulueKit-W11`) will fail (Wails frontend not built) — that is expected and pre-existing; ignore it.

---

## Chunk 1: Foundation — `detector.RunCheckDetailed` + `state.StatePath`

Two small additions to existing packages. No new files.

**Files:**
- Modify: `internal/detector/detector.go` — add exported `RunCheckDetailed`
- Modify: `internal/detector/detector_test.go` — add tests for `RunCheckDetailed`
- Modify: `internal/state/state.go` — export `StatePath()`
- Modify: `internal/state/state_test.go` — add test for `StatePath()`

---

### Task 1: Export `state.StatePath()`

- [ ] **Step 1: Write the failing test**

Add to `internal/state/state_test.go`:

```go
func TestStatePath_ReturnsNonEmptyString(t *testing.T) {
    // state_test.go is package state (internal), so no package qualifier needed.
    p := StatePath()
    if p == "" {
        t.Error("StatePath() returned empty string")
    }
}
```

- [ ] **Step 2: Run to verify it fails**

```
go test ./internal/state/... -v -run TestStatePath
```
Expected: compile error — `state.StatePath` undefined.

- [ ] **Step 3: Add the export to `internal/state/state.go`**

Add immediately after the `statePath()` function (after line 28):

```go
// StatePath returns the resolved path for the state file.
// Exported so callers (e.g. cmd/export.go) can check file existence
// without replicating path resolution logic.
func StatePath() string {
    return statePath()
}
```

- [ ] **Step 4: Run to verify it passes**

```
go test ./internal/state/... -v -run TestStatePath
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/state/state.go internal/state/state_test.go
git commit -m "feat(state): export StatePath() for use by export command"
```

---

### Task 2: Add `detector.RunCheckDetailed`

- [ ] **Step 1: Write the failing tests**

Add to `internal/detector/detector_test.go`:

```go
// --- RunCheckDetailed tests ---
// These run real shell commands — they must be fast and OS-independent.
// "echo hi" exits 0 on Windows (cmd.exe) and is always present.

func TestRunCheckDetailed_ExitZero_ReturnsInstalled(t *testing.T) {
    installed, timedOut := detector.RunCheckDetailed("echo hi")
    if !installed {
        t.Error("expected installed=true for exit-0 command")
    }
    if timedOut {
        t.Error("expected timedOut=false for fast command")
    }
}

func TestRunCheckDetailed_ExitNonZero_ReturnsAbsent(t *testing.T) {
    // "exit 1" exits non-zero on Windows cmd.exe
    installed, timedOut := detector.RunCheckDetailed("exit 1")
    if installed {
        t.Error("expected installed=false for non-zero exit")
    }
    if timedOut {
        t.Error("expected timedOut=false — non-zero exit is not a timeout")
    }
}
```

- [ ] **Step 2: Run to verify they fail**

```
go test ./internal/detector/... -v -run TestRunCheckDetailed
```
Expected: compile error — `detector.RunCheckDetailed` undefined.

- [ ] **Step 3: Implement `RunCheckDetailed` in `internal/detector/detector.go`**

Add after the `runCheckSilent` function (after line 133):

```go
// RunCheckDetailed runs a check command and returns whether it passed and
// whether it timed out. timedOut implies installed==false.
// Uses the same 15-second timeout as runCheckSilent.
func RunCheckDetailed(checkCmd string) (installed, timedOut bool) {
    ctx, cancel := context.WithTimeout(context.Background(), checkTimeoutSeconds*time.Second)
    defer cancel()

    cmd := exec.CommandContext(ctx, "cmd", "/C", checkCmd)
    cmd.Stdout = io.Discard
    cmd.Stderr = io.Discard
    err := cmd.Run()

    if ctx.Err() == context.DeadlineExceeded {
        return false, true
    }
    return err == nil, false
}
```

- [ ] **Step 4: Run to verify they pass**

```
go test ./internal/detector/... -v -run TestRunCheckDetailed
```
Expected: PASS.

- [ ] **Step 5: Run full detector suite to check for regressions**

```
go test ./internal/detector/... -v
```
Expected: all tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/detector/detector.go internal/detector/detector_test.go
git commit -m "feat(detector): export RunCheckDetailed with timeout distinction"
```

---

## Chunk 2: `internal/exporter` Package

New package — pure logic, no file I/O, fully unit-testable.

**Files:**
- Create: `internal/exporter/exporter.go`
- Create: `internal/exporter/exporter_test.go`

---

### Task 3: Create `internal/exporter/exporter.go` (types only, no logic yet)

- [ ] **Step 1: Create the file with types and a stub `Export` function**

Create `internal/exporter/exporter.go`:

```go
package exporter

import (
    "fmt"
    "time"

    "github.com/Ktulue/KtulueKit-W11/internal/config"
    "github.com/Ktulue/KtulueKit-W11/internal/state"
)

// CheckResult is the outcome of a single check command.
type CheckResult int

const (
    CheckInstalled CheckResult = iota // check exited 0
    CheckAbsent                       // check exited non-zero
    CheckTimedOut                     // check exceeded 15-second timeout
)

// Options configures an Export call.
type Options struct {
    Fast         bool
    SourceConfig string            // absolute path to reference config, written into snapshot metadata
    ToolVersion  string            // from ldflags; "dev" if empty
    Machine      string            // os.Hostname()
    State        *state.State      // non-nil only in fast mode
    CheckFn      func(cmd string) CheckResult // nil only in fast mode; wraps detector.RunCheckDetailed in production
}

// SnapshotMeta is the snapshot metadata block written into the output JSON.
type SnapshotMeta struct {
    GeneratedAt  string `json:"generated_at"`
    Machine      string `json:"machine"`
    SourceConfig string `json:"source_config"`
    ToolVersion  string `json:"tool_version"`
    Mode         string `json:"mode"` // "check" | "fast"
}

// Result is returned by Export and carries both the filtered config slices
// and the summary counts needed for terminal output.
type Result struct {
    Packages   []config.Package
    Commands   []config.Command
    Extensions []config.Extension
    Profiles   []config.Profile
    Snapshot   SnapshotMeta
    Checked    int // total items probed (0 in fast mode)
    Included   int // total items in output
}

// Export scans cfg against the machine and returns a Result containing only
// installed items. In check mode, opts.CheckFn is called for each package and
// command. In fast mode, opts.State.Succeeded is used directly.
// Export has no file I/O — the caller is responsible for writing the output.
func Export(cfg *config.Config, opts Options) (Result, error) {
    if opts.ToolVersion == "" {
        opts.ToolVersion = "dev"
    }

    mode := "check"
    if opts.Fast {
        mode = "fast"
    }

    meta := SnapshotMeta{
        GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
        Machine:      opts.Machine,
        SourceConfig: opts.SourceConfig,
        ToolVersion:  opts.ToolVersion,
        Mode:         mode,
    }

    var res Result
    res.Snapshot = meta

    if opts.Fast {
        res = exportFast(cfg, opts, meta)
    } else {
        res = exportCheck(cfg, opts, meta)
    }

    return res, nil
}

// exportCheck runs check commands for each package and command.
func exportCheck(cfg *config.Config, opts Options, meta SnapshotMeta) Result {
    var pkgs []config.Package
    var cmds []config.Command
    var warnings []string

    for _, pkg := range cfg.Packages {
        if pkg.Check == "" || pkg.Check == "echo skip" {
            continue // no check available — skip
        }
        result := opts.CheckFn(pkg.Check)
        switch result {
        case CheckInstalled:
            pkgs = append(pkgs, pkg)
        case CheckTimedOut:
            warnings = append(warnings, fmt.Sprintf("[warn] %s: check timed out, treated as absent", pkg.Name))
        }
        // CheckAbsent: silently omitted
    }

    for _, cmd := range cfg.Commands {
        if cmd.Check == "" || cmd.Check == "echo skip" {
            continue
        }
        result := opts.CheckFn(cmd.Check)
        switch result {
        case CheckInstalled:
            cmds = append(cmds, cmd)
        case CheckTimedOut:
            warnings = append(warnings, fmt.Sprintf("[warn] %s: check timed out, treated as absent", cmd.Name))
        }
    }

    for _, w := range warnings {
        fmt.Println(w)
    }

    checked := 0
    for _, pkg := range cfg.Packages {
        if pkg.Check != "" && pkg.Check != "echo skip" {
            checked++
        }
    }
    for _, cmd := range cfg.Commands {
        if cmd.Check != "" && cmd.Check != "echo skip" {
            checked++
        }
    }
    included := len(pkgs) + len(cmds)

    includedIDs := buildIncludedSet(pkgs, cmds, nil)
    profiles := filterProfiles(cfg.Profiles, includedIDs)

    return Result{
        Packages:  pkgs,
        Commands:  cmds,
        Extensions: nil, // extensions have no check field in check mode
        Profiles:  profiles,
        Snapshot:  meta,
        Checked:   checked,
        Included:  included,
    }
}

// exportFast reads state.Succeeded to determine installed items.
func exportFast(cfg *config.Config, opts Options, meta SnapshotMeta) Result {
    s := opts.State
    var pkgs []config.Package
    var cmds []config.Command
    var exts []config.Extension

    for _, pkg := range cfg.Packages {
        if s.Succeeded[pkg.ID] {
            pkgs = append(pkgs, pkg)
        }
    }
    for _, cmd := range cfg.Commands {
        if s.Succeeded[cmd.ID] {
            cmds = append(cmds, cmd)
        }
    }
    for _, ext := range cfg.Extensions {
        if s.Succeeded[ext.ID] {
            exts = append(exts, ext)
        }
    }

    included := len(pkgs) + len(cmds) + len(exts)
    includedIDs := buildIncludedSet(pkgs, cmds, exts)
    profiles := filterProfiles(cfg.Profiles, includedIDs)

    return Result{
        Packages:   pkgs,
        Commands:   cmds,
        Extensions: exts,
        Profiles:   profiles,
        Snapshot:   meta,
        Checked:    0, // fast mode skips check commands
        Included:   included,
    }
}

// buildIncludedSet returns a set of all IDs present in the provided slices.
func buildIncludedSet(pkgs []config.Package, cmds []config.Command, exts []config.Extension) map[string]bool {
    ids := make(map[string]bool, len(pkgs)+len(cmds)+len(exts))
    for _, p := range pkgs {
        ids[p.ID] = true
    }
    for _, c := range cmds {
        ids[c.ID] = true
    }
    for _, e := range exts {
        ids[e.ID] = true
    }
    return ids
}

// filterProfiles returns profiles filtered to only included IDs.
// Profiles where all IDs are filtered out are omitted entirely.
func filterProfiles(profiles []config.Profile, includedIDs map[string]bool) []config.Profile {
    var out []config.Profile
    for _, p := range profiles {
        var filtered []string
        for _, id := range p.IDs {
            if includedIDs[id] {
                filtered = append(filtered, id)
            }
        }
        if len(filtered) > 0 {
            out = append(out, config.Profile{Name: p.Name, IDs: filtered})
        }
    }
    return out
}
```

- [ ] **Step 2: Verify it compiles**

```
go build ./internal/exporter/...
```
Expected: no errors.

- [ ] **Step 3: Commit the stub**

```bash
git add internal/exporter/exporter.go
git commit -m "feat(exporter): add exporter package with types and Export stub"
```

---

### Task 4: Write and pass exporter unit tests

- [ ] **Step 1: Create `internal/exporter/exporter_test.go` with all test cases**

```go
package exporter_test

import (
    "testing"

    "github.com/Ktulue/KtulueKit-W11/internal/config"
    "github.com/Ktulue/KtulueKit-W11/internal/exporter"
    "github.com/Ktulue/KtulueKit-W11/internal/state"
)

// mockCheckFn returns CheckInstalled for any command containing "installed",
// CheckTimedOut for any command containing "timeout", and CheckAbsent otherwise.
func mockCheckFn(cmd string) exporter.CheckResult {
    for _, c := range cmd {
        _ = c
        break
    }
    switch {
    case len(cmd) > 0 && cmd[0] == 'i': // starts with 'i' → installed
        return exporter.CheckInstalled
    case len(cmd) > 0 && cmd[0] == 't': // starts with 't' → timedout
        return exporter.CheckTimedOut
    default:
        return exporter.CheckAbsent
    }
}

// checkFnAlwaysInstalled always returns CheckInstalled.
func checkFnAlwaysInstalled(cmd string) exporter.CheckResult {
    return exporter.CheckInstalled
}

// checkFnAlwaysAbsent always returns CheckAbsent.
func checkFnAlwaysAbsent(cmd string) exporter.CheckResult {
    return exporter.CheckAbsent
}

// checkFnAlwaysTimeout always returns CheckTimedOut.
func checkFnAlwaysTimeout(cmd string) exporter.CheckResult {
    return exporter.CheckTimedOut
}

// baseOpts builds a minimal Options for check mode with the given CheckFn.
func baseOpts(fn func(string) exporter.CheckResult) exporter.Options {
    return exporter.Options{
        Fast:         false,
        SourceConfig: "/path/to/ktuluekit.json",
        ToolVersion:  "1.0.0",
        Machine:      "TESTMACHINE",
        CheckFn:      fn,
    }
}

func makeCfg(pkgs []config.Package, cmds []config.Command, exts []config.Extension, profiles []config.Profile) *config.Config {
    return &config.Config{
        Version:    "1.0",
        Packages:   pkgs,
        Commands:   cmds,
        Extensions: exts,
        Profiles:   profiles,
    }
}

// --- Check mode tests ---

func TestExport_CheckMode_AllInstalled(t *testing.T) {
    cfg := makeCfg(
        []config.Package{{ID: "Git.Git", Name: "Git", Phase: 1, Check: "installed-check"}},
        []config.Command{{ID: "claude-code", Name: "Claude Code", Phase: 4, Check: "installed-check"}},
        nil, nil,
    )
    res, err := exporter.Export(cfg, baseOpts(checkFnAlwaysInstalled))
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(res.Packages) != 1 {
        t.Errorf("expected 1 package, got %d", len(res.Packages))
    }
    if len(res.Commands) != 1 {
        t.Errorf("expected 1 command, got %d", len(res.Commands))
    }
    if res.Included != 2 {
        t.Errorf("expected Included=2, got %d", res.Included)
    }
    if res.Checked != 2 {
        t.Errorf("expected Checked=2, got %d", res.Checked)
    }
}

func TestExport_CheckMode_NoneInstalled(t *testing.T) {
    cfg := makeCfg(
        []config.Package{{ID: "Git.Git", Name: "Git", Phase: 1, Check: "absent-check"}},
        []config.Command{{ID: "claude-code", Name: "Claude Code", Phase: 4, Check: "absent-check"}},
        nil, nil,
    )
    res, err := exporter.Export(cfg, baseOpts(checkFnAlwaysAbsent))
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(res.Packages) != 0 {
        t.Errorf("expected 0 packages, got %d", len(res.Packages))
    }
    if len(res.Commands) != 0 {
        t.Errorf("expected 0 commands, got %d", len(res.Commands))
    }
    if res.Included != 0 {
        t.Errorf("expected Included=0, got %d", res.Included)
    }
}

func TestExport_CheckMode_MixedResults(t *testing.T) {
    cfg := makeCfg(
        []config.Package{
            {ID: "Git.Git", Name: "Git", Phase: 1, Check: "installed-check"},
            {ID: "GoLang.Go", Name: "Go", Phase: 1, Check: "absent-check"},
        },
        nil, nil, nil,
    )
    res, err := exporter.Export(cfg, baseOpts(checkFnAlwaysInstalled))
    // checkFnAlwaysInstalled returns installed for both — test the count
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(res.Packages) != 2 {
        t.Errorf("expected 2 packages (both installed), got %d", len(res.Packages))
    }
}

func TestExport_CheckMode_MixedWithAbsent(t *testing.T) {
    callCount := 0
    results := []exporter.CheckResult{exporter.CheckInstalled, exporter.CheckAbsent}
    fn := func(cmd string) exporter.CheckResult {
        r := results[callCount%len(results)]
        callCount++
        return r
    }
    cfg := makeCfg(
        []config.Package{
            {ID: "pkg-a", Name: "Pkg A", Phase: 1, Check: "check-a"},
            {ID: "pkg-b", Name: "Pkg B", Phase: 1, Check: "check-b"},
        },
        nil, nil, nil,
    )
    res, err := exporter.Export(cfg, exporter.Options{
        Fast: false, SourceConfig: "/x", ToolVersion: "1.0", Machine: "M", CheckFn: fn,
    })
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(res.Packages) != 1 {
        t.Errorf("expected 1 package (first installed, second absent), got %d", len(res.Packages))
    }
    if res.Packages[0].ID != "pkg-a" {
        t.Errorf("expected pkg-a to be included, got %q", res.Packages[0].ID)
    }
}

func TestExport_CheckMode_EmptyConfig(t *testing.T) {
    cfg := makeCfg(nil, nil, nil, nil)
    res, err := exporter.Export(cfg, baseOpts(checkFnAlwaysInstalled))
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if res.Checked != 0 {
        t.Errorf("expected Checked=0 for empty config, got %d", res.Checked)
    }
    if res.Included != 0 {
        t.Errorf("expected Included=0 for empty config, got %d", res.Included)
    }
}

func TestExport_CheckMode_ExtensionsAlwaysOmitted(t *testing.T) {
    cfg := makeCfg(
        nil, nil,
        []config.Extension{{ID: "ublock", Name: "uBlock", Phase: 5}},
        nil,
    )
    res, err := exporter.Export(cfg, baseOpts(checkFnAlwaysInstalled))
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(res.Extensions) != 0 {
        t.Errorf("expected 0 extensions in check mode, got %d", len(res.Extensions))
    }
}

func TestExport_CheckMode_NoCheckCmd_Skipped(t *testing.T) {
    // Items with no check command or "echo skip" are excluded regardless of CheckFn.
    called := false
    fn := func(cmd string) exporter.CheckResult {
        called = true
        return exporter.CheckInstalled
    }
    cfg := makeCfg(
        []config.Package{
            {ID: "pkg-no-check", Name: "No Check", Phase: 1, Check: ""},
            {ID: "pkg-echo-skip", Name: "Echo Skip", Phase: 1, Check: "echo skip"},
        },
        nil, nil, nil,
    )
    res, err := exporter.Export(cfg, exporter.Options{
        Fast: false, SourceConfig: "/x", ToolVersion: "1.0", Machine: "M", CheckFn: fn,
    })
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if called {
        t.Error("CheckFn should not be called for items with no check command or 'echo skip'")
    }
    if len(res.Packages) != 0 {
        t.Errorf("expected 0 packages (no-check items skipped), got %d", len(res.Packages))
    }
}

func TestExport_CheckMode_TimeoutTreatedAsAbsent(t *testing.T) {
    cfg := makeCfg(
        []config.Package{{ID: "Git.Git", Name: "Git", Phase: 1, Check: "slow-check"}},
        nil, nil, nil,
    )
    res, err := exporter.Export(cfg, baseOpts(checkFnAlwaysTimeout))
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(res.Packages) != 0 {
        t.Errorf("expected 0 packages after timeout, got %d", len(res.Packages))
    }
}

func TestExport_CheckMode_SnapshotMetadata(t *testing.T) {
    cfg := makeCfg(nil, nil, nil, nil)
    res, err := exporter.Export(cfg, exporter.Options{
        Fast: false, SourceConfig: "/abs/path.json", ToolVersion: "2.0.0", Machine: "MYMACHINE", CheckFn: checkFnAlwaysInstalled,
    })
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if res.Snapshot.SourceConfig != "/abs/path.json" {
        t.Errorf("expected SourceConfig '/abs/path.json', got %q", res.Snapshot.SourceConfig)
    }
    if res.Snapshot.ToolVersion != "2.0.0" {
        t.Errorf("expected ToolVersion '2.0.0', got %q", res.Snapshot.ToolVersion)
    }
    if res.Snapshot.Machine != "MYMACHINE" {
        t.Errorf("expected Machine 'MYMACHINE', got %q", res.Snapshot.Machine)
    }
    if res.Snapshot.Mode != "check" {
        t.Errorf("expected Mode 'check', got %q", res.Snapshot.Mode)
    }
    if res.Snapshot.GeneratedAt == "" {
        t.Error("GeneratedAt should not be empty")
    }
}

func TestExport_CheckMode_ToolVersionDefaultsToDev(t *testing.T) {
    cfg := makeCfg(nil, nil, nil, nil)
    res, err := exporter.Export(cfg, exporter.Options{
        Fast: false, SourceConfig: "/x", ToolVersion: "", Machine: "M", CheckFn: checkFnAlwaysInstalled,
    })
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if res.Snapshot.ToolVersion != "dev" {
        t.Errorf("expected ToolVersion 'dev' when empty, got %q", res.Snapshot.ToolVersion)
    }
}

// --- Profile filtering tests ---

func TestExport_CheckMode_ProfileFullyFiltered_Omitted(t *testing.T) {
    cfg := makeCfg(
        []config.Package{{ID: "Git.Git", Name: "Git", Phase: 1, Check: "absent-check"}},
        nil, nil,
        []config.Profile{{Name: "Dev Only", IDs: []string{"Git.Git"}}},
    )
    res, err := exporter.Export(cfg, baseOpts(checkFnAlwaysAbsent))
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(res.Profiles) != 0 {
        t.Errorf("expected 0 profiles when all IDs filtered, got %d", len(res.Profiles))
    }
}

func TestExport_CheckMode_ProfilePartiallyFiltered_Emitted(t *testing.T) {
    callCount := 0
    results := []exporter.CheckResult{exporter.CheckInstalled, exporter.CheckAbsent}
    fn := func(cmd string) exporter.CheckResult {
        r := results[callCount%len(results)]
        callCount++
        return r
    }
    cfg := makeCfg(
        []config.Package{
            {ID: "pkg-a", Name: "Pkg A", Phase: 1, Check: "check-a"},
            {ID: "pkg-b", Name: "Pkg B", Phase: 1, Check: "check-b"},
        },
        nil, nil,
        []config.Profile{{Name: "Both", IDs: []string{"pkg-a", "pkg-b"}}},
    )
    res, err := exporter.Export(cfg, exporter.Options{
        Fast: false, SourceConfig: "/x", ToolVersion: "1.0", Machine: "M", CheckFn: fn,
    })
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(res.Profiles) != 1 {
        t.Errorf("expected 1 profile (partial), got %d", len(res.Profiles))
    }
    if len(res.Profiles[0].IDs) != 1 {
        t.Errorf("expected profile to have 1 ID after filtering, got %d", len(res.Profiles[0].IDs))
    }
    if res.Profiles[0].IDs[0] != "pkg-a" {
        t.Errorf("expected remaining profile ID to be 'pkg-a', got %q", res.Profiles[0].IDs[0])
    }
}

// --- Fast mode tests ---

func makeFastOpts(s *state.State) exporter.Options {
    return exporter.Options{
        Fast:         true,
        SourceConfig: "/path/to/ktuluekit.json",
        ToolVersion:  "1.0.0",
        Machine:      "TESTMACHINE",
        State:        s,
    }
}

func TestExport_FastMode_IncludesSucceededItems(t *testing.T) {
    s := &state.State{
        Succeeded: map[string]bool{"Git.Git": true, "claude-code": true},
        Failed:    map[string]bool{},
    }
    cfg := makeCfg(
        []config.Package{{ID: "Git.Git", Name: "Git", Phase: 1}},
        []config.Command{{ID: "claude-code", Name: "Claude Code", Phase: 4}},
        nil, nil,
    )
    res, err := exporter.Export(cfg, makeFastOpts(s))
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(res.Packages) != 1 {
        t.Errorf("expected 1 package, got %d", len(res.Packages))
    }
    if len(res.Commands) != 1 {
        t.Errorf("expected 1 command, got %d", len(res.Commands))
    }
    if res.Snapshot.Mode != "fast" {
        t.Errorf("expected Mode 'fast', got %q", res.Snapshot.Mode)
    }
    if res.Checked != 0 {
        t.Errorf("expected Checked=0 in fast mode, got %d", res.Checked)
    }
}

func TestExport_FastMode_ExcludesFailedItems(t *testing.T) {
    s := &state.State{
        Succeeded: map[string]bool{},
        Failed:    map[string]bool{"Git.Git": true},
    }
    cfg := makeCfg(
        []config.Package{{ID: "Git.Git", Name: "Git", Phase: 1}},
        nil, nil, nil,
    )
    res, err := exporter.Export(cfg, makeFastOpts(s))
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(res.Packages) != 0 {
        t.Errorf("expected 0 packages (failed item excluded), got %d", len(res.Packages))
    }
}

func TestExport_FastMode_IncludesExtensions(t *testing.T) {
    s := &state.State{
        Succeeded: map[string]bool{"ublock": true},
        Failed:    map[string]bool{},
    }
    cfg := makeCfg(
        nil, nil,
        []config.Extension{{ID: "ublock", Name: "uBlock Origin", Phase: 5}},
        nil,
    )
    res, err := exporter.Export(cfg, makeFastOpts(s))
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(res.Extensions) != 1 {
        t.Errorf("expected 1 extension in fast mode, got %d", len(res.Extensions))
    }
}
```

- [ ] **Step 2: Run to verify tests fail (Export is a stub returning empty Result)**

```
go test ./internal/exporter/... -v
```
Expected: multiple FAIL — counts will be wrong since the stub is incomplete. (The file compiles; the logic is just not yet correct for all cases.)

Actually, re-check: the stub in Task 3 already has the full implementation. Run tests and they should PASS. If they do, great — skip to step 4.

- [ ] **Step 3: If any tests fail, fix `exporter.go` until all pass**

Common issues to check:
- `Checked` count: only counts items with a non-empty, non-"echo skip" check command
- `Included` count: sum of packages + commands (check mode) or packages + commands + extensions (fast mode)
- Profile filtering: uses IDs from included slices only

- [ ] **Step 4: Run the full test suite**

```
go test ./internal/exporter/... -v
```
Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/exporter/exporter.go internal/exporter/exporter_test.go
git commit -m "feat(exporter): implement Export with check and fast modes"
```

---

## Chunk 3: `cmd/export.go`, `cmd/main.go` wiring, and JSON schema update

**Files:**
- Create: `cmd/export.go`
- Modify: `cmd/main.go` — register `exportCmd`
- Modify: `schema/ktuluekit.schema.json` — add `snapshot` as optional root property

---

### Task 5: Create `cmd/export.go`

- [ ] **Step 1: Create `cmd/export.go`**

```go
package main

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "runtime"

    "github.com/spf13/cobra"

    "github.com/Ktulue/KtulueKit-W11/internal/config"
    "github.com/Ktulue/KtulueKit-W11/internal/detector"
    "github.com/Ktulue/KtulueKit-W11/internal/exporter"
    "github.com/Ktulue/KtulueKit-W11/internal/state"
)

// Version is set at build time via -ldflags "-X main.Version=x.y.z".
// Falls back to "dev" if not set (handled in exporter.Export).
var Version string

var (
    exportOutput string
    exportFast   bool
)

// snapshotFile is the JSON structure written to disk.
// It mirrors config.Config but adds the snapshot metadata block
// and omits $schema (which would have a broken relative path).
type snapshotFile struct {
    Version    string              `json:"version"`
    Snapshot   exporter.SnapshotMeta `json:"snapshot"`
    Metadata   config.Metadata     `json:"metadata"`
    Settings   config.Settings     `json:"settings"`
    Packages   []config.Package    `json:"packages"`
    Commands   []config.Command    `json:"commands"`
    Extensions []config.Extension  `json:"extensions"`
    Profiles   []config.Profile    `json:"profiles"`
}

func runExport(cmd *cobra.Command, args []string) error {
    // Resolve --config default (cwd-relative).
    paths := configPaths
    if len(paths) == 0 {
        paths = []string{config.DefaultConfigPath}
    }

    // Resolve --output default (cwd-relative).
    outPath := exportOutput
    if outPath == "" {
        outPath = "ktuluekit-snapshot.json"
    }

    // Load and validate the reference config.
    cfg, err := config.LoadAll(paths)
    if err != nil {
        return fmt.Errorf("config error: %w", err)
    }
    if errs := config.Validate(cfg); len(errs) > 0 {
        for _, e := range errs {
            fmt.Fprintf(os.Stderr, "  ERROR  %-30s  %s\n", e.Field, e.Message)
        }
        return fmt.Errorf("%d config error(s) — fix the above before exporting", len(errs))
    }

    // Resolve absolute source config path for metadata.
    absConfig, err := filepath.Abs(paths[0])
    if err != nil {
        absConfig = paths[0] // best-effort
    }

    // Resolve machine name.
    machine, err := os.Hostname()
    if err != nil {
        machine = "unknown"
    }

    var opts exporter.Options
    opts.SourceConfig = absConfig
    opts.ToolVersion = Version
    opts.Machine = machine

    if exportFast {
        // Pre-check: state file must exist for --fast to be meaningful.
        statePath := state.StatePath()
        if _, statErr := os.Stat(statePath); os.IsNotExist(statErr) {
            return fmt.Errorf(
                "state file not found at %s\n"+
                    "  Run ktuluekit first to build the state file, or use export without --fast",
                statePath,
            )
        }
        s, loadErr := state.Load()
        if loadErr != nil {
            return fmt.Errorf("state file corrupt: %w\n  Try running export without --fast", loadErr)
        }
        opts.Fast = true
        opts.State = s
    } else {
        // Check mode: inject production CheckFn wrapping detector.RunCheckDetailed.
        opts.CheckFn = func(checkCmd string) exporter.CheckResult {
            installed, timedOut := detector.RunCheckDetailed(checkCmd)
            switch {
            case timedOut:
                return exporter.CheckTimedOut
            case installed:
                return exporter.CheckInstalled
            default:
                return exporter.CheckAbsent
            }
        }
    }

    fmt.Printf("Exporting snapshot from: %v\n", paths)
    if exportFast {
        fmt.Println("Mode: fast (state file)")
    } else {
        fmt.Printf("Mode: check (running %d check commands...)\n",
            countCheckable(cfg))
    }
    fmt.Println()

    res, err := exporter.Export(cfg, opts)
    if err != nil {
        return fmt.Errorf("export error: %w", err)
    }

    // Build the output struct.
    out := snapshotFile{
        Version:    cfg.Version,
        Snapshot:   res.Snapshot,
        Metadata:   cfg.Metadata,
        Settings:   cfg.Settings,
        Packages:   nilIfEmpty(res.Packages),
        Commands:   nilIfEmpty(res.Commands),
        Extensions: nilIfEmpty(res.Extensions),
        Profiles:   nilIfEmpty(res.Profiles),
    }

    data, err := json.MarshalIndent(out, "", "  ")
    if err != nil {
        return fmt.Errorf("json marshal error: %w", err)
    }
    data = append(data, '\n')

    if err := os.WriteFile(outPath, data, 0644); err != nil {
        return fmt.Errorf("could not write snapshot to %s: %w", outPath, err)
    }

    _ = runtime.GOOS // suppress unused import if needed

    fmt.Printf("Checked:  %d items\n", res.Checked)
    fmt.Printf("Included: %d items\n", res.Included)
    fmt.Printf("Output:   %s\n", outPath)
    return nil
}

// countCheckable returns the number of items with a real check command.
func countCheckable(cfg *config.Config) int {
    n := 0
    for _, p := range cfg.Packages {
        if p.Check != "" && p.Check != "echo skip" {
            n++
        }
    }
    for _, c := range cfg.Commands {
        if c.Check != "" && c.Check != "echo skip" {
            n++
        }
    }
    return n
}

// nilIfEmpty returns nil instead of an empty slice so JSON output
// omits the field rather than writing [].
// Note: json.Marshal treats nil slice as null, but we want [].
// Keep as-is — empty slices marshal as [] which is valid JSON.
func nilIfEmpty[T any](s []T) []T {
    return s
}
```

- [ ] **Step 2: Register `exportCmd` in `cmd/main.go`**

In `cmd/main.go`, after the `listCmd` block (around line 74), add:

```go
exportCmd := &cobra.Command{
    Use:   "export",
    Short: "Scan machine and write a replay-ready ktuluekit-snapshot.json",
    Long: `Export scans the current machine against the reference config.
Items whose check command passes are included in the snapshot.
The snapshot is a valid ktuluekit.json (replay on a new machine)
and the handoff artifact for KtulueKit-Migration.

Use --fast to skip check commands and use the KtulueKit state file instead.`,
    RunE: runExport,
}
exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "Output path (default: ktuluekit-snapshot.json in cwd)")
exportCmd.Flags().BoolVar(&exportFast, "fast", false, "Use state file instead of running check commands")
root.AddCommand(exportCmd)
```

- [ ] **Step 3: Verify it builds**

```
go build ./cmd/...
```
Expected: no errors. (Root package may still fail due to Wails frontend — that's OK.)

- [ ] **Step 4: Quick smoke test — validate works (checks Cobra wiring)**

```
go run ./cmd/... export --help
```
Expected: shows export command usage with `--output` and `--fast` flags.

- [ ] **Step 5: Commit**

```bash
git add cmd/export.go cmd/main.go
git commit -m "feat(cmd): add export subcommand"
```

---

### Task 6: Update JSON schema

**Design note:** The spec defines snapshot files as valid `ktuluekit.json` files with one extra top-level key (`snapshot`). Rather than creating a separate schema file, we extend the existing `ktuluekit.schema.json` to allow the `snapshot` key — this way `ktuluekit validate` works on both config files and snapshot files without changes to the validate command. The `additionalProperties: false` at line 8 of `schema/ktuluekit.schema.json` must be removed (it would reject the `snapshot` key).

- [ ] **Step 1: Open `schema/ktuluekit.schema.json` and make two changes**

**Change 1:** Remove the `"additionalProperties": false` line (line 8). The file currently has it immediately after `"type": "object"`. Delete that line entirely.

**Change 2:** In the `"properties"` object, add the `"snapshot"` entry alongside `"packages"`, `"commands"`, etc.:

```json
"snapshot": {
  "type": "object",
  "description": "Present in snapshot files only. Records when and how the snapshot was generated.",
  "properties": {
    "generated_at": { "type": "string", "description": "ISO 8601 timestamp of snapshot generation." },
    "machine": { "type": "string", "description": "Hostname of the machine that generated the snapshot." },
    "source_config": { "type": "string", "description": "Absolute path to the reference config used." },
    "tool_version": { "type": "string", "description": "KtulueKit version that generated the snapshot." },
    "mode": { "type": "string", "enum": ["check", "fast"], "description": "Scan mode used." }
  }
}
```

- [ ] **Step 2: Verify `ktuluekit validate` still passes on the main config**

```
go run ./cmd/... validate --config ktuluekit.json
```
Expected: `OK — no errors found`.

- [ ] **Step 3: Commit**

```bash
git add schema/ktuluekit.schema.json
git commit -m "feat(schema): add optional snapshot property for export files"
```

---

### Task 7: Update TODO.md

- [ ] **Step 1: Mark the export/scan item as done in `TODO.md`**

Find this line in `TODO.md`:
```
- [ ] **Export/scan mode** — `ktuluekit export` scans the machine via `winget list` and generates a `ktuluekit.json` from what's currently installed. Great for bootstrapping a config from an existing machine.
```

Replace with:
```
- [x] **Export/scan mode** — `ktuluekit export` scans the machine via check commands and generates a `ktuluekit-snapshot.json` from what's currently installed. Replay-ready config and KtulueKit-Migration handoff artifact. Supports `--fast` (state file) and `--output` flags.
```

- [ ] **Step 2: Commit**

```bash
git add TODO.md
git commit -m "chore(todo): mark export/scan mode as done"
```

---

## Final Verification

- [ ] **Run all tests one last time**

```
go test ./cmd/... ./internal/config/... ./internal/detector/... ./internal/exporter/... ./internal/installer/... ./internal/reporter/... ./internal/runner/... ./internal/scheduler/... ./internal/state/...
```
Expected: all packages PASS. Root package may still fail (Wails frontend) — expected.

- [ ] **Build the binary**

```
go build -o ktuluekit.exe ./cmd/...
```
Expected: `ktuluekit.exe` created, no errors.

- [ ] **Verify subcommand is listed**

```
./ktuluekit.exe --help
```
Expected: `export` appears in the list of available commands.
