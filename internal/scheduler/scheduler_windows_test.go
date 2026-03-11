package scheduler

import (
	"strings"
	"testing"
)

func TestBuildTaskScript_ContainsAllParts(t *testing.T) {
	script := buildTaskScript(
		`C:\tools\ktuluekit.exe`,
		`C:\config\ktuluekit.json`,
		`C:\tools`,
		4,
	)

	checks := []struct {
		label string
		want  string
	}{
		{"binary path", `C:\tools\ktuluekit.exe`},
		{"config path", `C:\config\ktuluekit.json`},
		{"working dir", `C:\tools`},
		{"resume phase", `--resume-phase=4`},
		{"task name", `KtulueKit-Resume`},
		{"run level", `Highest`},
		{"logon type", `Interactive`},
		{"force flag", `-Force`},
	}

	for _, c := range checks {
		if !strings.Contains(script, c.want) {
			t.Errorf("script missing %s: expected to find %q", c.label, c.want)
		}
	}
}

func TestBuildTaskScript_EscapesSingleQuotes(t *testing.T) {
	// Windows paths won't normally have single quotes, but we must handle it.
	script := buildTaskScript(
		`C:\it's\ktuluekit.exe`,
		`C:\config\ktuluekit.json`,
		`C:\tools`,
		1,
	)

	if strings.Contains(script, `it's`) {
		t.Error("unescaped single quote found in script — PowerShell will break")
	}
	if !strings.Contains(script, `it''s`) {
		t.Error("expected single quote to be doubled (it''s) for PowerShell string escaping")
	}
}

func TestBuildTaskScript_QuotesBinaryPathWithSpaces(t *testing.T) {
	// -Execute must double-quote the binary path so Windows CreateProcess handles
	// paths containing spaces correctly (e.g. C:\Program Files\ktuluekit.exe).
	script := buildTaskScript(
		`C:\Program Files\ktuluekit.exe`,
		`C:\config\ktuluekit.json`,
		`C:\tools`,
		2,
	)

	// The binary path must be wrapped in double-quotes inside the PS string.
	if !strings.Contains(script, `"C:\Program Files\ktuluekit.exe"`) {
		t.Error("binary path with spaces is not double-quoted in -Execute — will fail at runtime")
	}
}
