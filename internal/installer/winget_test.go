package installer

import (
	"testing"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
	"github.com/Ktulue/KtulueKit-W11/internal/reporter"
)

func TestUninstallPackage_DryRun(t *testing.T) {
	pkg := config.Package{ID: "Git.Git", Name: "Git", TimeoutSeconds: 60}
	res := UninstallPackage(pkg, true)
	if res.Status != reporter.StatusDryRun {
		t.Errorf("status = %q, want %q", res.Status, reporter.StatusDryRun)
	}
	if res.Detail == "" {
		t.Error("Detail should contain the winget uninstall command")
	}
}

func TestUninstallPackage_SkippedWhenCheckFails(t *testing.T) {
	pkg := config.Package{
		ID:             "NotInstalled.Package",
		Name:           "Not Installed",
		Check:          "cmd /C exit 1",
		TimeoutSeconds: 15,
	}
	res := UninstallPackage(pkg, false)
	if res.Status != reporter.StatusSkipped {
		t.Errorf("status = %q, want %q", res.Status, reporter.StatusSkipped)
	}
}
