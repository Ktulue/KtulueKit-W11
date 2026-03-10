package installer

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
	"github.com/Ktulue/KtulueKit-W11/internal/reporter"
	"github.com/Ktulue/KtulueKit-W11/internal/state"
)

// RefreshPath updates the current process PATH from the Windows registry.
// Call this after installing runtimes so that npm, go, etc. are findable.
func RefreshPath() {
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		`[System.Environment]::GetEnvironmentVariable("Path","Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path","User")`)
	out, err := cmd.Output()
	if err != nil {
		fmt.Println("  warning: could not refresh PATH from registry")
		return
	}
	newPath := strings.TrimSpace(string(out))
	if newPath != "" {
		// Set it in the current process environment
		_ = setenv("PATH", newPath)
		fmt.Println("  PATH refreshed from registry.")
	}
}

// RunCommand executes a Tier 2 shell command, checking if it's already done first.
func RunCommand(cmd config.Command, dryRun bool, retryCount int, s *state.State) reporter.Result {
	res := reporter.Result{
		ID:   cmd.ID,
		Name: cmd.Name,
		Tier: "command",
	}

	if cmd.Notes != "" && dryRun {
		fmt.Printf("    note: %s\n", cmd.Notes)
	}

	if dryRun {
		res.Status = reporter.StatusDryRun
		res.Detail = cmd.Cmd
		return res
	}

	// Check if already installed
	if isAlreadyInstalled(cmd.Check) {
		res.Status = reporter.StatusAlready
		res.Detail = fmt.Sprintf("check passed: %s", cmd.Check)
		return res
	}

	var exitCode int
	var err error

	for attempt := 0; attempt <= retryCount; attempt++ {
		if attempt > 0 {
			fmt.Printf("    retrying %s (attempt %d)...\n", cmd.Name, attempt+1)
			time.Sleep(3 * time.Second)
		}

		exitCode, err = runShellWithTimeout(cmd.Cmd, cmd.TimeoutSeconds)
		if err == nil || exitCode == 0 {
			break
		}
	}

	res.ExitCode = exitCode
	if exitCode == 0 && err == nil {
		res.Status = reporter.StatusInstalled
		if cmd.RebootAfter {
			res.Status = reporter.StatusReboot
			res.Detail = "reboot required before this is usable"
		}
	} else {
		res.Status = reporter.StatusFailed
		res.Detail = fmt.Sprintf("exit code %d", exitCode)
		if err != nil {
			res.Detail += fmt.Sprintf(": %s", err.Error())
		}
	}

	return res
}

// checkTimeoutSeconds is a short timeout used for "already installed?" detection commands.
// These commands (e.g. "claude --version") should complete in well under 15 seconds.
const checkTimeoutSeconds = 15

// isAlreadyInstalled runs the check command and returns true if exit code is 0.
func isAlreadyInstalled(checkCmd string) bool {
	if checkCmd == "" || checkCmd == "echo skip" {
		return false
	}
	code, _ := runShellWithTimeout(checkCmd, checkTimeoutSeconds)
	return code == 0
}

// runShellWithTimeout runs a shell command via cmd.exe with a timeout.
func runShellWithTimeout(command string, timeoutSeconds int) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "cmd", "/C", command)
	output, err := cmd.CombinedOutput()

	if len(output) > 0 {
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
			return exitErr.ExitCode(), nil
		}
		return -1, err
	}

	return 0, nil
}
