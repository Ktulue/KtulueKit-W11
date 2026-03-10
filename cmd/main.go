package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
	"github.com/Ktulue/KtulueKit-W11/internal/reporter"
	"github.com/Ktulue/KtulueKit-W11/internal/runner"
	"github.com/Ktulue/KtulueKit-W11/internal/state"
)

var (
	configPath  string
	dryRun      bool
	resumePhase int
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

	root.PersistentFlags().StringVarP(&configPath, "config", "c", "ktuluekit.json", "Path to config file")
	root.PersistentFlags().BoolVarP(&dryRun, "dry-run", "d", false, "Show what would be installed without doing it")
	root.PersistentFlags().IntVar(&resumePhase, "resume-phase", 1, "Skip all phases before this number (for post-reboot resume)")

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runInstall(cmd *cobra.Command, args []string) error {
	if !dryRun && !isAdmin() {
		return fmt.Errorf("ktuluekit must be run as Administrator\n  Right-click your terminal and select 'Run as administrator', then try again")
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("config error: %w", err)
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

	fmt.Printf("Config:  %s\n", configPath)
	fmt.Printf("Packages: %d winget  |  %d commands  |  %d extensions\n\n",
		len(cfg.Packages), len(cfg.Commands), len(cfg.Extensions))

	r := runner.New(cfg, rep, s, dryRun, resumePhase)
	r.Run()

	rep.Summary()

	// Only clear state on a fully clean run. If anything failed or was skipped,
	// preserve state so --resume-phase re-runs know what already succeeded.
	if !dryRun && !rep.HasFailures() {
		_ = state.Clear()
	}

	return nil
}
