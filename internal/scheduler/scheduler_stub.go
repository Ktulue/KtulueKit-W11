//go:build !windows

package scheduler

// CreateResumeTask is a no-op on non-Windows platforms.
func CreateResumeTask(binaryPath, configPath, workDir string, resumePhase int, dryRun bool) error {
	return nil
}

// DeleteResumeTask is a no-op on non-Windows platforms.
func DeleteResumeTask() error {
	return nil
}
