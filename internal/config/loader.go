package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Load reads and parses the single config file at path.
// It is a convenience wrapper around LoadAll.
func Load(path string) (*Config, error) {
	return LoadAll([]string{path})
}

// LoadAll merges one or more config files left-to-right and returns the combined Config.
// Later files override earlier files on ID/name collision (last-wins).
// validate() and applyDefaults() are called exactly once on the merged result.
// If paths is empty, it defaults to ["ktuluekit.json"].
func LoadAll(paths []string) (*Config, error) {
	if len(paths) == 0 {
		paths = []string{"ktuluekit.json"}
	}

	var merged Config

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("cannot read config file %q: %w", path, err)
		}

		var cfg Config
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("cannot parse config file %q: %w", path, err)
		}

		mergeInto(&merged, &cfg)
	}

	if err := validate(&merged); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	applyDefaults(&merged)
	return &merged, nil
}

// mergeInto applies src on top of dst using last-wins semantics.
func mergeInto(dst, src *Config) {
	// Metadata: first config is authoritative.
	if dst.Metadata.Name == "" {
		dst.Metadata = src.Metadata
	}

	// Version/Schema: first config is authoritative.
	if dst.Version == "" {
		dst.Version = src.Version
	}
	if dst.Schema == "" {
		dst.Schema = src.Schema
	}

	// Settings: non-zero src fields overwrite dst fields.
	mergeSettings(&dst.Settings, &src.Settings)

	// Packages: last-wins by ID, preserving first-seen position.
	dst.Packages = mergePackages(dst.Packages, src.Packages)

	// Commands: last-wins by ID, preserving first-seen position.
	dst.Commands = mergeCommands(dst.Commands, src.Commands)

	// Extensions: last-wins by ID, preserving first-seen position.
	dst.Extensions = mergeExtensions(dst.Extensions, src.Extensions)

	// Profiles: last-wins by Name.
	dst.Profiles = mergeProfiles(dst.Profiles, src.Profiles)
}

// mergeSettings overwrites dst fields with src fields where src is non-zero.
func mergeSettings(dst, src *Settings) {
	if src.LogDir != "" {
		dst.LogDir = src.LogDir
	}
	if src.RetryCount != 0 {
		dst.RetryCount = src.RetryCount
	}
	if src.DefaultTimeoutSeconds != 0 {
		dst.DefaultTimeoutSeconds = src.DefaultTimeoutSeconds
	}
	if src.DefaultScope != "" {
		dst.DefaultScope = src.DefaultScope
	}
	if src.ExtensionMode != "" {
		dst.ExtensionMode = src.ExtensionMode
	}
	if src.UpgradeIfInstalled {
		dst.UpgradeIfInstalled = src.UpgradeIfInstalled
	}
}

// mergePackages returns base with src entries merged in (last-wins by ID, position preserved).
func mergePackages(base, src []Package) []Package {
	index := make(map[string]int, len(base))
	result := make([]Package, len(base))
	copy(result, base)
	for i, p := range result {
		index[p.ID] = i
	}
	for _, p := range src {
		if i, exists := index[p.ID]; exists {
			result[i] = p
		} else {
			index[p.ID] = len(result)
			result = append(result, p)
		}
	}
	return result
}

// mergeCommands returns base with src entries merged in (last-wins by ID, position preserved).
func mergeCommands(base, src []Command) []Command {
	index := make(map[string]int, len(base))
	result := make([]Command, len(base))
	copy(result, base)
	for i, c := range result {
		index[c.ID] = i
	}
	for _, c := range src {
		if i, exists := index[c.ID]; exists {
			result[i] = c
		} else {
			index[c.ID] = len(result)
			result = append(result, c)
		}
	}
	return result
}

// mergeExtensions returns base with src entries merged in (last-wins by ID, position preserved).
func mergeExtensions(base, src []Extension) []Extension {
	index := make(map[string]int, len(base))
	result := make([]Extension, len(base))
	copy(result, base)
	for i, e := range result {
		index[e.ID] = i
	}
	for _, e := range src {
		if i, exists := index[e.ID]; exists {
			result[i] = e
		} else {
			index[e.ID] = len(result)
			result = append(result, e)
		}
	}
	return result
}

// mergeProfiles returns base with src profiles merged in (last-wins by Name).
func mergeProfiles(base, src []Profile) []Profile {
	index := make(map[string]int, len(base))
	result := make([]Profile, len(base))
	copy(result, base)
	for i, p := range result {
		index[p.Name] = i
	}
	for _, p := range src {
		if i, exists := index[p.Name]; exists {
			result[i] = p
		} else {
			index[p.Name] = len(result)
			result = append(result, p)
		}
	}
	return result
}

// validate checks required fields and catches obvious mistakes.
func validate(cfg *Config) error {
	if cfg.Version == "" {
		return fmt.Errorf("missing required field: version")
	}
	if cfg.Metadata.Name == "" {
		return fmt.Errorf("missing required field: metadata.name")
	}

	ids := make(map[string]bool)

	for i, p := range cfg.Packages {
		if p.ID == "" {
			return fmt.Errorf("packages[%d]: missing required field 'id'", i)
		}
		if p.Name == "" {
			return fmt.Errorf("packages[%d] (%s): missing required field 'name'", i, p.ID)
		}
		if p.Phase < 1 {
			return fmt.Errorf("packages[%d] (%s): phase must be >= 1", i, p.ID)
		}
		if ids[p.ID] {
			return fmt.Errorf("packages[%d]: duplicate id %q", i, p.ID)
		}
		ids[p.ID] = true
	}

	for i, c := range cfg.Commands {
		if c.ID == "" {
			return fmt.Errorf("commands[%d]: missing required field 'id'", i)
		}
		if c.Name == "" {
			return fmt.Errorf("commands[%d] (%s): missing required field 'name'", i, c.ID)
		}
		if c.Phase < 1 {
			return fmt.Errorf("commands[%d] (%s): phase must be >= 1", i, c.ID)
		}
		if c.Check == "" {
			return fmt.Errorf("commands[%d] (%s): missing required field 'check'", i, c.ID)
		}
		if c.Cmd == "" {
			return fmt.Errorf("commands[%d] (%s): missing required field 'command'", i, c.ID)
		}
		if ids[c.ID] {
			return fmt.Errorf("commands[%d]: duplicate id %q", i, c.ID)
		}
		ids[c.ID] = true
	}

	for i, e := range cfg.Extensions {
		if e.ID == "" {
			return fmt.Errorf("extensions[%d]: missing required field 'id'", i)
		}
		if e.Name == "" {
			return fmt.Errorf("extensions[%d] (%s): missing required field 'name'", i, e.ID)
		}
		if e.Phase < 1 {
			return fmt.Errorf("extensions[%d] (%s): phase must be >= 1", i, e.ID)
		}
		if e.ExtensionID == "" {
			return fmt.Errorf("extensions[%d] (%s): missing required field 'extension_id'", i, e.ID)
		}
		if len(e.ExtensionID) != 32 {
			return fmt.Errorf("extensions[%d] (%s): extension_id must be 32 characters", i, e.ID)
		}
		if ids[e.ID] {
			return fmt.Errorf("extensions[%d]: duplicate id %q", i, e.ID)
		}
		ids[e.ID] = true
	}

	return nil
}

// applyDefaults fills in zero-value fields from settings.
func applyDefaults(cfg *Config) {
	if cfg.Settings.LogDir == "" {
		cfg.Settings.LogDir = "./logs"
	}
	if cfg.Settings.RetryCount == 0 {
		cfg.Settings.RetryCount = 1
	}
	if cfg.Settings.DefaultTimeoutSeconds == 0 {
		cfg.Settings.DefaultTimeoutSeconds = 300
	}
	if cfg.Settings.DefaultScope == "" {
		cfg.Settings.DefaultScope = "machine"
	}
	if cfg.Settings.ExtensionMode == "" {
		cfg.Settings.ExtensionMode = "url"
	}

	for i := range cfg.Packages {
		if cfg.Packages[i].Scope == "" {
			cfg.Packages[i].Scope = cfg.Settings.DefaultScope
		}
		if cfg.Packages[i].TimeoutSeconds == 0 {
			cfg.Packages[i].TimeoutSeconds = cfg.Settings.DefaultTimeoutSeconds
		}
	}

	for i := range cfg.Commands {
		if cfg.Commands[i].TimeoutSeconds == 0 {
			cfg.Commands[i].TimeoutSeconds = cfg.Settings.DefaultTimeoutSeconds
		}
	}

	for i := range cfg.Extensions {
		if cfg.Extensions[i].Mode == "" {
			cfg.Extensions[i].Mode = cfg.Settings.ExtensionMode
		}
	}
}
