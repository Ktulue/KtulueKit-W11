package installer

import (
	"os"
	"testing"
)

func TestVerifyRuntimePaths_AllPresent(t *testing.T) {
	// Point PATH at a temp dir containing stub executables for all required tools.
	dir := t.TempDir()
	tools := []string{"git", "node", "python", "go", "rustup", "pwsh"}
	for _, name := range tools {
		f, err := os.Create(dir + "/" + name + ".exe")
		if err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
	t.Setenv("PATH", dir)

	missing := VerifyRuntimePaths()
	if len(missing) != 0 {
		t.Errorf("expected no missing tools, got: %v", missing)
	}
}

func TestVerifyRuntimePaths_SomeMissing(t *testing.T) {
	// Point PATH at a temp dir that has only git and node.
	dir := t.TempDir()
	for _, name := range []string{"git", "node"} {
		f, _ := os.Create(dir + "/" + name + ".exe")
		f.Close()
	}
	t.Setenv("PATH", dir)

	missing := VerifyRuntimePaths()
	if len(missing) == 0 {
		t.Fatal("expected some missing tools, got none")
	}
	// python, go, rustup, pwsh should be missing
	missingSet := make(map[string]bool)
	for _, m := range missing {
		missingSet[m] = true
	}
	for _, expected := range []string{"python", "go", "rustup", "pwsh"} {
		if !missingSet[expected] {
			t.Errorf("expected %q in missing list, got %v", expected, missing)
		}
	}
}

func TestVerifyRuntimePaths_NonePresent(t *testing.T) {
	dir := t.TempDir() // empty dir — nothing on PATH
	t.Setenv("PATH", dir)

	missing := VerifyRuntimePaths()
	if len(missing) != 6 {
		t.Errorf("expected 6 missing tools, got %d: %v", len(missing), missing)
	}
}
