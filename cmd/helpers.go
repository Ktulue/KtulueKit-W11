package main

import (
	"github.com/Ktulue/KtulueKit-W11/internal/config"
	"github.com/Ktulue/KtulueKit-W11/internal/detector"
)

// filterItemsByIDs returns only those items whose ID is in the allowlist.
func filterItemsByIDs(items []detector.Item, ids map[string]bool) []detector.Item {
	out := make([]detector.Item, 0, len(items))
	for _, item := range items {
		if ids[item.ID] {
			out = append(out, item)
		}
	}
	return out
}

// filterConfigByIDs mutates cfg to include only Packages, Commands, and Extensions
// whose IDs appear in the ids slice. Used by --profile on export and status.
func filterConfigByIDs(cfg *config.Config, ids []string) {
	allow := make(map[string]bool, len(ids))
	for _, id := range ids {
		allow[id] = true
	}

	filtered := cfg.Packages[:0]
	for _, p := range cfg.Packages {
		if allow[p.ID] {
			filtered = append(filtered, p)
		}
	}
	cfg.Packages = filtered

	filteredCmds := cfg.Commands[:0]
	for _, c := range cfg.Commands {
		if allow[c.ID] {
			filteredCmds = append(filteredCmds, c)
		}
	}
	cfg.Commands = filteredCmds

	filteredExts := cfg.Extensions[:0]
	for _, e := range cfg.Extensions {
		if allow[e.ID] {
			filteredExts = append(filteredExts, e)
		}
	}
	cfg.Extensions = filteredExts
}
