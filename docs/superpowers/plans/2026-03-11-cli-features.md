# CLI Features Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `validate`, `list` subcommands, `--only`/`--exclude` flags, and a consecutive-failure pause to KtulueKit's CLI.

**Architecture:** `Validate()` replaces the private `validate()` in loader.go — LoadAll no longer validates internally; callers do. The runner gains a `trackResult()` helper and `promptConsecutiveFailures()`. All new CLI surface is wired in cmd/main.go.

**Tech Stack:** Go 1.21+, cobra, existing internal/config and internal/runner packages.

**Spec:** `docs/superpowers/specs/2026-03-11-cli-features-design.md`

---

## File Map

| File | Action | Responsibility |
|---|---|---|
| `internal/config/validate.go` | **Create** | `ValidationError`, public `Validate()` — all config checks |
| `internal/config/validate_test.go` | **Create** | Unit tests for every `Validate()` check |
| `internal/config/loader.go` | **Modify** | Delete private `validate()`, remove its call from `LoadAll` |
| `internal/runner/runner.go` | **Modify** | Add `consecutiveFails`, `pauseResponse`, `onPause`, setters, `trackResult()`, `promptConsecutiveFailures()` |
| `internal/runner/runner_test.go` | **Modify** | Add consecutive-failure counter tests |
| `cmd/main.go` | **Modify** | Add `validate`/`list` subcommands, `--only`/`--exclude` flags, call `Validate()` in `runInstall`, add `selectedIDs` pre-run summary guard |

---

## Chunk 1: Validate logic — `internal/config/validate.go` + loader.go

### Task 1: Create `validate.go` with `ValidationError` type and stub

**Files:**
- Create: `internal/config/validate.go`
- Create: `internal/config/validate_test.go`

- [ ] **Write the failing test** — top-level required fields check

```go
// internal/config/validate_test.go
package config

import (
    "testing"
)

func cfg(version, metaName string) *Config {
    return &Config{Version: version, Metadata: Metadata{Name: metaName}}
}

func TestValidate_MissingVersion(t *testing.T) {
    errs := Validate(cfg("", "MyKit"))
    if len(errs) == 0 {
        t.Fatal("want error for missing version, got none")
    }
    found := false
    for _, e := range errs {
        if e.Field == "[top-level]" {
            found = true
        }
    }
    if !found {
        t.Errorf("want error with Field=[top-level], got %+v", errs)
    }
}

func TestValidate_MissingMetadataName(t *testing.T) {
    errs := Validate(cfg("1.0", ""))
    if len(errs) == 0 {
        t.Fatal("want error for missing metadata.name, got none")
    }
}

func TestValidate_CleanConfig(t *testing.T) {
    c := cfg("1.0", "MyKit")
    errs := Validate(c)
    if len(errs) != 0 {
        t.Errorf("clean config: want 0 errors, got %+v", errs)
    }
}
```

- [ ] **Run test to verify it fails**

```
cd F:/GDriveClone/Claude_Code/KtulueKit-W11 && go test ./internal/config/... -run TestValidate -v
```
Expected: compile error — `Validate` undefined.

- [ ] **Create `validate.go` with `ValidationError` and top-level checks**

```go
// internal/config/validate.go
package config

import "fmt"

// ValidationError describes a single config validation problem.
type ValidationError struct {
    Field   string
    Message string
}

func (e ValidationError) Error() string {
    return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// Validate checks all config fields and cross-references.
// It collects ALL errors (does not fail fast) and returns them.
// Returns nil if the config is valid.
func Validate(cfg *Config) []ValidationError {
    var errs []ValidationError

    add := func(field, msg string) {
        errs = append(errs, ValidationError{Field: field, Message: msg})
    }

    // 1. Top-level required fields.
    if cfg.Version == "" {
        add("[top-level]", "version is required")
    }
    if cfg.Metadata.Name == "" {
        add("[top-level]", "metadata.name is required")
    }

    // Build ID set for cross-reference checks (populated as we walk items).
    ids := make(map[string]bool)

    // 2-5. Packages.
    for i, p := range cfg.Packages {
        prefix := fmt.Sprintf("packages[%d]", i)
        if p.ID == "" {
            add(prefix+".id", "required field 'id' is missing")
            continue // can't do further checks without an ID
        }
        if p.Name == "" {
            add(fmt.Sprintf("%s(%s).name", prefix, p.ID), "required field 'name' is missing")
        }
        if p.Phase < 1 {
            add(fmt.Sprintf("%s(%s).phase", prefix, p.ID), "phase must be >= 1")
        }
        if ids[p.ID] {
            add(fmt.Sprintf("%s.id", prefix), fmt.Sprintf("duplicate ID %q", p.ID))
        } else {
            ids[p.ID] = true
        }
    }

    // 3-5. Commands.
    for i, c := range cfg.Commands {
        prefix := fmt.Sprintf("commands[%d]", i)
        if c.ID == "" {
            add(prefix+".id", "required field 'id' is missing")
            continue
        }
        if c.Name == "" {
            add(fmt.Sprintf("%s(%s).name", prefix, c.ID), "required field 'name' is missing")
        }
        if c.Phase < 1 {
            add(fmt.Sprintf("%s(%s).phase", prefix, c.ID), "phase must be >= 1")
        }
        if c.Check == "" {
            add(fmt.Sprintf("%s(%s).check", prefix, c.ID), "required field 'check' is missing")
        }
        if c.Cmd == "" {
            add(fmt.Sprintf("%s(%s).command", prefix, c.ID), "required field 'command' is missing")
        }
        if ids[c.ID] {
            add(fmt.Sprintf("%s.id", prefix), fmt.Sprintf("duplicate ID %q", c.ID))
        } else {
            ids[c.ID] = true
        }
    }

    // 4-5. Extensions.
    for i, e := range cfg.Extensions {
        prefix := fmt.Sprintf("extensions[%d]", i)
        if e.ID == "" {
            add(prefix+".id", "required field 'id' is missing")
            continue
        }
        if e.Name == "" {
            add(fmt.Sprintf("%s(%s).name", prefix, e.ID), "required field 'name' is missing")
        }
        if e.Phase < 1 {
            add(fmt.Sprintf("%s(%s).phase", prefix, e.ID), "phase must be >= 1")
        }
        if e.ExtensionID == "" {
            add(fmt.Sprintf("%s(%s).extension_id", prefix, e.ID), "required field 'extension_id' is missing")
        } else if len(e.ExtensionID) != 32 {
            add(fmt.Sprintf("%s(%s).extension_id", prefix, e.ID),
                fmt.Sprintf("extension_id must be 32 characters, got %d", len(e.ExtensionID)))
        }
        if ids[e.ID] {
            add(fmt.Sprintf("%s.id", prefix), fmt.Sprintf("duplicate ID %q", e.ID))
        } else {
            ids[e.ID] = true
        }
    }

    // 6. depends_on cross-references (Commands only — Package and Extension have no DependsOn).
    for i, c := range cfg.Commands {
        for _, dep := range c.DependsOn {
            if !ids[dep] {
                add(fmt.Sprintf("commands[%d](%s).depends_on", i, c.ID),
                    fmt.Sprintf("unknown ID %q (not in packages or commands)", dep))
            }
        }
    }

    // 7. Profile ids cross-references.
    for i, p := range cfg.Profiles {
        for _, id := range p.IDs {
            if !ids[id] {
                add(fmt.Sprintf("profiles[%d](%s).ids", i, p.Name),
                    fmt.Sprintf("unknown ID %q", id))
            }
        }
    }

    return errs
}
```

- [ ] **Run test to verify it passes**

```
cd F:/GDriveClone/Claude_Code/KtulueKit-W11 && go test ./internal/config/... -run TestValidate -v
```
Expected: PASS (3 tests).

---

### Task 2: Add remaining `Validate()` tests for all checks

**Files:**
- Modify: `internal/config/validate_test.go`

- [ ] **Add tests for all remaining checks**

Append to `validate_test.go`:

```go
func TestValidate_PackageMissingName(t *testing.T) {
    c := cfg("1.0", "MyKit")
    c.Packages = []Package{{ID: "p1", Phase: 1}} // name missing
    errs := Validate(c)
    if len(errs) == 0 {
        t.Fatal("want error for missing package name")
    }
}

func TestValidate_PackageBadPhase(t *testing.T) {
    c := cfg("1.0", "MyKit")
    c.Packages = []Package{{ID: "p1", Name: "P1", Phase: 0}}
    errs := Validate(c)
    if len(errs) == 0 {
        t.Fatal("want error for phase < 1")
    }
}

func TestValidate_DuplicateIDCrossPackageCommand(t *testing.T) {
    c := cfg("1.0", "MyKit")
    c.Packages = []Package{{ID: "dup", Name: "Dup", Phase: 1}}
    c.Commands = []Command{{ID: "dup", Name: "Dup", Phase: 1, Check: "x", Cmd: "y"}}
    errs := Validate(c)
    if len(errs) == 0 {
        t.Fatal("want error for duplicate ID across tiers")
    }
}

func TestValidate_CommandMissingCheck(t *testing.T) {
    c := cfg("1.0", "MyKit")
    c.Commands = []Command{{ID: "c1", Name: "C1", Phase: 1, Cmd: "echo hi"}} // check missing
    errs := Validate(c)
    if len(errs) == 0 {
        t.Fatal("want error for missing check")
    }
}

func TestValidate_CommandMissingCmd(t *testing.T) {
    c := cfg("1.0", "MyKit")
    c.Commands = []Command{{ID: "c1", Name: "C1", Phase: 1, Check: "echo"}} // cmd missing
    errs := Validate(c)
    if len(errs) == 0 {
        t.Fatal("want error for missing command")
    }
}

func TestValidate_ExtensionBadExtensionID(t *testing.T) {
    c := cfg("1.0", "MyKit")
    c.Extensions = []Extension{{ID: "e1", Name: "E1", Phase: 1, ExtensionID: "tooshort"}}
    errs := Validate(c)
    if len(errs) == 0 {
        t.Fatal("want error for extension_id != 32 chars")
    }
}

func TestValidate_DependsOnUnknownID(t *testing.T) {
    c := cfg("1.0", "MyKit")
    c.Commands = []Command{{
        ID: "c1", Name: "C1", Phase: 1, Check: "x", Cmd: "y",
        DependsOn: []string{"does-not-exist"},
    }}
    errs := Validate(c)
    if len(errs) == 0 {
        t.Fatal("want error for unknown depends_on ID")
    }
}

func TestValidate_DependsOnKnownID(t *testing.T) {
    c := cfg("1.0", "MyKit")
    c.Packages = []Package{{ID: "git", Name: "Git", Phase: 1}}
    c.Commands = []Command{{
        ID: "c1", Name: "C1", Phase: 1, Check: "x", Cmd: "y",
        DependsOn: []string{"git"},
    }}
    errs := Validate(c)
    if len(errs) != 0 {
        t.Errorf("want 0 errors for valid depends_on, got %+v", errs)
    }
}

func TestValidate_ProfileUnknownID(t *testing.T) {
    c := cfg("1.0", "MyKit")
    c.Profiles = []Profile{{Name: "Dev", IDs: []string{"ghost-id"}}}
    errs := Validate(c)
    if len(errs) == 0 {
        t.Fatal("want error for unknown profile ID")
    }
}

func TestValidate_ProfileKnownID(t *testing.T) {
    c := cfg("1.0", "MyKit")
    c.Packages = []Package{{ID: "git", Name: "Git", Phase: 1}}
    c.Profiles = []Profile{{Name: "Dev", IDs: []string{"git"}}}
    errs := Validate(c)
    if len(errs) != 0 {
        t.Errorf("want 0 errors for valid profile ID, got %+v", errs)
    }
}

func TestValidate_CollectsAllErrors(t *testing.T) {
    // Both version and metadata.name missing — should get 2 errors, not fail-fast on 1.
    c := &Config{}
    errs := Validate(c)
    if len(errs) < 2 {
        t.Errorf("want >= 2 errors for empty config, got %d: %+v", len(errs), errs)
    }
}
```

- [ ] **Run tests**

```
cd F:/GDriveClone/Claude_Code/KtulueKit-W11 && go test ./internal/config/... -run TestValidate -v
```
Expected: all PASS.

---

### Task 3: Remove private `validate()` from `loader.go`, make `LoadAll` validation-free

**Files:**
- Modify: `internal/config/loader.go`

- [ ] **Delete the private `validate()` function** (lines 179–251) and its call in `LoadAll` (line 40)

In `loader.go`:

Remove the line:
```go
if err := validate(&merged); err != nil {
    return nil, fmt.Errorf("config validation failed: %w", err)
}
```

Delete the entire `func validate(cfg *Config) error { ... }` block.

Update the `LoadAll` doc comment to remove the reference to `validate()`:
```go
// LoadAll merges one or more config files left-to-right and returns the combined Config.
// Later files override earlier files on ID/name collision (last-wins).
// applyDefaults() is called on the merged result. Validation is NOT performed here —
// callers must call Validate() explicitly after LoadAll.
// If paths is empty, it defaults to ["ktuluekit.json"].
```

- [ ] **Build to verify no compile errors**

```
cd F:/GDriveClone/Claude_Code/KtulueKit-W11 && go build ./... 2>&1
```
Expected: compile error — `cmd/main.go` calls `LoadAll` but expects validation; also `loader_test.go` may fail. That's expected — we'll fix in the next step.

- [ ] **Fix `loader_test.go` — `TestLoadAll_CrossTierIDCollision` relied on `LoadAll` returning a validation error**

In `internal/config/loader_test.go`, update `TestLoadAll_CrossTierIDCollision` (currently at ~line 246):

```go
func TestLoadAll_CrossTierIDCollision(t *testing.T) {
    base := writeJSON(t, `{
        "version": "1.0",
        "metadata": {"name": "Base"},
        "packages": [{"id": "foo", "name": "Foo Package", "phase": 1}]
    }`)
    extras := writeJSON(t, `{
        "version": "1.0",
        "metadata": {"name": "Extras"},
        "commands": [{"id": "foo", "name": "Foo Command", "phase": 2, "check": "foo -v", "command": "install foo"}]
    }`)
    cfg, err := LoadAll([]string{base, extras})
    if err != nil {
        t.Fatalf("LoadAll should not validate: unexpected error %v", err)
    }
    errs := Validate(cfg)
    if len(errs) == 0 {
        t.Fatal("expected Validate to catch cross-tier ID collision, got no errors")
    }
}
```

- [ ] **Update `cmd/main.go` to call `Validate()` explicitly**

In `cmd/main.go`, update `runInstall` after `config.LoadAll`:
```go
cfg, err := config.LoadAll(configPaths)
if err != nil {
    return fmt.Errorf("config error: %w", err)
}
if errs := config.Validate(cfg); len(errs) > 0 {
    return fmt.Errorf("config validation failed: %w", errs[0])
}
if noUpgrade {
    cfg.Settings.UpgradeIfInstalled = false
}
```

Check `internal/config/loader_test.go` — if any test relies on `LoadAll` returning a validation error, update it to call `Validate()` instead.

- [ ] **Run all config and cmd tests**

```
cd F:/GDriveClone/Claude_Code/KtulueKit-W11 && go test ./internal/config/... ./cmd/... -v 2>&1
```
Expected: all PASS.

- [ ] **Commit**

```
cd F:/GDriveClone/Claude_Code/KtulueKit-W11 && git add internal/config/validate.go internal/config/validate_test.go internal/config/loader.go cmd/main.go && git commit -m "feat(config): add public Validate() with full cross-reference checks; remove private validate()"
```

---

## Chunk 2: `validate` and `list` subcommands

### Task 4: Add `validate` subcommand to `cmd/main.go`

**Files:**
- Modify: `cmd/main.go`

- [ ] **Add `runValidate` function and register the subcommand**

In `cmd/main.go`, add after the `statusCmd` block:

```go
validateCmd := &cobra.Command{
    Use:   "validate",
    Short: "Validate config file(s) and report all errors",
    RunE:  runValidate,
}
root.AddCommand(validateCmd)
```

Add `runValidate` function (can go in a new file `cmd/validate.go` or directly in `cmd/main.go` — keep it in `main.go` since it's small, consistent with status pattern of keeping handler in `status.go`; add as new file `cmd/validate.go`):

```go
// cmd/validate.go
package main

import (
    "fmt"

    "github.com/spf13/cobra"

    "github.com/Ktulue/KtulueKit-W11/internal/config"
)

func runValidate(cmd *cobra.Command, args []string) error {
    // configPaths defaults handled internally by LoadAll.
    cfg, err := config.LoadAll(configPaths)
    if err != nil {
        return fmt.Errorf("config parse error: %w", err)
    }

    paths := configPaths
    if len(paths) == 0 {
        paths = []string{"ktuluekit.json"}
    }
    fmt.Printf("Validating config: %v\n", paths)

    errs := config.Validate(cfg)
    if len(errs) == 0 {
        total := len(cfg.Packages) + len(cfg.Commands) + len(cfg.Extensions)
        fmt.Printf("  OK — no errors found (%d packages + %d commands + %d extensions = %d items validated)\n",
            len(cfg.Packages), len(cfg.Commands), len(cfg.Extensions), total)
        return nil
    }

    for _, e := range errs {
        fmt.Printf("  %sERROR%s  %-30s  %s\n", colorRed, colorReset, e.Field, e.Message)
    }
    return fmt.Errorf("%d error(s) found — fix the above before running", len(errs))
}
```

Note: `colorRed` and `colorReset` are already defined in `cmd/status.go` in `package main`.

- [ ] **Build to verify no compile errors**

```
cd F:/GDriveClone/Claude_Code/KtulueKit-W11 && go build ./cmd/... 2>&1
```
Expected: clean.

- [ ] **Add validate subcommand test to `cmd/validate_test.go`**

```go
// cmd/validate_test.go
package main

import (
    "os"
    "path/filepath"
    "testing"

    "github.com/Ktulue/KtulueKit-W11/internal/config"
)

func TestValidateCmd_ValidConfig(t *testing.T) {
    dir := t.TempDir()
    f := filepath.Join(dir, "k.json")
    os.WriteFile(f, []byte(`{
        "version": "1.0",
        "metadata": {"name": "Test"},
        "packages": [{"id": "Git.Git", "name": "Git", "phase": 1}],
        "settings": {}
    }`), 0644)

    cfg, err := config.LoadAll([]string{f})
    if err != nil {
        t.Fatalf("LoadAll: %v", err)
    }
    errs := config.Validate(cfg)
    if len(errs) != 0 {
        t.Errorf("want 0 errors for valid config, got %+v", errs)
    }
}

func TestValidateCmd_InvalidConfig(t *testing.T) {
    dir := t.TempDir()
    f := filepath.Join(dir, "k.json")
    // version missing, duplicate ID
    os.WriteFile(f, []byte(`{
        "metadata": {"name": "Test"},
        "packages": [
            {"id": "dup", "name": "A", "phase": 1},
            {"id": "dup", "name": "B", "phase": 1}
        ],
        "settings": {}
    }`), 0644)

    cfg, err := config.LoadAll([]string{f})
    if err != nil {
        t.Fatalf("LoadAll: %v", err)
    }
    errs := config.Validate(cfg)
    if len(errs) == 0 {
        t.Fatal("want errors for invalid config, got none")
    }
}
```

- [ ] **Run cmd tests**

```
cd F:/GDriveClone/Claude_Code/KtulueKit-W11 && go test ./cmd/... -v 2>&1
```
Expected: PASS.

---

### Task 5: Add `list` subcommand

**Files:**
- Create: `cmd/list.go`

- [ ] **Create `cmd/list.go`**

```go
// cmd/list.go
package main

import (
    "fmt"
    "sort"

    "github.com/spf13/cobra"

    "github.com/Ktulue/KtulueKit-W11/internal/config"
)

func runList(cmd *cobra.Command, args []string) error {
    cfg, err := config.LoadAll(configPaths)
    if err != nil {
        return fmt.Errorf("config error: %w", err)
    }

    // Build phase → items map.
    type listItem struct {
        tier string
        id   string
        name string
    }
    byPhase := make(map[int][]listItem)

    for _, p := range cfg.Packages {
        byPhase[p.Phase] = append(byPhase[p.Phase], listItem{"winget", p.ID, p.Name})
    }
    for _, c := range cfg.Commands {
        byPhase[c.Phase] = append(byPhase[c.Phase], listItem{"command", c.ID, c.Name})
    }
    for _, e := range cfg.Extensions {
        byPhase[e.Phase] = append(byPhase[e.Phase], listItem{"extension", e.ID, e.Name})
    }

    phases := make([]int, 0, len(byPhase))
    for ph := range byPhase {
        phases = append(phases, ph)
    }
    sort.Ints(phases)

    for _, ph := range phases {
        fmt.Printf("\n── Phase %d ──────────────────────────────────────\n", ph)
        for _, item := range byPhase[ph] {
            fmt.Printf("  %-12s  %-40s  %s\n", "["+item.tier+"]", item.id, item.name)
        }
    }

    fmt.Printf("\nTotal: %d winget  |  %d commands  |  %d extensions\n",
        len(cfg.Packages), len(cfg.Commands), len(cfg.Extensions))
    return nil
}
```

- [ ] **Register `list` subcommand in `cmd/main.go`**

Add after `validateCmd`:
```go
listCmd := &cobra.Command{
    Use:   "list",
    Short: "List all configured items grouped by phase and tier",
    RunE:  runList,
}
root.AddCommand(listCmd)
```

- [ ] **Build**

```
cd F:/GDriveClone/Claude_Code/KtulueKit-W11 && go build ./cmd/... 2>&1
```
Expected: clean.

- [ ] **Add list test to `cmd/list_test.go`**

```go
// cmd/list_test.go
package main

import (
    "os"
    "path/filepath"
    "testing"

    "github.com/Ktulue/KtulueKit-W11/internal/config"
)

func TestListCmd_LoadsConfig(t *testing.T) {
    dir := t.TempDir()
    f := filepath.Join(dir, "k.json")
    os.WriteFile(f, []byte(`{
        "version": "1.0",
        "metadata": {"name": "Test"},
        "packages": [{"id": "Git.Git", "name": "Git", "phase": 1}],
        "commands": [{"id": "npm-ts", "name": "TypeScript", "phase": 4, "check": "tsc --v", "command": "npm i -g typescript"}],
        "settings": {}
    }`), 0644)

    cfg, err := config.LoadAll([]string{f})
    if err != nil {
        t.Fatalf("LoadAll: %v", err)
    }
    if len(cfg.Packages) != 1 {
        t.Errorf("want 1 package, got %d", len(cfg.Packages))
    }
    if len(cfg.Commands) != 1 {
        t.Errorf("want 1 command, got %d", len(cfg.Commands))
    }
}
```

- [ ] **Run cmd tests**

```
cd F:/GDriveClone/Claude_Code/KtulueKit-W11 && go test ./cmd/... -v 2>&1
```
Expected: PASS.

- [ ] **Commit**

```
cd F:/GDriveClone/Claude_Code/KtulueKit-W11 && git add cmd/validate.go cmd/validate_test.go cmd/list.go cmd/list_test.go cmd/main.go && git commit -m "feat(cli): add validate and list subcommands"
```

---

## Chunk 3: `--only` and `--exclude` flags

### Task 6: Add `--only` / `--exclude` flags and wire to runner

**Files:**
- Modify: `cmd/main.go`
- Modify: `internal/runner/runner.go` (pre-run summary guard)

- [ ] **Add flag variables and register flags in `cmd/main.go`**

Add to the `var` block:
```go
onlyIDs   string
excludeIDs string
```

Register in `main()` on the root command (install only — not persistent):
```go
root.Flags().StringVar(&onlyIDs, "only", "", "Comma-separated IDs to install; skip all others")
root.Flags().StringVar(&excludeIDs, "exclude", "", "Comma-separated IDs to skip during install")
```

- [ ] **Add ID filtering logic in `runInstall`, after `Validate()` and before runner construction**

Add a helper function (can be in `cmd/main.go`):

```go
// allConfigIDs returns the set of all IDs in the config across all tiers.
func allConfigIDs(cfg *config.Config) map[string]bool {
    ids := make(map[string]bool, len(cfg.Packages)+len(cfg.Commands)+len(cfg.Extensions))
    for _, p := range cfg.Packages {
        ids[p.ID] = true
    }
    for _, c := range cfg.Commands {
        ids[c.ID] = true
    }
    for _, e := range cfg.Extensions {
        ids[e.ID] = true
    }
    return ids
}
```

In `runInstall`, after validation and before `runner.New(...)`:

```go
// Mutual exclusion check.
if onlyIDs != "" && excludeIDs != "" {
    return fmt.Errorf("--only and --exclude are mutually exclusive")
}

var selectedIDs []string

if onlyIDs != "" {
    ids := strings.Split(onlyIDs, ",")
    allIDs := allConfigIDs(cfg)
    for _, id := range ids {
        id = strings.TrimSpace(id)
        if !allIDs[id] {
            fmt.Printf("%s[WARNING]%s --only: unknown ID %q (not in config)\n", colorYellow, colorReset, id)
        }
    }
    selectedIDs = ids
}

if excludeIDs != "" {
    toExclude := make(map[string]bool)
    allIDs := allConfigIDs(cfg)
    for _, id := range strings.Split(excludeIDs, ",") {
        id = strings.TrimSpace(id)
        if !allIDs[id] {
            fmt.Printf("%s[WARNING]%s --exclude: unknown ID %q (not in config)\n", colorYellow, colorReset, id)
        }
        toExclude[id] = true
    }
    for id := range allConfigIDs(cfg) {
        if !toExclude[id] {
            selectedIDs = append(selectedIDs, id)
        }
    }
}
```

- [ ] **Add `"strings"` to imports in `cmd/main.go`** — `strings.Split` and `strings.TrimSpace` are used above; add `"strings"` to the import block if not already present.

After `runner.New(...)`, wire in the selection:
```go
r := runner.New(cfg, rep, s, dryRun, resumePhase, reportingPath[0], shortcutMode)

if len(selectedIDs) > 0 {
    r.SetSelectedIDs(selectedIDs)
}
```

- [ ] **Add the `selectedIDs != nil` pre-run summary guard in `runner.go`**

In `internal/runner/runner.go`, in `printPreRunSummary`, add after the `r.onProgress != nil` guard:

```go
// Skip in filtered-run mode — counts would reflect the full config, not the selection.
if r.selectedIDs != nil {
    return false
}
```

- [ ] **Build**

```
cd F:/GDriveClone/Claude_Code/KtulueKit-W11 && go build ./cmd/... 2>&1
```
Expected: clean.

- [ ] **Add `--only` / `--exclude` tests in `cmd/main_test.go`** (new file or append to existing)

```go
// cmd/filter_test.go
package main

import (
    "os"
    "path/filepath"
    "testing"

    "github.com/Ktulue/KtulueKit-W11/internal/config"
)

func writeTestConfig(t *testing.T) string {
    t.Helper()
    dir := t.TempDir()
    f := filepath.Join(dir, "k.json")
    os.WriteFile(f, []byte(`{
        "version": "1.0",
        "metadata": {"name": "T"},
        "packages": [
            {"id": "a", "name": "A", "phase": 1},
            {"id": "b", "name": "B", "phase": 1},
            {"id": "c", "name": "C", "phase": 2}
        ],
        "settings": {}
    }`), 0644)
    return f
}

func TestAllConfigIDs(t *testing.T) {
    cfg, _ := config.LoadAll([]string{writeTestConfig(t)})
    ids := allConfigIDs(cfg)
    if len(ids) != 3 {
        t.Errorf("want 3 IDs, got %d", len(ids))
    }
    if !ids["a"] || !ids["b"] || !ids["c"] {
        t.Errorf("missing expected IDs in %v", ids)
    }
}
```

- [ ] **Run all tests**

```
cd F:/GDriveClone/Claude_Code/KtulueKit-W11 && go test -count=1 ./cmd/... ./internal/... 2>&1
```
Expected: all PASS.

- [ ] **Commit**

```
cd F:/GDriveClone/Claude_Code/KtulueKit-W11 && git add cmd/main.go cmd/filter_test.go internal/runner/runner.go && git commit -m "feat(cli): add --only and --exclude flags for targeted installs"
```

---

## Chunk 4: Consecutive-failure pause

### Task 7: Add `trackResult()` and `promptConsecutiveFailures()` to runner

**Files:**
- Modify: `internal/runner/runner.go`
- Modify: `internal/runner/runner_test.go`

- [ ] **Write the failing tests first**

Append to `internal/runner/runner_test.go`:

```go
func TestConsecutiveFailures_ThreeFailed_TriggersPause(t *testing.T) {
    r := &Runner{
        cfg:   &config.Config{},
        state: &state.State{Succeeded: make(map[string]bool), Failed: make(map[string]bool)},
    }

    pauseCalled := 0
    r.onPause = func() { pauseCalled++ }

    r.trackResult(reporter.StatusFailed)
    r.trackResult(reporter.StatusFailed)
    r.trackResult(reporter.StatusFailed)

    if pauseCalled != 1 {
        t.Errorf("want pause called once after 3 consecutive failures, got %d", pauseCalled)
    }
    if r.consecutiveFails != 0 {
        t.Errorf("want consecutiveFails reset to 0 after pause, got %d", r.consecutiveFails)
    }
}

func TestConsecutiveFailures_TwoFailedOnePassed_NoPause(t *testing.T) {
    r := &Runner{
        cfg:   &config.Config{},
        state: &state.State{Succeeded: make(map[string]bool), Failed: make(map[string]bool)},
    }

    pauseCalled := 0
    r.onPause = func() { pauseCalled++ }

    r.trackResult(reporter.StatusFailed)
    r.trackResult(reporter.StatusFailed)
    r.trackResult(reporter.StatusInstalled) // resets counter

    if pauseCalled != 0 {
        t.Errorf("want no pause, got %d", pauseCalled)
    }
    if r.consecutiveFails != 0 {
        t.Errorf("want consecutiveFails reset to 0, got %d", r.consecutiveFails)
    }
}

func TestConsecutiveFailures_SkippedCountsAsFail(t *testing.T) {
    r := &Runner{
        cfg:   &config.Config{},
        state: &state.State{Succeeded: make(map[string]bool), Failed: make(map[string]bool)},
    }

    pauseCalled := 0
    r.onPause = func() { pauseCalled++ }

    r.trackResult(reporter.StatusFailed)
    r.trackResult(reporter.StatusSkipped) // dependency skip also counts
    r.trackResult(reporter.StatusSkipped)

    if pauseCalled != 1 {
        t.Errorf("want pause called once (StatusSkipped counts), got %d", pauseCalled)
    }
}

func TestConsecutiveFailures_AlreadyStatusResetsCounter(t *testing.T) {
    r := &Runner{
        cfg:   &config.Config{},
        state: &state.State{Succeeded: make(map[string]bool), Failed: make(map[string]bool)},
    }

    pauseCalled := 0
    r.onPause = func() { pauseCalled++ }

    r.trackResult(reporter.StatusFailed)
    r.trackResult(reporter.StatusFailed)
    r.trackResult(reporter.StatusAlready) // "already installed" resets
    r.trackResult(reporter.StatusFailed)

    if pauseCalled != 0 {
        t.Errorf("want no pause (counter reset by StatusAlready), got %d", pauseCalled)
    }
    if r.consecutiveFails != 1 {
        t.Errorf("want consecutiveFails = 1 after sequence, got %d", r.consecutiveFails)
    }
}
```

- [ ] **Run tests to verify they fail**

```
cd F:/GDriveClone/Claude_Code/KtulueKit-W11 && go test ./internal/runner/... -run TestConsecutiveFailures -v 2>&1
```
Expected: compile error — `trackResult`, `onPause`, `consecutiveFails` undefined.

- [ ] **Add fields and methods to `Runner` struct in `runner.go`**

Add to the `Runner` struct (after `rebootResponse`):
```go
consecutiveFails int                // tracks back-to-back failures/skips
pauseResponse    chan bool          // GUI: send true to continue after a pause
onPause          func()            // test hook: called instead of blocking on stdin
```

Add setter methods:
```go
// SetPauseResponse wires the channel the runner blocks on during a consecutive-failure
// pause in GUI mode. The app sends true to continue. Unlike rebootResponse, this channel
// is reused for multiple pauses — it is never set to nil after use.
func (r *Runner) SetPauseResponse(ch chan bool) {
    r.pauseResponse = ch
}

// SetOnPause wires a test hook that fires instead of blocking on stdin when a
// consecutive-failure pause occurs. Leave nil in production.
func (r *Runner) SetOnPause(fn func()) {
    r.onPause = fn
}
```

Add `trackResult()`:
```go
// trackResult updates the consecutive-failure counter after each fresh install attempt.
// StatusFailed and StatusSkipped increment the counter; anything else resets it.
// State-aware skips (handled before install attempts) must NOT call this method.
// When the counter reaches 3, promptConsecutiveFailures is called and the counter resets.
func (r *Runner) trackResult(status string) {
    switch status {
    case reporter.StatusFailed, reporter.StatusSkipped:
        r.consecutiveFails++
        if r.consecutiveFails >= 3 {
            r.promptConsecutiveFailures()
            r.consecutiveFails = 0
        }
    default:
        r.consecutiveFails = 0
    }
}
```

Add `promptConsecutiveFailures()`:
```go
// promptConsecutiveFailures pauses the run after 3 consecutive failures.
// In test mode (onPause set), calls the hook and returns immediately.
// In GUI mode (pauseResponse set), emits a paused event and blocks on the channel.
// In CLI mode, prints a warning and blocks on stdin.
func (r *Runner) promptConsecutiveFailures() {
    // Test hook — bypasses all blocking.
    if r.onPause != nil {
        r.onPause()
        return
    }

    // GUI mode.
    if r.onProgress != nil {
        r.onProgress(ProgressEvent{
            Index:  r.itemIdx,
            Total:  r.totalItems,
            Status: "paused",
            Detail: "3 consecutive failures",
        })
        if r.pauseResponse != nil {
            <-r.pauseResponse
        }
        return
    }

    // CLI mode.
    sep := strings.Repeat("─", 50)
    fmt.Printf("\n  %s⚠️  3 consecutive failures. Something may be wrong.%s\n", colorYellow, colorReset)
    fmt.Printf("  %s\n", sep)
    fmt.Println("  Press Enter to continue, or Ctrl+C to abort and investigate.")
    reader := bufio.NewReader(os.Stdin)
    reader.ReadString('\n')
}
```

- [ ] **Call `trackResult()` in all three tier-processing functions**

`runCommandsInPhase` has **two** paths that emit a result — both need `trackResult`:

1. **Dependency-skip path** (the `!r.dependenciesMet` block): add `r.trackResult(res.Status)` immediately after `r.rep.Add(res)`, before the `continue`:
```go
r.rep.Add(res)
r.trackResult(res.Status) // ← add here (StatusSkipped counts as failure)
if r.onProgress != nil { ... }
r.state.MarkFailed(cmd.ID)
continue
```

2. **Normal install path**: add `r.trackResult(res.Status)` after `r.rep.Add(res)` and the elapsed/progress lines:
```go
r.rep.Add(res)
elapsed := time.Since(start).Round(time.Second)
fmt.Printf("      elapsed: %s\n", elapsed)
if r.onProgress != nil { ... }
r.trackResult(res.Status) // ← add here
```

In `runPackagesInPhase`, add after `r.rep.Add(res)` and elapsed/progress lines:
```go
r.trackResult(res.Status)
```

In `runExtensionsInPhase`, add after `r.rep.Add(res)` and elapsed/progress lines:
```go
r.trackResult(res.Status)
```

**Do NOT call `trackResult` in the state-aware-skip paths** (the early-continue blocks that check `r.state.Succeeded[id]`). Those are neutral — they neither increment nor reset the counter.

- [ ] **Run the new tests**

```
cd F:/GDriveClone/Claude_Code/KtulueKit-W11 && go test ./internal/runner/... -run TestConsecutiveFailures -v 2>&1
```
Expected: all 4 tests PASS.

- [ ] **Run all tests**

```
cd F:/GDriveClone/Claude_Code/KtulueKit-W11 && go test -count=1 ./cmd/... ./internal/... 2>&1
```
Expected: all PASS.

- [ ] **Commit**

```
cd F:/GDriveClone/Claude_Code/KtulueKit-W11 && git add internal/runner/runner.go internal/runner/runner_test.go && git commit -m "feat(runner): pause after 3 consecutive failures with GUI and CLI support"
```

---

## Chunk 5: Final wiring, TODO update, branch and PR

### Task 8: Update TODO.md, verify full build, branch, push

- [ ] **Mark TODO items done in `TODO.md`**

Mark these items as `[x]`:
- `validate` subcommand
- `list` subcommand
- `--only <ids>` flag
- `--exclude <ids>` flag

- [ ] **Final build and test run**

```
cd F:/GDriveClone/Claude_Code/KtulueKit-W11 && go build ./cmd/... 2>&1 && go test -count=1 ./cmd/... ./internal/... 2>&1
```
Expected: clean build, all tests PASS.

- [ ] **Create branch, squash-review, push**

```
cd F:/GDriveClone/Claude_Code/KtulueKit-W11 && git checkout -b feat/cli-features 2>&1
```

Push and open PR targeting main.
