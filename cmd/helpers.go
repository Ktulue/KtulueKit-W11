package main

import (
	"fmt"
	"strings"

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

// resolveFilter returns the ID allowset for --profile or --only.
// Returns nil if neither is set (meaning "all items").
func resolveFilter(cfg *config.Config, profile, only string) (map[string]bool, error) {
	if profile != "" {
		for _, p := range cfg.Profiles {
			if p.Name == profile {
				m := make(map[string]bool, len(p.IDs))
				for _, id := range p.IDs {
					m[id] = true
				}
				return m, nil
			}
		}
		return nil, fmt.Errorf("profile %q not found", profile)
	}
	if only != "" {
		return parseIDList(only), nil
	}
	return nil, nil
}

// parseIDList parses a comma-separated ID string into a set.
func parseIDList(csv string) map[string]bool {
	m := make(map[string]bool)
	for _, id := range strings.Split(csv, ",") {
		id = strings.TrimSpace(id)
		if id != "" {
			m[id] = true
		}
	}
	return m
}

// buildSelectedMap builds the map[string]bool passed to r.SetSelectedIDs,
// applying the filter allowset and exclude denylist.
func buildSelectedMap(cfg *config.Config, filter map[string]bool, exclude map[string]bool) map[string]bool {
	m := make(map[string]bool)
	add := func(id string) {
		if (filter == nil || filter[id]) && !exclude[id] {
			m[id] = true
		}
	}
	for _, p := range cfg.Packages {
		add(p.ID)
	}
	for _, c := range cfg.Commands {
		add(c.ID)
	}
	for _, e := range cfg.Extensions {
		add(e.ID)
	}
	return m
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
