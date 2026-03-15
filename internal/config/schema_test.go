package config

import (
	"encoding/json"
	"testing"
)

func TestCommandUninstallCmdFieldRoundTrip(t *testing.T) {
	input := `{"id":"x","name":"X","command":"install.exe","uninstall_cmd":"uninstall.exe"}`
	var cmd Command
	if err := json.Unmarshal([]byte(input), &cmd); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if cmd.UninstallCmd != "uninstall.exe" {
		t.Errorf("UninstallCmd = %q, want %q", cmd.UninstallCmd, "uninstall.exe")
	}
	out, err := json.Marshal(cmd)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var cmd2 Command
	if err := json.Unmarshal(out, &cmd2); err != nil {
		t.Fatalf("re-unmarshal error: %v", err)
	}
	if cmd2.UninstallCmd != "uninstall.exe" {
		t.Errorf("after round-trip UninstallCmd = %q, want %q", cmd2.UninstallCmd, "uninstall.exe")
	}
}

func TestCommandUninstallCmdOmitted(t *testing.T) {
	input := `{"id":"x","name":"X","command":"install.exe"}`
	var cmd Command
	if err := json.Unmarshal([]byte(input), &cmd); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if cmd.UninstallCmd != "" {
		t.Errorf("UninstallCmd should be empty when omitted, got %q", cmd.UninstallCmd)
	}
}
