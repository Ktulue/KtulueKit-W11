package main

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
	"github.com/Ktulue/KtulueKit-W11/internal/detector"
	"github.com/Ktulue/KtulueKit-W11/internal/state"
)

// ANSI color codes for status output.
const (
	colorGreen  = "\033[32m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorReset  = "\033[0m"
)

func runStatus(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	s, err := state.Load()
	if err != nil {
		return fmt.Errorf("state error: %w", err)
	}

	fmt.Printf("KtulueKit Status — %s\n\n", time.Now().Format("2006-01-02 15:04:05"))

	items := detector.FlattenItems(cfg)
	results := detector.CheckAll(items, s)

	printStatusTable(results)
	return nil
}

// printStatusTable groups results by phase and prints a formatted table.
func printStatusTable(results []detector.Result) {
	byPhase := make(map[int][]detector.Result)
	for _, r := range results {
		byPhase[r.Item.Phase] = append(byPhase[r.Item.Phase], r)
	}

	phases := make([]int, 0, len(byPhase))
	for phase := range byPhase {
		phases = append(phases, phase)
	}
	sort.Ints(phases)

	var totalInstalled, totalMissing, totalUnknown int

	for _, phase := range phases {
		fmt.Printf("Phase %d\n", phase)
		for _, r := range byPhase[phase] {
			label, color := statusLabel(r.Status)
			fmt.Printf("  %s%-9s%s  %-40s  %s\n",
				color, label, colorReset,
				r.Item.ID,
				r.Item.Name,
			)
			switch r.Status {
			case detector.StatusInstalled:
				totalInstalled++
			case detector.StatusMissing:
				totalMissing++
			case detector.StatusUnknown:
				totalUnknown++
			}
		}
		fmt.Println()
	}

	fmt.Println("─────────────────────────────────────────────────")
	fmt.Printf("Installed: %d   Missing: %d   Unknown: %d\n",
		totalInstalled, totalMissing, totalUnknown)
}

// statusLabel returns the display text and ANSI color for a status.
func statusLabel(s detector.Status) (label, color string) {
	switch s {
	case detector.StatusInstalled:
		return "[OK]", colorGreen
	case detector.StatusMissing:
		return "[MISSING]", colorRed
	default:
		return "[?]", colorYellow
	}
}
