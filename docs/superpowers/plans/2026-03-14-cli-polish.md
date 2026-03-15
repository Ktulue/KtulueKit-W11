# CLI Polish Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add post-install hooks, --profile flag, and --output-format json|md to the CLI.

**Architecture:** Three additive features on a single branch. Schema gets two new fields; Reporter gains a progressWriter abstraction; cmd/main.go wires new flags. All new code is tested in this branch.

**Tech Stack:** Go 1.25, Cobra, standard library only.

---

## Pre-work

- [ ] Create the feature branch: `git checkout -b feat/cli-polish`
- [ ] Verify build is green before touching anything: `go build ./...` and `go test ./...`

---

## Chunk 1: Schema + Reporter

### Task 1: Add `post_install` field to schema (Package + Command)

**Files:**
- Modify: `internal/config/schema.go`
- Test: `internal/config/validate_test.go` (add round-trip test)

#### Steps

- [ ] **Write the failing test**

  Add to `internal/config/validate_test.go`:

  ```go
  func TestPostInstallFieldRoundTrip(t *testing.T) {
      // Verify the JSON tag is correct by marshalling and unmarshalling.
      input := `{
          "id": "Git.Git", "name": "Git", "phase": 1,
          "post_install": "echo done"
      }`
      var pkg config.Package
      if err := json.Unmarshal([]byte(input), &pkg); err != nil {
          t.Fatalf("Unmarshal Package failed: %v", err)
      }
      if pkg.PostInstall != "echo done" {
          t.Errorf("Package.PostInstall: want %q, got %q", "echo done", pkg.PostInstall)
      }

      cmdInput := `{
          "id": "wsl2", "name": "WSL 2", "phase": 1,
          "command": "wsl --install", "post_install": "wsl --version"
      }`
      var cmd config.Command
      if err := json.Unmarshal([]byte(cmdInput), &cmd); err != nil {
          t.Fatalf("Unmarshal Command failed: %v", err)
      }
      if cmd.PostInstall != "wsl --version" {
          t.Errorf("Command.PostInstall: want %q, got %q", "wsl --version", cmd.PostInstall)
      }
  }
  ```

  Add `"encoding/json"` to the imports in `validate_test.go` if not already present.

- [ ] **Run test to verify it fails**

  ```
  go test ./internal/config/... -run TestPostInstallFieldRoundTrip -v
  ```

  Expected output: `# internal/config [build failed]` — `PostInstall` field does not exist yet.

- [ ] **Write minimal implementation**

  In `internal/config/schema.go`, add `PostInstall string` to `Package` after `Notes`:

  ```go
  // Package is a Tier 1 winget package.
  type Package struct {
      ID             string `json:"id"`
      Name           string `json:"name"`
      Phase          int    `json:"phase"`
      Category       string `json:"category"`
      Description    string `json:"description"`
      Scope          string `json:"scope"`
      Check          string `json:"check"`
      Version        string `json:"version"`
      RebootAfter    bool   `json:"reboot_after"`
      TimeoutSeconds int    `json:"timeout_seconds"`
      Notes          string `json:"notes"`
      PostInstall    string `json:"post_install,omitempty"` // Optional shell command run after StatusInstalled or StatusUpgraded
  }
  ```

  Add `PostInstall string` to `Command` after `InstallArgs`:

  ```go
  PostInstall string `json:"post_install,omitempty"` // Optional shell command run after StatusInstalled or StatusUpgraded
  ```

- [ ] **Run test to verify it passes**

  ```
  go test ./internal/config/... -run TestPostInstallFieldRoundTrip -v
  ```

  Expected output: `--- PASS: TestPostInstallFieldRoundTrip`

- [ ] **Build check:** `go build ./...`

- [ ] **Commit**

  ```
  git add internal/config/schema.go internal/config/validate_test.go
  git commit -m "feat(schema): add post_install field to Package and Command"
  ```

---

### Task 2: Add `progressWriter io.Writer` to Reporter + route all progress writes through it

**Files:**
- Modify: `internal/reporter/reporter.go`
- Modify: `internal/reporter/reporter_test.go`

The goal is to decouple all live-progress output from `os.Stdout` so cmd/main.go can redirect it to `os.Stderr` when `--output-format` is set, keeping `os.Stdout` clean for the final structured summary.

**Scope of "all progress writes":** Every `fmt.Printf`/`fmt.Println`/`fmt.Fprintln` in `reporter.go` that currently writes to stdout routes through `progressWriter`. The log file writes (`fmt.Fprintln(r.logFile, ...)`) remain unchanged — they always go to the log file.

Writes in `reporter.go` that must route through `progressWriter`:
- `New()`: `fmt.Printf("Logging to: %s\n\n", logPath)` → `fmt.Fprintf(r.progressWriter, ...)`
- `Add()`: `fmt.Println(line)` → `fmt.Fprintln(r.progressWriter, line)`
- `Summary()`: all `fmt.Println(header)`, `fmt.Println(heading)`, `fmt.Println(line)`, `fmt.Println()` calls

**Note:** Writes in `cmd/main.go`, `runner/runner.go`, and other packages that call `fmt.Printf` directly are NOT in scope for this task — those are runner-layer concerns. This task only covers `reporter.go`.

#### Steps

- [ ] **Write the failing test**

  Add to `internal/reporter/reporter_test.go`:

  ```go
  import (
      "bytes"
      "strings"
      "testing"
  )

  func TestProgressWriterCapture(t *testing.T) {
      var buf bytes.Buffer
      r := &Reporter{progressWriter: &buf}
      r.results = []Result{
          {ID: "Git.Git", Name: "Git", Tier: "winget", Status: StatusInstalled},
      }
      r.Add(Result{ID: "Git.Git", Name: "Git", Tier: "winget", Status: StatusInstalled})

      if !strings.Contains(buf.String(), "Git") {
          t.Errorf("expected progress output to contain 'Git', got: %q", buf.String())
      }
  }

  func TestProgressWriterDefault(t *testing.T) {
      // Reporter with nil logFile and nil progressWriter should not panic on Add.
      // After the change, nil progressWriter must default to os.Stdout internally.
      r := &Reporter{progressWriter: nil}
      // We cannot capture os.Stdout in a unit test without pipe tricks;
      // this test just verifies no panic occurs.
      defer func() {
          if p := recover(); p != nil {
              t.Fatalf("Add() panicked with nil progressWriter: %v", p)
          }
      }()
      r.Add(Result{ID: "test", Name: "Test", Tier: "winget", Status: StatusInstalled})
  }
  ```

- [ ] **Run test to verify it fails**

  ```
  go test ./internal/reporter/... -run TestProgressWriter -v
  ```

  Expected output: `# internal/reporter [build failed]` — `progressWriter` field does not exist yet.

- [ ] **Write minimal implementation**

  In `internal/reporter/reporter.go`:

  1. Add `"io"` to the import block (`"os"` is already present; `"io"` must be added).

  2. Add `progressWriter io.Writer` field to `Reporter`:

     ```go
     type Reporter struct {
         results        []Result
         logDir         string
         logFile        *os.File
         progressWriter io.Writer
     }
     ```

  3. Update `New()` signature and body:

     ```go
     // New creates a Reporter that writes live progress to progressWriter.
     // Pass os.Stdout for normal CLI use; pass os.Stderr when --output-format is set.
     func New(logDir string, progressWriter io.Writer) (*Reporter, error) {
         if progressWriter == nil {
             progressWriter = os.Stdout
         }
         if err := os.MkdirAll(logDir, 0755); err != nil {
             return nil, fmt.Errorf("cannot create log directory: %w", err)
         }

         timestamp := time.Now().Format("2006-01-02_15-04-05")
         logPath := filepath.Join(logDir, fmt.Sprintf("KtulueKit_%s_results.log", timestamp))

         f, err := os.Create(logPath)
         if err != nil {
             return nil, fmt.Errorf("cannot create log file: %w", err)
         }

         fmt.Fprintf(f, "KtulueKit-W11 Install Log — %s\n", time.Now().Format(time.RFC1123))
         fmt.Fprintln(f, strings.Repeat("=", 60))
         fmt.Fprintln(f)

         fmt.Fprintf(progressWriter, "Logging to: %s\n\n", logPath)

         return &Reporter{logDir: logDir, logFile: f, progressWriter: progressWriter}, nil
     }
     ```

  4. Update `Add()` to write to `progressWriter`:

     ```go
     func (r *Reporter) Add(res Result) {
         r.results = append(r.results, res)

         icon := statusIcon(res.Status)
         line := fmt.Sprintf("  %s  %-40s  [%s]", icon, res.Name, res.Tier)
         if res.Detail != "" {
             line += fmt.Sprintf("  — %s", res.Detail)
         }

         w := r.progressWriter
         if w == nil {
             w = os.Stdout
         }
         fmt.Fprintln(w, line)
         if r.logFile != nil {
             fmt.Fprintln(r.logFile, line)
         }
     }
     ```

  5. Update `Summary()` — replace every `fmt.Println(...)` with `fmt.Fprintln(w, ...)` and every `fmt.Printf(...)` with `fmt.Fprintf(w, ...)`, where `w := r.progressWriter; if w == nil { w = os.Stdout }` is declared at the top of the function. The `fmt.Fprintln(r.logFile, ...)` calls remain as-is.

  6. Update all call sites in `cmd/main.go` — `reporter.New(cfg.Settings.LogDir)` becomes `reporter.New(cfg.Settings.LogDir, os.Stdout)`. (The `--output-format` wiring is Task 4 and will change this again; for now use `os.Stdout`.)

- [ ] **Run test to verify it passes**

  ```
  go test ./internal/reporter/... -run TestProgressWriter -v
  ```

  Expected output:
  ```
  --- PASS: TestProgressWriterCapture
  --- PASS: TestProgressWriterDefault
  ```

- [ ] **Verify the existing NamesBy test still passes:**

  ```
  go test ./internal/reporter/... -v
  ```

- [ ] **Build check:** `go build ./...`

- [ ] **Commit**

  ```
  git add internal/reporter/reporter.go internal/reporter/reporter_test.go cmd/main.go
  git commit -m "feat(reporter): add progressWriter abstraction; route all live output through it"
  ```

---

### Task 3: Add `SummaryJSON()` and `SummaryMD()` to Reporter

**Files:**
- Modify: `internal/reporter/reporter.go`
- Modify: `internal/reporter/reporter_test.go`

`SummaryJSON()` returns the results as a JSON byte slice. `SummaryMD()` returns a Markdown string. Both write to the log file as a side effect.

#### Steps

- [ ] **Write the failing test**

  Add to `internal/reporter/reporter_test.go`:

  ```go
  import (
      "encoding/json"
      // ...existing imports...
  )

  func TestSummaryJSON(t *testing.T) {
      r := &Reporter{}
      r.results = []Result{
          {ID: "Git.Git", Name: "Git", Tier: "winget", Status: StatusInstalled},
          {ID: "wsl2", Name: "WSL 2", Tier: "command", Status: StatusFailed, Detail: "exit code 1"},
      }

      data, err := r.SummaryJSON()
      if err != nil {
          t.Fatalf("SummaryJSON() returned error: %v", err)
      }

      var out struct {
          Results []Result `json:"results"`
      }
      if err := json.Unmarshal(data, &out); err != nil {
          t.Fatalf("SummaryJSON() output is not valid JSON: %v", err)
      }
      if len(out.Results) != 2 {
          t.Errorf("want 2 results, got %d", len(out.Results))
      }
      if out.Results[0].ID != "Git.Git" {
          t.Errorf("want first result ID 'Git.Git', got %q", out.Results[0].ID)
      }
      if out.Results[1].Status != StatusFailed {
          t.Errorf("want second result status 'failed', got %q", out.Results[1].Status)
      }
  }

  func TestSummaryMD(t *testing.T) {
      r := &Reporter{}
      r.results = []Result{
          {ID: "Git.Git", Name: "Git", Tier: "winget", Status: StatusInstalled},
          {ID: "wsl2", Name: "WSL 2", Tier: "command", Status: StatusFailed, Detail: "exit code 1"},
      }

      md := r.SummaryMD()
      if !strings.Contains(md, "# KtulueKit Install Summary") {
          t.Error("SummaryMD() output missing expected H1 heading")
      }
      if !strings.Contains(md, "Git") {
          t.Error("SummaryMD() output missing 'Git'")
      }
      if !strings.Contains(md, "Installed successfully") {
          t.Error("SummaryMD() output missing installed section heading")
      }
      if !strings.Contains(md, "Failed") {
          t.Error("SummaryMD() output missing failed section heading")
      }
  }
  ```

- [ ] **Run test to verify it fails**

  ```
  go test ./internal/reporter/... -run TestSummaryJSON -v
  go test ./internal/reporter/... -run TestSummaryMD -v
  ```

  Expected output: `# internal/reporter [build failed]` — methods do not exist yet.

- [ ] **Write minimal implementation**

  Add the following to `internal/reporter/reporter.go`. Add `"encoding/json"` to the import block.

  ```go
  // summaryJSON is the envelope type for JSON output.
  type summaryJSON struct {
      Results []Result `json:"results"`
  }

  // SummaryJSON serializes all results to JSON and also writes to the log file.
  // Returns the raw JSON bytes for cmd/main.go to write to os.Stdout.
  func (r *Reporter) SummaryJSON() ([]byte, error) {
      payload := summaryJSON{Results: r.results}
      if payload.Results == nil {
          payload.Results = []Result{}
      }
      data, err := json.MarshalIndent(payload, "", "  ")
      if err != nil {
          return nil, err
      }
      data = append(data, '\n')
      if r.logFile != nil {
          fmt.Fprintln(r.logFile, "\n--- JSON SUMMARY ---")
          fmt.Fprintln(r.logFile, string(data))
      }
      return data, nil
  }

  // SummaryMD returns the install summary as a Markdown string and also writes to the log file.
  // Returns the Markdown string for cmd/main.go to write to os.Stdout.
  func (r *Reporter) SummaryMD() string {
      sections := []struct {
          status string
          label  string
      }{
          {StatusInstalled,       "Installed successfully"},
          {StatusUpgraded,        "Updated to newer version"},
          {StatusAlready,         "Already installed (skipped)"},
          {StatusDryRun,          "Would install (dry run)"},
          {StatusFailed,          "Failed"},
          {StatusSkipped,         "Skipped (dependency missing)"},
          {StatusReboot,          "Reboot required"},
          {StatusShortcutRemoved, "Desktop shortcuts removed"},
      }

      var sb strings.Builder
      sb.WriteString("# KtulueKit Install Summary\n\n")

      for _, s := range sections {
          items := r.filterBy(s.status)
          if len(items) == 0 {
              continue
          }
          fmt.Fprintf(&sb, "## %s (%d)\n\n", s.label, len(items))
          for _, res := range items {
              line := fmt.Sprintf("- **%s** (`%s`)", res.Name, res.ID)
              if res.Detail != "" {
                  line += fmt.Sprintf(": %s", res.Detail)
              }
              sb.WriteString(line + "\n")
          }
          sb.WriteString("\n")
      }

      md := sb.String()
      if r.logFile != nil {
          fmt.Fprintln(r.logFile, "\n--- MARKDOWN SUMMARY ---")
          fmt.Fprintln(r.logFile, md)
      }
      return md
  }
  ```

- [ ] **Run test to verify it passes**

  ```
  go test ./internal/reporter/... -run TestSummaryJSON -v
  go test ./internal/reporter/... -run TestSummaryMD -v
  ```

  Expected output:
  ```
  --- PASS: TestSummaryJSON
  --- PASS: TestSummaryMD
  ```

- [ ] **Run full reporter suite:** `go test ./internal/reporter/... -v`

- [ ] **Build check:** `go build ./...`

- [ ] **Commit**

  ```
  git add internal/reporter/reporter.go internal/reporter/reporter_test.go
  git commit -m "feat(reporter): add SummaryJSON() and SummaryMD() export methods"
  ```

---

### Task 4: Add `--output-format` flag to install command; wire `progressWriter` in cmd/main.go

**Files:**
- Modify: `cmd/main.go`
- Test: `cmd/filter_test.go` (add output format validation helper test)

`--output-format` is a root-level flag on the `install` subcommand (which is the root cobra command itself). Valid values: `json`, `md`, or empty string (default terminal output). When set, `reporter.New` is called with `os.Stderr` as `progressWriter`. After `rep.Summary()` is called, the format-specific output is written to `os.Stdout`.

#### Steps

- [ ] **Write the failing test**

  Add to `cmd/filter_test.go`:

  ```go
  func TestOutputFormatFlagsError_InvalidFormat(t *testing.T) {
      if err := outputFormatError("xml"); err == nil {
          t.Fatal("want error for unsupported format 'xml', got nil")
      }
  }

  func TestOutputFormatFlagsError_ValidFormats(t *testing.T) {
      for _, f := range []string{"", "json", "md"} {
          if err := outputFormatError(f); err != nil {
              t.Errorf("want no error for format %q, got %v", f, err)
          }
      }
  }
  ```

- [ ] **Run test to verify it fails**

  ```
  go test ./cmd/... -run TestOutputFormatFlags -v
  ```

  Expected output: `# command-line-arguments [build failed]` — `outputFormatError` does not exist yet.

- [ ] **Write minimal implementation**

  In `cmd/main.go`:

  1. Add the package-level var:

     ```go
     var outputFormat string // "json" | "md" | "" (default terminal)
     ```

  2. Register the flag on the root command (before `root.AddCommand` calls):

     ```go
     root.Flags().StringVar(&outputFormat, "output-format", "", `Summary format: "json" or "md". Progress goes to stderr; summary goes to stdout.`)
     ```

     Note: Use `root.Flags()` (not `PersistentFlags()`) — this flag lives only on the install command, not on subcommands.

  3. Add the validator:

     ```go
     // outputFormatError returns an error if the requested format is not supported.
     func outputFormatError(format string) error {
         switch format {
         case "", "json", "md":
             return nil
         default:
             return fmt.Errorf("--output-format %q is not supported; valid values: json, md", format)
         }
     }
     ```

  4. In `runInstall()`, after the existing flag validation block, add:

     ```go
     if err := outputFormatError(outputFormat); err != nil {
         return err
     }
     ```

  5. Replace the `reporter.New` call in `runInstall()`:

     ```go
     progressOut := io.Writer(os.Stdout)
     if outputFormat != "" {
         progressOut = os.Stderr
     }
     rep, err := reporter.New(cfg.Settings.LogDir, progressOut)
     ```

     Add `"io"` to the import block.

  6. After `rep.Summary()` and the elapsed-time print, add the format-specific output block:

     ```go
     switch outputFormat {
     case "json":
         data, err := rep.SummaryJSON()
         if err != nil {
             fmt.Fprintf(os.Stderr, "error marshalling JSON summary: %v\n", err)
         } else {
             os.Stdout.Write(data)
         }
     case "md":
         fmt.Fprint(os.Stdout, rep.SummaryMD())
     }
     ```

- [ ] **Run test to verify it passes**

  ```
  go test ./cmd/... -run TestOutputFormatFlags -v
  ```

  Expected output:
  ```
  --- PASS: TestOutputFormatFlagsError_InvalidFormat
  --- PASS: TestOutputFormatFlagsError_ValidFormats
  ```

- [ ] **Run full cmd suite:** `go test ./cmd/... -v`

- [ ] **Build check:** `go build ./...`

- [ ] **Commit**

  ```
  git add cmd/main.go cmd/filter_test.go
  git commit -m "feat(cmd): add --output-format flag; wire progressWriter for json/md summary output"
  ```

---

## Chunk 2: Flags + Runner

### Task 5: Add `LookupProfile()` helper to config package

**Files:**
- Create: `internal/config/profile.go`
- Create: `internal/config/profile_test.go`

`LookupProfile(cfg *Config, name string) ([]string, error)` returns the IDs slice for the named profile. Returns an error if not found. Case-sensitive.

#### Steps

- [ ] **Write the failing test**

  Create `internal/config/profile_test.go`:

  ```go
  package config

  import "testing"

  func TestLookupProfile_Found(t *testing.T) {
      cfg := &Config{
          Profiles: []Profile{
              {Name: "streaming", IDs: []string{"OBSProject.OBSStudio", "Spotify.Spotify"}},
              {Name: "dev", IDs: []string{"Git.Git", "Microsoft.VisualStudioCode"}},
          },
      }

      ids, err := LookupProfile(cfg, "streaming")
      if err != nil {
          t.Fatalf("LookupProfile returned unexpected error: %v", err)
      }
      if len(ids) != 2 {
          t.Fatalf("want 2 IDs, got %d", len(ids))
      }
      if ids[0] != "OBSProject.OBSStudio" {
          t.Errorf("want first ID 'OBSProject.OBSStudio', got %q", ids[0])
      }
  }

  func TestLookupProfile_NotFound(t *testing.T) {
      cfg := &Config{
          Profiles: []Profile{
              {Name: "streaming", IDs: []string{"OBSProject.OBSStudio"}},
          },
      }

      _, err := LookupProfile(cfg, "Streaming") // wrong case
      if err == nil {
          t.Fatal("want error for unknown profile name, got nil")
      }
  }

  func TestLookupProfile_EmptyProfiles(t *testing.T) {
      cfg := &Config{}
      _, err := LookupProfile(cfg, "anything")
      if err == nil {
          t.Fatal("want error for empty profiles list, got nil")
      }
  }

  func TestLookupProfile_CaseSensitive(t *testing.T) {
      cfg := &Config{
          Profiles: []Profile{
              {Name: "Dev", IDs: []string{"Git.Git"}},
          },
      }
      // "Dev" exists but "dev" (lowercase) should not match.
      _, err := LookupProfile(cfg, "dev")
      if err == nil {
          t.Fatal("want error: profile lookup is case-sensitive")
      }
      // Exact case must work.
      ids, err := LookupProfile(cfg, "Dev")
      if err != nil {
          t.Fatalf("want no error for exact case match, got %v", err)
      }
      if len(ids) != 1 {
          t.Errorf("want 1 ID, got %d", len(ids))
      }
  }
  ```

- [ ] **Run test to verify it fails**

  ```
  go test ./internal/config/... -run TestLookupProfile -v
  ```

  Expected output: `# internal/config [build failed]` — `LookupProfile` does not exist yet.

- [ ] **Write minimal implementation**

  Create `internal/config/profile.go`:

  ```go
  package config

  import "fmt"

  // LookupProfile returns the IDs slice for the named profile.
  // Returns an error if no profile with that exact name exists (case-sensitive).
  func LookupProfile(cfg *Config, name string) ([]string, error) {
      for _, p := range cfg.Profiles {
          if p.Name == name {
              return p.IDs, nil
          }
      }
      return nil, fmt.Errorf("profile %q not found (available: %v)", name, profileNames(cfg))
  }

  // profileNames returns the names of all profiles in cfg, for use in error messages.
  func profileNames(cfg *Config) []string {
      names := make([]string, len(cfg.Profiles))
      for i, p := range cfg.Profiles {
          names[i] = p.Name
      }
      return names
  }
  ```

- [ ] **Run test to verify it passes**

  ```
  go test ./internal/config/... -run TestLookupProfile -v
  ```

  Expected output:
  ```
  --- PASS: TestLookupProfile_Found
  --- PASS: TestLookupProfile_NotFound
  --- PASS: TestLookupProfile_EmptyProfiles
  --- PASS: TestLookupProfile_CaseSensitive
  ```

- [ ] **Run full config suite:** `go test ./internal/config/... -v`

- [ ] **Build check:** `go build ./...`

- [ ] **Commit**

  ```
  git add internal/config/profile.go internal/config/profile_test.go
  git commit -m "feat(config): add LookupProfile() helper for named profile resolution"
  ```

---

### Task 6: Add `--profile` flag to install subcommand

**Files:**
- Modify: `cmd/main.go`
- Modify: `cmd/filter_test.go`

`--profile <name>` resolves to `onlyIDs` internally via `config.LookupProfile()`. Mutually exclusive with `--only` (error). Compatible with `--exclude`.

#### Steps

- [ ] **Write the failing test**

  Add to `cmd/filter_test.go`:

  ```go
  func TestProfileFlagsMutualExclusion(t *testing.T) {
      if err := profileFlagsError("streaming", "Git.Git"); err == nil {
          t.Fatal("want error when --profile and --only are both set, got nil")
      }
  }

  func TestProfileFlagsError_ProfileOnly(t *testing.T) {
      if err := profileFlagsError("streaming", ""); err != nil {
          t.Fatalf("want no error with only --profile set, got %v", err)
      }
  }

  func TestProfileFlagsError_NeitherSet(t *testing.T) {
      if err := profileFlagsError("", ""); err != nil {
          t.Fatalf("want no error with neither flag set, got %v", err)
      }
  }
  ```

- [ ] **Run test to verify it fails**

  ```
  go test ./cmd/... -run TestProfileFlags -v
  ```

  Expected output: `# command-line-arguments [build failed]` — `profileFlagsError` does not exist yet.

- [ ] **Write minimal implementation**

  In `cmd/main.go`:

  1. Add package-level var:

     ```go
     var profileName string
     ```

  2. Register on root persistent flags (so install, status, export all inherit it):

     ```go
     root.PersistentFlags().StringVar(&profileName, "profile", "", "Named profile from config to install (mutually exclusive with --only)")
     ```

  3. Add the validator function:

     ```go
     // profileFlagsError returns an error if --profile and --only are both set.
     func profileFlagsError(profile, only string) error {
         if profile != "" && only != "" {
             return fmt.Errorf("--profile and --only are mutually exclusive")
         }
         return nil
     }
     ```

  4. In `runInstall()`, after the `filterFlagsError` call, add:

     ```go
     if err := profileFlagsError(profileName, onlyIDs); err != nil {
         return err
     }
     if profileName != "" {
         ids, err := config.LookupProfile(cfg, profileName)
         if err != nil {
             return err
         }
         onlyIDs = strings.Join(ids, ",")
     }
     ```

     This runs before the existing `buildOnlySet` block that reads `onlyIDs`, so the profile IDs flow through the existing filter mechanism transparently.

- [ ] **Run test to verify it passes**

  ```
  go test ./cmd/... -run TestProfileFlags -v
  ```

  Expected output:
  ```
  --- PASS: TestProfileFlagsMutualExclusion
  --- PASS: TestProfileFlagsError_ProfileOnly
  --- PASS: TestProfileFlagsError_NeitherSet
  ```

- [ ] **Run full cmd suite:** `go test ./cmd/... -v`

- [ ] **Build check:** `go build ./...`

- [ ] **Commit**

  ```
  git add cmd/main.go cmd/filter_test.go
  git commit -m "feat(cmd): add --profile flag to install; resolves to --only internally"
  ```

---

### Task 7: Add `--profile` flag to status and export subcommands

**Files:**
- Modify: `cmd/status.go`
- Modify: `cmd/export.go`
- Modify: `cmd/status_test.go` (or create alongside export test)

Because `--profile` is registered as a `PersistentFlag` on root in Task 6, the `profileName` var is already available in `runStatus` and `runExport`. This task wires the filtering logic into those two handlers.

**Status filtering:** Filter the `detector.Item` slice returned by `detector.FlattenItems(cfg)` to only items whose ID is in the profile's IDs.

**Export filtering:** Pre-filter `cfg.Packages`, `cfg.Commands`, and `cfg.Extensions` slices to only IDs in the profile before calling `exporter.Export(cfg, opts)`. T4 scrape-download items that are in Commands will be included if their ID is in the profile; items with only `ScrapeURL` set are Commands and follow the same ID filter — no special handling needed. Items with no matching IDs in the profile are silently omitted (same as --only behavior on install).

#### Steps

- [ ] **Write the failing tests**

  Add to `cmd/status_test.go` (or create it):

  ```go
  package main

  import (
      "testing"

      "github.com/Ktulue/KtulueKit-W11/internal/config"
      "github.com/Ktulue/KtulueKit-W11/internal/detector"
  )

  func TestFilterItemsByIDs(t *testing.T) {
      items := []detector.Item{
          {ID: "Git.Git", Name: "Git"},
          {ID: "OBSProject.OBSStudio", Name: "OBS"},
          {ID: "wsl2", Name: "WSL 2"},
      }
      want := map[string]bool{"Git.Git": true, "wsl2": true}

      got := filterItemsByIDs(items, want)
      if len(got) != 2 {
          t.Fatalf("want 2 items, got %d", len(got))
      }
      for _, item := range got {
          if !want[item.ID] {
              t.Errorf("unexpected item %q in filtered result", item.ID)
          }
      }
  }

  func TestFilterConfigByIDs(t *testing.T) {
      cfg := &config.Config{
          Packages:   []config.Package{{ID: "Git.Git"}, {ID: "7zip.7zip"}},
          Commands:   []config.Command{{ID: "wsl2"}, {ID: "npm-global"}},
          Extensions: []config.Extension{{ID: "ext1"}},
      }
      ids := []string{"Git.Git", "wsl2"}

      filterConfigByIDs(cfg, ids)

      if len(cfg.Packages) != 1 || cfg.Packages[0].ID != "Git.Git" {
          t.Errorf("unexpected packages after filter: %v", cfg.Packages)
      }
      if len(cfg.Commands) != 1 || cfg.Commands[0].ID != "wsl2" {
          t.Errorf("unexpected commands after filter: %v", cfg.Commands)
      }
      if len(cfg.Extensions) != 0 {
          t.Errorf("want 0 extensions after filter, got %d", len(cfg.Extensions))
      }
  }
  ```

- [ ] **Run test to verify it fails**

  ```
  go test ./cmd/... -run TestFilterItemsByIDs -v
  go test ./cmd/... -run TestFilterConfigByIDs -v
  ```

  Expected output: `# command-line-arguments [build failed]` — helpers do not exist yet.

- [ ] **Write minimal implementation**

  Add two helper functions to `cmd/main.go` (they are used by both status and export):

  ```go
  // filterItemsByIDs returns only those items whose ID is in the allowlist.
  func filterItemsByIDs(items []detector.Item, ids map[string]bool) []detector.Item {
      out := make([]detector.Item, 0, len(items))
      for _, item := range items {
          if ids[item.ID] {
              out = append(out, item)
          }
      }
      return out
  }

  // filterConfigByIDs mutates cfg to include only Packages, Commands, and Extensions
  // whose IDs appear in the ids slice. Used by --profile on export and status.
  func filterConfigByIDs(cfg *config.Config, ids []string) {
      allow := make(map[string]bool, len(ids))
      for _, id := range ids {
          allow[id] = true
      }

      filtered := cfg.Packages[:0]
      for _, p := range cfg.Packages {
          if allow[p.ID] {
              filtered = append(filtered, p)
          }
      }
      cfg.Packages = filtered

      filteredCmds := cfg.Commands[:0]
      for _, c := range cfg.Commands {
          if allow[c.ID] {
              filteredCmds = append(filteredCmds, c)
          }
      }
      cfg.Commands = filteredCmds

      filteredExts := cfg.Extensions[:0]
      for _, e := range cfg.Extensions {
          if allow[e.ID] {
              filteredExts = append(filteredExts, e)
          }
      }
      cfg.Extensions = filteredExts
  }
  ```

  Wire into `runStatus()` — after loading `cfg` and before calling `detector.FlattenItems`:

  ```go
  if profileName != "" {
      if err := profileFlagsError(profileName, onlyIDs); err != nil {
          return err
      }
      ids, err := config.LookupProfile(cfg, profileName)
      if err != nil {
          return err
      }
      filterConfigByIDs(cfg, ids)
  }
  ```

  Wire into `runExport()` — after validation and before calling `exporter.Export`:

  ```go
  if profileName != "" {
      if err := profileFlagsError(profileName, ""); err != nil { // --only doesn't exist on export
          return err
      }
      ids, err := config.LookupProfile(cfg, profileName)
      if err != nil {
          return err
      }
      filterConfigByIDs(cfg, ids)
  }
  ```

  Place the two helper functions in a new file `cmd/helpers.go` (`package main`) rather than in `cmd/main.go`. This keeps `main.go` from growing and avoids adding a `detector` import to `main.go` — the import is already present in `status.go` and `export.go` which call `filterItemsByIDs` from the shared `package main` build unit.

- [ ] **Run test to verify it passes**

  ```
  go test ./cmd/... -run TestFilterItemsByIDs -v
  go test ./cmd/... -run TestFilterConfigByIDs -v
  ```

  Expected output:
  ```
  --- PASS: TestFilterItemsByIDs
  --- PASS: TestFilterConfigByIDs
  ```

- [ ] **Run full cmd suite:** `go test ./cmd/... -v`

- [ ] **Build check:** `go build ./...`

- [ ] **Commit**

  ```
  git add cmd/helpers.go cmd/status.go cmd/export.go cmd/status_test.go cmd/main.go
  git commit -m "feat(cmd): wire --profile to status and export subcommands"
  ```

---

### Task 8: Add post-install hook execution to runner

**Files:**
- Modify: `internal/runner/runner.go`
- Modify: `internal/runner/runner_test.go`

After a successful `InstallPackage` or `RunCommand` call returns `StatusInstalled` or `StatusUpgraded`, call a `runPostInstall` helper. Hook failures are warnings only — they do not change the result status and do not increment `consecutiveFails`. The hook runs via `cmd /C` using the existing `runShellWithTimeout` pattern from the installer package. Since `runShellWithTimeout` lives in `internal/installer` (unexported), the runner should call `installer.RunHook` — a new thin exported wrapper added to the installer package.

**Why a wrapper, not a direct `exec.Command`?** Keeps the shell-execution pattern in one place and lets tests verify the hook was attempted without needing to mock OS-level processes.

#### Steps

- [ ] **Write the failing test**

  Add to `internal/runner/runner_test.go`. The existing test file uses `package runner`. Add:

  ```go
  func TestRunPostInstall_EmptyHookSkips(t *testing.T) {
      // When PostInstall is empty, runPostInstall must be a no-op.
      // Verified by timing: it should return almost instantly (< 100ms).
      r := &Runner{}
      start := time.Now()
      r.runPostInstall("Git.Git", "", 30)
      if time.Since(start) > 100*time.Millisecond {
          t.Error("runPostInstall with empty hook took longer than expected — should be a no-op")
      }
  }

  func TestRunPostInstall_DryRunPrintsPreview(t *testing.T) {
      // In dry-run mode, runPostInstall must print a preview and not execute.
      // We verify it returns almost instantly (a real exec would take longer).
      r := &Runner{dryRun: true}
      start := time.Now()
      r.runPostInstall("Git.Git", "echo done", 30)
      if time.Since(start) > 100*time.Millisecond {
          t.Error("dry-run runPostInstall should skip execution and return immediately")
      }
  }
  ```

  Add `"time"` to the test file imports.

- [ ] **Run test to verify it fails**

  ```
  go test ./internal/runner/... -run TestRunPostInstall -v
  ```

  Expected output: `# internal/runner [build failed]` — `runPostInstall` method does not exist yet.

- [ ] **Write minimal implementation**

  Step A — Add `RunHook` to the installer package. Add to `internal/installer/command.go`:

  ```go
  // RunHook runs a post-install shell command via cmd /C with the command's timeout.
  // Returns an error if the command fails; the caller decides how to handle it (warning only).
  func RunHook(hook string, timeoutSeconds int) error {
      code, err := runShellWithTimeout(hook, timeoutSeconds)
      if err != nil {
          return err
      }
      if code != 0 {
          return fmt.Errorf("exit code %d", code)
      }
      return nil
  }
  ```

  Step B — Add `runPostInstall` method to `Runner` in `internal/runner/runner.go`:

  ```go
  // runPostInstall executes a post-install hook after a successful install.
  // Hook failures are warnings only and do not affect the result status.
  // timeoutSeconds is the item's configured TimeoutSeconds (already resolved to
  // the global default by applyDefaults before the runner sees it).
  func (r *Runner) runPostInstall(itemName, hook string, timeoutSeconds int) {
      if hook == "" {
          return
      }
      if r.dryRun {
          fmt.Printf("    [DRY RUN] Would run post_install: %s\n", hook)
          return
      }
      fmt.Printf("    running post-install hook for %s...\n", itemName)
      if err := installer.RunHook(hook, timeoutSeconds); err != nil {
          fmt.Printf("    %s[WARN]%s  post-install hook failed for %s: %v\n",
              colorYellow, colorReset, itemName, err)
      }
  }
  ```

  Step C — Call `runPostInstall` in `runPackagesInPhase` and `runCommandsInPhase`.

  In `runPackagesInPhase`, after the state-marking block and before `cleanupShortcuts`, add:

  ```go
  if res.Status == reporter.StatusInstalled || res.Status == reporter.StatusUpgraded {
      r.runPostInstall(pkg.Name, pkg.PostInstall, pkg.TimeoutSeconds)
  }
  ```

  In `runCommandsInPhase`, find the state-marking block that ends with `r.state.MarkSucceeded(cmd.ID)` or `r.state.MarkFailed(cmd.ID)`. Insert the hook call on the very next line — immediately before the `r.trackResult(res.Status)` call:

  ```go
  // ... existing state-marking block ends here ...
  if res.Status == reporter.StatusInstalled || res.Status == reporter.StatusUpgraded {
      r.runPostInstall(cmd.Name, cmd.PostInstall, cmd.TimeoutSeconds)
  }
  r.trackResult(res.Status)  // ← this line already exists; insert hook call above it
  ```

  The hook call must appear AFTER state is marked and BEFORE `trackResult` and the reboot prompt. In runner.go, `trackResult` is the first call after the state block — insert between them.

- [ ] **Run test to verify it passes**

  ```
  go test ./internal/runner/... -run TestRunPostInstall -v
  ```

  Expected output:
  ```
  --- PASS: TestRunPostInstall_EmptyHookSkips
  --- PASS: TestRunPostInstall_DryRunPrintsPreview
  ```

- [ ] **Run full runner suite:** `go test ./internal/runner/... -v`

- [ ] **Build check:** `go build ./...`

- [ ] **Commit**

  ```
  git add internal/runner/runner.go internal/runner/runner_test.go internal/installer/command.go
  git commit -m "feat(runner): add post-install hook execution after StatusInstalled/StatusUpgraded"
  ```

---

## Final Verification

- [ ] Run the full test suite from the repo root:

  ```
  go test ./... -v 2>&1 | tail -30
  ```

  Expected: all tests pass; no compilation errors.

- [ ] Run the build:

  ```
  go build ./...
  ```

- [ ] Run `/security-review` per the workflow policy before opening the PR.

- [ ] Open the PR against `main`:

  ```
  git push -u origin feat/cli-polish
  gh pr create --title "feat: CLI polish — post-install hooks, --profile flag, --output-format json|md" \
    --body "Closes #N — adds post-install hooks, named profile selection, and structured JSON/MD summary output."
  ```

---

## Quick Reference: File Change Map

| File | Change |
|------|--------|
| `internal/config/schema.go` | Add `PostInstall string` to `Package` and `Command` |
| `internal/config/profile.go` | New file: `LookupProfile()` |
| `internal/config/profile_test.go` | New file: tests for `LookupProfile()` |
| `internal/reporter/reporter.go` | Add `progressWriter io.Writer`; update `New()` signature; route all stdout writes through it; add `SummaryJSON()` and `SummaryMD()` |
| `internal/reporter/reporter_test.go` | Add `TestProgressWriter*`, `TestSummaryJSON`, `TestSummaryMD` |
| `internal/installer/command.go` | Add exported `RunHook()` wrapper |
| `internal/runner/runner.go` | Add `runPostInstall()` method; call after `StatusInstalled`/`StatusUpgraded` in `runPackagesInPhase` and `runCommandsInPhase` |
| `internal/runner/runner_test.go` | Add `TestRunPostInstall_*` tests |
| `cmd/main.go` | Add `outputFormat`, `profileName` vars; add `--output-format`, `--profile` flags; add `outputFormatError()`, `profileFlagsError()` helpers; update `runInstall()` to wire all new flags; update `reporter.New()` call signature |
| `cmd/helpers.go` | New file (`package main`): `filterItemsByIDs()`, `filterConfigByIDs()` |
| `cmd/status.go` | Wire `--profile` filter before `detector.FlattenItems` |
| `cmd/export.go` | Wire `--profile` filter before `exporter.Export` |
| `cmd/filter_test.go` | Add `TestOutputFormatFlags*`, `TestProfileFlags*` |
| `cmd/status_test.go` | Add `TestFilterItemsByIDs`, `TestFilterConfigByIDs` |
