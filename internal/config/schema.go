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
	Profiles   []Profile   `json:"profiles"`
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
	UpgradeIfInstalled    bool   `json:"upgrade_if_installed"` // If true and check passes, run winget upgrade instead of skipping
}

// Package is a Tier 1 winget package.
type Package struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Phase          int    `json:"phase"`
	Category       string `json:"category"`        // GUI display grouping — does not affect install order
	Description    string `json:"description"`    // Short user-facing tooltip: "What does this do?"
	Scope          string `json:"scope"`           // "machine" | "user" — empty means use Settings.DefaultScope
	Check          string `json:"check"`           // Optional shell command — exit 0 = already installed, skip winget
	Version        string `json:"version"`         // Optional — pin to a specific winget package version
	RebootAfter    bool   `json:"reboot_after"`
	TimeoutSeconds int    `json:"timeout_seconds"` // 0 means use Settings.DefaultTimeoutSeconds
	Notes          string `json:"notes"`
}

// Command is a Tier 2 shell command.
type Command struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Phase           int      `json:"phase"`
	Category        string   `json:"category"`        // GUI display grouping — does not affect install order
	Description     string   `json:"description"`     // Short user-facing tooltip: "What does this do?"
	Check           string   `json:"check"`            // Shell command — exit 0 = already installed, skip
	Cmd             string   `json:"command"`          // Install command to run
	DependsOn       []string `json:"depends_on"`       // Winget IDs or command IDs that must have succeeded
	RebootAfter     bool     `json:"reboot_after"`
	TimeoutSeconds  int      `json:"timeout_seconds"`
	OnFailurePrompt string   `json:"on_failure_prompt"` // If set, printed to the user when the command fails, then waits for Enter
	Notes           string   `json:"notes"`
	// Scrape-download fields — mutually exclusive with Cmd.
	ScrapeURL   string `json:"scrape_url"`
	URLPattern  string `json:"url_pattern"`
	InstallArgs string `json:"install_args"`
}

// Extension is a Tier 3 browser extension.
type Extension struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Phase       int    `json:"phase"`
	Category    string `json:"category"`         // GUI display grouping — does not affect install order
	Description string `json:"description"`     // Short user-facing tooltip: "What does this do?"
	ExtensionID string `json:"extension_id"`
	Browser     string `json:"browser"` // "brave" | "chrome" | "firefox"
	Mode        string `json:"mode"`    // "force" | "url" — empty means use Settings.ExtensionMode
	Notes       string `json:"notes"`
}

// Profile is a named selection preset for the GUI.
type Profile struct {
	Name string   `json:"name"`
	IDs  []string `json:"ids"`
}
