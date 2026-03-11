package scheduler

import (
	"fmt"
	"os/exec"
	"strings"
)

const taskName = "KtulueKit-Resume"

// CreateResumeTask registers a one-shot Windows Scheduled Task that re-launches
// KtulueKit at the next interactive logon with the given resume phase.
// The task runs with HIGHEST privileges under the current user.
// If dryRun is true, prints intent and the generated script, then returns without executing.
func CreateResumeTask(binaryPath, configPath, workDir string, resumePhase int, dryRun bool) error {
	script := buildTaskScript(binaryPath, configPath, workDir, resumePhase)

	if dryRun {
		fmt.Printf("  [dry-run] Would register Scheduled Task '%s' via PowerShell:\n", taskName)
		fmt.Printf("    %s\n", script)
		return nil
	}

	cmd := exec.Command("powershell", "-NonInteractive", "-NoProfile", "-Command", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not register scheduled task: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// DeleteResumeTask removes the KtulueKit-Resume scheduled task if it exists.
// Returns nil regardless — absence of the task is not an error.
func DeleteResumeTask() error {
	// Intentionally ignores all errors: non-zero exit when task doesn't exist is expected.
	_ = exec.Command("schtasks", "/delete", "/tn", taskName, "/f").Run()
	return nil
}

// buildTaskScript returns the PowerShell one-liner that creates (or replaces)
// the KtulueKit-Resume scheduled task with the correct action, trigger, and principal.
func buildTaskScript(binaryPath, configPath, workDir string, resumePhase int) string {
	bin := escapeSingleQuote(binaryPath)
	cfg := escapeSingleQuote(configPath)
	wd := escapeSingleQuote(workDir)

	// NOTE: -Execute uses '"%s"' (double-quotes inside single-quoted PS string) so that
	// Windows CreateProcess correctly handles paths containing spaces. WorkingDirectory
	// uses single-quotes only — it is passed as a directory path, not a command token.
	return fmt.Sprintf(
		`$a = New-ScheduledTaskAction -Execute '"%s"' -Argument '--config "%s" --resume-phase=%d' -WorkingDirectory '%s'; `+
			`$t = New-ScheduledTaskTrigger -AtLogOn; `+
			`$s = New-ScheduledTaskSettingsSet -ExecutionTimeLimit (New-TimeSpan -Hours 4) -MultipleInstances IgnoreNew; `+
			`$p = New-ScheduledTaskPrincipal -UserId $env:USERNAME -RunLevel Highest -LogonType Interactive; `+
			`Register-ScheduledTask -TaskName '%s' -Action $a -Trigger $t -Settings $s -Principal $p -Force`,
		bin, cfg, resumePhase, wd, taskName,
	)
}

// escapeSingleQuote doubles any single quotes in s so it can be safely embedded
// inside a PowerShell single-quoted string.
func escapeSingleQuote(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
