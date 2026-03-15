package installer

import (
	"fmt"
	"strings"
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

func TestClassifyWingetExit(t *testing.T) {
	cases := []struct {
		name       string
		code       int
		runErr     error
		wantStatus string
		wantDetail string // substring match; empty string means detail must be exactly empty
	}{
		{
			name:       "zero exit is installed",
			code:       0,
			wantStatus: reporter.StatusInstalled,
			wantDetail: "",
		},
		{
			name:       "UPDATE_NOT_APPLICABLE is already",
			code:       int(uint32(0x8A15002B)), // arrives as large positive on amd64
			wantStatus: reporter.StatusAlready,
			wantDetail: "no update applicable",
		},
		{
			name:       "NO_APPLICATIONS_FOUND is already",
			code:       int(uint32(0x8A150014)),
			wantStatus: reporter.StatusAlready,
			wantDetail: "no applicable upgrade",
		},
		{
			name:       "NO_APPLICABLE_INSTALLER is already",
			code:       int(uint32(0x8A150010)),
			wantStatus: reporter.StatusAlready,
			wantDetail: "no applicable installer",
		},
		{
			name:       "reboot pending code",
			code:       int(uint32(0x8A150011)),
			wantStatus: reporter.StatusReboot,
			wantDetail: "reboot",
		},
		{
			name:       "INSTALLER_PROHIBITS_ELEVATION is failed",
			code:       int(uint32(0x8A150056)),
			wantStatus: reporter.StatusFailed,
			wantDetail: "elevation",
		},
		{
			name:       "INSTALL_CONTACT_SUPPORT is failed",
			code:       int(uint32(0x8A150108)),
			wantStatus: reporter.StatusFailed,
			wantDetail: "contact support",
		},
		{
			name:       "INSTALL_SYSTEM_NOT_SUPPORTED is failed",
			code:       int(uint32(0x8A150113)),
			wantStatus: reporter.StatusFailed,
			wantDetail: "system does not meet",
		},
		{
			name:       "unknown code includes hex in detail",
			code:       42,
			wantStatus: reporter.StatusFailed,
			wantDetail: "0x0000002A",
		},
		{
			name:       "unknown code with run error appends error message",
			code:       1,
			runErr:     fmt.Errorf("exec failed"),
			wantStatus: reporter.StatusFailed,
			wantDetail: "exec failed",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotStatus, gotDetail := classifyWingetExit(tc.code, tc.runErr)
			if gotStatus != tc.wantStatus {
				t.Errorf("status = %q, want %q", gotStatus, tc.wantStatus)
			}
			if tc.wantDetail == "" {
				if gotDetail != "" {
					t.Errorf("detail = %q, want empty", gotDetail)
				}
			} else if !strings.Contains(gotDetail, tc.wantDetail) {
				t.Errorf("detail = %q, want it to contain %q", gotDetail, tc.wantDetail)
			}
		})
	}
}

func TestBuildWingetArgs(t *testing.T) {
	cases := []struct {
		name     string
		pkg      config.Package
		wantArgs []string // exact ordered match
	}{
		{
			name: "basic package with machine scope",
			pkg:  config.Package{ID: "Git.Git", Scope: "machine"},
			wantArgs: []string{
				"install", "-e",
				"--id", "Git.Git",
				"--scope", "machine",
				"--accept-package-agreements",
				"--accept-source-agreements",
				"--disable-interactivity",
			},
		},
		{
			name: "package with version appends --version flag",
			pkg:  config.Package{ID: "Node.JS", Scope: "machine", Version: "20.0.0"},
			wantArgs: []string{
				"install", "-e",
				"--id", "Node.JS",
				"--scope", "machine",
				"--accept-package-agreements",
				"--accept-source-agreements",
				"--disable-interactivity",
				"--version", "20.0.0",
			},
		},
		{
			name: "user-scoped package uses user scope",
			pkg:  config.Package{ID: "Spotify.Spotify", Scope: "user"},
			wantArgs: []string{
				"install", "-e",
				"--id", "Spotify.Spotify",
				"--scope", "user",
				"--accept-package-agreements",
				"--accept-source-agreements",
				"--disable-interactivity",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildWingetArgs(tc.pkg)
			if len(got) != len(tc.wantArgs) {
				t.Fatalf("args length = %d, want %d\n  got:  %v\n  want: %v",
					len(got), len(tc.wantArgs), got, tc.wantArgs)
			}
			for i := range got {
				if got[i] != tc.wantArgs[i] {
					t.Errorf("args[%d] = %q, want %q", i, got[i], tc.wantArgs[i])
				}
			}
		})
	}
}
