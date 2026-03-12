package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
	"github.com/Ktulue/KtulueKit-W11/internal/detector"
	"github.com/Ktulue/KtulueKit-W11/internal/exporter"
	"github.com/Ktulue/KtulueKit-W11/internal/state"
)

// Version is set at build time via ldflags: -X main.Version=x.y.z
// Falls back to "dev" when empty (handled in exporter.Export).
var Version string

var (
	exportOutput string
	exportFast   bool
)

// snapshotFile is the JSON output structure. It mirrors config.Config but adds
// snapshot metadata and omits $schema (not valid for replay snapshots).
type snapshotFile struct {
	Version    string               `json:"version"`
	Snapshot   exporter.SnapshotMeta `json:"snapshot"`
	Metadata   config.Metadata      `json:"metadata"`
	Settings   config.Settings      `json:"settings"`
	Packages   []config.Package     `json:"packages"`
	Commands   []config.Command     `json:"commands"`
	Extensions []config.Extension   `json:"extensions"`
	Profiles   []config.Profile     `json:"profiles"`
}

func runExport(_ *cobra.Command, _ []string) error {
	// Resolve config path.
	paths := configPaths
	if len(paths) == 0 {
		paths = []string{"ktuluekit.json"}
	}

	// Resolve output path.
	outPath := exportOutput
	if outPath == "" {
		outPath = "ktuluekit-snapshot.json"
	}

	// Load and validate config.
	cfg, err := config.LoadAll(paths)
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}
	if errs := config.Validate(cfg); len(errs) > 0 {
		return fmt.Errorf("config validation failed: %w", errs[0])
	}

	// Resolve absolute config path for snapshot metadata.
	absConfig, err := filepath.Abs(paths[0])
	if err != nil {
		absConfig = paths[0]
	}

	// Get machine name.
	machine, err := os.Hostname()
	if err != nil {
		machine = "unknown"
	}

	opts := exporter.Options{
		SourceConfig: absConfig,
		ToolVersion:  Version,
		Machine:      machine,
	}

	if exportFast {
		// Fast mode: use the state file instead of running check commands.
		sp := state.StatePath()
		if _, err := os.Stat(sp); err != nil {
			return fmt.Errorf("--fast requires a state file at %s (not found): %w", sp, err)
		}
		s, err := state.Load()
		if err != nil {
			return fmt.Errorf("state file is corrupt or unreadable: %w", err)
		}
		opts.Fast = true
		opts.State = s
	} else {
		// Check mode: wrap detector.RunCheckDetailed into exporter.CheckResult.
		opts.CheckFn = func(checkCmd string) exporter.CheckResult {
			installed, timedOut := detector.RunCheckDetailed(checkCmd)
			switch {
			case timedOut:
				return exporter.CheckTimedOut
			case installed:
				return exporter.CheckInstalled
			default:
				return exporter.CheckAbsent
			}
		}
	}

	// Print progress header.
	mode := "check"
	if exportFast {
		mode = "fast"
	}
	checkable := countCheckable(cfg)
	fmt.Printf("Config:  %v\n", paths)
	fmt.Printf("Mode:    %s\n", mode)
	fmt.Printf("Items:   %d checkable\n\n", checkable)

	// Run export.
	result, err := exporter.Export(cfg, opts)
	if err != nil {
		return fmt.Errorf("export error: %w", err)
	}

	// Ensure nil slices marshal as [] rather than null for snapshot consumers.
	pkgs := result.Packages
	if pkgs == nil {
		pkgs = []config.Package{}
	}
	cmds := result.Commands
	if cmds == nil {
		cmds = []config.Command{}
	}
	exts := result.Extensions
	if exts == nil {
		exts = []config.Extension{}
	}
	profiles := result.Profiles
	if profiles == nil {
		profiles = []config.Profile{}
	}

	// Build output struct.
	snap := snapshotFile{
		Version:    cfg.Version,
		Snapshot:   result.Snapshot,
		Metadata:   cfg.Metadata,
		Settings:   cfg.Settings,
		Packages:   pkgs,
		Commands:   cmds,
		Extensions: exts,
		Profiles:   profiles,
	}

	// Marshal and write.
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(outPath, data, 0644); err != nil {
		return fmt.Errorf("write error (%s): %w", outPath, err)
	}

	// Print summary.
	fmt.Printf("Checked:  %d items\n", result.Checked)
	fmt.Printf("Included: %d items\n", result.Included)
	fmt.Printf("Output:   %s\n", outPath)

	return nil
}

// countCheckable returns the number of packages and commands that have a
// non-empty check command that is not "echo skip".
func countCheckable(cfg *config.Config) int {
	n := 0
	for _, pkg := range cfg.Packages {
		if pkg.Check != "" && pkg.Check != "echo skip" {
			n++
		}
	}
	for _, cmd := range cfg.Commands {
		if cmd.Check != "" && cmd.Check != "echo skip" {
			n++
		}
	}
	return n
}
