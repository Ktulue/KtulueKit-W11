package runner

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
	"github.com/Ktulue/KtulueKit-W11/internal/desktop"
	"github.com/Ktulue/KtulueKit-W11/internal/installer"
	"github.com/Ktulue/KtulueKit-W11/internal/reporter"
	"github.com/Ktulue/KtulueKit-W11/internal/restore"
	"github.com/Ktulue/KtulueKit-W11/internal/state"
)

// Runner orchestrates the full install sequence.
type Runner struct {
	cfg          *config.Config
	rep          *reporter.Reporter
	state        *state.State
	dryRun       bool
	resumePhase  int
	configPath   string               // preserved so resume commands can reference the right config file
	shortcutMode desktop.ShortcutMode // how to handle .lnk files dropped by installers
	plannedIDs   map[string]bool      // all IDs declared in config (packages + commands)
}

func New(cfg *config.Config, rep *reporter.Reporter, s *state.State, dryRun bool, resumePhase int, configPath string, shortcutMode desktop.ShortcutMode) *Runner {
	planned := make(map[string]bool, len(cfg.Packages)+len(cfg.Commands))
	for _, p := range cfg.Packages {
		planned[p.ID] = true
	}
	for _, c := range cfg.Commands {
		planned[c.ID] = true
	}
	return &Runner{
		cfg:          cfg,
		rep:          rep,
		state:        s,
		dryRun:       dryRun,
		resumePhase:  resumePhase,
		configPath:   configPath,
		shortcutMode: shortcutMode,
		plannedIDs:   planned,
	}
}

// Run executes all phases in order.
func (r *Runner) Run() {
	// Create a System Restore point before touching anything.
	// Skipped on resume runs (user already has the pre-run snapshot).
	if r.resumePhase <= 1 {
		restore.CreateRestorePoint(r.dryRun)
	}

	phases := r.collectPhases()

	pathRefreshed := false

	for _, phase := range phases {
		if phase < r.resumePhase {
			fmt.Printf("\n── Phase %d: skipping (resuming from phase %d) ──\n", phase, r.resumePhase)
			continue
		}

		fmt.Printf("\n── Phase %d ──────────────────────────────────────\n", phase)

		// Refresh PATH once before the first command/extension phase
		// (after all winget packages have had a chance to install)
		if !pathRefreshed && phase >= r.firstCommandPhase() {
			installer.RefreshPath()
			pathRefreshed = true
		}

		r.runPackagesInPhase(phase)
		r.runCommandsInPhase(phase)
		r.runExtensionsInPhase(phase)
	}
}

// runPackagesInPhase runs all Tier 1 winget packages in this phase.
func (r *Runner) runPackagesInPhase(phase int) {
	for _, pkg := range r.cfg.Packages {
		if pkg.Phase != phase {
			continue
		}

		// Snapshot desktops before install so we can detect new .lnk files.
		var desktopBefore map[string]bool
		if !r.dryRun && r.shortcutMode != desktop.ShortcutKeep {
			desktopBefore = desktop.Snapshot()
		}

		fmt.Printf("\n  Installing: %s\n", pkg.Name)
		res := installer.InstallPackage(pkg, r.dryRun, r.cfg.Settings.RetryCount, r.cfg.Settings.UpgradeIfInstalled)
		r.rep.Add(res)

		if res.Status == reporter.StatusInstalled || res.Status == reporter.StatusUpgraded || res.Status == reporter.StatusAlready {
			r.state.MarkSucceeded(pkg.ID)
		} else if res.Status == reporter.StatusFailed {
			r.state.MarkFailed(pkg.ID)
		}

		// Clean up any shortcuts the installer dropped on the desktop.
		if desktopBefore != nil {
			r.cleanupShortcuts(pkg.Name, desktopBefore)
		}

		if pkg.RebootAfter && !r.dryRun && (res.Status == reporter.StatusInstalled || res.Status == reporter.StatusReboot) {
			r.promptReboot(pkg.Name, phase)
		}
	}
}

// cleanupShortcuts finds .lnk files added since before and handles them per shortcutMode.
func (r *Runner) cleanupShortcuts(pkgName string, before map[string]bool) {
	newLinks := desktop.NewShortcuts(before)
	for _, path := range newLinks {
		name := filepath.Base(path)

		var remove bool
		switch r.shortcutMode {
		case desktop.ShortcutRemove:
			remove = true
		case desktop.ShortcutAsk:
			fmt.Printf("\n  New shortcut from %s: %s\n", pkgName, name)
			remove = desktop.PromptRemove(path)
		}

		if !remove {
			continue
		}

		if err := desktop.Remove(path); err != nil {
			fmt.Printf("  [warning] Could not remove shortcut %q: %v\n", name, err)
			continue
		}

		fmt.Printf("  🗑️  Removed shortcut: %s\n", name)
		r.rep.Add(reporter.Result{
			ID:     "shortcut:" + path,
			Name:   name,
			Tier:   "shortcut",
			Status: reporter.StatusShortcutRemoved,
			Detail: fmt.Sprintf("from %s, created by %s", filepath.Dir(path), pkgName),
		})
	}
}

// runCommandsInPhase runs all Tier 2 shell commands in this phase.
func (r *Runner) runCommandsInPhase(phase int) {
	for _, cmd := range r.cfg.Commands {
		if cmd.Phase != phase {
			continue
		}

		fmt.Printf("\n  Running: %s\n", cmd.Name)

		if !r.dependenciesMet(cmd.DependsOn) {
			res := reporter.Result{
				ID:     cmd.ID,
				Name:   cmd.Name,
				Tier:   "command",
				Status: reporter.StatusSkipped,
				Detail: fmt.Sprintf("dependency not met: %s", strings.Join(cmd.DependsOn, ", ")),
			}
			r.rep.Add(res)
			r.state.MarkFailed(cmd.ID)
			continue
		}

		res := installer.RunCommand(cmd, r.dryRun, r.cfg.Settings.RetryCount, r.state)
		r.rep.Add(res)

		if res.Status == reporter.StatusInstalled || res.Status == reporter.StatusAlready || res.Status == reporter.StatusReboot {
			r.state.MarkSucceeded(cmd.ID)
		} else if res.Status == reporter.StatusFailed || res.Status == reporter.StatusSkipped {
			r.state.MarkFailed(cmd.ID)
			if cmd.OnFailurePrompt != "" && !r.dryRun {
				r.promptManualInstall(cmd.Name, cmd.OnFailurePrompt)
			}
		}

		if cmd.RebootAfter && !r.dryRun && res.Status != reporter.StatusFailed {
			r.promptReboot(cmd.Name, phase)
		}
	}
}

// runExtensionsInPhase runs all Tier 3 browser extensions in this phase.
func (r *Runner) runExtensionsInPhase(phase int) {
	for _, ext := range r.cfg.Extensions {
		if ext.Phase != phase {
			continue
		}

		fmt.Printf("\n  Extension: %s\n", ext.Name)
		res := installer.InstallExtension(ext, r.dryRun)
		r.rep.Add(res)

		if res.Status == reporter.StatusInstalled || res.Status == reporter.StatusAlready {
			r.state.MarkSucceeded(ext.ID)
		}
	}
}

// dependenciesMet returns true if all listed IDs are satisfied.
// In dry-run mode, a dep is satisfied if it's in the plan (it would have been installed).
// In a real run, it must have actually succeeded.
func (r *Runner) dependenciesMet(deps []string) bool {
	for _, dep := range deps {
		if r.dryRun {
			if !r.plannedIDs[dep] && !r.state.Succeeded[dep] {
				return false
			}
		} else {
			if !r.state.Succeeded[dep] {
				return false
			}
		}
	}
	return true
}

// promptManualInstall prints fallback guidance when an install command fails,
// then pauses so the user can read it before the run continues.
func (r *Runner) promptManualInstall(itemName, guidance string) {
	fmt.Printf("\n  ⚠️  %s failed to install automatically.\n", itemName)
	fmt.Println("  ──────────────────────────────────────────────────")
	fmt.Println("  Manual install instructions:")
	for _, line := range strings.Split(guidance, "\n") {
		fmt.Printf("    %s\n", line)
	}
	fmt.Println("  ──────────────────────────────────────────────────")
	fmt.Print("  Press Enter when done (or to skip and continue)... ")
	reader := bufio.NewReader(os.Stdin)
	reader.ReadString('\n')
}

// promptReboot saves state, logs the resume command to the terminal and log file,
// then triggers a 30-second Windows reboot countdown via shutdown /r /t 30.
// The user can press Enter within that window to cancel the reboot and continue.
func (r *Runner) promptReboot(itemName string, currentPhase int) {
	nextPhase := currentPhase + 1
	resumeCmd := fmt.Sprintf("ktuluekit --config %q --resume-phase=%d", r.configPath, nextPhase)

	// Persist before doing anything else so state survives the reboot.
	if err := r.state.SaveResumePhase(nextPhase); err != nil {
		fmt.Printf("  [warning] Could not save resume phase to state: %v\n", err)
	}

	sep := strings.Repeat("─", 56)
	banner := fmt.Sprintf(`
  🔄  %s requires a reboot.
  %s
  RESUME COMMAND — run this after restarting:
    %s
  Log file: %s
  %s
  Rebooting in 30 seconds. Press Enter to CANCEL and continue without rebooting.
  (To cancel from another terminal: shutdown /a)
`, itemName, sep, resumeCmd, r.rep.LogPath(), sep)

	fmt.Print(banner)

	// Write the resume command to the log file so it's recoverable after reboot.
	r.rep.LogLine(fmt.Sprintf("\n[REBOOT REQUIRED — %s]", itemName))
	r.rep.LogLine("  Resume command: " + resumeCmd)
	r.rep.LogLine("")

	// Kick off the OS-level reboot countdown.
	shutdownMsg := fmt.Sprintf("KtulueKit: %s requires restart. After reboot run: %s", itemName, resumeCmd)
	// /r = restart, /t 30 = 30-second countdown, /c = comment shown in shutdown dialog
	_ = exec.Command("shutdown", "/r", "/t", "30", "/c", shutdownMsg).Run()

	// Block on stdin — if the user presses Enter we cancel the countdown.
	reader := bufio.NewReader(os.Stdin)
	reader.ReadString('\n')

	// Cancel the scheduled reboot and continue the run.
	_ = exec.Command("shutdown", "/a").Run()
	fmt.Println("  Reboot cancelled. Continuing installation...")
}

// collectPhases returns a sorted list of unique phase numbers across all items.
func (r *Runner) collectPhases() []int {
	seen := make(map[int]bool)
	for _, p := range r.cfg.Packages {
		seen[p.Phase] = true
	}
	for _, c := range r.cfg.Commands {
		seen[c.Phase] = true
	}
	for _, e := range r.cfg.Extensions {
		seen[e.Phase] = true
	}

	phases := make([]int, 0, len(seen))
	for phase := range seen {
		phases = append(phases, phase)
	}
	sort.Ints(phases)
	return phases
}

// firstCommandPhase returns the lowest phase number that contains a Command or Extension.
// PATH refresh runs just before this phase.
func (r *Runner) firstCommandPhase() int {
	min := int(^uint(0) >> 1) // max int
	for _, c := range r.cfg.Commands {
		if c.Phase < min {
			min = c.Phase
		}
	}
	for _, e := range r.cfg.Extensions {
		if e.Phase < min {
			min = e.Phase
		}
	}
	return min
}
