package main

// ConfigView is the Go→Svelte data contract for the selection screen.
type ConfigView struct {
	Categories []CategoryView
	Profiles   []ProfileView
}

// categoryOrder defines the display order of categories in the GUI.
// Items whose category is not listed fall into an appended "Other" bucket.
var categoryOrder = []string{
	"Dev Tools", "Terminal & Shell", "Editors & IDEs", "AI Tools",
	"Creative", "3D & Making", "Streaming", "Gaming & Game Dev",
	"Media & Music", "Utilities", "Browsers & Social", "Networking", "Windows Config",
}

// CategoryView is a named group of items for display in the selection screen.
type CategoryView struct {
	Name  string
	Items []ItemView // sorted alphabetically by Name
}

// ItemView is a single selectable item in the GUI.
type ItemView struct {
	ID          string
	Name        string
	Description string // user-facing tooltip; falls back to Notes if empty
	Notes       string
}

// ProfileView is a named selection preset.
type ProfileView struct {
	Name string
	IDs  []string
}

// SummaryResult is the payload of the "complete" Wails event.
type SummaryResult struct {
	Installed        []string
	Upgraded         []string
	Already          []string
	Failed           []string
	Skipped          []string
	Reboot           []string
	ShortcutsRemoved []string
	TotalElapsed     string
	LogPath          string
}
