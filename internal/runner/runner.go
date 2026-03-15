package runner

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
	"github.com/Ktulue/KtulueKit-W11/internal/detector"
	"github.com/Ktulue/KtulueKit-W11/internal/desktop"
	"github.com/Ktulue/KtulueKit-W11/internal/installer"
	"github.com/Ktulue/KtulueKit-W11/internal/reporter"
	"github.com/Ktulue/KtulueKit-W11/internal/restore"
	"github.com/Ktulue/KtulueKit-W11/internal/scheduler"
	"github.com/Ktulue/KtulueKit-W11/internal/state"
)

// ANSI color codes for terminal output.
const (
	colorGreen  = "\033[32m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorReset  = "\033[0m"
)

// consecutiveFailThreshold is the number of back-to-back failures that triggers a pause.
const consecutiveFailThreshold = 3

// ProgressEvent is emitted by the runner to report GUI progress.
// When OnProgress is nil (CLI mode), fmt.Printf is used instead.
type ProgressEvent struct {
	Index   int    // 1-based position in the run
	Total   int    // total items in this run
	ID      string // item ID
	Name    string // item display name
	Status  string // "installing"|"installed"|"upgraded"|"already"|"failed"|"skipped"|"reboot"|"reboot_cancelled"|"shortcut_removed"
	Detail  string // raw output line, error message, or OnFailurePrompt text
	Elapsed string // "1m23s" — empty for "installing" events
}

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
	totalItems     int                  // total items in phases >= resumePhase
	itemIdx        int                  // current item index (1-based, increments each item)
	selectedIDs    map[string]bool      // nil = run all (CLI mode); set by SetSelectedIDs
	onlyPhase      int                  // 0 = run all phases; > 0 = run only this phase
	upgradeOnly    bool                 // if true, skip items not yet installed; force upgrade on installed ones
	onProgress     func(ProgressEvent)  // nil = print to stdout (CLI mode); set by SetOnProgress
	rebootResponse chan bool             // nil = no reboot pending; set by SetRebootResponse
	consecutiveFails int            // counts back-to-back StatusFailed or StatusSkipped results
	pauseResponse    chan bool       // GUI mode: GUI sends true when user clicks "continue"
	onPause          func()         // test hook: when non-nil, replaces both CLI and GUI pause behavior; production code leaves this nil
	interrupted bool            // set true when a SIGINT is received during a run
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

// SetSelectedIDs limits the run to the given item IDs. Items not in the set
// are silently skipped and not counted in totalItems. nil = run all (CLI mode).
func (r *Runner) SetSelectedIDs(ids map[string]bool) {
	r.selectedIDs = ids
}

// SetOnlyPhase restricts the run to a single phase. 0 = run all (default).
func (r *Runner) SetOnlyPhase(n int) {
	r.onlyPhase = n
}

// SetUpgradeOnly restricts the run to already-installed items and forces upgrade.
func (r *Runner) SetUpgradeOnly(v bool) {
	r.upgradeOnly = v
}

// SetOnProgress wires a callback for live GUI progress events.
// When nil (CLI mode), the runner prints to stdout via fmt.Printf as usual.
func (r *Runner) SetOnProgress(fn func(ProgressEvent)) {
	r.onProgress = fn
}

// SetRebootResponse provides the channel the runner blocks on when a reboot
// is required in GUI mode. ConfirmReboot/CancelReboot send on this channel.
func (r *Runner) SetRebootResponse(ch chan bool) {
	r.rebootResponse = ch
}

// SetPauseResponse provides the channel the runner blocks on when a consecutive-failure
// pause fires in GUI mode. The app sends true each time the user confirms "continue".
// Unlike rebootResponse, this is NOT set to nil after use (pauses can occur multiple times).
func (r *Runner) SetPauseResponse(ch chan bool) {
	r.pauseResponse = ch
}

// SetOnPause wires a test hook that replaces the stdin block in promptConsecutiveFailures.
// Production code leaves this nil. Tests set it to verify the counter fires correctly.
func (r *Runner) SetOnPause(fn func()) {
	r.onPause = fn
}

// markInterrupted prints a one-time interrupt message and sets r.interrupted.
// The caller is responsible for returning from the current loop body.
func (r *Runner) markInterrupted(phase int) {
	if !r.interrupted {
		fmt.Printf("\n  Interrupted — finishing current item then stopping. Run with --resume-phase=%d to continue.\n", phase)
		r.interrupted = true
	}
}

// WasInterrupted reports whether the run was stopped by a Ctrl+C signal.
func (r *Runner) WasInterrupted() bool {
	return r.interrupted
}

// countItemsFromPhase returns the total number of items across all tiers
// in phases >= fromPhase. Used to drive the [N/Total] progress counter.
func (r *Runner) countItemsFromPhase(fromPhase int) int {
	count := 0
	for _, p := range r.cfg.Packages {
		if p.Phase >= fromPhase && (r.selectedIDs == nil || r.selectedIDs[p.ID]) {
			count++
		}
	}
	for _, c := range r.cfg.Commands {
		if c.Phase >= fromPhase && (r.selectedIDs == nil || r.selectedIDs[c.ID]) {
			count++
		}
	}
	for _, e := range r.cfg.Extensions {
		if e.Phase >= fromPhase && (r.selectedIDs == nil || r.selectedIDs[e.ID]) {
			count++
		}
	}
	return count
}

// countItemsInPhase returns the total number of items in exactly phase n.
// Used to drive the [N/Total] progress counter when --phase is set.
func (r *Runner) countItemsInPhase(n int) int {
	count := 0
	for _, p := range r.cfg.Packages {
		if p.Phase == n && (r.selectedIDs == nil || r.selectedIDs[p.ID]) {
			count++
		}
	}
	for _, c := range r.cfg.Commands {
		if c.Phase == n && (r.selectedIDs == nil || r.selectedIDs[c.ID]) {
			count++
		}
	}
	for _, e := range r.cfg.Extensions {
		if e.Phase == n && (r.selectedIDs == nil || r.selectedIDs[e.ID]) {
			count++
		}
	}
	return count
}

// Run executes all phases in order.
func (r *Runner) Run(ctx context.Context) {
	if r.onlyPhase > 0 {
		r.totalItems = r.countItemsInPhase(r.onlyPhase)
	} else {
		r.totalItems = r.countItemsFromPhase(r.resumePhase)
	}

	// Fail fast if winget is missing or broken.
	if !r.dryRun {
		if ok, reason := installer.CheckConnectivity(); !ok {
			fmt.Printf("%s[WARNING]%s %s\n\n", colorYellow, colorReset, reason)
		}
		if err := installer.CheckWingetAvailable(); err != nil {
			fmt.Printf("ERROR: winget is not available: %v\n", err)
			fmt.Println("Install App Installer from the Microsoft Store, then re-run.")
			return
		}
	}

	// Create a System Restore point before touching anything.
	// Skipped on resume runs (user already has the pre-run snapshot).
	if r.resumePhase <= 1 {
		restore.CreateRestorePoint(r.dryRun)
	}

	if r.printPreRunSummary() {
		return
	}

	if !r.dryRun {
		fmt.Println("Updating winget sources...")
		if err := installer.UpdateSources(); err != nil {
			fmt.Printf("  [warning] winget source update failed: %v\n", err)
		}
		fmt.Println()
	}

	phases := r.collectPhases()
	phaseIdx := make(map[int]int, len(phases))
	for i, p := range phases {
		phaseIdx[p] = i + 1
	}

	pathRefreshed := false

	for _, phase := range phases {
		// --phase: skip non-matching phases silently (intentional — no log line,
		// unlike the resumePhase skip below which does emit a log).
		if r.onlyPhase > 0 && phase != r.onlyPhase {
			continue
		}
		if phase < r.resumePhase {
			fmt.Printf("\n── Phase %d: skipping (resuming from phase %d) ──\n", phase, r.resumePhase)
			continue
		}

		fmt.Println(phaseHeader(phase, phaseIdx[phase], len(phases)))

		// Refresh PATH once before the first command/extension phase
		// (after all winget packages have had a chance to install)
		if !pathRefreshed && phase >= r.firstCommandPhase() {
			installer.RefreshPath()
			pathRefreshed = true
			if !r.dryRun {
				missing := installer.VerifyRuntimePaths()
				missingSet := make(map[string]bool, len(missing))
				for _, m := range missing {
					missingSet[m] = true
				}
				var present []string
				for _, tool := range installer.RuntimeTools() {
					if !missingSet[tool] {
						present = append(present, tool)
					}
				}
				if len(missing) == 0 {
					fmt.Printf("  %s[OK]%s  All runtime tools found on PATH.\n", colorGreen, colorReset)
				} else {
					fmt.Println("  PATH check after refresh:")
					if len(present) > 0 {
						fmt.Printf("    %s[OK]%s    %s\n", colorGreen, colorReset, strings.Join(present, ", "))
					}
					for _, m := range missing {
						fmt.Printf("    %s[WARN]%s  %s — not found on PATH (install may not have completed)\n", colorYellow, colorReset, m)
					}
				}
			}
		}

		r.runPackagesInPhase(ctx, phase)
		r.runCommandsInPhase(ctx, phase)
		r.runExtensionsInPhase(ctx, phase)
	}

	// Play a completion beep (skipped in dry-run).
	if !r.dryRun {
		_ = exec.Command("powershell", "-NoProfile", "-Command", "[console]::beep(800,300)").Run()
	}
}

// printPreRunSummary scans all config items and prints counts before the install loop starts.
// Returns true if nothing needs installing (caller should skip the phase loop).
// Dry-run mode always returns false — it proceeds to show what would be done.
func (r *Runner) printPreRunSummary() (nothingToDo bool) {
	if r.dryRun {
		return false
	}
	// Skip the pre-run summary on resume runs — phases before resumePhase are
	// already done, and scanning all phases would give misleading counts.
	if r.resumePhase > 1 {
		return false
	}
	// Skip in GUI mode — the selection screen already shows item counts, and
	// detector results would not reflect the user's selection filter.
	if r.onProgress != nil {
		return false
	}
	// Skip when an ID filter is active — the scan would cover all items but
	// the phase loop will only run the filtered subset, giving misleading counts.
	if r.selectedIDs != nil {
		return false
	}
	// Skip the pre-run summary when --phase is set — the scan would cover all
	// phases but only one will run, giving misleading counts.
	if r.onlyPhase > 0 {
		return false
	}

	fmt.Println("Scanning machine...")
	items := detector.FlattenItems(r.cfg)
	results := detector.CheckAll(items, r.state)

	var installed, missing, unknown int
	for _, res := range results {
		switch res.Status {
		case detector.StatusInstalled:
			installed++
		case detector.StatusMissing:
			missing++
		case detector.StatusUnknown:
			unknown++
		}
	}

	fmt.Println()
	fmt.Printf("  %s[OK]%s      Already installed: %d\n", colorGreen, colorReset, installed)
	fmt.Printf("  %s[MISSING]%s To install:        %d\n", colorRed, colorReset, missing)
	fmt.Printf("  %s[?]%s       Unknown:           %d\n", colorYellow, colorReset, unknown)
	fmt.Println()

	// Unknown items (extensions without check commands) are not counted as "needs install"
	// — they will be handled during the install loop. Only Missing items block early exit.
	if missing == 0 {
		fmt.Println("Nothing to install. All known items are already present.")
		return true
	}

	fmt.Println("Starting installation...")
	fmt.Println("─────────────────────────────────────────────────")
	return false
}

// runPostInstall executes a post-install hook after a successful install.
// Hook failures are warnings only and do not affect the result status.
func (r *Runner) runPostInstall(itemName, hook string, timeoutSeconds int) {
	if hook == "" {
		return
	}
	if r.dryRun {
		fmt.Printf("    [DRY RUN] Would run post_install: %s\n", hook)
		return
	}
	fmt.Printf("    running post-install hook for %s...\n", itemName)
	if err := installer.RunHook(hook, timeoutSeconds); err != nil {
		fmt.Printf("    %s[WARN]%s  post-install hook failed for %s: %v\n",
			colorYellow, colorReset, itemName, err)
	}
}

// runPackagesInPhase runs all Tier 1 winget packages in this phase.
func (r *Runner) runPackagesInPhase(ctx context.Context, phase int) {
	for _, pkg := range r.cfg.Packages {
		if ctx.Err() != nil {
			r.markInterrupted(phase)
			return
		}
		if pkg.Phase != phase {
			continue
		}
		if r.selectedIDs != nil && !r.selectedIDs[pkg.ID] {
			continue
		}

		// upgrade-only: skip items not yet installed.
		if r.upgradeOnly {
			item := detector.Item{ID: pkg.ID, Name: pkg.Name, Phase: pkg.Phase, Tier: "winget", CheckCmd: pkg.Check}
			result := detector.CheckItem(item, r.state)
			switch result.Status {
			case detector.StatusMissing:
				continue // skip silently
			case detector.StatusUnknown:
				fmt.Printf("  %sWARN%s  --upgrade-only: no check command for %q, skipping\n", colorYellow, colorReset, pkg.Name)
				continue
			}
			// StatusInstalled: fall through — bypass state-aware skip so we actually upgrade
		}

		// State-aware skip: if a previous run already succeeded, don't re-check or re-install.
		if r.state.Succeeded[pkg.ID] && !r.upgradeOnly {
			r.itemIdx++
			fmt.Printf("\n  [%d/%d] Skipping (already succeeded): %s\n", r.itemIdx, r.totalItems, pkg.Name)
			r.rep.Add(reporter.Result{
				ID:     pkg.ID,
				Name:   pkg.Name,
				Tier:   "winget",
				Status: reporter.StatusAlready,
				Detail: "already succeeded in a previous run",
			})
			if r.onProgress != nil {
				r.onProgress(ProgressEvent{Index: r.itemIdx, Total: r.totalItems, ID: pkg.ID, Name: pkg.Name, Status: "already", Detail: "already succeeded in a previous run"})
			}
			continue
		}

		// Snapshot desktops before install so we can detect new .lnk files.
		var desktopBefore map[string]bool
		if !r.dryRun && r.shortcutMode != desktop.ShortcutKeep {
			desktopBefore = desktop.Snapshot()
		}

		r.itemIdx++
		fmt.Printf("\n  [%d/%d] Installing: %s\n", r.itemIdx, r.totalItems, pkg.Name)
		if r.onProgress != nil {
			r.onProgress(ProgressEvent{Index: r.itemIdx, Total: r.totalItems, ID: pkg.ID, Name: pkg.Name, Status: "installing"})
		}
		start := time.Now()
		res := installer.InstallPackage(pkg, r.dryRun, r.cfg.Settings.RetryCount, r.cfg.Settings.UpgradeIfInstalled)
		r.rep.Add(res)
		elapsed := time.Since(start).Round(time.Second)
		fmt.Printf("      elapsed: %s\n", elapsed)
		if r.onProgress != nil {
			r.onProgress(ProgressEvent{Index: r.itemIdx, Total: r.totalItems, ID: pkg.ID, Name: pkg.Name, Status: reporterStatusToGUI(res.Status), Detail: res.Detail, Elapsed: elapsed.String()})
		}

		if res.Status == reporter.StatusInstalled || res.Status == reporter.StatusUpgraded || res.Status == reporter.StatusAlready {
			r.state.MarkSucceeded(pkg.ID)
		} else if res.Status == reporter.StatusFailed {
			r.state.MarkFailed(pkg.ID)
		}

		if res.Status == reporter.StatusInstalled || res.Status == reporter.StatusUpgraded {
			r.runPostInstall(pkg.Name, pkg.PostInstall, pkg.TimeoutSeconds)
		}

		// Clean up any shortcuts the installer dropped on the desktop.
		if desktopBefore != nil {
			r.cleanupShortcuts(pkg.Name, desktopBefore)
		}

		r.trackResult(res.Status)

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

		if err := desktop.Backup(path); err != nil {
			fmt.Printf("  [warning] Could not back up shortcut %q: %v\n", name, err)
			continue
		}

		fmt.Printf("  📁  Backed up shortcut: %s → KtulueKit Shortcuts/\n", name)
		r.rep.Add(reporter.Result{
			ID:     "shortcut:" + path,
			Name:   name,
			Tier:   "shortcut",
			Status: reporter.StatusShortcutRemoved,
			Detail: fmt.Sprintf("moved to KtulueKit Shortcuts/ on desktop (created by %s)", pkgName),
		})
	}
}

// runCommandsInPhase runs all Tier 2 shell commands in this phase.
func (r *Runner) runCommandsInPhase(ctx context.Context, phase int) {
	for _, cmd := range r.cfg.Commands {
		if ctx.Err() != nil {
			r.markInterrupted(phase)
			return
		}
		if cmd.Phase != phase {
			continue
		}
		if r.selectedIDs != nil && !r.selectedIDs[cmd.ID] {
			continue
		}

		// upgrade-only: skip commands not yet installed. Commands with no check are skipped silently
		// (shell commands rarely have reliable check mechanisms).
		if r.upgradeOnly {
			item := detector.Item{ID: cmd.ID, Name: cmd.Name, Phase: cmd.Phase, Tier: "command", CheckCmd: cmd.Check}
			result := detector.CheckItem(item, r.state)
			if result.Status != detector.StatusInstalled {
				continue // skip silently (missing or unknown)
			}
		}

		// State-aware skip: if a previous run already succeeded, don't re-check or re-run.
		if r.state.Succeeded[cmd.ID] && !r.upgradeOnly {
			r.itemIdx++
			fmt.Printf("\n  [%d/%d] Skipping (already succeeded): %s\n", r.itemIdx, r.totalItems, cmd.Name)
			r.rep.Add(reporter.Result{
				ID:     cmd.ID,
				Name:   cmd.Name,
				Tier:   "command",
				Status: reporter.StatusAlready,
				Detail: "already succeeded in a previous run",
			})
			if r.onProgress != nil {
				r.onProgress(ProgressEvent{Index: r.itemIdx, Total: r.totalItems, ID: cmd.ID, Name: cmd.Name, Status: "already", Detail: "already succeeded in a previous run"})
			}
			continue
		}

		r.itemIdx++
		fmt.Printf("\n  [%d/%d] Running: %s\n", r.itemIdx, r.totalItems, cmd.Name)

		if !r.dependenciesMet(cmd.DependsOn) {
			detail := fmt.Sprintf("dependency not met: %s", strings.Join(cmd.DependsOn, ", "))
			res := reporter.Result{
				ID:     cmd.ID,
				Name:   cmd.Name,
				Tier:   "command",
				Status: reporter.StatusSkipped,
				Detail: detail,
			}
			r.rep.Add(res)
			if r.onProgress != nil {
				r.onProgress(ProgressEvent{Index: r.itemIdx, Total: r.totalItems, ID: cmd.ID, Name: cmd.Name, Status: "skipped", Detail: detail})
			}
			r.trackResult(reporter.StatusSkipped)
			r.state.MarkFailed(cmd.ID)
			continue
		}

		if r.onProgress != nil {
			r.onProgress(ProgressEvent{Index: r.itemIdx, Total: r.totalItems, ID: cmd.ID, Name: cmd.Name, Status: "installing"})
		}
		start := time.Now()
		res := installer.RunCommand(cmd, r.dryRun, r.cfg.Settings.RetryCount, r.state)
		r.rep.Add(res)
		elapsed := time.Since(start).Round(time.Second)
		fmt.Printf("      elapsed: %s\n", elapsed)
		if r.onProgress != nil {
			r.onProgress(ProgressEvent{Index: r.itemIdx, Total: r.totalItems, ID: cmd.ID, Name: cmd.Name, Status: reporterStatusToGUI(res.Status), Detail: res.Detail, Elapsed: elapsed.String()})
		}

		if res.Status == reporter.StatusInstalled || res.Status == reporter.StatusAlready || res.Status == reporter.StatusReboot {
			r.state.MarkSucceeded(cmd.ID)
		} else if res.Status == reporter.StatusFailed || res.Status == reporter.StatusSkipped {
			r.state.MarkFailed(cmd.ID)
			if cmd.OnFailurePrompt != "" && !r.dryRun {
				r.promptManualInstall(cmd.Name, cmd.OnFailurePrompt)
			}
		}

		if res.Status == reporter.StatusInstalled || res.Status == reporter.StatusUpgraded {
			r.runPostInstall(cmd.Name, cmd.PostInstall, cmd.TimeoutSeconds)
		}

		r.trackResult(res.Status)

		if cmd.RebootAfter && !r.dryRun && res.Status != reporter.StatusFailed {
			r.promptReboot(cmd.Name, phase)
		}
	}
}

// runExtensionsInPhase runs all Tier 3 browser extensions in this phase.
func (r *Runner) runExtensionsInPhase(ctx context.Context, phase int) {
	for _, ext := range r.cfg.Extensions {
		if ctx.Err() != nil {
			r.markInterrupted(phase)
			return
		}
		if ext.Phase != phase {
			continue
		}
		if r.selectedIDs != nil && !r.selectedIDs[ext.ID] {
			continue
		}

		// upgrade-only: extensions have no check command and always return StatusUnknown.
		// Skip them silently — extensions cannot be programmatically upgraded.
		if r.upgradeOnly {
			continue
		}

		// State-aware skip: if a previous run already succeeded, don't re-install.
		if r.state.Succeeded[ext.ID] && !r.upgradeOnly {
			r.itemIdx++
			fmt.Printf("\n  [%d/%d] Skipping (already succeeded): %s\n", r.itemIdx, r.totalItems, ext.Name)
			r.rep.Add(reporter.Result{
				ID:     ext.ID,
				Name:   ext.Name,
				Tier:   "extension",
				Status: reporter.StatusAlready,
				Detail: "already succeeded in a previous run",
			})
			if r.onProgress != nil {
				r.onProgress(ProgressEvent{Index: r.itemIdx, Total: r.totalItems, ID: ext.ID, Name: ext.Name, Status: "already", Detail: "already succeeded in a previous run"})
			}
			continue
		}

		r.itemIdx++
		fmt.Printf("\n  [%d/%d] Extension: %s\n", r.itemIdx, r.totalItems, ext.Name)
		if r.onProgress != nil {
			r.onProgress(ProgressEvent{Index: r.itemIdx, Total: r.totalItems, ID: ext.ID, Name: ext.Name, Status: "installing"})
		}
		start := time.Now()
		res := installer.InstallExtension(ext, r.dryRun)
		r.rep.Add(res)
		elapsed := time.Since(start).Round(time.Second)
		fmt.Printf("      elapsed: %s\n", elapsed)
		if r.onProgress != nil {
			r.onProgress(ProgressEvent{Index: r.itemIdx, Total: r.totalItems, ID: ext.ID, Name: ext.Name, Status: reporterStatusToGUI(res.Status), Detail: res.Detail, Elapsed: elapsed.String()})
		}

		if res.Status == reporter.StatusInstalled || res.Status == reporter.StatusAlready {
			r.state.MarkSucceeded(ext.ID)
		}

		r.trackResult(res.Status)
	}
}

// dependenciesMet returns true if all listed IDs are satisfied.
// In dry-run mode, a dep is satisfied if it's in the plan (it would have been installed).
// In a real run, it must have actually succeeded.
func (r *Runner) dependenciesMet(deps []string) bool {
	for _, dep := range deps {
		if r.dryRun {
			depWillRun := r.plannedIDs[dep] && (r.selectedIDs == nil || r.selectedIDs[dep])
			if !depWillRun && !r.state.Succeeded[dep] {
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

// trackResult updates the consecutive-failure counter after a fresh install attempt.
// StatusFailed and StatusSkipped increment the counter; on reaching consecutiveFailThreshold, the run pauses.
// Any non-failure status resets the counter to 0.
// State-aware skips (items already succeeded in a prior run) must NOT call trackResult.
func (r *Runner) trackResult(status string) {
	switch status {
	case reporter.StatusFailed, reporter.StatusSkipped:
		r.consecutiveFails++
		if r.consecutiveFails >= consecutiveFailThreshold {
			r.promptConsecutiveFailures()
			r.consecutiveFails = 0
		}
	default:
		r.consecutiveFails = 0
	}
}

// promptConsecutiveFailures fires when 3 or more installs fail or are dependency-skipped
// back-to-back. It pauses the run and asks the user to investigate before continuing.
func (r *Runner) promptConsecutiveFailures() {
	if r.onPause != nil {
		r.onPause()
		return
	}
	if r.pauseResponse != nil {
		if r.onProgress != nil {
			r.onProgress(ProgressEvent{Index: r.itemIdx, Total: r.totalItems, Status: "paused"})
		}
		<-r.pauseResponse
		return
	}
	// CLI mode: block on stdin.
	fmt.Printf("\n  %s⚠️  3 consecutive failures. Something may be wrong.%s\n", colorYellow, colorReset)
	fmt.Printf("  ──────────────────────────────────────────────────\n")
	fmt.Printf("  Press Enter to continue, or Ctrl+C to abort and investigate.\n")
	bufio.NewReader(os.Stdin).ReadString('\n') //nolint:errcheck
}

// promptManualInstall prints fallback guidance when an install command fails,
// then pauses so the user can read it before the run continues.
func (r *Runner) promptManualInstall(itemName, guidance string) {
	if r.onProgress != nil {
		// GUI mode: emit the guidance text as a failed event detail so the
		// frontend can render it in the Raw Output drawer. No stdin block.
		r.onProgress(ProgressEvent{
			Index:  r.itemIdx,
			Total:  r.totalItems,
			Name:   itemName,
			Status: "failed",
			Detail: guidance,
		})
		return
	}
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

// promptReboot saves state, registers an auto-resume Scheduled Task, logs the
// resume command, then triggers a 30-second Windows reboot countdown.
// The user can press Enter to cancel the reboot and continue installing.
func (r *Runner) promptReboot(itemName string, currentPhase int) {
	nextPhase := currentPhase + 1
	resumeCmd := fmt.Sprintf("ktuluekit --config %q --resume-phase=%d", r.configPath, nextPhase)

	// Persist before doing anything else so state survives the reboot.
	if err := r.state.SaveResumePhase(nextPhase); err != nil {
		fmt.Printf("  [warning] Could not save resume phase to state: %v\n", err)
	}

	// Register the auto-resume Scheduled Task.
	binaryPath, _ := os.Executable()
	absConfig, _ := filepath.Abs(r.configPath)
	cwd, _ := os.Getwd()

	taskRegistered := false
	if err := scheduler.CreateResumeTask(binaryPath, absConfig, cwd, nextPhase, r.dryRun); err != nil {
		fmt.Printf("  [warning] Could not register auto-resume task: %v\n", err)
	} else {
		taskRegistered = true
	}

	// GUI mode: emit a reboot event and block on the response channel.
	// The frontend shows a modal; ConfirmReboot/CancelReboot send on the channel.
	if r.onProgress != nil {
		r.rep.LogLine(fmt.Sprintf("\n[REBOOT REQUIRED — %s]", itemName))
		r.rep.LogLine("  Resume command: " + resumeCmd)
		r.rep.LogLine("")
		r.onProgress(ProgressEvent{
			Index:  r.itemIdx,
			Total:  r.totalItems,
			ID:     "reboot",
			Name:   itemName,
			Status: "reboot",
		})
		if r.rebootResponse != nil {
			confirmed := <-r.rebootResponse
			r.rebootResponse = nil
			if confirmed {
				// Runner calls shutdown; app.go goroutine will emit "complete" after Run() returns.
				exec.Command("shutdown", "/r", "/t", "30").Run()
				return
			}
			// User cancelled reboot — delete task and continue.
			scheduler.DeleteResumeTask()
			r.onProgress(ProgressEvent{
				Index:  r.itemIdx,
				Total:  r.totalItems,
				Name:   itemName,
				Status: "reboot_cancelled",
			})
		}
		return
	}

	// Build and print the reboot banner.
	sep := strings.Repeat("─", 56)
	var taskLine string
	if r.dryRun {
		taskLine = "  [dry-run] Auto-resume task would be registered.\n"
	} else if taskRegistered {
		taskLine = "  ✅ Auto-resume task registered — will run automatically after login.\n" +
			"  To cancel task: schtasks /delete /tn KtulueKit-Resume /f\n"
	} else {
		taskLine = "  ⚠️  Auto-resume task NOT registered. Run manually after reboot:\n" +
			"    " + resumeCmd + "\n"
	}

	banner := fmt.Sprintf(`
  🔄  %s requires a reboot.
  %s
%s  Log file: %s
  %s
  Rebooting in 30 seconds. Press Enter to CANCEL and continue without rebooting.
  (To cancel from another terminal: shutdown /a)
`, itemName, sep, taskLine, r.rep.LogPath(), sep)

	fmt.Print(banner)

	// Always write resume command to log — recoverable regardless of task status.
	r.rep.LogLine(fmt.Sprintf("\n[REBOOT REQUIRED — %s]", itemName))
	r.rep.LogLine("  Resume command: " + resumeCmd)
	r.rep.LogLine("")

	// Kick off the OS-level reboot countdown.
	shutdownMsg := fmt.Sprintf("KtulueKit: %s requires restart. After reboot run: %s", itemName, resumeCmd)
	_ = exec.Command("shutdown", "/r", "/t", "30", "/c", shutdownMsg).Run()

	// Block on stdin — if the user presses Enter we cancel the countdown.
	reader := bufio.NewReader(os.Stdin)
	reader.ReadString('\n')

	// Cancel the scheduled reboot and continue the run.
	_ = exec.Command("shutdown", "/a").Run()

	// Remove the resume task — the user chose to continue without rebooting,
	// so we don't want it firing at the next unrelated logon.
	_ = scheduler.DeleteResumeTask()

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

// phaseHeader returns the formatted phase separator line with position context.
// e.g. "── Phase 2 | [2 of 4] ──────────────────────────────"
func phaseHeader(phase, idx, total int) string {
	return fmt.Sprintf("\n── Phase %d | [%d of %d] ──────────────────────────────", phase, idx, total)
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

// reporterStatusToGUI converts a reporter.Status* constant to the GUI event status string.
// Most constants match already; only two differ.
func reporterStatusToGUI(s string) string {
	switch s {
	case reporter.StatusAlready:
		return "already"
	case reporter.StatusReboot:
		return "reboot"
	default:
		return s
	}
}
