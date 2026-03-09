package config

// Config is the top-level structure of ktuluekit.json.
type Config struct {
	Schema   string   `json:"$schema"`
	Version  string   `json:"version"`
	Metadata Metadata `json:"metadata"`
	Settings Settings `json:"settings"`
	Packages []Package  `json:"packages"`
	Commands []Command  `json:"commands"`
	Extensions []Extension `json:"extensions"`
}

type Metadata struct {
	Name        string `json:"name"`
	Author      string `json:"author"`
	Description string `json:"description"`
	Repo        string `json:"repo"`
}

type Settings struct {
	LogDir                string `json:"log_dir"`
	RetryCount            int    `json:"retry_count"`
	DefaultTimeoutSeconds int    `json:"default_timeout_seconds"`
	DefaultScope          string `json:"default_scope"`
	ExtensionMode         string `json:"extension_mode"`
}

// Package is a Tier 1 winget package.
type Package struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Phase          int    `json:"phase"`
	Scope          string `json:"scope"`           // "machine" | "user" — empty means use Settings.DefaultScope
	RebootAfter    bool   `json:"reboot_after"`
	TimeoutSeconds int    `json:"timeout_seconds"` // 0 means use Settings.DefaultTimeoutSeconds
	Notes          string `json:"notes"`
}

// Command is a Tier 2 shell command.
type Command struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Phase          int      `json:"phase"`
	Check          string   `json:"check"`      // Shell command — exit 0 = already installed, skip
	Cmd            string   `json:"command"`    // Install command to run
	DependsOn      []string `json:"depends_on"` // Winget IDs or command IDs that must have succeeded
	RebootAfter    bool     `json:"reboot_after"`
	TimeoutSeconds int      `json:"timeout_seconds"`
	Notes          string   `json:"notes"`
}

// Extension is a Tier 3 browser extension.
type Extension struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Phase       int    `json:"phase"`
	ExtensionID string `json:"extension_id"`
	Browser     string `json:"browser"` // "brave" | "chrome" | "firefox"
	Mode        string `json:"mode"`    // "force" | "url" — empty means use Settings.ExtensionMode
	Notes       string `json:"notes"`
}
