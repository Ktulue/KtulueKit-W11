package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
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
	onlyIDs            string
	excludeIDs         string
	onlyPhase          int
	upgradeOnly        bool
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
	root.PersistentFlags().StringVar(&onlyIDs, "only", "", "comma-separated IDs to install (skip all others)")
	root.PersistentFlags().StringVar(&excludeIDs, "exclude", "", "comma-separated IDs to exclude from install")
	root.PersistentFlags().IntVar(&onlyPhase, "phase", 0, "Run only this phase number (0 = run all phases)")
	root.PersistentFlags().BoolVar(&upgradeOnly, "upgrade-only", false, "Skip packages not yet installed; force upgrade on installed ones")

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Scan machine and show install status for all configured items",
		RunE:  runStatus,
	}
	root.AddCommand(statusCmd)

	validateCmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate config file(s) and report all errors",
		RunE:  runValidate,
	}
	root.AddCommand(validateCmd)

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all configured items grouped by phase and tier",
		RunE:  runList,
	}
	root.AddCommand(listCmd)

	exportCmd := &cobra.Command{
		Use:   "export",
		Short: "Scan machine and write a replay-ready ktuluekit-snapshot.json",
		Long: `Export scans the current machine against the reference config.
Items whose check command passes are included in the snapshot.
The snapshot is a valid ktuluekit.json (replay on a new machine)
and the handoff artifact for KtulueKit-Migration.

Use --fast to skip check commands and use the KtulueKit state file instead.`,
		RunE: runExport,
	}
	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "Output path (default: ktuluekit-snapshot.json in cwd)")
	exportCmd.Flags().BoolVar(&exportFast, "fast", false, "Use state file instead of running check commands")
	root.AddCommand(exportCmd)

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
	if errs := config.Validate(cfg); len(errs) > 0 {
		return fmt.Errorf("config validation failed: %w", errs[0])
	}
	if noUpgrade {
		cfg.Settings.UpgradeIfInstalled = false
	}
	if upgradeOnly {
		cfg.Settings.UpgradeIfInstalled = true
	}

	if err := filterFlagsError(onlyIDs, excludeIDs); err != nil {
		return err
	}
	if err := phaseFlagsError(onlyPhase, resumePhase); err != nil {
		return err
	}
	if err := upgradeOnlyFlagsError(upgradeOnly, noUpgrade); err != nil {
		return err
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

	if onlyPhase > 0 {
		r.SetOnlyPhase(onlyPhase)
	}
	if upgradeOnly {
		r.SetUpgradeOnly(true)
	}

	if onlyIDs != "" {
		selected, unknowns := buildOnlySet(onlyIDs, allConfigIDs(cfg))
		for _, id := range unknowns {
			fmt.Printf("%sWARN%s  unknown ID in --only: %q\n", colorYellow, colorReset, id)
		}
		r.SetSelectedIDs(selected)
	} else if excludeIDs != "" {
		remaining, unknowns := buildExcludeSet(excludeIDs, allConfigIDs(cfg))
		for _, id := range unknowns {
			fmt.Printf("%sWARN%s  unknown ID in --exclude: %q\n", colorYellow, colorReset, id)
		}
		r.SetSelectedIDs(remaining)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	runStart := time.Now()
	r.Run(ctx)

	rep.Summary()
	fmt.Printf("Total elapsed: %s\n", time.Since(runStart).Round(time.Second))

	// Only clear state on a fully clean, complete run.
	// Preserve state if there were failures OR if the run was interrupted early.
	if !dryRun && !rep.HasFailures() && !r.WasInterrupted() {
		_ = state.Clear()
	}

	return nil
}

// buildOnlySet parses a comma-separated raw string and returns (selected, unknowns).
// IDs absent from known are appended to unknowns but still added to selected.
func buildOnlySet(raw string, known map[string]bool) (selected map[string]bool, unknowns []string) {
	selected = make(map[string]bool)
	for _, id := range strings.Split(raw, ",") {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if !known[id] {
			unknowns = append(unknowns, id)
		}
		selected[id] = true
	}
	return selected, unknowns
}

// buildExcludeSet starts with all known IDs and removes those in raw.
// IDs absent from all are appended to unknowns; the delete is a no-op.
func buildExcludeSet(raw string, all map[string]bool) (remaining map[string]bool, unknowns []string) {
	remaining = make(map[string]bool, len(all))
	for id := range all {
		remaining[id] = true
	}
	for _, id := range strings.Split(raw, ",") {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if !all[id] {
			unknowns = append(unknowns, id)
		}
		delete(remaining, id)
	}
	return remaining, unknowns
}

// filterFlagsError returns an error if --only and --exclude are both set.
func filterFlagsError(only, exclude string) error {
	if only != "" && exclude != "" {
		return fmt.Errorf("--only and --exclude are mutually exclusive")
	}
	return nil
}

// phaseFlagsError returns an error if --phase and --resume-phase are both set to non-default values.
func phaseFlagsError(phase, resumePhase int) error {
	if phase > 0 && resumePhase > 1 {
		return fmt.Errorf("--phase and --resume-phase are mutually exclusive")
	}
	return nil
}

// upgradeOnlyFlagsError returns an error if --upgrade-only and --no-upgrade are both set.
func upgradeOnlyFlagsError(upgradeOnly, noUpgrade bool) error {
	if upgradeOnly && noUpgrade {
		return fmt.Errorf("--upgrade-only and --no-upgrade are mutually exclusive")
	}
	return nil
}

// allConfigIDs returns a set of all item IDs declared across all three tiers.
func allConfigIDs(cfg *config.Config) map[string]bool {
	ids := make(map[string]bool)
	for _, p := range cfg.Packages {
		ids[p.ID] = true
	}
	for _, c := range cfg.Commands {
		ids[c.ID] = true
	}
	for _, e := range cfg.Extensions {
		ids[e.ID] = true
	}
	return ids
}
