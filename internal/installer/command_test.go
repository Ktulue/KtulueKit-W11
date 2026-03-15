package installer

import (
	"testing"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
	"github.com/Ktulue/KtulueKit-W11/internal/reporter"
)

func TestRunUninstallCommand_ScrapeURLSkipped(t *testing.T) {
	cmd := config.Command{
		ID: "app", Name: "App",
		ScrapeURL:    "https://example.com/download",
		UninstallCmd: "uninstall.exe", // ScrapeURL takes precedence
	}
	res := RunUninstallCommand(cmd, false)
	if res.Status != reporter.StatusSkipped {
		t.Errorf("status = %q, want StatusSkipped (T4 must skip)", res.Status)
	}
}

func TestRunUninstallCommand_NoUninstallCmdSkipped(t *testing.T) {
	cmd := config.Command{ID: "npm-tool", Name: "npm tool", Cmd: "npm install -g something"}
	res := RunUninstallCommand(cmd, false)
	if res.Status != reporter.StatusSkipped {
		t.Errorf("status = %q, want StatusSkipped", res.Status)
	}
}

func TestRunUninstallCommand_DryRun(t *testing.T) {
	cmd := config.Command{
		ID: "npm-tool", Name: "npm tool",
		UninstallCmd: "npm uninstall -g something",
	}
	res := RunUninstallCommand(cmd, true)
	if res.Status != reporter.StatusDryRun {
		t.Errorf("status = %q, want StatusDryRun", res.Status)
	}
	if res.Detail == "" {
		t.Error("Detail should contain the uninstall command preview")
	}
}
