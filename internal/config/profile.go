package config

import "fmt"

// LookupProfile returns the IDs slice for the named profile.
// Returns an error if no profile with that exact name exists (case-sensitive).
func LookupProfile(cfg *Config, name string) ([]string, error) {
	for _, p := range cfg.Profiles {
		if p.Name == name {
			return p.IDs, nil
		}
	}
	return nil, fmt.Errorf("profile %q not found (available: %v)", name, profileNames(cfg))
}

// profileNames returns the names of all profiles in cfg, for use in error messages.
func profileNames(cfg *Config) []string {
	names := make([]string, len(cfg.Profiles))
	for i, p := range cfg.Profiles {
		names[i] = p.Name
	}
	return names
}
