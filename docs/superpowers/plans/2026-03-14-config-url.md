# Config URL Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow --config to accept https:// URLs by fetching them to temp files before passing to LoadAll.

**Architecture:** URL detection and fetch live in cmd/main.go (resolveConfigPaths). Config package unchanged. Temp files cleaned up via deferred cleanup function.

**Tech Stack:** Go 1.25, net/http, os.CreateTemp, standard library only.

---

## Chunk 1: resolveConfigPaths

### Task 1: Write resolveConfigPaths() — http:// rejection and local path pass-through

**Files:**
- Modify: `cmd/main.go`
- Create: `cmd/resolve_config_test.go`

**Context:** `resolveConfigPaths` is a new package-level function in `cmd/main.go` (package `main`). It takes the raw `[]string` from the `--config` flag and returns a resolved list of file paths plus a cleanup func. The http:// rejection and local pass-through paths require no network — they test cleanly with plain string inputs.

The signature:

```go
func resolveConfigPaths(paths []string) (resolved []string, cleanup func(), err error)
```

Behaviour for this task (no network yet):
- `http://` prefix → return `nil, nil, fmt.Errorf("insecure URL rejected: %q — only https:// is supported", path)`
- No URL prefix → append path as-is to resolved
- Return a cleanup func that is a no-op when there are no temp files

---

- [ ] **Step 1: Write failing tests** — create `cmd/resolve_config_test.go`:

```go
package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestResolveConfigPaths_LocalPassThrough verifies that plain file paths are
// returned unchanged.
func TestResolveConfigPaths_LocalPassThrough(t *testing.T) {
	input := []string{"base.json", "/abs/path/extras.json"}
	resolved, cleanup, err := resolveConfigPaths(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cleanup()

	if len(resolved) != 2 {
		t.Fatalf("expected 2 resolved paths, got %d", len(resolved))
	}
	if resolved[0] != "base.json" {
		t.Errorf("expected resolved[0] = %q, got %q", "base.json", resolved[0])
	}
	if resolved[1] != "/abs/path/extras.json" {
		t.Errorf("expected resolved[1] = %q, got %q", "/abs/path/extras.json", resolved[1])
	}
}

// TestResolveConfigPaths_EmptySlice verifies a nil/empty input returns an empty
// resolved list without error.
func TestResolveConfigPaths_EmptySlice(t *testing.T) {
	resolved, cleanup, err := resolveConfigPaths(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cleanup()
	if len(resolved) != 0 {
		t.Errorf("expected 0 resolved paths, got %d", len(resolved))
	}
}

// TestResolveConfigPaths_HTTPRejected verifies that http:// URLs are rejected
// immediately with a clear error.
func TestResolveConfigPaths_HTTPRejected(t *testing.T) {
	input := []string{"http://example.com/config.json"}
	_, cleanup, err := resolveConfigPaths(input)
	if cleanup != nil {
		defer cleanup()
	}
	if err == nil {
		t.Fatal("expected error for http:// URL, got nil")
	}
	if !containsAll(err.Error(), "http://example.com/config.json", "https://") {
		t.Errorf("error message should mention the rejected URL and suggest https://, got: %v", err)
	}
}

// TestResolveConfigPaths_HTTPRejectedMidList verifies that http:// rejection fires
// even when valid local paths precede the bad URL (fail-fast, no partial state).
func TestResolveConfigPaths_HTTPRejectedMidList(t *testing.T) {
	input := []string{"base.json", "http://example.com/bad.json", "extras.json"}
	_, cleanup, err := resolveConfigPaths(input)
	if cleanup != nil {
		defer cleanup()
	}
	if err == nil {
		t.Fatal("expected error for http:// URL in middle of list, got nil")
	}
}

// TestResolveConfigPaths_MixedLocalPaths verifies multiple local paths (relative
// and absolute) are all passed through in order.
func TestResolveConfigPaths_MixedLocalPaths(t *testing.T) {
	dir := t.TempDir()
	abs := filepath.Join(dir, "extra.json")
	if err := os.WriteFile(abs, []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	input := []string{"base.json", abs}
	resolved, cleanup, err := resolveConfigPaths(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cleanup()

	if len(resolved) != 2 {
		t.Fatalf("expected 2, got %d", len(resolved))
	}
	if resolved[1] != abs {
		t.Errorf("expected resolved[1] = %q, got %q", abs, resolved[1])
	}
}

// containsAll is a test helper that checks all substrings appear in s.
func containsAll(s string, substrings ...string) bool {
	for _, sub := range substrings {
		found := false
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
```

- [ ] **Step 2: Run to confirm they fail**

```
cd /f/GDriveClone/Claude_Code/KtulueKit-W11 && go test ./cmd/... -run TestResolveConfigPaths -v
```

Expected: compile error — `resolveConfigPaths undefined`.

- [ ] **Step 3: Implement resolveConfigPaths (local + http rejection only)** — add to `cmd/main.go`:

```go
// resolveConfigPaths resolves a mixed list of local file paths and https:// URLs
// into a flat list of local file paths, downloading remote configs to temp files.
//
// Rules:
//   - https:// URLs: fetched to a temp file; temp path appended to resolved.
//   - http:// URLs: rejected immediately with an error.
//   - All other paths: appended as-is (no existence check performed here).
//
// The returned cleanup func removes all temp files created during resolution.
// Call it with defer after config.LoadAll returns. cleanup is always non-nil.
func resolveConfigPaths(paths []string) (resolved []string, cleanup func(), err error) {
	var temps []string
	cleanup = func() {
		for _, f := range temps {
			_ = os.Remove(f)
		}
	}

	for _, p := range paths {
		switch {
		case strings.HasPrefix(p, "https://"):
			tmp, fetchErr := fetchToTemp(p)
			if fetchErr != nil {
				cleanup()
				return nil, func() {}, fetchErr
			}
			temps = append(temps, tmp)
			resolved = append(resolved, tmp)

		case strings.HasPrefix(p, "http://"):
			cleanup()
			return nil, func() {}, fmt.Errorf("insecure URL rejected: %q — only https:// is supported", p)

		default:
			resolved = append(resolved, p)
		}
	}

	return resolved, cleanup, nil
}
```

Add a stub for `fetchToTemp` (to be implemented in Task 2) so the file compiles:

```go
// fetchToTemp is implemented in Task 2.
func fetchToTemp(url string) (string, error) {
	return "", fmt.Errorf("fetchToTemp: not yet implemented")
}
```

- [ ] **Step 4: Run to confirm tests pass**

```
cd /f/GDriveClone/Claude_Code/KtulueKit-W11 && go test ./cmd/... -run TestResolveConfigPaths -v
```

Expected: `TestResolveConfigPaths_LocalPassThrough`, `TestResolveConfigPaths_EmptySlice`, `TestResolveConfigPaths_HTTPRejected`, `TestResolveConfigPaths_HTTPRejectedMidList`, `TestResolveConfigPaths_MixedLocalPaths` all PASS.

- [ ] **Step 5: Build check**

```
cd /f/GDriveClone/Claude_Code/KtulueKit-W11 && go build ./...
```

Expected: no errors.

- [ ] **Step 6: Commit**

```
git add cmd/main.go cmd/resolve_config_test.go
git commit -m "feat(cmd): add resolveConfigPaths skeleton with http rejection and local pass-through"
```

---

### Task 2: Add HTTPS fetch with timeout and size cap to resolveConfigPaths()

**Files:**
- Modify: `cmd/main.go` (replace `fetchToTemp` stub with real implementation)
- Modify: `cmd/resolve_config_test.go` (add HTTPS fetch tests)

**Context:** `fetchToTemp` uses `net/http` with a 15-second context deadline, reads the body with an `io.LimitReader` capped at 1 MiB + 1 byte (to distinguish exact-cap from over-cap), writes the body to a temp file via `os.CreateTemp("", "ktuluekit-remote-*.json")`, and returns the temp file path. `cmd/main.go` already imports `"context"` and `"time"` — only `"io"` and `"net/http"` need to be added to the import block.

Tests use `net/http/httptest.NewTLSServer` which provides an in-process HTTPS server with a self-signed cert. The client created for the test must use `httptest.Server.Client()` to trust the test cert — the production `fetchToTemp` must accept an optional `*http.Client` override, OR the tests swap the default client via a package-level variable. Use a package-level `var httpClient = &http.Client{}` that tests can replace.

Guardrail values (define as constants in `cmd/main.go`):

```go
const (
	fetchTimeout    = 15 * time.Second
	fetchMaxBytes   = 1 << 20 // 1 MiB
)
```

---

- [ ] **Step 1: Write failing tests** — add to `cmd/resolve_config_test.go`. Merge the following imports into the existing import block at the top of the file (do NOT add a second `import` declaration — Go allows only one per file):

```go
// Add to existing import block:
"fmt"
"net/http"
"net/http/httptest"
"strings"
```

Then append the test functions below the existing tests:

// TestFetchToTemp_Success verifies a valid HTTPS URL is downloaded to a temp file
// whose contents match the server response.
func TestFetchToTemp_Success(t *testing.T) {
	body := `{"packages":[],"commands":[],"extensions":[]}`
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, body)
	}))
	defer srv.Close()

	// Swap the package-level HTTP client to trust the test TLS cert.
	origClient := httpClient
	httpClient = srv.Client()
	defer func() { httpClient = origClient }()

	tmp, err := fetchToTemp(srv.URL)
	if err != nil {
		t.Fatalf("fetchToTemp() error: %v", err)
	}
	defer os.Remove(tmp)

	got, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", tmp, err)
	}
	if string(got) != body {
		t.Errorf("temp file contents = %q, want %q", got, body)
	}
}

// TestFetchToTemp_SizeCap verifies that a response exceeding 1 MiB is rejected.
func TestFetchToTemp_SizeCap(t *testing.T) {
	bigBody := strings.Repeat("x", fetchMaxBytes+1)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, bigBody)
	}))
	defer srv.Close()

	origClient := httpClient
	httpClient = srv.Client()
	defer func() { httpClient = origClient }()

	_, err := fetchToTemp(srv.URL)
	if err == nil {
		t.Fatal("expected error for oversized response, got nil")
	}
	if !containsAll(err.Error(), "1 MiB") {
		t.Errorf("error should mention 1 MiB limit, got: %v", err)
	}
}

// TestFetchToTemp_Non200 verifies that non-200 HTTP status codes are rejected.
func TestFetchToTemp_Non200(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	origClient := httpClient
	httpClient = srv.Client()
	defer func() { httpClient = origClient }()

	_, err := fetchToTemp(srv.URL)
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
	if !containsAll(err.Error(), "404") {
		t.Errorf("error should mention 404, got: %v", err)
	}
}

// TestResolveConfigPaths_HTTPSFetch verifies the full resolveConfigPaths flow
// with an https:// URL resolves to a temp file containing the expected content.
func TestResolveConfigPaths_HTTPSFetch(t *testing.T) {
	body := `{"packages":[],"commands":[],"extensions":[]}`
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, body)
	}))
	defer srv.Close()

	origClient := httpClient
	httpClient = srv.Client()
	defer func() { httpClient = origClient }()

	input := []string{"local.json", srv.URL}
	resolved, cleanup, err := resolveConfigPaths(input)
	if err != nil {
		t.Fatalf("resolveConfigPaths error: %v", err)
	}
	defer cleanup()

	if len(resolved) != 2 {
		t.Fatalf("expected 2 resolved paths, got %d: %v", len(resolved), resolved)
	}
	if resolved[0] != "local.json" {
		t.Errorf("resolved[0] = %q, want %q", resolved[0], "local.json")
	}

	// resolved[1] is a temp file; verify its content.
	got, err := os.ReadFile(resolved[1])
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", resolved[1], err)
	}
	if string(got) != body {
		t.Errorf("temp file content = %q, want %q", got, body)
	}
}

// TestResolveConfigPaths_CleanupRemovesTempFiles verifies that calling cleanup()
// removes temp files created for https:// URLs.
func TestResolveConfigPaths_CleanupRemovesTempFiles(t *testing.T) {
	body := `{"packages":[]}`
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, body)
	}))
	defer srv.Close()

	origClient := httpClient
	httpClient = srv.Client()
	defer func() { httpClient = origClient }()

	resolved, cleanup, err := resolveConfigPaths([]string{srv.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resolved) != 1 {
		t.Fatalf("expected 1 resolved path, got %d", len(resolved))
	}
	tmpPath := resolved[0]

	// File exists before cleanup.
	if _, err := os.Stat(tmpPath); err != nil {
		t.Fatalf("temp file should exist before cleanup: %v", err)
	}

	cleanup()

	// File gone after cleanup.
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("temp file should be removed after cleanup()")
	}
}
```

- [ ] **Step 2: Run to confirm they fail**

```
cd /f/GDriveClone/Claude_Code/KtulueKit-W11 && go test ./cmd/... -run "TestFetchToTemp|TestResolveConfigPaths_HTTPS|TestResolveConfigPaths_Cleanup" -v
```

Expected: compile errors (missing `httpClient` var, missing imports) or test failures (`fetchToTemp` returns "not yet implemented").

- [ ] **Step 3: Implement fetchToTemp and httpClient** — in `cmd/main.go`:

Add imports: `"io"` and `"net/http"` (alongside existing imports — `"context"` and `"time"` are already present in `cmd/main.go`).

Add the package-level client variable and constants:

```go
// httpClient is the HTTP client used by fetchToTemp. Tests may replace this
// to inject a custom transport (e.g., httptest.Server.Client()).
var httpClient = &http.Client{}

const (
	fetchTimeout  = 15 * time.Second
	fetchMaxBytes = 1 << 20 // 1 MiB
)
```

Replace the `fetchToTemp` stub with the real implementation:

```go
// fetchToTemp downloads url to a temp file and returns the temp file path.
// The caller is responsible for removing the file when done.
//
// Guardrails:
//   - 15-second total timeout (context deadline)
//   - 1 MiB response body cap — returns error if exceeded
//   - Non-2xx status rejected with the status code in the error message
func fetchToTemp(url string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("fetch %q: build request: %w", url, err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch %q: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch %q: server returned %d", url, resp.StatusCode)
	}

	// Read up to fetchMaxBytes+1 to detect over-limit responses.
	limited := io.LimitReader(resp.Body, fetchMaxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("fetch %q: read body: %w", url, err)
	}
	if int64(len(data)) > fetchMaxBytes {
		return "", fmt.Errorf("fetch %q: response exceeds 1 MiB limit", url)
	}

	tmp, err := os.CreateTemp("", "ktuluekit-remote-*.json")
	if err != nil {
		return "", fmt.Errorf("fetch %q: create temp file: %w", url, err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return "", fmt.Errorf("fetch %q: write temp file: %w", url, err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return "", fmt.Errorf("fetch %q: close temp file: %w", url, err)
	}

	return tmp.Name(), nil
}
```

- [ ] **Step 4: Run HTTPS fetch tests**

```
cd /f/GDriveClone/Claude_Code/KtulueKit-W11 && go test ./cmd/... -run "TestFetchToTemp|TestResolveConfigPaths_HTTPS|TestResolveConfigPaths_Cleanup" -v
```

Expected: all 5 new tests PASS.

- [ ] **Step 5: Run full cmd test suite to check for regressions**

```
cd /f/GDriveClone/Claude_Code/KtulueKit-W11 && go test ./cmd/... -v
```

Expected: all PASS.

- [ ] **Step 6: Build check**

```
cd /f/GDriveClone/Claude_Code/KtulueKit-W11 && go build ./...
```

Expected: no errors.

- [ ] **Step 7: Commit**

```
git add cmd/main.go cmd/resolve_config_test.go
git commit -m "feat(cmd): implement fetchToTemp with 15s timeout and 1 MiB cap"
```

---

### Task 3: Wire resolveConfigPaths() into all subcommand handlers

**Files:**
- Modify: `cmd/main.go` (runInstall)
- Modify: `cmd/status.go` (runStatus)
- Modify: `cmd/validate.go` (runValidate)
- Modify: `cmd/list.go` (runList)
- Modify: `cmd/export.go` (runExport)

**Context:** Each handler currently calls `config.LoadAll(configPaths)` directly. The wire-up pattern is the same in every handler — call `resolveConfigPaths`, defer cleanup, pass resolved paths to `LoadAll`. There is one subtlety in `runExport`: it has a local `paths` variable that shadows `configPaths` (adds a default). That local variable must go through `resolveConfigPaths` too.

Wire-up pattern (identical for runInstall, runStatus, runValidate, runList):

```go
// At the top of the handler, before config.LoadAll:
resolved, cleanup, err := resolveConfigPaths(configPaths)
if err != nil {
    return err
}
defer cleanup()
// Then replace: config.LoadAll(configPaths)
// With:         config.LoadAll(resolved)
```

Wire-up pattern for `runExport` (uses local `paths` variable):

```go
// Replace the existing local paths assignment:
//   paths := configPaths
//   if len(paths) == 0 { paths = []string{"ktuluekit.json"} }
// With:
paths := configPaths
if len(paths) == 0 {
    paths = []string{"ktuluekit.json"}
}
resolved, cleanup, err := resolveConfigPaths(paths)
if err != nil {
    return err
}
defer cleanup()
// Then replace: config.LoadAll(paths)
// With:         config.LoadAll(resolved)
// Note: keep using `paths` (not `resolved`) for display/metadata (absConfig, fmt.Printf).
```

No new test file is needed — the existing per-subcommand tests in `cmd/status_test.go`, `cmd/validate_test.go`, `cmd/list_test.go` exercise the handlers with local paths and will catch regressions. Add one integration-style test to confirm the wire-up is reachable end-to-end.

---

- [ ] **Step 1: Write failing integration test** — add to `cmd/resolve_config_test.go`:

```go
// TestRunValidate_WithLocalConfigPath verifies that runValidate correctly resolves
// a local config path through resolveConfigPaths before passing to LoadAll.
// This is an integration-style guard that the wire-up in each handler is in place.
func TestRunValidate_WithLocalConfigPath(t *testing.T) {
	// Write a minimal valid config to a temp file.
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ktuluekit.json")
	minimalCfg := `{
		"$schema": "",
		"version": "1",
		"metadata": {"name": "test"},
		"settings": {},
		"packages": [],
		"commands": [],
		"extensions": []
	}`
	if err := os.WriteFile(cfgPath, []byte(minimalCfg), 0644); err != nil {
		t.Fatal(err)
	}

	// Set the package-level configPaths as the handler reads from it.
	origPaths := configPaths
	configPaths = []string{cfgPath}
	defer func() { configPaths = origPaths }()

	// Build a cobra command to invoke runValidate.
	cmd := &cobra.Command{}
	err := runValidate(cmd, nil)
	// A minimal config may produce validation errors — that is OK.
	// What we are testing is that no panic occurs and the path resolves correctly
	// (i.e., no "insecure URL" or "not yet implemented" error).
	if err != nil && containsAll(err.Error(), "not yet implemented") {
		t.Errorf("runValidate returned stub error: %v", err)
	}
	if err != nil && containsAll(err.Error(), "insecure URL") {
		t.Errorf("runValidate rejected a local path as a URL: %v", err)
	}
}
```

Note: this test imports `"github.com/spf13/cobra"` — add it to the import block of `resolve_config_test.go`.

- [ ] **Step 2: Run to confirm current behaviour**

```
cd /f/GDriveClone/Claude_Code/KtulueKit-W11 && go test ./cmd/... -run TestRunValidate_WithLocalConfigPath -v
```

Expected: PASS already (local path was always pass-through before Task 3 too). This test's role is to be a regression guard — it must still pass after wiring.

- [ ] **Step 3: Wire resolveConfigPaths into runInstall** — in `cmd/main.go`, replace:

```go
cfg, err := config.LoadAll(configPaths)
if err != nil {
    return fmt.Errorf("config error: %w", err)
}
```

with:

```go
resolved, cleanup, err := resolveConfigPaths(configPaths)
if err != nil {
    return err
}
defer cleanup()

cfg, err := config.LoadAll(resolved)
if err != nil {
    return fmt.Errorf("config error: %w", err)
}
```

Also update the `reportingPath` line lower in `runInstall` — it currently reads `configPaths` for display purposes. Keep using `configPaths` (not `resolved`) there so displayed paths show the original URLs or local paths the user supplied, not the opaque temp file names:

```go
// Use the first config path for reporting, defaulting if empty.
// Display the original configPaths (pre-resolve) so URLs remain human-readable.
reportingPath := configPaths
if len(reportingPath) == 0 {
    reportingPath = []string{"ktuluekit.json"}
}
```

No change needed there — `configPaths` is still in scope. Leave it as-is.

- [ ] **Step 4: Wire resolveConfigPaths into runStatus** — in `cmd/status.go`, replace:

```go
cfg, err := config.LoadAll(configPaths)
if err != nil {
    return fmt.Errorf("config error: %w", err)
}
```

with:

```go
resolved, cleanup, err := resolveConfigPaths(configPaths)
if err != nil {
    return err
}
defer cleanup()

cfg, err := config.LoadAll(resolved)
if err != nil {
    return fmt.Errorf("config error: %w", err)
}
```

- [ ] **Step 5: Wire resolveConfigPaths into runValidate** — in `cmd/validate.go`, replace:

```go
cfg, err := config.LoadAll(configPaths)
if err != nil {
    return fmt.Errorf("config parse error: %w", err)
}
```

with:

```go
resolved, cleanup, err := resolveConfigPaths(configPaths)
if err != nil {
    return err
}
defer cleanup()

cfg, err := config.LoadAll(resolved)
if err != nil {
    return fmt.Errorf("config parse error: %w", err)
}
```

Keep `displayPaths := configPaths` below unchanged — it intentionally shows the original user-supplied paths.

- [ ] **Step 6: Wire resolveConfigPaths into runList** — in `cmd/list.go`, replace:

```go
cfg, err := config.LoadAll(configPaths)
if err != nil {
    return fmt.Errorf("config error: %w", err)
}
```

with:

```go
resolved, cleanup, err := resolveConfigPaths(configPaths)
if err != nil {
    return err
}
defer cleanup()

cfg, err := config.LoadAll(resolved)
if err != nil {
    return fmt.Errorf("config error: %w", err)
}
```

- [ ] **Step 7: Wire resolveConfigPaths into runExport** — in `cmd/export.go`, the handler has a local `paths` variable. Replace the block:

```go
// Load and validate config.
cfg, err := config.LoadAll(paths)
if err != nil {
    return fmt.Errorf("config error: %w", err)
}
```

with:

```go
// Resolve any https:// entries to temp files before loading.
resolved, cleanup, err := resolveConfigPaths(paths)
if err != nil {
    return err
}
defer cleanup()

// Load and validate config.
cfg, err := config.LoadAll(resolved)
if err != nil {
    return fmt.Errorf("config error: %w", err)
}
```

Note: `absConfig` is derived from `paths[0]`, not `resolved[0]` — keep it that way so snapshot metadata shows the original URL or local path, not the temp file name.

- [ ] **Step 8: Build check**

```
cd /f/GDriveClone/Claude_Code/KtulueKit-W11 && go build ./...
```

Expected: no errors.

- [ ] **Step 9: Run full test suite**

```
cd /f/GDriveClone/Claude_Code/KtulueKit-W11 && go test ./cmd/... -v
```

Expected: all PASS (including pre-existing tests in `status_test.go`, `validate_test.go`, `list_test.go`, `filter_test.go`).

- [ ] **Step 10: Commit**

```
git add cmd/main.go cmd/status.go cmd/validate.go cmd/list.go cmd/export.go cmd/resolve_config_test.go
git commit -m "feat(cmd): wire resolveConfigPaths into all subcommand handlers"
```

---

## Final: Mark TODO.md item done and open PR

- [ ] **Step 1: Update `TODO.md`** — mark the remote config fetching item done (if present).

- [ ] **Step 2: Run full test suite one last time**

```
cd /f/GDriveClone/Claude_Code/KtulueKit-W11 && go test ./... && go build ./...
```

Expected: all PASS, clean build.

- [ ] **Step 3: Commit TODO update**

```
git add TODO.md
git commit -m "chore(todo): mark remote config URL fetching as done"
```

- [ ] **Step 4: Push and open PR**

```
git push -u origin feat/config-url
gh pr create --title "feat(cmd): add https:// remote config fetching via --config" --body "$(cat <<'EOF'
## Summary

- Adds `resolveConfigPaths()` to `cmd/main.go` that maps `https://` URLs to temp files before passing to `config.LoadAll`.
- `http://` URLs are rejected immediately with a clear error; local paths pass through unchanged.
- Fetch uses a 15-second timeout and a 1 MiB response body cap.
- Temp files are cleaned up via a deferred `cleanup()` func after `LoadAll` returns.
- Wired into all five subcommand handlers: `runInstall`, `runStatus`, `runValidate`, `runList`, `runExport`.
- Config package (`internal/config`) is unchanged — no `net/http` dependency added there.
- Merge order: argument-list position determines priority (last wins), local or remote treated identically.

## Test plan

- [ ] `go test ./cmd/... -run TestResolveConfigPaths -v` — all local/http cases pass
- [ ] `go test ./cmd/... -run TestFetchToTemp -v` — success, size cap, non-200 cases pass
- [ ] `go test ./cmd/... -run TestResolveConfigPaths_HTTPS -v` — end-to-end temp file flow passes
- [ ] `go test ./cmd/... -run TestResolveConfigPaths_Cleanup -v` — cleanup removes temp files
- [ ] `go test ./cmd/... -v` — full cmd suite passes with no regressions
- [ ] `go build ./...` — clean build

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```
