package main

import (
	"testing"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
)

func TestParseIDList(t *testing.T) {
	tests := []struct {
		name    string
		csv     string
		wantIDs []string
		wantLen int
	}{
		{
			name:    "single ID",
			csv:     "Git.Git",
			wantIDs: []string{"Git.Git"},
			wantLen: 1,
		},
		{
			name:    "comma-separated list",
			csv:     "Git.Git,Mozilla.Firefox,Spotify.Spotify",
			wantIDs: []string{"Git.Git", "Mozilla.Firefox", "Spotify.Spotify"},
			wantLen: 3,
		},
		{
			name:    "whitespace trimmed around IDs",
			csv:     " Git.Git , Mozilla.Firefox ",
			wantIDs: []string{"Git.Git", "Mozilla.Firefox"},
			wantLen: 2,
		},
		{
			name:    "empty string returns empty map",
			csv:     "",
			wantIDs: []string{},
			wantLen: 0,
		},
		{
			name:    "whitespace-only string returns empty map",
			csv:     "   ",
			wantIDs: []string{},
			wantLen: 0,
		},
		{
			name:    "trailing comma ignored",
			csv:     "Git.Git,",
			wantIDs: []string{"Git.Git"},
			wantLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseIDList(tt.csv)
			if len(got) != tt.wantLen {
				t.Fatalf("parseIDList(%q) len = %d, want %d", tt.csv, len(got), tt.wantLen)
			}
			for _, id := range tt.wantIDs {
				if !got[id] {
					t.Errorf("parseIDList(%q): expected %q in result, got %v", tt.csv, id, got)
				}
			}
		})
	}
}

func TestBuildSelectedMap(t *testing.T) {
	cfg := &config.Config{
		Packages:   []config.Package{{ID: "Git.Git"}, {ID: "Mozilla.Firefox"}},
		Commands:   []config.Command{{ID: "wsl2"}},
		Extensions: []config.Extension{{ID: "ext-vimium"}},
	}

	tests := []struct {
		name        string
		filter      map[string]bool
		exclude     map[string]bool
		wantIDs     []string
		wantMissing []string
	}{
		{
			name:    "nil filter and nil exclude returns all IDs",
			filter:  nil,
			exclude: nil,
			wantIDs: []string{"Git.Git", "Mozilla.Firefox", "wsl2", "ext-vimium"},
		},
		{
			name:        "filter applied — only matching IDs returned",
			filter:      map[string]bool{"Git.Git": true, "wsl2": true},
			exclude:     nil,
			wantIDs:     []string{"Git.Git", "wsl2"},
			wantMissing: []string{"Mozilla.Firefox", "ext-vimium"},
		},
		{
			name:        "exclude applied — excluded IDs absent",
			filter:      nil,
			exclude:     map[string]bool{"wsl2": true},
			wantIDs:     []string{"Git.Git", "Mozilla.Firefox", "ext-vimium"},
			wantMissing: []string{"wsl2"},
		},
		{
			name:        "filter and exclude both applied",
			filter:      map[string]bool{"Git.Git": true, "wsl2": true, "Mozilla.Firefox": true},
			exclude:     map[string]bool{"Mozilla.Firefox": true},
			wantIDs:     []string{"Git.Git", "wsl2"},
			wantMissing: []string{"Mozilla.Firefox", "ext-vimium"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildSelectedMap(cfg, tt.filter, tt.exclude)
			for _, id := range tt.wantIDs {
				if !got[id] {
					t.Errorf("expected %q in result, not found", id)
				}
			}
			for _, id := range tt.wantMissing {
				if got[id] {
					t.Errorf("expected %q absent from result, but it was present", id)
				}
			}
		})
	}
}
