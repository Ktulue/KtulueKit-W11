package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
	"github.com/Ktulue/KtulueKit-W11/internal/desktop"
	"github.com/Ktulue/KtulueKit-W11/internal/reporter"
	"github.com/Ktulue/KtulueKit-W11/internal/runner"
	"github.com/Ktulue/KtulueKit-W11/internal/scheduler"
	"github.com/Ktulue/KtulueKit-W11/internal/state"
)

var (
	configPaths        []string
	dryRun             bool
	resumePhase        int
	noDesktopShortcuts bool
	noUpgrade          bool
)

func main() {
	root := &cobra.Command{
		Use:   "ktuluekit",
		Short: "KtulueKit-W11 — personal software stack installer",
		Long: `KtulueKit-W11 reads a declarative JSON config and installs your full
Windows 11 software stack in dependency order across three tiers:
  Tier 1 — Winget packages
  Tier 2 — Shell commands (npm, wsl, etc.)
  Tier 3 — Browser extensions`,
		RunE: runInstall,
	}

	root.PersistentFlags().StringArrayVarP(&configPaths, "config", "c", nil, "Path to config file (repeatable: --config base.json --config extras.json)")
	root.PersistentFlags().BoolVarP(&dryRun, "dry-run", "d", false, "Show what would be installed without doing it")
	root.PersistentFlags().IntVar(&resumePhase, "resume-phase", 1, "Skip all phases before this number (for post-reboot resume)")
	root.PersistentFlags().BoolVar(&noDesktopShortcuts, "no-desktop-shortcuts", false, "Automatically remove all desktop shortcuts created by installers (skips prompt)")
	root.PersistentFlags().BoolVar(&noUpgrade, "no-upgrade", false, "Skip upgrades for already-installed packages even if upgrade_if_installed is set in config")

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Scan machine and show install status for all configured items",
		RunE:  runStatus,
	}
	root.AddCommand(statusCmd)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runInstall(cmd *cobra.Command, args []string) error {
	// Always delete the resume task first — cleans up after a previous reboot run.
	// No-op (and error ignored) if the task doesn't exist.
	_ = scheduler.DeleteResumeTask()

	if !dryRun && !isAdmin() {
		return fmt.Errorf("ktuluekit must be run as Administrator\n  Right-click your terminal and select 'Run as administrator', then try again")
	}

	cfg, err := config.LoadAll(configPaths)
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}
	if noUpgrade {
		cfg.Settings.UpgradeIfInstalled = false
	}

	rep, err := reporter.New(cfg.Settings.LogDir)
	if err != nil {
		return fmt.Errorf("reporter error: %w", err)
	}
	defer rep.Close()

	s, err := state.Load()
	if err != nil {
		return fmt.Errorf("state error: %w", err)
	}

	if dryRun {
		fmt.Println("DRY RUN — no changes will be made.")
		fmt.Println()
	}

	// Use the first config path for reporting, defaulting if empty
	reportingPath := configPaths
	if len(reportingPath) == 0 {
		reportingPath = []string{"ktuluekit.json"}
	}

	fmt.Printf("Config:  %v\n", reportingPath)
	fmt.Printf("Packages: %d winget  |  %d commands  |  %d extensions\n\n",
		len(cfg.Packages), len(cfg.Commands), len(cfg.Extensions))

	// Determine how to handle desktop shortcuts created by installers.
	var shortcutMode desktop.ShortcutMode
	switch {
	case dryRun:
		shortcutMode = desktop.ShortcutKeep // nothing is installed, nothing to clean up
	case noDesktopShortcuts:
		shortcutMode = desktop.ShortcutRemove
		fmt.Println("  Desktop shortcuts: auto-remove (--no-desktop-shortcuts)")
	default:
		shortcutMode = desktop.PromptMode()
	}

	r := runner.New(cfg, rep, s, dryRun, resumePhase, reportingPath[0], shortcutMode)

	runStart := time.Now()
	r.Run()

	rep.Summary()
	fmt.Printf("Total elapsed: %s\n", time.Since(runStart).Round(time.Second))

	// Only clear state on a fully clean run. If anything failed or was skipped,
	// preserve state so --resume-phase re-runs know what already succeeded.
	if !dryRun && !rep.HasFailures() {
		_ = state.Clear()
	}

	return nil
}
