package reporter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Status values for each install result.
const (
	StatusInstalled        = "installed"
	StatusUpgraded         = "upgraded"
	StatusAlready          = "already_installed"
	StatusFailed           = "failed"
	StatusSkipped          = "skipped"
	StatusReboot           = "reboot_required"
	StatusDryRun           = "dry_run"
	StatusShortcutRemoved  = "shortcut_removed"
)

// Result holds the outcome of a single install item.
type Result struct {
	ID       string
	Name     string
	Tier     string // "winget" | "command" | "extension"
	Status   string
	ExitCode int
	Detail   string // Error message, skip reason, etc.
}

// Reporter collects results and writes the final summary.
type Reporter struct {
	results []Result
	logDir  string
	logFile *os.File
}

func New(logDir string) (*Reporter, error) {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("cannot create log directory: %w", err)
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	logPath := filepath.Join(logDir, fmt.Sprintf("KtulueKit_%s_results.log", timestamp))

	f, err := os.Create(logPath)
	if err != nil {
		return nil, fmt.Errorf("cannot create log file: %w", err)
	}

	fmt.Fprintf(f, "KtulueKit-W11 Install Log — %s\n", time.Now().Format(time.RFC1123))
	fmt.Fprintln(f, strings.Repeat("=", 60))
	fmt.Fprintln(f)

	fmt.Printf("Logging to: %s\n\n", logPath)

	return &Reporter{logDir: logDir, logFile: f}, nil
}

// Add records a result and streams it to stdout + log file in real time.
func (r *Reporter) Add(res Result) {
	r.results = append(r.results, res)

	icon := statusIcon(res.Status)
	line := fmt.Sprintf("  %s  %-40s  [%s]", icon, res.Name, res.Tier)
	if res.Detail != "" {
		line += fmt.Sprintf("  — %s", res.Detail)
	}

	fmt.Println(line)
	fmt.Fprintln(r.logFile, line)
}

// Summary prints and writes the final categorized report.
func (r *Reporter) Summary() {
	sections := []struct {
		status string
		icon   string
		label  string
	}{
		{StatusInstalled,       "✅", "Installed successfully"},
		{StatusUpgraded,        "⬆️ ", "Updated to newer version"},
		{StatusAlready,         "⏭️ ", "Already installed (skipped)"},
		{StatusDryRun,          "🔍", "Would install (dry run)"},
		{StatusFailed,          "❌", "Failed"},
		{StatusSkipped,         "⚠️ ", "Skipped (dependency missing)"},
		{StatusReboot,          "🔄", "Reboot required"},
		{StatusShortcutRemoved, "🗑️ ", "Desktop shortcuts removed"},
	}

	header := "\n" + strings.Repeat("=", 60) + "\nSUMMARY\n" + strings.Repeat("=", 60)
	fmt.Println(header)
	fmt.Fprintln(r.logFile, header)

	for _, s := range sections {
		items := r.filterBy(s.status)
		if len(items) == 0 {
			continue
		}

		heading := fmt.Sprintf("\n%s %s (%d)", s.icon, s.label, len(items))
		fmt.Println(heading)
		fmt.Fprintln(r.logFile, heading)

		for _, res := range items {
			line := fmt.Sprintf("    • %s", res.Name)
			if res.Detail != "" {
				line += fmt.Sprintf(": %s", res.Detail)
			}
			fmt.Println(line)
			fmt.Fprintln(r.logFile, line)
		}
	}

	fmt.Println()
	fmt.Fprintln(r.logFile)
}

// HasFailures returns true if any item failed or was skipped due to a missing dependency.
// Used to determine whether to preserve state for resume on next run.
func (r *Reporter) HasFailures() bool {
	for _, res := range r.results {
		if res.Status == StatusFailed || res.Status == StatusSkipped {
			return true
		}
	}
	return false
}

// LogLine writes an arbitrary line directly to the log file (not stdout).
// Used to record critical messages (e.g. resume commands) that must survive a reboot.
func (r *Reporter) LogLine(msg string) {
	if r.logFile != nil {
		fmt.Fprintln(r.logFile, msg)
	}
}

// LogPath returns the absolute path of the current log file.
func (r *Reporter) LogPath() string {
	if r.logFile == nil {
		return ""
	}
	return r.logFile.Name()
}

func (r *Reporter) Close() {
	if r.logFile != nil {
		r.logFile.Close()
	}
}

func (r *Reporter) filterBy(status string) []Result {
	var out []Result
	for _, res := range r.results {
		if res.Status == status {
			out = append(out, res)
		}
	}
	return out
}

// NamesBy returns the Name field of all results with the given status.
func (r *Reporter) NamesBy(status string) []string {
	items := r.filterBy(status)
	names := make([]string, len(items))
	for i, res := range items {
		names[i] = res.Name
	}
	return names
}

func statusIcon(status string) string {
	switch status {
	case StatusInstalled:
		return "✅"
	case StatusUpgraded:
		return "⬆️ "
	case StatusAlready:
		return "⏭️ "
	case StatusFailed:
		return "❌"
	case StatusSkipped:
		return "⚠️ "
	case StatusReboot:
		return "🔄"
	case StatusDryRun:
		return "🔍"
	case StatusShortcutRemoved:
		return "🗑️ "
	default:
		return "  "
	}
}
