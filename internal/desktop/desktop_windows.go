package desktop

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ShortcutMode controls how the tool handles .lnk files created by installers.
type ShortcutMode int

const (
	ShortcutKeep   ShortcutMode = iota // leave all shortcuts in place
	ShortcutRemove                      // delete all new shortcuts automatically
	ShortcutAsk                         // prompt the user for each new shortcut
)

// Snapshot returns the lowercase paths of all .lnk files currently on both desktops.
// Call this just before an install; compare with NewShortcuts after.
func Snapshot() map[string]bool {
	seen := make(map[string]bool)
	for _, dir := range desktopDirs() {
		matches, err := filepath.Glob(filepath.Join(dir, "*.lnk"))
		if err != nil {
			continue
		}
		for _, m := range matches {
			seen[strings.ToLower(m)] = true
		}
	}
	return seen
}

// NewShortcuts returns .lnk paths that exist now but were not present in before.
func NewShortcuts(before map[string]bool) []string {
	now := Snapshot()
	var added []string
	for path := range now {
		if !before[path] {
			added = append(added, path)
		}
	}
	return added
}

// backupDir returns the path to the KtulueKit Shortcuts folder on the user's desktop.
func backupDir() string {
	home := os.Getenv("USERPROFILE")
	return filepath.Join(home, "Desktop", "KtulueKit Shortcuts")
}

// Backup moves a .lnk file into the "KtulueKit Shortcuts" folder on the desktop
// instead of permanently deleting it. Creates the folder if it doesn't exist.
func Backup(path string) error {
	dir := backupDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create backup dir: %w", err)
	}
	dest := filepath.Join(dir, filepath.Base(path))
	return os.Rename(path, dest)
}

// PromptMode asks the user once at startup how to handle new desktop shortcuts.
func PromptMode() ShortcutMode {
	fmt.Println()
	fmt.Println("  Desktop shortcuts: How should KtulueKit handle shortcuts created by installers?")
	fmt.Println("    [Y] Remove all automatically")
	fmt.Println("    [N] Keep all")
	fmt.Println("    [A] Ask me for each one")
	fmt.Print("  Choice [Y/N/A]: ")

	reader := bufio.NewReader(os.Stdin)
	for {
		line, _ := reader.ReadString('\n')
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "y", "yes":
			return ShortcutRemove
		case "n", "no":
			return ShortcutKeep
		case "a", "ask":
			return ShortcutAsk
		default:
			fmt.Print("  Please enter Y, N, or A: ")
		}
	}
}

// PromptRemove asks whether to remove a specific shortcut. Returns true to delete it.
func PromptRemove(path string) bool {
	fmt.Printf("    Remove %q? [Y/N]: ", filepath.Base(path))
	reader := bufio.NewReader(os.Stdin)
	for {
		line, _ := reader.ReadString('\n')
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "y", "yes":
			return true
		case "n", "no":
			return false
		default:
			fmt.Print("    [Y/N]: ")
		}
	}
}

// desktopDirs returns the two desktop directories to monitor.
func desktopDirs() []string {
	var dirs []string
	if home := os.Getenv("USERPROFILE"); home != "" {
		dirs = append(dirs, filepath.Join(home, "Desktop"))
	}
	if pub := os.Getenv("PUBLIC"); pub != "" {
		dirs = append(dirs, filepath.Join(pub, "Desktop"))
	}
	return dirs
}
