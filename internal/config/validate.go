package config

import (
	"fmt"
	"regexp"
)

// chromeExtIDRe matches the Chrome/Brave extension ID format: exactly 32
// lowercase a-p characters (base-26 encoded, no shell metacharacters).
var chromeExtIDRe = regexp.MustCompile(`^[a-p]{32}$`)

// firefoxExtIDRe matches the Firefox AMO add-on slug format.
var firefoxExtIDRe = regexp.MustCompile(`^[a-z0-9_@.-]+$`)

// ValidationError describes a single config validation problem.
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// Validate checks all config fields and cross-references.
// It collects ALL errors (does not fail fast) and returns them.
// Returns nil if the config is valid.
func Validate(cfg *Config) []ValidationError {
	var errs []ValidationError

	add := func(field, msg string) {
		errs = append(errs, ValidationError{Field: field, Message: msg})
	}

	// 1. Top-level required fields.
	if cfg.Version == "" {
		add("[top-level]", "version is required")
	}
	if cfg.Metadata.Name == "" {
		add("[top-level]", "metadata.name is required")
	}

	// Build ID set for cross-reference checks.
	ids := make(map[string]bool)

	// 2-5. Packages.
	for i, p := range cfg.Packages {
		prefix := fmt.Sprintf("packages[%d]", i)
		if p.ID == "" {
			add(prefix+".id", "required field 'id' is missing")
			continue
		}
		if p.Name == "" {
			add(fmt.Sprintf("%s(%s).name", prefix, p.ID), "required field 'name' is missing")
		}
		if p.Phase < 1 {
			add(fmt.Sprintf("%s(%s).phase", prefix, p.ID), "phase must be >= 1")
		}
		if ids[p.ID] {
			add(fmt.Sprintf("%s.id", prefix), fmt.Sprintf("duplicate ID %q", p.ID))
		} else {
			ids[p.ID] = true
		}
	}

	// Commands.
	for i, c := range cfg.Commands {
		prefix := fmt.Sprintf("commands[%d]", i)
		if c.ID == "" {
			add(prefix+".id", "required field 'id' is missing")
			continue
		}
		if c.Name == "" {
			add(fmt.Sprintf("%s(%s).name", prefix, c.ID), "required field 'name' is missing")
		}
		if c.Phase < 1 {
			add(fmt.Sprintf("%s(%s).phase", prefix, c.ID), "phase must be >= 1")
		}
		if c.Check == "" {
			add(fmt.Sprintf("%s(%s).check", prefix, c.ID), "required field 'check' is missing")
		}
		hasCmd := c.Cmd != ""
		hasBothScrape := c.ScrapeURL != "" && c.URLPattern != ""
		hasAnyScrape := c.ScrapeURL != "" || c.URLPattern != ""
		switch {
		case !hasAnyScrape && !hasCmd:
			// Neither command nor scrape fields — entry is incomplete.
			add(fmt.Sprintf("%s(%s).command", prefix, c.ID),
				"must have either 'command' or both 'scrape_url' and 'url_pattern'")
		case hasAnyScrape && hasCmd:
			// Has command AND at least one scrape field — mutually exclusive.
			add(fmt.Sprintf("%s(%s).command", prefix, c.ID),
				"cannot have both 'command' and 'scrape_url'/'url_pattern'")
		case hasAnyScrape && !hasBothScrape:
			// Has one scrape field but not both — partial scrape entry.
			add(fmt.Sprintf("%s(%s).scrape_url", prefix, c.ID),
				"scrape-type entries must have both 'scrape_url' and 'url_pattern'")
		// else: valid standard command or valid scrape command — no error.
		}
		if ids[c.ID] {
			add(fmt.Sprintf("%s.id", prefix), fmt.Sprintf("duplicate ID %q", c.ID))
		} else {
			ids[c.ID] = true
		}
	}

	// Extensions.
	for i, e := range cfg.Extensions {
		prefix := fmt.Sprintf("extensions[%d]", i)
		if e.ID == "" {
			add(prefix+".id", "required field 'id' is missing")
			continue
		}
		if e.Name == "" {
			add(fmt.Sprintf("%s(%s).name", prefix, e.ID), "required field 'name' is missing")
		}
		if e.Phase < 1 {
			add(fmt.Sprintf("%s(%s).phase", prefix, e.ID), "phase must be >= 1")
		}
		if e.ExtensionID == "" {
			add(fmt.Sprintf("%s(%s).extension_id", prefix, e.ID), "required field 'extension_id' is missing")
		} else {
			switch e.Browser {
			case "chrome", "brave":
				if !chromeExtIDRe.MatchString(e.ExtensionID) {
					add(fmt.Sprintf("%s(%s).extension_id", prefix, e.ID),
						"chrome/brave extension_id must be exactly 32 lowercase a-p characters")
				}
			case "firefox":
				if !firefoxExtIDRe.MatchString(e.ExtensionID) {
					add(fmt.Sprintf("%s(%s).extension_id", prefix, e.ID),
						"firefox extension_id must match ^[a-z0-9_@.-]+$ (AMO slug format)")
				}
			default:
				// Unknown or empty browser: fall back to the original 32-char length check.
				if len(e.ExtensionID) != 32 {
					add(fmt.Sprintf("%s(%s).extension_id", prefix, e.ID),
						fmt.Sprintf("extension_id must be 32 characters, got %d", len(e.ExtensionID)))
				}
			}
		}
		if ids[e.ID] {
			add(fmt.Sprintf("%s.id", prefix), fmt.Sprintf("duplicate ID %q", e.ID))
		} else {
			ids[e.ID] = true
		}
	}

	// 6. depends_on cross-references (Commands only — Package and Extension have no DependsOn).
	for i, c := range cfg.Commands {
		for _, dep := range c.DependsOn {
			if !ids[dep] {
				add(fmt.Sprintf("commands[%d](%s).depends_on", i, c.ID),
					fmt.Sprintf("unknown ID %q (not in packages or commands)", dep))
			}
		}
	}

	// 7. Profile ids cross-references.
	for i, p := range cfg.Profiles {
		for _, id := range p.IDs {
			if !ids[id] {
				add(fmt.Sprintf("profiles[%d](%s).ids", i, p.Name),
					fmt.Sprintf("unknown ID %q", id))
			}
		}
	}

	return errs
}
