package main

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
)

func runList(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadAll(configPaths)
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	type listItem struct {
		tier string
		id   string
		name string
	}
	byPhase := make(map[int][]listItem)

	for _, p := range cfg.Packages {
		byPhase[p.Phase] = append(byPhase[p.Phase], listItem{"winget", p.ID, p.Name})
	}
	for _, c := range cfg.Commands {
		byPhase[c.Phase] = append(byPhase[c.Phase], listItem{"command", c.ID, c.Name})
	}
	for _, e := range cfg.Extensions {
		byPhase[e.Phase] = append(byPhase[e.Phase], listItem{"extension", e.ID, e.Name})
	}

	phases := make([]int, 0, len(byPhase))
	for ph := range byPhase {
		phases = append(phases, ph)
	}
	sort.Ints(phases)

	for _, ph := range phases {
		fmt.Printf("\n── Phase %d ──────────────────────────────────────\n", ph)
		for _, item := range byPhase[ph] {
			fmt.Printf("  %-12s  %-40s  %s\n", "["+item.tier+"]", item.id, item.name)
		}
	}

	fmt.Printf("\nTotal: %d winget  |  %d commands  |  %d extensions\n",
		len(cfg.Packages), len(cfg.Commands), len(cfg.Extensions))
	return nil
}
