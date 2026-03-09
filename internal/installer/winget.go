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
// Reference: https://github.com/microsoft/winget-cli/blob/master/doc/windows/package-manager/winget/returnCodes.md
func classifyWingetExit(code int, runErr error) (status, detail string) {
	switch code {
	case 0:
		return reporter.StatusInstalled, ""
	case -1978335189: // 0x8A150013 — already installed
		return reporter.StatusAlready, "winget: already installed"
	case -1978335212: // 0x8A1500F4 — no applicable upgrade
		return reporter.StatusAlready, "winget: no available upgrade found"
	case -1978335215: // 0x8A150011 — install blocked by policy / reboot pending
		return reporter.StatusReboot, "winget: reboot may be required"
	default:
		detail := fmt.Sprintf("exit code %d", code)
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
