package state

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// legacyStateFile is the old CWD-relative state path, kept for migration.
const legacyStateFile = ".ktuluekit-state.json"

// State tracks which item IDs have completed, so the tool can resume after reboot.
type State struct {
	Succeeded   map[string]bool `json:"succeeded"`
	Failed      map[string]bool `json:"failed"`
	ResumePhase int             `json:"resume_phase,omitempty"`
}

// statePath returns the resolved path for the state file.
// Primary: %LOCALAPPDATA%\KtulueKit\state.json
// Fallback (LOCALAPPDATA empty): CWD-relative .ktuluekit-state.json
func statePath() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		return legacyStateFile
	}
	return filepath.Join(base, "KtulueKit", "state.json")
}

func Load() (*State, error) {
	s := &State{
		Succeeded: make(map[string]bool),
		Failed:    make(map[string]bool),
	}

	base := os.Getenv("LOCALAPPDATA")

	// If LOCALAPPDATA is available, try new path first.
	if base != "" {
		newPath := statePath()
		data, err := os.ReadFile(newPath)
		if err == nil {
			if err := json.Unmarshal(data, s); err != nil {
				return nil, err
			}
			return s, nil
		}
		if !os.IsNotExist(err) {
			return nil, err
		}
	}

	// Try legacy CWD path (also the only path when LOCALAPPDATA is empty).
	data, err := os.ReadFile(legacyStateFile)
	if os.IsNotExist(err) {
		return s, nil // fresh run
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, err
	}

	// Migrate: write to new path and delete legacy file (only when LOCALAPPDATA is set).
	if base != "" {
		if writeErr := s.Save(); writeErr == nil {
			_ = os.Remove(legacyStateFile) // best-effort; orphan is harmless
		}
	}

	return s, nil
}

func (s *State) Save() error {
	path := statePath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (s *State) MarkSucceeded(id string) {
	s.Succeeded[id] = true
	delete(s.Failed, id)
	_ = s.Save()
}

func (s *State) MarkFailed(id string) {
	s.Failed[id] = true
	_ = s.Save()
}

// SaveResumePhase records the next phase to start from and persists state.
func (s *State) SaveResumePhase(phase int) error {
	s.ResumePhase = phase
	return s.Save()
}

// Clear deletes the state file after a clean run completes.
func Clear() error {
	err := os.Remove(statePath())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
