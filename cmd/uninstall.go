package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
	"github.com/Ktulue/KtulueKit-W11/internal/desktop"
	"github.com/Ktulue/KtulueKit-W11/internal/installer"
	"github.com/Ktulue/KtulueKit-W11/internal/reporter"
	"github.com/Ktulue/KtulueKit-W11/internal/runner"
	"github.com/Ktulue/KtulueKit-W11/internal/state"
)

func runUninstall(cmd *cobra.Command, args []string) error {
	if err := filterFlagsError(onlyIDs, excludeIDs); err != nil {
		return err
	}
	if profileName != "" && onlyIDs != "" {
		return fmt.Errorf("--profile and --only are mutually exclusive")
	}

	resolved, cleanup, err := resolveConfigPaths(configPaths)
	if err != nil {
		return err
	}
	defer cleanup()

	cfg, err := config.LoadAll(resolved)
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	filter, err := resolveFilter(cfg, profileName, onlyIDs)
	if err != nil {
		return err
	}
	excludeSet := parseIDList(excludeIDs)

	items := buildUninstallList(cfg, filter, excludeSet)
	if len(items) == 0 {
		fmt.Println("No items to uninstall.")
		return nil
	}

	if !dryRun {
		fmt.Println("Items to be removed:")
		for _, name := range items {
			fmt.Printf("  - %s\n", name)
		}
		fmt.Println()
		fmt.Print("Type 'yes' to confirm: ")
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		answer := strings.TrimSpace(scanner.Text())
		if !strings.EqualFold(answer, "yes") {
			fmt.Println("Uninstall cancelled.")
			return nil
		}
	}

	if err := installer.CheckWingetAvailable(); err != nil {
		return fmt.Errorf("winget not available: %w", err)
	}

	s, err := state.Load()
	if err != nil {
		return fmt.Errorf("state load: %w", err)
	}

	rep, err := reporter.New(cfg.Settings.LogDir, os.Stdout)
	if err != nil {
		return fmt.Errorf("reporter: %w", err)
	}
	defer rep.Close()

	selectedMap := buildSelectedMap(cfg, filter, excludeSet)
	firstPath := ""
	if len(configPaths) > 0 {
		firstPath = configPaths[0]
	}
	r := runner.New(cfg, rep, s, dryRun, 1, firstPath, desktop.ShortcutRemove)
	r.SetSelectedIDs(selectedMap)

	pauseCh := make(chan bool, 1)
	r.SetPauseResponse(pauseCh)
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		// Non-TTY (piped stdin): auto-approve any consecutive-failure pauses.
		go func() {
			for {
				pauseCh <- true
			}
		}()
	}

	r.RunUninstall(cmd.Context())
	rep.Summary()
	return nil
}

// buildUninstallList returns display names of items matching filter and not excluded.
// filter nil = all items. exclude nil = no exclusions.
func buildUninstallList(cfg *config.Config, filter map[string]bool, exclude map[string]bool) []string {
	var names []string
	for _, p := range cfg.Packages {
		if (filter == nil || filter[p.ID]) && !exclude[p.ID] {
			names = append(names, p.Name)
		}
	}
	for _, c := range cfg.Commands {
		if (filter == nil || filter[c.ID]) && !exclude[c.ID] {
			names = append(names, c.Name)
		}
	}
	for _, e := range cfg.Extensions {
		if (filter == nil || filter[e.ID]) && !exclude[e.ID] {
			names = append(names, e.Name)
		}
	}
	return names
}
