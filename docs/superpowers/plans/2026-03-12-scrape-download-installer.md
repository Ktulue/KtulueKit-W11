# Scrape-Download Installer Type Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a scrape-download installer type that fetches a tool's download page, extracts the latest installer URL via regex, downloads it, and runs it silently — applied to Frame0 (new), Plexamp, and Streamer.bot.

**Architecture:** Three new optional fields (`scrape_url`, `url_pattern`, `install_args`) on the `Command` config struct. A new `ScrapeAndInstall` function in `internal/installer/scrape.go` handles the full lifecycle. `RunCommand` branches to it at the top when `ScrapeURL` is set. Validation replaces the unconditional `command`-required check with an XOR rule.

**Tech Stack:** Go stdlib (`net/http`, `regexp`, `os/exec`, `net/http/httptest` for tests), existing `reporter`, `config`, `state` packages.

---

## Chunk 1: Config Struct, Schema, and Validation

### Task 1: Add new fields to the Go config struct

**Files:**
- Modify: `internal/config/schema.go:47-60`

- [ ] **Step 1: Add the three new fields to `Command`**

In `internal/config/schema.go`, add after the `Notes` field:

```go
// Command is a Tier 2 shell command.
type Command struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Phase           int      `json:"phase"`
	Category        string   `json:"category"`
	Description     string   `json:"description"`
	Check           string   `json:"check"`
	Cmd             string   `json:"command"`
	DependsOn       []string `json:"depends_on"`
	RebootAfter     bool     `json:"reboot_after"`
	TimeoutSeconds  int      `json:"timeout_seconds"`
	OnFailurePrompt string   `json:"on_failure_prompt"`
	Notes           string   `json:"notes"`
	// Scrape-download fields — mutually exclusive with Cmd.
	ScrapeURL   string `json:"scrape_url"`
	URLPattern  string `json:"url_pattern"`
	InstallArgs string `json:"install_args"`
}
```

- [ ] **Step 2: Verify the project still compiles**

```bash
cd F:/GDriveClone/Claude_Code/KtulueKit-W11
go build ./...
```

Expected: no output, exit 0.

- [ ] **Step 3: Commit**

```bash
git add internal/config/schema.go
git commit -m "feat(config): add scrape_url, url_pattern, install_args fields to Command"
```

---

### Task 2: Update the JSON schema

**Files:**
- Modify: `schema/ktuluekit.schema.json`

- [ ] **Step 1: Remove `"command"` from `Command.required` and add new properties**

In `schema/ktuluekit.schema.json`, find the `Command` definition (around line 185). Make these two changes:

**Change 1** — remove `"command"` from required:
```json
"required": ["id", "name", "phase", "check"],
```

**Change 2** — add the three new properties inside `Command.properties`, after the `"notes"` property:
```json
"scrape_url": {
  "type": "string",
  "description": "Page URL to fetch HTML from to find the download link. Mutually exclusive with 'command'."
},
"url_pattern": {
  "type": "string",
  "description": "Go regex pattern to extract the download URL from the page HTML. Required when scrape_url is set."
},
"install_args": {
  "type": "string",
  "description": "Space-separated CLI flags for the installer (e.g. /S for silent NSIS). Simple flags only — no quoted tokens with spaces."
}
```

- [ ] **Step 2: Verify the schema file is valid JSON**

```bash
python -m json.tool schema/ktuluekit.schema.json > /dev/null && echo "valid JSON"
```

Expected: `valid JSON`. If Python is unavailable: `node -e "JSON.parse(require('fs').readFileSync('schema/ktuluekit.schema.json','utf8'))" && echo "valid JSON"`

- [ ] **Step 3: Verify the existing config still passes Go validation**

```bash
cd F:/GDriveClone/Claude_Code/KtulueKit-W11
go run . validate
```

Expected: `Config is valid.` — confirms the runtime config (which has no scrape entries yet) still passes.

- [ ] **Step 4: Commit**

```bash
git add schema/ktuluekit.schema.json
git commit -m "feat(schema): add scrape_url, url_pattern, install_args to Command definition"
```

---

### Task 3: Replace the validation check (TDD)

**Files:**
- Modify: `internal/config/validate_test.go`
- Modify: `internal/config/validate.go:72-74`

**Note:** Do NOT commit between Steps 1–4. All four steps are part of one TDD cycle — write tests, see them fail, implement, see them pass — then commit once at Step 5.

- [ ] **Step 1: Write the failing tests**

Add these test cases to `internal/config/validate_test.go` after the existing `TestValidate_CommandMissingCmd` test:

```go
// --- Scrape-type command validation ---

func validScrapeCmd() Command {
	return Command{
		ID:         "tool",
		Name:       "Tool",
		Phase:      1,
		Check:      "echo skip",
		ScrapeURL:  "https://example.com/download",
		URLPattern: `https://example\.com/files/tool-[\d]+\.exe`,
	}
}

func TestValidate_ScrapeCmd_Valid(t *testing.T) {
	c := cfgBase("1.0", "MyKit")
	c.Commands = []Command{validScrapeCmd()}
	errs := Validate(c)
	if len(errs) != 0 {
		t.Errorf("valid scrape command: want 0 errors, got %+v", errs)
	}
}

func TestValidate_ScrapeCmd_ValidWithInstallArgs(t *testing.T) {
	c := cfgBase("1.0", "MyKit")
	cmd := validScrapeCmd()
	cmd.InstallArgs = "/S"
	c.Commands = []Command{cmd}
	errs := Validate(c)
	if len(errs) != 0 {
		t.Errorf("scrape command with install_args: want 0 errors, got %+v", errs)
	}
}

func TestValidate_ScrapeCmd_MissingBoth(t *testing.T) {
	c := cfgBase("1.0", "MyKit")
	c.Commands = []Command{{ID: "c1", Name: "C1", Phase: 1, Check: "echo skip"}}
	errs := Validate(c)
	if len(errs) == 0 {
		t.Fatal("want error when neither command nor scrape_url+url_pattern is set")
	}
}

func TestValidate_ScrapeCmd_HasBothCmdAndScrape(t *testing.T) {
	c := cfgBase("1.0", "MyKit")
	cmd := validScrapeCmd()
	cmd.Cmd = "echo hi"
	c.Commands = []Command{cmd}
	errs := Validate(c)
	if len(errs) == 0 {
		t.Fatal("want error when both command and scrape_url are set")
	}
}

func TestValidate_ScrapeCmd_MissingScrapeURL(t *testing.T) {
	c := cfgBase("1.0", "MyKit")
	c.Commands = []Command{{
		ID: "c1", Name: "C1", Phase: 1, Check: "echo skip",
		URLPattern: `https://example\.com/files/tool\.exe`,
	}}
	errs := Validate(c)
	if len(errs) == 0 {
		t.Fatal("want error when url_pattern is set but scrape_url is missing")
	}
}

func TestValidate_ScrapeCmd_MissingURLPattern(t *testing.T) {
	c := cfgBase("1.0", "MyKit")
	c.Commands = []Command{{
		ID: "c1", Name: "C1", Phase: 1, Check: "echo skip",
		ScrapeURL: "https://example.com/download",
	}}
	errs := Validate(c)
	if len(errs) == 0 {
		t.Fatal("want error when scrape_url is set but url_pattern is missing")
	}
}
```

- [ ] **Step 2: Run the new tests to verify they fail**

```bash
cd F:/GDriveClone/Claude_Code/KtulueKit-W11
go test ./internal/config/... -run "TestValidate_ScrapeCmd" -v
```

Expected: `TestValidate_ScrapeCmd_Valid` FAILS (because the current code rejects commands without `Cmd`). Others may pass or fail — the important thing is `_Valid` and `_ValidWithInstallArgs` fail.

- [ ] **Step 3: Replace the validation check in `validate.go`**

In `internal/config/validate.go`, replace lines 72–74:

**Before:**
```go
if c.Cmd == "" {
    add(fmt.Sprintf("%s(%s).command", prefix, c.ID), "required field 'command' is missing")
}
```

**After:**
```go
hasCmd    := c.Cmd != ""
hasBothScrape := c.ScrapeURL != "" && c.URLPattern != ""
hasAnyScrape  := c.ScrapeURL != "" || c.URLPattern != ""
switch {
case !hasAnyScrape && !hasCmd:
    // Neither command nor scrape fields — entry is incomplete.
    add(fmt.Sprintf("%s(%s).command", prefix, c.ID),
        "must have either 'command' or both 'scrape_url' and 'url_pattern'")
case hasAnyScrape && hasCmd:
    // Has command AND at least one scrape field — mutually exclusive.
    add(fmt.Sprintf("%s(%s).command", prefix, c.ID),
        "cannot have both 'command' and 'scrape_url'/'url_pattern'")
case hasAnyScrape && !hasBothScrape:
    // Has one scrape field but not both — partial scrape entry.
    add(fmt.Sprintf("%s(%s).scrape_url", prefix, c.ID),
        "scrape-type entries must have both 'scrape_url' and 'url_pattern'")
// else: hasCmd && !hasAnyScrape (standard command) OR hasBothScrape && !hasCmd (valid scrape) — both valid.
}
```

- [ ] **Step 4: Run all config tests**

```bash
go test ./internal/config/... -v
```

Expected: all tests PASS. Verify `TestValidate_CommandMissingCmd` still passes (it tests `Cmd == ""` with no scrape fields, which maps to `!hasScrape && !hasCmd` → error, which is correct).

- [ ] **Step 5: Commit**

```bash
git add internal/config/validate.go internal/config/validate_test.go
git commit -m "feat(config): replace command-required check with scrape XOR validation rule"
```

---

## Chunk 2: ScrapeAndInstall, RunCommand Branch, and Config Entries

> **Prerequisite:** Chunk 1 must be fully executed and committed before starting this chunk. Chunk 2 depends on `config.Command` having the `ScrapeURL`, `URLPattern`, and `InstallArgs` fields (Task 1), and on `validate.go` accepting scrape-mode entries with no `command` field (Task 3). Verify with `go build ./...` before proceeding.

### Task 4: Implement `ScrapeAndInstall` (TDD)

**Files:**
- Create: `internal/installer/scrape_test.go`
- Create: `internal/installer/scrape.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/installer/scrape_test.go`:

```go
package installer

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
	"github.com/Ktulue/KtulueKit-W11/internal/reporter"
)

// makeCmd builds a minimal scrape-type Command pointing at the given servers.
func makeCmd(pageURL, pattern, downloadURL string) config.Command {
	return config.Command{
		ID:         "test-tool",
		Name:       "Test Tool",
		Phase:      5,
		Check:      "echo skip",
		ScrapeURL:  pageURL,
		URLPattern: pattern,
		InstallArgs: "",
		TimeoutSeconds: 30,
	}
}

// TestScrapeAndInstall_DryRun verifies no network calls happen and StatusDryRun is returned.
func TestScrapeAndInstall_DryRun(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()

	cmd := makeCmd(srv.URL, `http://.*\.exe`, "")
	res := ScrapeAndInstall(cmd, true)

	if called {
		t.Error("dry-run: server was called, want no network requests")
	}
	if res.Status != reporter.StatusDryRun {
		t.Errorf("dry-run: want StatusDryRun, got %q", res.Status)
	}
}

// TestScrapeAndInstall_AlreadyInstalled verifies the check short-circuits without network calls.
func TestScrapeAndInstall_AlreadyInstalled(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()

	cmd := makeCmd(srv.URL, `http://.*\.exe`, "")
	cmd.Check = "cmd /C exit 0" // always passes — simulates already installed
	res := ScrapeAndInstall(cmd, false)

	if called {
		t.Error("already-installed: server was called, want no network requests")
	}
	if res.Status != reporter.StatusAlready {
		t.Errorf("already-installed: want StatusAlready, got %q", res.Status)
	}
}

// TestScrapeAndInstall_PageFetchFailure verifies failure when the page server returns 500.
func TestScrapeAndInstall_PageFetchFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cmd := makeCmd(srv.URL, `http://.*\.exe`, "")
	cmd.Check = "cmd /C exit 1" // not installed
	res := ScrapeAndInstall(cmd, false)

	if res.Status != reporter.StatusFailed {
		t.Errorf("page fetch failure: want StatusFailed, got %q", res.Status)
	}
	if !strings.Contains(res.Detail, "page") && !strings.Contains(res.Detail, "fetch") && !strings.Contains(res.Detail, "500") {
		t.Errorf("page fetch failure: detail %q should mention fetch/page/500", res.Detail)
	}
}

// TestScrapeAndInstall_NoURLMatch verifies failure when regex finds nothing in the HTML.
func TestScrapeAndInstall_NoURLMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "<html><body>No download link here</body></html>")
	}))
	defer srv.Close()

	cmd := makeCmd(srv.URL, `https://files\.example\.com/tool-[\d]+\.exe`, "")
	cmd.Check = "cmd /C exit 1"
	res := ScrapeAndInstall(cmd, false)

	if res.Status != reporter.StatusFailed {
		t.Errorf("no URL match: want StatusFailed, got %q", res.Status)
	}
	if !strings.Contains(res.Detail, "no download URL") {
		t.Errorf("no URL match: detail %q should mention 'no download URL'", res.Detail)
	}
}

// TestScrapeAndInstall_DownloadFailure verifies failure when the download server returns 404.
func TestScrapeAndInstall_DownloadFailure(t *testing.T) {
	// Download server — always 404.
	dlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer dlSrv.Close()

	dlURL := dlSrv.URL + "/tool.exe"
	// Page server — embeds the download URL.
	pageSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<a href="%s">Download</a>`, dlURL)
	}))
	defer pageSrv.Close()

	pattern := strings.ReplaceAll(dlURL, ".", `\.`) // escape dots for regex
	cmd := makeCmd(pageSrv.URL, pattern, dlURL)
	cmd.Check = "cmd /C exit 1"
	res := ScrapeAndInstall(cmd, false)

	if res.Status != reporter.StatusFailed {
		t.Errorf("download failure: want StatusFailed, got %q", res.Status)
	}
}

// TestScrapeAndInstall_TempFileCleanup verifies the temp file is removed after a failed download.
func TestScrapeAndInstall_TempFileCleanup(t *testing.T) {
	dlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer dlSrv.Close()

	dlURL := dlSrv.URL + "/tool.exe"
	pageSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<a href="%s">Download</a>`, dlURL)
	}))
	defer pageSrv.Close()

	pattern := strings.ReplaceAll(dlURL, ".", `\.`)
	cmd := makeCmd(pageSrv.URL, pattern, dlURL)
	cmd.Check = "cmd /C exit 1"
	ScrapeAndInstall(cmd, false)

	tempPath := filepath.Join(os.TempDir(), cmd.ID+"-setup.exe")
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Errorf("temp file %q should have been removed after install", tempPath)
		os.Remove(tempPath) // clean up if test fails
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

```bash
cd F:/GDriveClone/Claude_Code/KtulueKit-W11
go test ./internal/installer/... -run "TestScrapeAndInstall" -v
```

Expected: compilation error — `ScrapeAndInstall undefined`. That's the right failure.

- [ ] **Step 3: Create `internal/installer/scrape.go`**

> **Note on `runInstaller`:** This function is intentionally separate from the existing `runShellWithTimeout` in `command.go`. `runShellWithTimeout` executes commands via `cmd /C` (a shell wrapper), while `runInstaller` calls the downloaded `.exe` directly with args — these are different execution models. The output-printing and timeout logic are similar by necessity, not by accident.

```go
package installer

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
	"github.com/Ktulue/KtulueKit-W11/internal/reporter"
)

const (
	scrapePageTimeoutSeconds = 30
	scrapeDefaultExecTimeout = 300
)

// ScrapeAndInstall discovers the latest installer URL by scraping a download
// page, downloads it, and runs it silently. It handles its own dry-run and
// already-installed checks so it can be branched to before RunCommand's
// dry-run block.
func ScrapeAndInstall(cmd config.Command, dryRun bool) reporter.Result {
	res := reporter.Result{
		ID:   cmd.ID,
		Name: cmd.Name,
		Tier: "command",
	}

	// 1. Dry-run guard — no network or exec calls.
	if dryRun {
		fmt.Printf("    [dry-run] scrape: %s\n", cmd.ScrapeURL)
		fmt.Printf("    [dry-run] pattern: %s\n", cmd.URLPattern)
		if cmd.InstallArgs != "" {
			fmt.Printf("    [dry-run] install args: %s\n", cmd.InstallArgs)
		}
		res.Status = reporter.StatusDryRun
		res.Detail = fmt.Sprintf("would scrape %s", cmd.ScrapeURL)
		return res
	}

	// 2. Already-installed check — short-circuits before any network call.
	if isAlreadyInstalled(cmd.Check) {
		res.Status = reporter.StatusAlready
		res.Detail = fmt.Sprintf("check passed: %s", cmd.Check)
		return res
	}

	// 3. Fetch the download page.
	pageBody, err := fetchPage(cmd.ScrapeURL)
	if err != nil {
		res.Status = reporter.StatusFailed
		res.Detail = fmt.Sprintf("fetch page %s: %v", cmd.ScrapeURL, err)
		return res
	}

	// 4. Extract the download URL using the regex pattern.
	re, err := regexp.Compile(cmd.URLPattern)
	if err != nil {
		res.Status = reporter.StatusFailed
		res.Detail = fmt.Sprintf("compile url_pattern: %v", err)
		return res
	}
	downloadURL := re.FindString(pageBody)
	if downloadURL == "" {
		res.Status = reporter.StatusFailed
		res.Detail = fmt.Sprintf("no download URL found matching pattern %q in %s", cmd.URLPattern, cmd.ScrapeURL)
		return res
	}
	fmt.Printf("    found: %s\n", downloadURL)

	// 5. Download the installer to a temp file. defer ensures cleanup.
	tempPath := filepath.Join(os.TempDir(), cmd.ID+"-setup.exe")
	defer os.Remove(tempPath)

	if err := downloadFile(downloadURL, tempPath); err != nil {
		res.Status = reporter.StatusFailed
		res.Detail = fmt.Sprintf("download %s: %v", downloadURL, err)
		return res
	}
	fmt.Printf("    downloaded to: %s\n", tempPath)

	// 6. Execute the installer.
	timeoutSecs := cmd.TimeoutSeconds
	if timeoutSecs <= 0 {
		timeoutSecs = scrapeDefaultExecTimeout
	}
	exitCode, err := runInstaller(tempPath, cmd.InstallArgs, timeoutSecs)
	if exitCode == 0 && err == nil {
		res.Status = reporter.StatusInstalled
	} else {
		res.Status = reporter.StatusFailed
		res.Detail = fmt.Sprintf("installer exit code %d", exitCode)
		if err != nil {
			res.Detail += fmt.Sprintf(": %v", err)
		}
	}
	return res
}

// fetchPage GETs the given URL and returns the response body as a string.
// Returns an error on network failure or non-200 status.
func fetchPage(url string) (string, error) {
	client := &http.Client{Timeout: scrapePageTimeoutSeconds * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	return string(body), err
}

// downloadFile streams the response from url into destPath, truncating any
// existing file (os.Create semantics).
func downloadFile(url, destPath string) error {
	resp, err := http.Get(url) //nolint:gosec // URL comes from config, not user input
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

// runInstaller runs the downloaded .exe with the given space-separated args
// and a timeout. Returns the exit code and any execution error.
func runInstaller(exePath, installArgs string, timeoutSeconds int) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	args := strings.Fields(installArgs)
	cmd := exec.CommandContext(ctx, exePath, args...)
	output, err := cmd.CombinedOutput()

	if len(output) > 0 {
		for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
			if strings.TrimSpace(line) != "" {
				fmt.Printf("    │ %s\n", line)
			}
		}
	}

	if ctx.Err() == context.DeadlineExceeded {
		return -1, fmt.Errorf("timed out after %ds", timeoutSeconds)
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return -1, err
	}
	return 0, nil
}
```

- [ ] **Step 4: Run the scrape tests**

```bash
go test ./internal/installer/... -run "TestScrapeAndInstall" -v
```

Expected: all tests PASS. If `TestScrapeAndInstall_AlreadyInstalled` fails — it depends on `cmd /C exit 0` working in the test environment. If it doesn't, use `"echo skip"` as the check and verify the `isAlreadyInstalled` branch behaviour directly.

- [ ] **Step 5: Run the full installer test suite to check nothing broke**

```bash
go test ./internal/installer/... -v
```

Expected: all existing tests still PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/installer/scrape.go internal/installer/scrape_test.go
git commit -m "feat(installer): add ScrapeAndInstall for auto-scrape-and-download installs"
```

---

### Task 5: Wire ScrapeAndInstall into RunCommand

**Files:**
- Modify: `internal/installer/command.go:34`

- [ ] **Step 1: Add the branch at the top of `RunCommand`**

In `internal/installer/command.go`, insert after the function signature (line 34) and before the `res := reporter.Result{` line:

```go
func RunCommand(cmd config.Command, dryRun bool, retryCount int, s *state.State) reporter.Result {
	// Scrape-download path — handles dry-run and already-installed internally.
	// Must be first: this branch fires before the dryRun early-return below.
	if cmd.ScrapeURL != "" {
		return ScrapeAndInstall(cmd, dryRun)
	}

	res := reporter.Result{
	// ... existing code unchanged from here
```

- [ ] **Step 2: Verify the full test suite passes**

```bash
go test ./... -v 2>&1 | tail -20
```

Expected: all tests PASS. The `_test.go` files in `internal/runner/` exercise `RunCommand` indirectly — they should all still pass since the branch only fires when `ScrapeURL != ""`, which no existing test uses.

- [ ] **Step 3: Commit**

```bash
git add internal/installer/command.go
git commit -m "feat(installer): branch RunCommand to ScrapeAndInstall when scrape_url is set"
```

---

### Task 6: Update `ktuluekit.json` config entries

**Files:**
- Modify: `ktuluekit.json`

- [ ] **Step 1: Add Frame0 to `commands[]`**

In `ktuluekit.json`, add a new entry after `"peon-ping"` and before `"dragonruby-control"` (both are phase-5 entries in `commands[]`):

```json
{
  "id": "frame0",
  "name": "Frame0",
  "phase": 5,
  "category": "Dev Tools",
  "description": "UI template design and wireframing tool.",
  "check": "winget list --name \"Frame0\" -e --accept-source-agreements",
  "scrape_url": "https://frame0.app/download",
  "url_pattern": "https://files\\.frame0\\.app/releases/win32/x64/Frame0-[\\d\\.]+ Setup\\.exe",
  "install_args": "/S",
  "notes": "Not on winget. Scraped from frame0.app/download. NSIS installer — /S for silent."
},
```

- [ ] **Step 2: Replace the Plexamp entry**

Find the existing `"id": "plexamp"` entry in `commands[]` and replace it entirely:

```json
{
  "id": "plexamp",
  "name": "Plexamp",
  "phase": 5,
  "category": "Media & Music",
  "description": "Beautiful music player for your Plex library.",
  "check": "winget list --name \"Plexamp\" -e --accept-source-agreements",
  "scrape_url": "https://www.plex.tv/media-server-downloads/",
  "url_pattern": "https://plexamp\\.plex\\.tv/plexamp\\.plex\\.tv/desktop/Plexamp%20Setup%20[\\d\\.]+\\.exe",
  "install_args": "--silent",
  "notes": "Not on winget. Scraped from plex.tv/media-server-downloads. Electron/Squirrel — --silent flag may need verification on first run."
},
```

- [ ] **Step 3: Replace the Streamer.bot entry**

Find the existing `"id": "streamerbot"` entry in `commands[]` and replace it entirely:

```json
{
  "id": "streamerbot",
  "name": "Streamer.bot",
  "phase": 3,
  "category": "Streaming",
  "description": "Automation software for streamers. Integrates with OBS, Twitch, and more.",
  "check": "winget list --name \"Streamer.bot\" -e --accept-source-agreements",
  "scrape_url": "https://streamer.bot",
  "url_pattern": "https://streamer\\.bot/api/releases/streamer\\.bot/[\\d\\.]+/download",
  "install_args": "",
  "notes": "Not on winget. Scraped from streamer.bot. Silent install flag unknown — leave install_args empty; update after testing without a rebuild."
},
```

- [ ] **Step 4: Add `"frame0"` to both profiles**

In the `"Full Setup"` profile `ids` array, add `"frame0"` near the other phase-5 tools:
```json
"frame0",
```

In the `"Dev Only"` profile `ids` array, add `"frame0"` near the other dev tools.

- [ ] **Step 5: Validate the config**

```bash
go run . validate
```

Expected: `Config is valid.` with no errors.

- [ ] **Step 6: Dry-run the three new entries**

```bash
go run . install --dry-run --ids frame0,plexamp,streamerbot
```

Expected output should include lines like (note leading four spaces):
```
    [dry-run] scrape: https://frame0.app/download
    [dry-run] pattern: https://files\.frame0\.app/releases/win32/x64/Frame0-[\d\.]+ Setup\.exe
    [dry-run] install args: /S
```

No errors, no network calls.

- [ ] **Step 7: Commit**

```bash
git add ktuluekit.json
git commit -m "feat(config): add Frame0 (scrape), upgrade Plexamp and Streamer.bot to scrape-download"
```

---

### Task 7: Final verification

- [ ] **Step 1: Run the complete test suite**

```bash
go test ./... -v 2>&1 | grep -E "^(ok|FAIL|---)"
```

Expected: all packages show `ok`, no `FAIL` lines.

- [ ] **Step 2: Full dry-run**

```bash
go run . install --dry-run
```

Expected: runs all phases without error, shows `[dry-run] scrape:` lines for frame0, plexamp, streamerbot.

- [ ] **Step 3: Final commit if any cleanup needed, then push**

```bash
git log --oneline -6
```

Verify commits look clean. Push to branch for PR.
