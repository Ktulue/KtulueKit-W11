package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
	"github.com/Ktulue/KtulueKit-W11/internal/desktop"
	"github.com/Ktulue/KtulueKit-W11/internal/detector"
	"github.com/Ktulue/KtulueKit-W11/internal/reporter"
	"github.com/Ktulue/KtulueKit-W11/internal/runner"
	"github.com/Ktulue/KtulueKit-W11/internal/scheduler"
	"github.com/Ktulue/KtulueKit-W11/internal/state"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the Wails application struct. Methods on App are bound to the frontend.
type App struct {
	ctx        context.Context
	configPath string

	mu             sync.Mutex
	running        bool
	rebootResponse chan bool
}

// NewApp creates the application instance.
func NewApp(configPath string) *App {
	return &App{configPath: configPath}
}

// startup is called by Wails when the app is ready.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// GetConfig parses ktuluekit.json and returns the display model for the selection screen.
func (a *App) GetConfig() ConfigView {
	cfg, err := config.Load(a.configPath)
	if err != nil {
		return ConfigView{}
	}

	// Collect all items by category.
	byCategory := make(map[string][]ItemView)
	addItem := func(id, name, category, description, notes string) {
		if category == "" {
			category = "Other"
		}
		desc := description
		if desc == "" {
			desc = notes
		}
		byCategory[category] = append(byCategory[category], ItemView{
			ID:          id,
			Name:        name,
			Description: desc,
			Notes:       notes,
		})
	}

	for _, p := range cfg.Packages {
		addItem(p.ID, p.Name, p.Category, p.Description, p.Notes)
	}
	for _, c := range cfg.Commands {
		addItem(c.ID, c.Name, c.Category, c.Description, c.Notes)
	}
	for _, e := range cfg.Extensions {
		addItem(e.ID, e.Name, e.Category, e.Description, e.Notes)
	}

	// Sort items within each category alphabetically.
	for cat := range byCategory {
		items := byCategory[cat]
		sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
		byCategory[cat] = items
	}

	// Build ordered category list.
	seen := make(map[string]bool)
	var categories []CategoryView
	for _, name := range categoryOrder {
		if items, ok := byCategory[name]; ok {
			categories = append(categories, CategoryView{Name: name, Items: items})
			seen[name] = true
		}
	}
	// Append any categories not in categoryOrder as "Other".
	var others []ItemView
	for cat, items := range byCategory {
		if !seen[cat] {
			others = append(others, items...)
		}
	}
	if len(others) > 0 {
		sort.Slice(others, func(i, j int) bool { return others[i].Name < others[j].Name })
		categories = append(categories, CategoryView{Name: "Other", Items: others})
	}

	// Build profiles.
	profiles := make([]ProfileView, len(cfg.Profiles))
	for i, p := range cfg.Profiles {
		profiles[i] = ProfileView{Name: p.Name, IDs: p.IDs}
	}

	return ConfigView{Categories: categories, Profiles: profiles}
}

// StartInstall validates the selection and launches the installer in a goroutine.
// Returns an error message string on validation failure, or "" on success.
func (a *App) StartInstall(ids []string) string {
	if len(ids) == 0 {
		return "No items selected."
	}

	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return "An install is already in progress."
	}
	a.running = true
	rebootCh := make(chan bool, 1)
	a.rebootResponse = rebootCh
	a.mu.Unlock()

	go func() {
		defer func() {
			a.mu.Lock()
			a.running = false
			a.rebootResponse = nil
			a.mu.Unlock()
		}()

		// Clean up any stale resume task from a previous CLI reboot run.
		_ = scheduler.DeleteResumeTask()

		cfg, err := config.Load(a.configPath)
		if err != nil {
			runtime.EventsEmit(a.ctx, "complete", SummaryResult{
				Failed:  []string{fmt.Sprintf("Failed to load config: %v", err)},
				LogPath: "",
			})
			return
		}

		rep, err := reporter.New(cfg.Settings.LogDir, os.Stdout)
		if err != nil {
			runtime.EventsEmit(a.ctx, "complete", SummaryResult{
				Failed:  []string{fmt.Sprintf("Failed to create log: %v", err)},
				LogPath: "",
			})
			return
		}
		defer rep.Close()

		s, err := state.Load()
		if err != nil {
			runtime.EventsEmit(a.ctx, "complete", SummaryResult{
				Failed:  []string{fmt.Sprintf("Failed to load state: %v", err)},
				LogPath: rep.LogPath(),
			})
			return
		}

		r := runner.New(cfg, rep, s, false, 1, a.configPath, desktop.ShortcutRemove)
		selectedMap := make(map[string]bool, len(ids))
		for _, id := range ids {
			selectedMap[id] = true
		}
		r.SetSelectedIDs(selectedMap)
		r.SetRebootResponse(rebootCh)
		pauseCh := make(chan bool, 1)
		r.SetPauseResponse(pauseCh)
		r.SetOnProgress(func(e runner.ProgressEvent) {
			runtime.EventsEmit(a.ctx, "progress", e)
		})

		runStart := time.Now()
		r.Run(context.Background())
		elapsed := time.Since(runStart).Round(time.Second).String()

		// Mirror CLI behaviour: clear succeeded state only on a fully clean run.
		if !rep.HasFailures() && !r.WasInterrupted() {
			_ = state.Clear()
		}

		summary := SummaryResult{
			Installed:        rep.NamesBy("installed"),
			Upgraded:         rep.NamesBy("upgraded"),
			Already:          rep.NamesBy("already_installed"),
			Failed:           rep.NamesBy("failed"),
			Skipped:          rep.NamesBy("skipped"),
			Reboot:           rep.NamesBy("reboot_required"),
			ShortcutsRemoved: rep.NamesBy("shortcut_removed"),
			TotalElapsed:     elapsed,
			LogPath:          rep.LogPath(),
		}

		runtime.EventsEmit(a.ctx, "complete", summary)
	}()

	return ""
}

// ConfirmReboot sends true on the reboot channel — runner will call shutdown /r /t 30 and return.
func (a *App) ConfirmReboot() {
	a.mu.Lock()
	ch := a.rebootResponse
	a.mu.Unlock()
	if ch != nil {
		ch <- true
	}
}

// CancelReboot sends false on the reboot channel — runner deletes the task and continues.
func (a *App) CancelReboot() {
	a.mu.Lock()
	ch := a.rebootResponse
	a.mu.Unlock()
	if ch != nil {
		ch <- false
	}
}

// StartUninstall uninstalls the given item IDs using runner.RunUninstall.
// Returns an error message string on validation failure, or "" on success.
func (a *App) StartUninstall(ids []string) string {
	if len(ids) == 0 {
		return "No items selected."
	}
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return "An operation is already in progress."
	}
	a.running = true
	a.mu.Unlock()

	go func() {
		defer func() {
			a.mu.Lock()
			a.running = false
			a.mu.Unlock()
		}()

		cfg, err := config.Load(a.configPath)
		if err != nil {
			runtime.EventsEmit(a.ctx, "uninstall_complete", SummaryResult{
				Failed: []string{fmt.Sprintf("Failed to load config: %v", err)},
			})
			return
		}
		rep, err := reporter.New(cfg.Settings.LogDir, os.Stdout)
		if err != nil {
			runtime.EventsEmit(a.ctx, "uninstall_complete", SummaryResult{
				Failed: []string{fmt.Sprintf("Failed to create log: %v", err)},
			})
			return
		}
		defer rep.Close()
		s, err := state.Load()
		if err != nil {
			runtime.EventsEmit(a.ctx, "uninstall_complete", SummaryResult{
				Failed:  []string{fmt.Sprintf("Failed to load state: %v", err)},
				LogPath: rep.LogPath(),
			})
			return
		}

		r := runner.New(cfg, rep, s, false, 1, a.configPath, desktop.ShortcutRemove)
		selectedMap := make(map[string]bool, len(ids))
		for _, id := range ids {
			selectedMap[id] = true
		}
		r.SetSelectedIDs(selectedMap)
		pauseCh := make(chan bool, 1)
		r.SetPauseResponse(pauseCh)
		r.SetOnProgress(func(e runner.ProgressEvent) {
			runtime.EventsEmit(a.ctx, "progress", e)
		})

		runStart := time.Now()
		r.RunUninstall(context.Background())
		elapsed := time.Since(runStart).Round(time.Second).String()

		summary := SummaryResult{
			Installed:    rep.NamesBy("installed"),
			Failed:       rep.NamesBy("failed"),
			Skipped:      rep.NamesBy("skipped"),
			TotalElapsed: elapsed,
			LogPath:      rep.LogPath(),
		}
		runtime.EventsEmit(a.ctx, "uninstall_complete", summary)
	}()

	return ""
}

// GetInstalledItems runs detector checks for the given IDs and returns those
// that are currently installed (detector check exits 0).
func (a *App) GetInstalledItems(ids []string) []string {
	cfg, err := config.Load(a.configPath)
	if err != nil {
		return nil
	}
	checkCmds := make(map[string]string)
	for _, p := range cfg.Packages {
		checkCmds[p.ID] = p.Check
	}
	for _, c := range cfg.Commands {
		checkCmds[c.ID] = c.Check
	}
	// Extensions have no Check field — excluded here.
	// GUI scan will not show extensions in the uninstall list.

	var installed []string
	for _, id := range ids {
		check := checkCmds[id]
		if check == "" {
			continue
		}
		if isInstalled, _ := detector.RunCheckDetailed(check); isInstalled {
			installed = append(installed, id)
		}
	}
	return installed
}
