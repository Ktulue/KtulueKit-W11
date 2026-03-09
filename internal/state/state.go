package state

import (
	"encoding/json"
	"os"
)

const stateFile = ".ktuluekit-state.json"

// State tracks which item IDs have completed, so the tool can resume after reboot.
type State struct {
	Succeeded map[string]bool `json:"succeeded"` // ID -> true
	Failed    map[string]bool `json:"failed"`    // ID -> true
}

func Load() (*State, error) {
	s := &State{
		Succeeded: make(map[string]bool),
		Failed:    make(map[string]bool),
	}

	data, err := os.ReadFile(stateFile)
	if os.IsNotExist(err) {
		return s, nil // fresh run
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, s); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *State) Save() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(stateFile, data, 0644)
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

// Clear deletes the state file after a clean run completes.
func Clear() error {
	err := os.Remove(stateFile)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
