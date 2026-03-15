# Unit Tests Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fill coverage gaps in pre-sprint existing code across config, installer, detector, and cmd packages.

**Architecture:** Tests only — no implementation changes. Each package gets focused table-driven tests for currently-untested pure functions.

**Tech Stack:** Go 1.25, standard testing package, table-driven tests.

---

## Context: What Already Exists

Before writing new tests, understand what is already covered so you don't duplicate:

- `internal/config/loader_test.go` — has integration tests for `LoadAll` with single/multiple paths, package ID override, position preservation, settings non-zero wins, profile name override, metadata first-wins, three-config chain, missing file, invalid JSON, cross-tier collision, empty paths. **Gap:** no direct unit tests for the unexported merge functions (`mergePackages`, `mergeCommands`, `mergeExtensions`, `mergeProfiles`, `mergeSettings`) and no `LookupProfile` tests (function added by cli-polish).
- `internal/installer/scrape_test.go` — has integration tests for `ScrapeAndInstall`: dry-run, already-installed, page fetch failure, no URL match, download failure, temp file cleanup. **Gap:** no unit tests for the URL pattern extraction edge cases (multiple matches uses first, empty HTML body, invalid regex pattern compile failure).
- `internal/detector/detector_test.go` — has tests for `CheckItem` (state-aware skip, no check cmd, echo skip, nil state), `FlattenItems` (all tiers, order, field mapping, empty config), `CheckAll` (length, order, state-aware skip per item), and two `RunCheckDetailed` cases (exit 0, exit non-zero). **Gap:** no timeout test for `RunCheckDetailed`.
- `cmd/filter_test.go` — has individual named tests for `filterFlagsError`, `phaseFlagsError`, `upgradeOnlyFlagsError`. **Gap:** not table-driven; missing a few edge cases.

---

## Chunk 1: Config Merge + Profile

### Task 1.1 — Direct unit tests for `mergePackages`

**File:** `internal/config/loader_test.go` (append to existing file)

**Read first:** `internal/config/loader.go` lines 103–120 — `mergePackages` signature: `func mergePackages(base, src []Package) []Package`

These functions are unexported but the test file is `package config` (internal test), so they are directly callable.

- [ ] Read `internal/config/loader.go` to confirm function signatures and package name.
- [ ] Append the following table-driven tests to `internal/config/loader_test.go`:

```go
func TestMergePackages(t *testing.T) {
	tests := []struct {
		name        string
		base        []Package
		src         []Package
		wantIDs     []string // expected order
		wantUpdated map[string]string // id -> expected Name after merge
	}{
		{
			name:    "empty base and src",
			base:    nil,
			src:     nil,
			wantIDs: []string{},
		},
		{
			name:    "src into empty base",
			base:    nil,
			src:     []Package{{ID: "A", Name: "Alpha"}},
			wantIDs: []string{"A"},
		},
		{
			name:    "base with no overlap",
			base:    []Package{{ID: "A", Name: "Alpha"}},
			src:     []Package{{ID: "B", Name: "Beta"}},
			wantIDs: []string{"A", "B"},
		},
		{
			name:        "last-wins on ID collision",
			base:        []Package{{ID: "A", Name: "Old Name"}},
			src:         []Package{{ID: "A", Name: "New Name"}},
			wantIDs:     []string{"A"},
			wantUpdated: map[string]string{"A": "New Name"},
		},
		{
			name: "position preserved on collision — colliding ID stays at original index",
			base: []Package{
				{ID: "A", Name: "Alpha"},
				{ID: "B", Name: "Beta"},
				{ID: "C", Name: "Gamma"},
			},
			src: []Package{
				{ID: "B", Name: "Beta v2"},
			},
			wantIDs:     []string{"A", "B", "C"},
			wantUpdated: map[string]string{"B": "Beta v2"},
		},
		{
			name: "new src item appended after base",
			base: []Package{
				{ID: "A", Name: "Alpha"},
			},
			src: []Package{
				{ID: "A", Name: "Alpha v2"},
				{ID: "D", Name: "Delta"},
			},
			wantIDs:     []string{"A", "D"},
			wantUpdated: map[string]string{"A": "Alpha v2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergePackages(tt.base, tt.src)

			// Verify order
			if len(tt.wantIDs) == 0 && len(got) != 0 {
				t.Fatalf("want empty result, got %d items", len(got))
			}
			gotIDs := make([]string, len(got))
			for i, p := range got {
				gotIDs[i] = p.ID
			}
			if len(tt.wantIDs) > 0 {
				for i, id := range tt.wantIDs {
					if i >= len(gotIDs) || gotIDs[i] != id {
						t.Errorf("position %d: want ID %q, got order %v", i, id, gotIDs)
					}
				}
				if len(got) != len(tt.wantIDs) {
					t.Errorf("len(result) = %d, want %d", len(got), len(tt.wantIDs))
				}
			}

			// Verify updated names
			for id, wantName := range tt.wantUpdated {
				var found bool
				for _, p := range got {
					if p.ID == id {
						found = true
						if p.Name != wantName {
							t.Errorf("id %q: Name = %q, want %q", id, p.Name, wantName)
						}
					}
				}
				if !found {
					t.Errorf("id %q not found in result", id)
				}
			}
		})
	}
}
```

- [ ] Run `go test ./internal/config/... -v -run TestMergePackages` — must PASS.
- [ ] Commit: `test(config): add table-driven unit tests for mergePackages`

---

### Task 1.2 — Direct unit tests for `mergeCommands` and `mergeExtensions`

**File:** `internal/config/loader_test.go` (append)

These mirror `mergePackages` but operate on `[]Command` (keyed by `c.ID`) and `[]Extension` (keyed by `e.ID`).

- [ ] Append the following tests:

```go
func TestMergeCommands(t *testing.T) {
	tests := []struct {
		name        string
		base        []Command
		src         []Command
		wantIDs     []string
		wantUpdated map[string]string // id -> expected Name
	}{
		{
			name:    "no overlap",
			base:    []Command{{ID: "a", Name: "A"}},
			src:     []Command{{ID: "b", Name: "B"}},
			wantIDs: []string{"a", "b"},
		},
		{
			name:        "last-wins on collision",
			base:        []Command{{ID: "x", Name: "Old"}},
			src:         []Command{{ID: "x", Name: "New"}},
			wantIDs:     []string{"x"},
			wantUpdated: map[string]string{"x": "New"},
		},
		{
			name: "position preserved",
			base: []Command{
				{ID: "a", Name: "A"},
				{ID: "b", Name: "B"},
			},
			src: []Command{
				{ID: "a", Name: "A2"},
			},
			wantIDs:     []string{"a", "b"},
			wantUpdated: map[string]string{"a": "A2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeCommands(tt.base, tt.src)
			if len(got) != len(tt.wantIDs) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.wantIDs))
			}
			for i, id := range tt.wantIDs {
				if got[i].ID != id {
					t.Errorf("[%d] ID = %q, want %q", i, got[i].ID, id)
				}
			}
			for id, wantName := range tt.wantUpdated {
				for _, c := range got {
					if c.ID == id && c.Name != wantName {
						t.Errorf("id %q: Name = %q, want %q", id, c.Name, wantName)
					}
				}
			}
		})
	}
}

func TestMergeExtensions(t *testing.T) {
	tests := []struct {
		name        string
		base        []Extension
		src         []Extension
		wantIDs     []string
		wantUpdated map[string]string // id -> expected Name
	}{
		{
			name:    "no overlap",
			base:    []Extension{{ID: "ext-a", Name: "Ext A"}},
			src:     []Extension{{ID: "ext-b", Name: "Ext B"}},
			wantIDs: []string{"ext-a", "ext-b"},
		},
		{
			name:        "last-wins on collision",
			base:        []Extension{{ID: "ext-a", Name: "Old"}},
			src:         []Extension{{ID: "ext-a", Name: "New"}},
			wantIDs:     []string{"ext-a"},
			wantUpdated: map[string]string{"ext-a": "New"},
		},
		{
			name: "position preserved",
			base: []Extension{
				{ID: "ext-a", Name: "A"},
				{ID: "ext-b", Name: "B"},
			},
			src: []Extension{
				{ID: "ext-a", Name: "A2"},
				{ID: "ext-c", Name: "C"},
			},
			wantIDs:     []string{"ext-a", "ext-b", "ext-c"},
			wantUpdated: map[string]string{"ext-a": "A2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeExtensions(tt.base, tt.src)
			if len(got) != len(tt.wantIDs) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.wantIDs))
			}
			for i, id := range tt.wantIDs {
				if got[i].ID != id {
					t.Errorf("[%d] ID = %q, want %q", i, got[i].ID, id)
				}
			}
			for id, wantName := range tt.wantUpdated {
				for _, e := range got {
					if e.ID == id && e.Name != wantName {
						t.Errorf("id %q: Name = %q, want %q", id, e.Name, wantName)
					}
				}
			}
		})
	}
}
```

- [ ] Run `go test ./internal/config/... -v -run "TestMergeCommands|TestMergeExtensions"` — must PASS.
- [ ] Commit: `test(config): add table-driven unit tests for mergeCommands and mergeExtensions`

---

### Task 1.3 — Direct unit tests for `mergeProfiles` and `mergeSettings`

**File:** `internal/config/loader_test.go` (append)

- `mergeProfiles` signature: `func mergeProfiles(base, src []Profile) []Profile` — keyed by `p.Name`, last-wins.
- `mergeSettings` signature: `func mergeSettings(dst, src *Settings)` — mutates dst in-place; non-zero src fields overwrite dst; `UpgradeIfInstalled` is one-way ratchet (true stays true).

- [ ] Append the following tests:

```go
func TestMergeProfiles(t *testing.T) {
	tests := []struct {
		name         string
		base         []Profile
		src          []Profile
		wantNames    []string
		wantIDCounts map[string]int // profile name -> expected len(IDs)
	}{
		{
			name:      "no overlap — both profiles kept",
			base:      []Profile{{Name: "Full", IDs: []string{"a", "b"}}},
			src:       []Profile{{Name: "Minimal", IDs: []string{"a"}}},
			wantNames: []string{"Full", "Minimal"},
		},
		{
			name: "last-wins on name collision",
			base: []Profile{{Name: "Full", IDs: []string{"a", "b"}}},
			src:  []Profile{{Name: "Full", IDs: []string{"a", "b", "c"}}},
			wantNames:    []string{"Full"},
			wantIDCounts: map[string]int{"Full": 3},
		},
		{
			name: "position preserved — collision stays at base index",
			base: []Profile{
				{Name: "Full", IDs: []string{"a"}},
				{Name: "Minimal", IDs: []string{"b"}},
			},
			src: []Profile{
				{Name: "Full", IDs: []string{"a", "b", "c"}},
			},
			wantNames:    []string{"Full", "Minimal"},
			wantIDCounts: map[string]int{"Full": 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeProfiles(tt.base, tt.src)
			if len(got) != len(tt.wantNames) {
				t.Fatalf("len = %d, want %d (got names: %v)", len(got), len(tt.wantNames), profileNames(got))
			}
			for i, name := range tt.wantNames {
				if got[i].Name != name {
					t.Errorf("[%d] Name = %q, want %q", i, got[i].Name, name)
				}
			}
			for name, wantCount := range tt.wantIDCounts {
				for _, p := range got {
					if p.Name == name && len(p.IDs) != wantCount {
						t.Errorf("profile %q: len(IDs) = %d, want %d", name, len(p.IDs), wantCount)
					}
				}
			}
		})
	}
}

// profileNames is a test helper that extracts profile names for error messages.
// Before appending: grep -rn "func profileNames" internal/config/
// This must search ALL .go files (not just *_test.go) — feat/cli-polish may define it
// in profile.go (a non-test file). If found anywhere in the package, omit this
// declaration to avoid a duplicate-function compile error.
func profileNames(profiles []Profile) []string {
	names := make([]string, len(profiles))
	for i, p := range profiles {
		names[i] = p.Name
	}
	return names
}

func TestMergeSettings(t *testing.T) {
	tests := []struct {
		name               string
		dst                Settings
		src                Settings
		wantLogDir         string
		wantRetryCount     int
		wantTimeout        int
		wantScope          string
		wantExtMode        string
		wantUpgradeEnabled bool
	}{
		{
			name:           "src non-zero overwrites dst",
			dst:            Settings{LogDir: "./logs", RetryCount: 2, DefaultTimeoutSeconds: 60, DefaultScope: "machine", ExtensionMode: "url"},
			src:            Settings{LogDir: "./new-logs", RetryCount: 5},
			wantLogDir:     "./new-logs",
			wantRetryCount: 5,
			wantTimeout:    60,   // dst preserved (src zero)
			wantScope:      "machine", // dst preserved
			wantExtMode:    "url",     // dst preserved
		},
		{
			name:           "src zero fields do not overwrite dst",
			dst:            Settings{LogDir: "./logs", RetryCount: 3},
			src:            Settings{}, // all zero
			wantLogDir:     "./logs",
			wantRetryCount: 3,
		},
		{
			name:               "UpgradeIfInstalled one-way ratchet: src true sets dst",
			dst:                Settings{UpgradeIfInstalled: false},
			src:                Settings{UpgradeIfInstalled: true},
			wantUpgradeEnabled: true,
		},
		{
			name:               "UpgradeIfInstalled one-way ratchet: src false cannot clear dst",
			dst:                Settings{UpgradeIfInstalled: true},
			src:                Settings{UpgradeIfInstalled: false},
			wantUpgradeEnabled: true, // stays true — cannot be disabled by overlay
		},
		{
			name:               "UpgradeIfInstalled both false stays false",
			dst:                Settings{UpgradeIfInstalled: false},
			src:                Settings{UpgradeIfInstalled: false},
			wantUpgradeEnabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dst := tt.dst // copy — mergeSettings mutates dst in place
			mergeSettings(&dst, &tt.src)

			if tt.wantLogDir != "" && dst.LogDir != tt.wantLogDir {
				t.Errorf("LogDir = %q, want %q", dst.LogDir, tt.wantLogDir)
			}
			if tt.wantRetryCount != 0 && dst.RetryCount != tt.wantRetryCount {
				t.Errorf("RetryCount = %d, want %d", dst.RetryCount, tt.wantRetryCount)
			}
			if tt.wantTimeout != 0 && dst.DefaultTimeoutSeconds != tt.wantTimeout {
				t.Errorf("DefaultTimeoutSeconds = %d, want %d", dst.DefaultTimeoutSeconds, tt.wantTimeout)
			}
			if tt.wantScope != "" && dst.DefaultScope != tt.wantScope {
				t.Errorf("DefaultScope = %q, want %q", dst.DefaultScope, tt.wantScope)
			}
			if tt.wantExtMode != "" && dst.ExtensionMode != tt.wantExtMode {
				t.Errorf("ExtensionMode = %q, want %q", dst.ExtensionMode, tt.wantExtMode)
			}
			if dst.UpgradeIfInstalled != tt.wantUpgradeEnabled {
				t.Errorf("UpgradeIfInstalled = %v, want %v", dst.UpgradeIfInstalled, tt.wantUpgradeEnabled)
			}
		})
	}
}
```

- [ ] Run `go test ./internal/config/... -v -run "TestMergeProfiles|TestMergeSettings"` — must PASS.
- [ ] Commit: `test(config): add table-driven unit tests for mergeProfiles and mergeSettings`

---

### Task 1.4 — `LookupProfile` tests

**Note:** `LookupProfile` will be added by the `feat/cli-polish` branch. This task should be deferred until that branch is merged. If it has already been merged when you start this branch, implement this task; otherwise leave it for a follow-up.

**File:** `internal/config/loader_test.go` (append) or a new `internal/config/profile_test.go`

- [ ] Check whether `LookupProfile` exists: `grep -n "LookupProfile" internal/config/*.go`
- [ ] If found, read its **exact** signature — the return types and whether it's a method or free function will vary. The cli-polish plan implements it as `func LookupProfile(cfg *Config, name string) ([]string, error)` returning a slice of IDs (not a `*Profile`). Do NOT copy the stub below literally — derive the test from the actual signature you observe. Example outline only:

```go
// IMPORTANT: Read the actual LookupProfile signature first.
// The test structure below shows the intent — adjust types to match reality.
//
// If signature is: func LookupProfile(cfg *Config, name string) ([]string, error)
//   then: ids, err := LookupProfile(cfg, tt.profileName)
//         check err == nil for found, err != nil for not-found
//         check len(ids) for wantIDCount
//
// If signature is: func LookupProfile(cfg *Config, name string) (*Profile, bool)
//   then: profile, ok := LookupProfile(cfg, tt.profileName)
//         check ok for found
//         check len(profile.IDs) for wantIDCount
//
// Write the test body AFTER confirming the signature.
func TestLookupProfile(t *testing.T) {
	cfg := &Config{
		Profiles: []Profile{
			{Name: "Full", IDs: []string{"Git.Git", "Mozilla.Firefox", "Spotify.Spotify"}},
			{Name: "Minimal", IDs: []string{"Git.Git"}},
		},
	}
	// Case 1: existing profile returns correct IDs
	// Case 2: second profile found
	// Case 3: non-existent profile returns not-found
	// Case 4: empty name returns not-found
	// Implement these cases using the actual return types.
	_ = cfg // replace with actual test body
}
```
- [ ] Run `go test ./internal/config/... -v -run TestLookupProfile` — must PASS.
- [ ] Commit: `test(config): add table-driven tests for LookupProfile`

---

## Chunk 2: Installer + Detector + Cmd

### Task 2.1 — Scrape URL pattern matching unit tests

**File:** `internal/installer/scrape_test.go` (append to existing file)

**Read first:** `internal/installer/scrape.go` — the URL extraction is inline in `ScrapeAndInstall` at the `regexp.Compile` + `re.FindString` block (lines 63–74). There is no separate exported function to test the regex step in isolation. Tests must exercise this path through `ScrapeAndInstall` with crafted HTTP test servers.

The existing tests already cover: page fetch failure, no URL match (StatusFailed + "no download URL" detail), download failure. The gaps are:

- **Multiple matches in HTML** — `re.FindString` returns the first match; verify the correct first URL is used (not a second one).
- **Empty HTML body** — page returns 200 but body is empty; regex finds nothing; expect StatusFailed.
- **Invalid regex pattern** — `regexp.Compile` fails; expect StatusFailed with "compile" in detail.

- [ ] Append the following tests to `internal/installer/scrape_test.go`:

```go
// TestScrapeAndInstall_MultipleMatches verifies that when multiple URLs match
// the pattern, the first match is used (re.FindString returns first).
func TestScrapeAndInstall_MultipleMatches(t *testing.T) {
	// Two download servers — we expect only the FIRST URL to be attempted.
	firstCalled := false
	secondCalled := false

	firstSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		firstCalled = true
		// Return a minimal valid .exe response (0 bytes, status 200).
		// The installer will run a 0-byte exe and fail, but that's after the URL selection step.
		w.WriteHeader(http.StatusOK)
	}))
	defer firstSrv.Close()

	secondSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer secondSrv.Close()

	// The page body contains both URLs — first server first.
	pageBody := fmt.Sprintf(
		`<a href="%s/tool.exe">First</a> <a href="%s/tool.exe">Second</a>`,
		firstSrv.URL, secondSrv.URL,
	)
	pageSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, pageBody)
	}))
	defer pageSrv.Close()

	// Pattern matches any http URL ending in .exe — both qualify.
	cmd := makeCmd(pageSrv.URL, `http://[^"]+\.exe`)
	cmd.Check = "cmd /C exit 1" // not installed

	// We don't care about the final status (will fail when the 0-byte exe runs),
	// only that the first server was hit and the second was not.
	ScrapeAndInstall(cmd, false)

	if !firstCalled {
		t.Error("first match: expected first download server to be called")
	}
	if secondCalled {
		t.Error("first match: second download server should NOT be called")
	}
}

// TestScrapeAndInstall_EmptyHTMLBody verifies that an empty page body returns StatusFailed.
func TestScrapeAndInstall_EmptyHTMLBody(t *testing.T) {
	pageSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return 200 with an empty body.
		w.WriteHeader(http.StatusOK)
	}))
	defer pageSrv.Close()

	cmd := makeCmd(pageSrv.URL, `http://[^"]+\.exe`)
	cmd.Check = "cmd /C exit 1"
	res := ScrapeAndInstall(cmd, false)

	if res.Status != reporter.StatusFailed {
		t.Errorf("empty body: want StatusFailed, got %q", res.Status)
	}
	if !strings.Contains(res.Detail, "no download URL") {
		t.Errorf("empty body: detail %q should contain 'no download URL'", res.Detail)
	}
}

// TestScrapeAndInstall_InvalidURLPattern verifies that a malformed regex returns StatusFailed.
func TestScrapeAndInstall_InvalidURLPattern(t *testing.T) {
	pageSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "<html>some content</html>")
	}))
	defer pageSrv.Close()

	cmd := makeCmd(pageSrv.URL, `[invalid(regex`)
	cmd.Check = "cmd /C exit 1"
	res := ScrapeAndInstall(cmd, false)

	if res.Status != reporter.StatusFailed {
		t.Errorf("invalid pattern: want StatusFailed, got %q", res.Status)
	}
	if !strings.Contains(res.Detail, "compile") {
		t.Errorf("invalid pattern: detail %q should contain 'compile'", res.Detail)
	}
}
```

- [ ] Run `go test ./internal/installer/... -v -run "TestScrapeAndInstall_MultipleMatches|TestScrapeAndInstall_EmptyHTMLBody|TestScrapeAndInstall_InvalidURLPattern"` — must PASS.
- [ ] Commit: `test(installer): add scrape URL pattern edge case tests`

---

### Task 2.2 — `RunCheckDetailed` timeout test

**File:** `internal/detector/detector_test.go` (append to existing file)

**Read first:** `internal/detector/detector.go` lines 138–151 — `RunCheckDetailed(checkCmd string) (installed, timedOut bool)`. The timeout is hard-coded at `checkTimeoutSeconds = 15`. The test must use a command that sleeps longer than 15 seconds to trigger the timeout path.

**Important — test duration:** A 15-second sleep per test run is too slow for a normal test suite. Use Go's `t.Skip` with a build tag or an environment variable guard so this test only runs explicitly. The preferred pattern is to check for `TEST_SLOW=1` env var.

- [ ] Append the following test to `internal/detector/detector_test.go`:

```go
// TestRunCheckDetailed_Timeout verifies that a command that exceeds the
// 15-second timeout returns timedOut=true and installed=false.
//
// This test sleeps for longer than the detector's checkTimeoutSeconds (15s),
// so it is guarded by TEST_SLOW=1 to avoid slowing the default test run.
// Run with: TEST_SLOW=1 go test ./internal/detector/... -v -run TestRunCheckDetailed_Timeout -timeout 30s
func TestRunCheckDetailed_Timeout(t *testing.T) {
	if os.Getenv("TEST_SLOW") == "" {
		t.Skip("skipping slow timeout test; set TEST_SLOW=1 to run")
	}

	// "timeout /T 20 /NOBREAK" sleeps for 20 seconds in both interactive and
	// non-interactive Windows cmd.exe contexts, exceeding the 15s detector timeout.
	// Do NOT use bare "timeout 20" — it exits immediately in non-interactive contexts.
	installed, timedOut := detector.RunCheckDetailed("timeout /T 20 /NOBREAK")

	if installed {
		t.Error("expected installed=false for timed-out command")
	}
	if !timedOut {
		t.Error("expected timedOut=true for command that exceeds timeout")
	}
}
```

**Note on imports:** The `detector_test.go` file uses `package detector_test`. The test above requires importing `"os"`. Verify the existing import block already includes `"os"` — if not, add it.

- [ ] Check the import block in `internal/detector/detector_test.go` and add `"os"` if missing.
- [ ] Run `go test ./internal/detector/... -v -run TestRunCheckDetailed_Timeout` — must output `SKIP` (not FAIL) without `TEST_SLOW=1`.
- [ ] Run `TEST_SLOW=1 go test ./internal/detector/... -v -run TestRunCheckDetailed_Timeout -timeout 30s` — must PASS (timedOut=true).
- [ ] Commit: `test(detector): add RunCheckDetailed timeout test (guarded by TEST_SLOW=1)`

---

### Task 2.3 — Table-driven flag conflict tests for cmd

**File:** `cmd/filter_test.go` (append — must stay in `package main`)

**Read first:** `cmd/filter_test.go` — existing tests are already correct and passing. This task converts the individual named tests into table-driven form and adds any missing edge cases. The existing tests cover the primary cases; do not delete them. Append new table-driven tests as additive coverage.

**Key facts:**
- `filterFlagsError(only, exclude string) error` — errors when both non-empty.
- `phaseFlagsError(phase, resumePhase int) error` — errors when both non-default (phase != 0 AND resumePhase != 1).
- `upgradeOnlyFlagsError(upgradeOnly, noUpgrade bool) error` — errors when both true.

- [ ] Append the following table-driven tests to `cmd/filter_test.go`:

```go
func TestFilterFlagsError_Table(t *testing.T) {
	tests := []struct {
		name    string
		only    string
		exclude string
		wantErr bool
	}{
		{"both set — error", "Git.Git", "Steam", true},
		{"only set — ok", "Git.Git", "", false},
		{"exclude set — ok", "", "Steam", false},
		{"neither set — ok", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := filterFlagsError(tt.only, tt.exclude)
			if (err != nil) != tt.wantErr {
				t.Errorf("filterFlagsError(%q, %q) error = %v, wantErr %v", tt.only, tt.exclude, err, tt.wantErr)
			}
		})
	}
}

func TestPhaseFlagsError_Table(t *testing.T) {
	// resumePhase default is 1. phaseFlagsError errors when phase != 0 AND resumePhase != 1.
	tests := []struct {
		name        string
		phase       int
		resumePhase int
		wantErr     bool
	}{
		{"both non-default — error", 2, 2, true},
		{"phase set, resumePhase default (1) — ok", 2, 1, false},
		{"phase default (0), resumePhase set — ok", 0, 2, false},
		{"both default — ok", 0, 1, false},
		{"phase set, resumePhase 3 — error", 1, 3, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := phaseFlagsError(tt.phase, tt.resumePhase)
			if (err != nil) != tt.wantErr {
				t.Errorf("phaseFlagsError(%d, %d) error = %v, wantErr %v", tt.phase, tt.resumePhase, err, tt.wantErr)
			}
		})
	}
}

func TestUpgradeOnlyFlagsError_Table(t *testing.T) {
	tests := []struct {
		name        string
		upgradeOnly bool
		noUpgrade   bool
		wantErr     bool
	}{
		{"both true — error", true, true, true},
		{"upgradeOnly only — ok", true, false, false},
		{"noUpgrade only — ok", false, true, false},
		{"neither — ok", false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := upgradeOnlyFlagsError(tt.upgradeOnly, tt.noUpgrade)
			if (err != nil) != tt.wantErr {
				t.Errorf("upgradeOnlyFlagsError(%v, %v) error = %v, wantErr %v", tt.upgradeOnly, tt.noUpgrade, err, tt.wantErr)
			}
		})
	}
}
```

- [ ] Run `go test ./cmd/... -v -run "TestFilterFlagsError_Table|TestPhaseFlagsError_Table|TestUpgradeOnlyFlagsError_Table"` — must PASS.
- [ ] Commit: `test(cmd): add table-driven flag conflict validation tests`

---

## Final verification

Run the full test suite for all affected packages to confirm nothing is broken:

- [ ] `go test ./internal/config/... -v`
- [ ] `go test ./internal/installer/... -v`
- [ ] `go test ./internal/detector/... -v`
- [ ] `go test ./cmd/... -v`

All tests should PASS (the slow timeout test will SKIP unless `TEST_SLOW=1` is set — that is the correct outcome for a normal run).

---

## Test commands reference

```
# Run all new config tests
go test ./internal/config/... -v -run "TestMergePackages|TestMergeCommands|TestMergeExtensions|TestMergeProfiles|TestMergeSettings|TestLookupProfile"

# Run all new installer tests
go test ./internal/installer/... -v -run "TestScrapeAndInstall_MultipleMatches|TestScrapeAndInstall_EmptyHTMLBody|TestScrapeAndInstall_InvalidURLPattern"

# Run all new detector tests (fast path — timeout test skipped)
go test ./internal/detector/... -v -run "TestRunCheckDetailed_Timeout"

# Run the slow timeout test explicitly
TEST_SLOW=1 go test ./internal/detector/... -v -run "TestRunCheckDetailed_Timeout" -timeout 30s

# Run all new cmd tests
go test ./cmd/... -v -run "TestFilterFlagsError_Table|TestPhaseFlagsError_Table|TestUpgradeOnlyFlagsError_Table"

# Full suite (normal run — slow test will skip)
go test ./internal/config/... ./internal/installer/... ./internal/detector/... ./cmd/... -v
```
