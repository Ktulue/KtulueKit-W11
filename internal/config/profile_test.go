package config

import "testing"

func TestLookupProfile_Found(t *testing.T) {
	cfg := &Config{
		Profiles: []Profile{
			{Name: "streaming", IDs: []string{"OBSProject.OBSStudio", "Spotify.Spotify"}},
			{Name: "dev", IDs: []string{"Git.Git", "Microsoft.VisualStudioCode"}},
		},
	}

	ids, err := LookupProfile(cfg, "streaming")
	if err != nil {
		t.Fatalf("LookupProfile returned unexpected error: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("want 2 IDs, got %d", len(ids))
	}
	if ids[0] != "OBSProject.OBSStudio" {
		t.Errorf("want first ID 'OBSProject.OBSStudio', got %q", ids[0])
	}
}

func TestLookupProfile_NotFound(t *testing.T) {
	cfg := &Config{
		Profiles: []Profile{
			{Name: "streaming", IDs: []string{"OBSProject.OBSStudio"}},
		},
	}

	_, err := LookupProfile(cfg, "Streaming") // wrong case
	if err == nil {
		t.Fatal("want error for unknown profile name, got nil")
	}
}

func TestLookupProfile_EmptyProfiles(t *testing.T) {
	cfg := &Config{}
	_, err := LookupProfile(cfg, "anything")
	if err == nil {
		t.Fatal("want error for empty profiles list, got nil")
	}
}

func TestLookupProfile_CaseSensitive(t *testing.T) {
	cfg := &Config{
		Profiles: []Profile{
			{Name: "Dev", IDs: []string{"Git.Git"}},
		},
	}
	// "Dev" exists but "dev" (lowercase) should not match.
	_, err := LookupProfile(cfg, "dev")
	if err == nil {
		t.Fatal("want error: profile lookup is case-sensitive")
	}
	// Exact case must work.
	ids, err := LookupProfile(cfg, "Dev")
	if err != nil {
		t.Fatalf("want no error for exact case match, got %v", err)
	}
	if len(ids) != 1 {
		t.Errorf("want 1 ID, got %d", len(ids))
	}
}
