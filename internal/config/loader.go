package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// DefaultConfigPath is the config file used when no --config flag is provided.
const DefaultConfigPath = "ktuluekit.json"

// Load reads and parses the single config file at path.
// It is a convenience wrapper around LoadAll.
func Load(path string) (*Config, error) {
	return LoadAll([]string{path})
}

// LoadAll merges one or more config files left-to-right and returns the combined Config.
// Later files override earlier files on ID/name collision (last-wins).
// applyDefaults() is called on the merged result.
// Validation is NOT performed — callers must call Validate() explicitly after LoadAll.
// If paths is empty, it defaults to ["ktuluekit.json"].
func LoadAll(paths []string) (*Config, error) {
	if len(paths) == 0 {
		paths = []string{DefaultConfigPath}
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
	// UpgradeIfInstalled is one-way: a later config can enable it but not disable it.
	// Intentional — clearing bool fields via overlay is not supported per spec.
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
