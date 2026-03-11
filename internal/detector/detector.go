package detector

import (
	"context"
	"os/exec"
	"time"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
	"github.com/Ktulue/KtulueKit-W11/internal/state"
)

// checkTimeoutSeconds is the timeout for detection check commands.
// Matches the installer package's check timeout.
const checkTimeoutSeconds = 15

// Status represents the detected install state of a single item.
type Status int

const (
	StatusInstalled Status = iota // check command returned exit 0 (or state says succeeded)
	StatusMissing                 // check command returned non-zero
	StatusUnknown                 // no check command, or check timed out / errored
)

// Item is a unified projection of any installable item (package, command, or extension)
// containing only the fields needed for detection.
// FlattenItems converts the three config types into this form.
type Item struct {
	ID       string
	Name     string
	Phase    int
	Tier     string // "winget" | "command" | "extension"
	CheckCmd string // empty = no check available → StatusUnknown
}

// Result is the detected state of a single Item.
type Result struct {
	Item   Item
	Status Status
}

// FlattenItems converts a Config into a single slice of Items across all tiers.
// Extensions have no check command and will always detect as StatusUnknown.
func FlattenItems(cfg *config.Config) []Item {
	items := make([]Item, 0, len(cfg.Packages)+len(cfg.Commands)+len(cfg.Extensions))

	for _, p := range cfg.Packages {
		items = append(items, Item{
			ID:       p.ID,
			Name:     p.Name,
			Phase:    p.Phase,
			Tier:     "winget",
			CheckCmd: p.Check,
		})
	}

	for _, c := range cfg.Commands {
		items = append(items, Item{
			ID:       c.ID,
			Name:     c.Name,
			Phase:    c.Phase,
			Tier:     "command",
			CheckCmd: c.Check,
		})
	}

	for _, e := range cfg.Extensions {
		items = append(items, Item{
			ID:    e.ID,
			Name:  e.Name,
			Phase: e.Phase,
			Tier:  "extension",
			// No check command for extensions — they show as Unknown
		})
	}

	return items
}

// CheckItem detects the install state of a single item.
//
// Logic:
//  1. If state.Succeeded[item.ID] is true → StatusInstalled (state-aware skip, no shell command run)
//  2. If item has no check command (or "echo skip") → StatusUnknown
//  3. Run check command silently (no output to terminal)
//     - exit 0 → StatusInstalled
//     - non-zero or timeout → StatusMissing
func CheckItem(item Item, s *state.State) Result {
	// State-aware skip: if a previous run already succeeded, trust it.
	if s != nil && s.Succeeded[item.ID] {
		return Result{Item: item, Status: StatusInstalled}
	}

	// No check command available.
	if item.CheckCmd == "" || item.CheckCmd == "echo skip" {
		return Result{Item: item, Status: StatusUnknown}
	}

	// Run the check command silently.
	if runCheckSilent(item.CheckCmd) {
		return Result{Item: item, Status: StatusInstalled}
	}
	return Result{Item: item, Status: StatusMissing}
}

// CheckAll runs CheckItem for every item in the slice and returns results in the same order.
func CheckAll(items []Item, s *state.State) []Result {
	results := make([]Result, len(items))
	for i, item := range items {
		results[i] = CheckItem(item, s)
	}
	return results
}

// runCheckSilent runs a check command and returns true if exit code is 0.
// All output is suppressed — this is purely detection, not installation.
func runCheckSilent(checkCmd string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeoutSeconds*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "cmd", "/C", checkCmd)
	err := cmd.Run()

	if ctx.Err() == context.DeadlineExceeded {
		return false // treat timeout as not-installed
	}
	return err == nil
}
