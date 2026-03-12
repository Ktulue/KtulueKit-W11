package exporter

import (
	"fmt"
	"time"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
	"github.com/Ktulue/KtulueKit-W11/internal/state"
)

// CheckResult is the outcome of a single check command.
type CheckResult int

const (
	CheckInstalled CheckResult = iota // check exited 0
	CheckAbsent                       // check exited non-zero
	CheckTimedOut                     // check exceeded 15-second timeout
)

// Options configures an Export call.
type Options struct {
	Fast         bool
	SourceConfig string           // absolute path to reference config, written into snapshot metadata
	ToolVersion  string           // from ldflags; "dev" if empty
	Machine      string           // os.Hostname()
	State        *state.State     // non-nil only in fast mode
	CheckFn      func(cmd string) CheckResult // nil only in fast mode; wraps detector.RunCheckDetailed in production
}

// SnapshotMeta is the snapshot metadata block written into the output JSON.
type SnapshotMeta struct {
	GeneratedAt  string `json:"generated_at"`
	Machine      string `json:"machine"`
	SourceConfig string `json:"source_config"`
	ToolVersion  string `json:"tool_version"`
	Mode         string `json:"mode"` // "check" | "fast"
}

// Result is returned by Export and carries both the filtered config slices
// and the summary counts needed for terminal output.
type Result struct {
	Packages   []config.Package
	Commands   []config.Command
	Extensions []config.Extension
	Profiles   []config.Profile
	Snapshot   SnapshotMeta
	Checked    int // total items probed (0 in fast mode)
	Included   int // total items in output
}

// Export scans cfg against the machine and returns a Result containing only
// installed items. In check mode, opts.CheckFn is called for each package and
// command. In fast mode, opts.State.Succeeded is used directly.
// Export has no file I/O — the caller is responsible for writing the output.
func Export(cfg *config.Config, opts Options) (Result, error) {
	if opts.ToolVersion == "" {
		opts.ToolVersion = "dev"
	}

	mode := "check"
	if opts.Fast {
		mode = "fast"
	}

	meta := SnapshotMeta{
		GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
		Machine:      opts.Machine,
		SourceConfig: opts.SourceConfig,
		ToolVersion:  opts.ToolVersion,
		Mode:         mode,
	}

	if opts.Fast {
		return exportFast(cfg, opts, meta), nil
	}
	return exportCheck(cfg, opts, meta), nil
}

// exportCheck runs check commands for each package and command.
func exportCheck(cfg *config.Config, opts Options, meta SnapshotMeta) Result {
	var pkgs []config.Package
	var cmds []config.Command
	var warnings []string

	for _, pkg := range cfg.Packages {
		if pkg.Check == "" || pkg.Check == "echo skip" {
			continue
		}
		switch opts.CheckFn(pkg.Check) {
		case CheckInstalled:
			pkgs = append(pkgs, pkg)
		case CheckTimedOut:
			warnings = append(warnings, fmt.Sprintf("[warn] %s: check timed out, treated as absent", pkg.Name))
		}
	}

	for _, cmd := range cfg.Commands {
		if cmd.Check == "" || cmd.Check == "echo skip" {
			continue
		}
		switch opts.CheckFn(cmd.Check) {
		case CheckInstalled:
			cmds = append(cmds, cmd)
		case CheckTimedOut:
			warnings = append(warnings, fmt.Sprintf("[warn] %s: check timed out, treated as absent", cmd.Name))
		}
	}

	for _, w := range warnings {
		fmt.Println(w)
	}

	checked := 0
	for _, pkg := range cfg.Packages {
		if pkg.Check != "" && pkg.Check != "echo skip" {
			checked++
		}
	}
	for _, cmd := range cfg.Commands {
		if cmd.Check != "" && cmd.Check != "echo skip" {
			checked++
		}
	}

	includedIDs := buildIncludedSet(pkgs, cmds, nil)
	return Result{
		Packages:   pkgs,
		Commands:   cmds,
		Extensions: nil,
		Profiles:   filterProfiles(cfg.Profiles, includedIDs),
		Snapshot:   meta,
		Checked:    checked,
		Included:   len(pkgs) + len(cmds),
	}
}

// exportFast reads state.Succeeded to determine installed items.
func exportFast(cfg *config.Config, opts Options, meta SnapshotMeta) Result {
	s := opts.State
	var pkgs []config.Package
	var cmds []config.Command
	var exts []config.Extension

	for _, pkg := range cfg.Packages {
		if s.Succeeded[pkg.ID] {
			pkgs = append(pkgs, pkg)
		}
	}
	for _, cmd := range cfg.Commands {
		if s.Succeeded[cmd.ID] {
			cmds = append(cmds, cmd)
		}
	}
	for _, ext := range cfg.Extensions {
		if s.Succeeded[ext.ID] {
			exts = append(exts, ext)
		}
	}

	includedIDs := buildIncludedSet(pkgs, cmds, exts)
	return Result{
		Packages:   pkgs,
		Commands:   cmds,
		Extensions: exts,
		Profiles:   filterProfiles(cfg.Profiles, includedIDs),
		Snapshot:   meta,
		Checked:    0,
		Included:   len(pkgs) + len(cmds) + len(exts),
	}
}

// buildIncludedSet returns a set of all IDs present in the provided slices.
func buildIncludedSet(pkgs []config.Package, cmds []config.Command, exts []config.Extension) map[string]bool {
	ids := make(map[string]bool, len(pkgs)+len(cmds)+len(exts))
	for _, p := range pkgs {
		ids[p.ID] = true
	}
	for _, c := range cmds {
		ids[c.ID] = true
	}
	for _, e := range exts {
		ids[e.ID] = true
	}
	return ids
}

// filterProfiles returns profiles filtered to only included IDs.
// Profiles where all IDs are filtered out are omitted entirely.
func filterProfiles(profiles []config.Profile, includedIDs map[string]bool) []config.Profile {
	var out []config.Profile
	for _, p := range profiles {
		var filtered []string
		for _, id := range p.IDs {
			if includedIDs[id] {
				filtered = append(filtered, id)
			}
		}
		if len(filtered) > 0 {
			out = append(out, config.Profile{Name: p.Name, IDs: filtered})
		}
	}
	return out
}
