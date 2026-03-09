package runner

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
	"github.com/Ktulue/KtulueKit-W11/internal/installer"
	"github.com/Ktulue/KtulueKit-W11/internal/reporter"
	"github.com/Ktulue/KtulueKit-W11/internal/state"
)

// Runner orchestrates the full install sequence.
type Runner struct {
	cfg         *config.Config
	rep         *reporter.Reporter
	state       *state.State
	dryRun      bool
	resumePhase int
}

func New(cfg *config.Config, rep *reporter.Reporter, s *state.State, dryRun bool, resumePhase int) *Runner {
	return &Runner{cfg: cfg, rep: rep, state: s, dryRun: dryRun, resumePhase: resumePhase}
}

// Run executes all phases in order.
func (r *Runner) Run() {
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

		fmt.Printf("\n  Installing: %s\n", pkg.Name)
		res := installer.InstallPackage(pkg, r.dryRun, r.cfg.Settings.RetryCount)
		r.rep.Add(res)

		if res.Status == reporter.StatusInstalled || res.Status == reporter.StatusAlready {
			r.state.MarkSucceeded(pkg.ID)
		} else if res.Status == reporter.StatusFailed {
			r.state.MarkFailed(pkg.ID)
		}

		if pkg.RebootAfter && !r.dryRun && (res.Status == reporter.StatusInstalled || res.Status == reporter.StatusReboot) {
			r.promptReboot(pkg.Name, phase)
		}
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

// dependenciesMet returns true if all listed IDs are in the succeeded state.
func (r *Runner) dependenciesMet(deps []string) bool {
	for _, dep := range deps {
		if !r.state.Succeeded[dep] {
			return false
		}
	}
	return true
}

// promptReboot pauses and asks the user whether to reboot now or continue.
func (r *Runner) promptReboot(itemName string, currentPhase int) {
	fmt.Printf("\n  🔄  %s requires a reboot.\n", itemName)
	fmt.Printf("  Reboot now and re-run with --resume-phase=%d to continue, or press Enter to keep going.\n", currentPhase+1)
	fmt.Print("  [Enter = continue | Ctrl+C = exit and reboot manually]: ")

	reader := bufio.NewReader(os.Stdin)
	reader.ReadString('\n')
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
