package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// resolvedNewPath returns the resolved state path under the given LOCALAPPDATA dir.
// Mirrors the internal statePath() logic for test assertions.
func resolvedNewPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(os.Getenv("LOCALAPPDATA"), "KtulueKit", "state.json")
}

func TestLoad_FreshState(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	s, err := Load()
	if err != nil {
		t.Fatalf("Load() on empty dir: %v", err)
	}
	if len(s.Succeeded) != 0 || len(s.Failed) != 0 {
		t.Error("expected empty state on first load")
	}
}

func TestLoad_UsesNewPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dir)

	newPath := resolvedNewPath(t)
	if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(&State{
		Succeeded: map[string]bool{"pkg1": true},
		Failed:    make(map[string]bool),
	})
	os.WriteFile(newPath, data, 0644)

	s, err := Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if !s.Succeeded["pkg1"] {
		t.Error("expected pkg1 in Succeeded")
	}
}

func TestLoad_MigratesLegacy(t *testing.T) {
	// NOTE: must not call t.Parallel() — uses os.Chdir which is process-global.
	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dir)

	// Write legacy CWD file in a temp dir acting as CWD
	cwdDir := t.TempDir()
	legacyPath := filepath.Join(cwdDir, legacyStateFile)
	data, _ := json.Marshal(&State{
		Succeeded: map[string]bool{"legacy-pkg": true},
		Failed:    make(map[string]bool),
	})
	os.WriteFile(legacyPath, data, 0644)

	// Override CWD resolution for this test
	origDir, _ := os.Getwd()
	os.Chdir(cwdDir)
	defer os.Chdir(origDir)

	s, err := Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if !s.Succeeded["legacy-pkg"] {
		t.Error("expected legacy-pkg migrated to Succeeded")
	}

	// New path must exist
	newPath := resolvedNewPath(t)
	if _, err := os.Stat(newPath); os.IsNotExist(err) {
		t.Error("expected new path to be written after migration")
	}

	// Old CWD file must be deleted
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Error("expected legacy CWD file to be deleted after migration")
	}
}

func TestLoad_NewPathTakesPrecedence(t *testing.T) {
	// NOTE: must not call t.Parallel() — uses os.Chdir which is process-global.
	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dir)

	// Write new path file
	newPath := resolvedNewPath(t)
	os.MkdirAll(filepath.Dir(newPath), 0755)
	newData, _ := json.Marshal(&State{Succeeded: map[string]bool{"new-pkg": true}, Failed: make(map[string]bool)})
	os.WriteFile(newPath, newData, 0644)

	// Write legacy CWD file
	cwdDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(cwdDir)
	defer os.Chdir(origDir)
	legacyPath := filepath.Join(cwdDir, legacyStateFile)
	oldData, _ := json.Marshal(&State{Succeeded: map[string]bool{"old-pkg": true}, Failed: make(map[string]bool)})
	os.WriteFile(legacyPath, oldData, 0644)

	s, err := Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if !s.Succeeded["new-pkg"] {
		t.Error("expected new-pkg from new path")
	}
	if s.Succeeded["old-pkg"] {
		t.Error("old-pkg from legacy path should not be loaded when new path exists")
	}
	// Legacy file must be untouched
	if _, err := os.Stat(legacyPath); os.IsNotExist(err) {
		t.Error("legacy file should be untouched when new path takes precedence")
	}
}

func TestLoad_EmptyLOCALAPPDATA(t *testing.T) {
	// NOTE: must not call t.Parallel() — uses os.Chdir which is process-global.
	t.Setenv("LOCALAPPDATA", "")

	cwdDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(cwdDir)
	defer os.Chdir(origDir)

	legacyPath := filepath.Join(cwdDir, legacyStateFile)
	data, _ := json.Marshal(&State{Succeeded: map[string]bool{"cwd-pkg": true}, Failed: make(map[string]bool)})
	os.WriteFile(legacyPath, data, 0644)

	s, err := Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if !s.Succeeded["cwd-pkg"] {
		t.Error("expected CWD fallback when LOCALAPPDATA is empty")
	}
}

func TestStatePath_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dir)

	s := &State{Succeeded: map[string]bool{"x": true}, Failed: make(map[string]bool)}
	if err := s.Save(); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	newPath := resolvedNewPath(t)
	if _, err := os.Stat(newPath); os.IsNotExist(err) {
		t.Error("Save() should create directory and write file")
	}
}

func TestStatePath_ReturnsNonEmptyString(t *testing.T) {
	// state_test.go is package state (internal), so no package qualifier needed.
	p := StatePath()
	if p == "" {
		t.Error("StatePath() returned empty string")
	}
}

func TestDeleteSucceeded_RemovesFromSucceededMap(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dir)

	s := &State{
		Succeeded: map[string]bool{"Git.Git": true, "Steam.Steam": true},
		Failed:    map[string]bool{"BadPkg": true},
	}
	if err := s.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	s.DeleteSucceeded("Git.Git")

	if s.Succeeded["Git.Git"] {
		t.Error("Git.Git should have been removed from Succeeded")
	}
	if !s.Succeeded["Steam.Steam"] {
		t.Error("Steam.Steam should still be in Succeeded")
	}
	if !s.Failed["BadPkg"] {
		t.Error("Failed map should be unchanged")
	}

	s2, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if s2.Succeeded["Git.Git"] {
		t.Error("Git.Git should not appear in reloaded state")
	}
	if !s2.Succeeded["Steam.Steam"] {
		t.Error("Steam.Steam should persist in reloaded state")
	}
}

func TestDeleteSucceeded_NoopForUnknownID(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dir)

	s := &State{
		Succeeded: map[string]bool{"A": true},
		Failed:    map[string]bool{},
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	s.DeleteSucceeded("DoesNotExist")
	if !s.Succeeded["A"] {
		t.Error("existing entry A should be unaffected")
	}
}
