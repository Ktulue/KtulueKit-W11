package installer

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
	"github.com/Ktulue/KtulueKit-W11/internal/reporter"
)

// InstallPackage runs a single winget install for a Tier 1 package.
// Returns a Result reflecting what happened.
func InstallPackage(pkg config.Package, dryRun bool, retryCount int) reporter.Result {
	res := reporter.Result{
		ID:   pkg.ID,
		Name: pkg.Name,
		Tier: "winget",
	}

	if pkg.Notes != "" && dryRun {
		fmt.Printf("    note: %s\n", pkg.Notes)
	}

	if dryRun {
		res.Status = reporter.StatusDryRun
		res.Detail = fmt.Sprintf("winget install -e --id %s --scope %s", pkg.ID, pkg.Scope)
		return res
	}

	args := buildWingetArgs(pkg)

	var exitCode int
	var err error

	for attempt := 0; attempt <= retryCount; attempt++ {
		if attempt > 0 {
			fmt.Printf("    retrying %s (attempt %d)...\n", pkg.Name, attempt+1)
			time.Sleep(3 * time.Second)
		}

		exitCode, err = runWithTimeout(args, pkg.TimeoutSeconds)
		if err == nil || exitCode == 0 {
			break
		}
	}

	res.ExitCode = exitCode
	res.Status, res.Detail = classifyWingetExit(exitCode, err)
	return res
}

// buildWingetArgs constructs the winget install argument slice.
func buildWingetArgs(pkg config.Package) []string {
	args := []string{
		"install",
		"-e",
		"--id", pkg.ID,
		"--scope", pkg.Scope,
		"--accept-package-agreements",
		"--accept-source-agreements",
		"--disable-interactivity",
	}
	return args
}

// classifyWingetExit maps winget exit codes to Result statuses.
//
// Windows HRESULTs are 32-bit signed values. Go's exec.ExitCode() returns an
// int (64-bit on amd64), so a code like 0x8A15002B arrives as 2316632107 (large
// positive) rather than -1978335189. Casting to int32 before the switch restores
// the expected sign so the cases match correctly.
//
// Reference: https://github.com/microsoft/winget-cli/blob/master/src/AppInstallerSharedLib/Public/AppInstallerErrors.h
func classifyWingetExit(code int, runErr error) (status, detail string) {
	switch int32(code) {
	case 0:
		return reporter.StatusInstalled, ""

	// ── Already up to date / nothing to do ────────────────────────────────
	case -1978335189: // 0x8A15002B — UPDATE_NOT_APPLICABLE
		return reporter.StatusAlready, "winget: no update applicable (already up to date)"
	case -1978335212: // 0x8A150014 — NO_APPLICATIONS_FOUND (no upgrade available)
		return reporter.StatusAlready, "winget: no applicable upgrade found"
	case -1978335216: // 0x8A150010 — NO_APPLICABLE_INSTALLER (already installed via other method)
		return reporter.StatusAlready, "winget: no applicable installer (package may already be installed)"

	// ── Reboot / environment ───────────────────────────────────────────────
	case -1978335215: // 0x8A150011 — install blocked / reboot pending
		return reporter.StatusReboot, "winget: reboot may be required"

	// ── Real failures ──────────────────────────────────────────────────────
	case -1978335146: // 0x8A150056 — INSTALLER_PROHIBITS_ELEVATION
		return reporter.StatusFailed, "winget: installer prohibits elevation (try --scope user or run without admin)"
	case -1978334968: // 0x8A150108 — INSTALL_CONTACT_SUPPORT
		return reporter.StatusFailed, "winget: installer error (contact support)"
	case -1978334957: // 0x8A150113 — INSTALL_SYSTEM_NOT_SUPPORTED
		return reporter.StatusFailed, "winget: system does not meet package requirements"

	default:
		detail := fmt.Sprintf("exit code %d (0x%08X)", code, uint32(code))
		if runErr != nil {
			detail += fmt.Sprintf(": %s", runErr.Error())
		}
		return reporter.StatusFailed, detail
	}
}

// runWithTimeout executes winget with the given args and a timeout.
// Returns the exit code and any execution error.
func runWithTimeout(args []string, timeoutSeconds int) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "winget", args...)

	output, err := cmd.CombinedOutput()
	if len(output) > 0 {
		// Stream output lines prefixed so they're visually nested
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
			return exitErr.ExitCode(), nil // non-zero exit, but not an exec failure
		}
		return -1, err
	}

	return 0, nil
}
