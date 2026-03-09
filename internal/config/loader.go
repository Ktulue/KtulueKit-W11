package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Load reads and parses the config file at path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read config file %q: %w", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("cannot parse config file %q: %w", path, err)
	}

	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	applyDefaults(&cfg)

	return &cfg, nil
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
