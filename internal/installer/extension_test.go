package installer

import (
	"testing"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
	"github.com/Ktulue/KtulueKit-W11/internal/reporter"
)

func TestUninstallExtension_URLModeSkipped(t *testing.T) {
	ext := config.Extension{
		ID: "darkreader", Name: "Dark Reader",
		ExtensionID: "eimadpbcbfnmbkopoojfekhnkhdbieeh",
		Browser: "brave", Mode: "url",
	}
	res := UninstallExtension(ext, false)
	if res.Status != reporter.StatusSkipped {
		t.Errorf("status = %q, want StatusSkipped for url mode", res.Status)
	}
}

func TestUninstallExtension_ForceDryRun(t *testing.T) {
	ext := config.Extension{
		ID: "darkreader", Name: "Dark Reader",
		ExtensionID: "eimadpbcbfnmbkopoojfekhnkhdbieeh",
		Browser: "brave", Mode: "force",
	}
	res := UninstallExtension(ext, true)
	if res.Status != reporter.StatusDryRun {
		t.Errorf("status = %q, want StatusDryRun", res.Status)
	}
	if res.Detail == "" {
		t.Error("Detail should describe the registry operation")
	}
}
