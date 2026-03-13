package config

import (
	"testing"
)

func cfgBase(version, metaName string) *Config {
	return &Config{Version: version, Metadata: Metadata{Name: metaName}}
}

func TestValidate_MissingVersion(t *testing.T) {
	errs := Validate(cfgBase("", "MyKit"))
	if len(errs) == 0 {
		t.Fatal("want error for missing version, got none")
	}
	found := false
	for _, e := range errs {
		if e.Field == "[top-level]" {
			found = true
		}
	}
	if !found {
		t.Errorf("want error with Field=[top-level], got %+v", errs)
	}
}

func TestValidate_MissingMetadataName(t *testing.T) {
	errs := Validate(cfgBase("1.0", ""))
	if len(errs) == 0 {
		t.Fatal("want error for missing metadata.name, got none")
	}
}

func TestValidate_CleanConfig(t *testing.T) {
	c := cfgBase("1.0", "MyKit")
	errs := Validate(c)
	if len(errs) != 0 {
		t.Errorf("clean config: want 0 errors, got %+v", errs)
	}
}

func TestValidate_PackageMissingName(t *testing.T) {
	c := cfgBase("1.0", "MyKit")
	c.Packages = []Package{{ID: "p1", Phase: 1}}
	errs := Validate(c)
	if len(errs) == 0 {
		t.Fatal("want error for missing package name")
	}
}

func TestValidate_PackageBadPhase(t *testing.T) {
	c := cfgBase("1.0", "MyKit")
	c.Packages = []Package{{ID: "p1", Name: "P1", Phase: 0}}
	errs := Validate(c)
	if len(errs) == 0 {
		t.Fatal("want error for phase < 1")
	}
}

func TestValidate_DuplicateIDCrossPackageCommand(t *testing.T) {
	c := cfgBase("1.0", "MyKit")
	c.Packages = []Package{{ID: "dup", Name: "Dup", Phase: 1}}
	c.Commands = []Command{{ID: "dup", Name: "Dup", Phase: 1, Check: "x", Cmd: "y"}}
	errs := Validate(c)
	if len(errs) == 0 {
		t.Fatal("want error for duplicate ID across tiers")
	}
}

func TestValidate_CommandMissingCheck(t *testing.T) {
	c := cfgBase("1.0", "MyKit")
	c.Commands = []Command{{ID: "c1", Name: "C1", Phase: 1, Cmd: "echo hi"}}
	errs := Validate(c)
	if len(errs) == 0 {
		t.Fatal("want error for missing check")
	}
}

func TestValidate_CommandMissingCmd(t *testing.T) {
	c := cfgBase("1.0", "MyKit")
	c.Commands = []Command{{ID: "c1", Name: "C1", Phase: 1, Check: "echo"}}
	errs := Validate(c)
	if len(errs) == 0 {
		t.Fatal("want error for missing command")
	}
}

func TestValidate_ExtensionBadExtensionID(t *testing.T) {
	c := cfgBase("1.0", "MyKit")
	c.Extensions = []Extension{{ID: "e1", Name: "E1", Phase: 1, ExtensionID: "tooshort"}}
	errs := Validate(c)
	if len(errs) == 0 {
		t.Fatal("want error for extension_id != 32 chars")
	}
}

func TestValidate_DependsOnUnknownID(t *testing.T) {
	c := cfgBase("1.0", "MyKit")
	c.Commands = []Command{{
		ID: "c1", Name: "C1", Phase: 1, Check: "x", Cmd: "y",
		DependsOn: []string{"does-not-exist"},
	}}
	errs := Validate(c)
	if len(errs) == 0 {
		t.Fatal("want error for unknown depends_on ID")
	}
}

func TestValidate_DependsOnKnownID(t *testing.T) {
	c := cfgBase("1.0", "MyKit")
	c.Packages = []Package{{ID: "git", Name: "Git", Phase: 1}}
	c.Commands = []Command{{
		ID: "c1", Name: "C1", Phase: 1, Check: "x", Cmd: "y",
		DependsOn: []string{"git"},
	}}
	errs := Validate(c)
	if len(errs) != 0 {
		t.Errorf("want 0 errors for valid depends_on, got %+v", errs)
	}
}

func TestValidate_ProfileUnknownID(t *testing.T) {
	c := cfgBase("1.0", "MyKit")
	c.Profiles = []Profile{{Name: "Dev", IDs: []string{"ghost-id"}}}
	errs := Validate(c)
	if len(errs) == 0 {
		t.Fatal("want error for unknown profile ID")
	}
}

func TestValidate_ProfileKnownID(t *testing.T) {
	c := cfgBase("1.0", "MyKit")
	c.Packages = []Package{{ID: "git", Name: "Git", Phase: 1}}
	c.Profiles = []Profile{{Name: "Dev", IDs: []string{"git"}}}
	errs := Validate(c)
	if len(errs) != 0 {
		t.Errorf("want 0 errors for valid profile ID, got %+v", errs)
	}
}

func TestValidate_CollectsAllErrors(t *testing.T) {
	c := &Config{}
	errs := Validate(c)
	if len(errs) < 2 {
		t.Errorf("want >= 2 errors for empty config, got %d: %+v", len(errs), errs)
	}
}

func TestValidate_CommandMissingName(t *testing.T) {
	c := cfgBase("1.0", "MyKit")
	c.Commands = []Command{{ID: "c1", Phase: 1, Check: "x", Cmd: "y"}}
	errs := Validate(c)
	if len(errs) == 0 {
		t.Fatal("want error for missing command name")
	}
}

func TestValidate_CommandBadPhase(t *testing.T) {
	c := cfgBase("1.0", "MyKit")
	c.Commands = []Command{{ID: "c1", Name: "C1", Phase: 0, Check: "x", Cmd: "y"}}
	errs := Validate(c)
	if len(errs) == 0 {
		t.Fatal("want error for command phase < 1")
	}
}

func TestValidate_ExtensionMissingName(t *testing.T) {
	c := cfgBase("1.0", "MyKit")
	c.Extensions = []Extension{{ID: "e1", Phase: 1, ExtensionID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}
	errs := Validate(c)
	if len(errs) == 0 {
		t.Fatal("want error for missing extension name")
	}
}

func TestValidate_ExtensionBadPhase(t *testing.T) {
	c := cfgBase("1.0", "MyKit")
	c.Extensions = []Extension{{ID: "e1", Name: "E1", Phase: 0, ExtensionID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}
	errs := Validate(c)
	if len(errs) == 0 {
		t.Fatal("want error for extension phase < 1")
	}
}

func TestValidate_ExtensionEmptyExtensionID(t *testing.T) {
	c := cfgBase("1.0", "MyKit")
	c.Extensions = []Extension{{ID: "e1", Name: "E1", Phase: 1, ExtensionID: ""}}
	errs := Validate(c)
	if len(errs) == 0 {
		t.Fatal("want error for empty extension_id")
	}
}

// --- Scrape-type command validation ---

func validScrapeCmd() Command {
	return Command{
		ID:         "tool",
		Name:       "Tool",
		Phase:      1,
		Check:      "echo skip",
		ScrapeURL:  "https://example.com/download",
		URLPattern: `https://example\.com/files/tool-[\d]+\.exe`,
	}
}

func TestValidate_ScrapeCmd_Valid(t *testing.T) {
	c := cfgBase("1.0", "MyKit")
	c.Commands = []Command{validScrapeCmd()}
	errs := Validate(c)
	if len(errs) != 0 {
		t.Errorf("valid scrape command: want 0 errors, got %+v", errs)
	}
}

func TestValidate_ScrapeCmd_ValidWithInstallArgs(t *testing.T) {
	c := cfgBase("1.0", "MyKit")
	cmd := validScrapeCmd()
	cmd.InstallArgs = "/S"
	c.Commands = []Command{cmd}
	errs := Validate(c)
	if len(errs) != 0 {
		t.Errorf("scrape command with install_args: want 0 errors, got %+v", errs)
	}
}

func TestValidate_ScrapeCmd_MissingBoth(t *testing.T) {
	c := cfgBase("1.0", "MyKit")
	c.Commands = []Command{{ID: "c1", Name: "C1", Phase: 1, Check: "echo skip"}}
	errs := Validate(c)
	if len(errs) == 0 {
		t.Fatal("want error when neither command nor scrape_url+url_pattern is set")
	}
}

func TestValidate_ScrapeCmd_HasBothCmdAndScrape(t *testing.T) {
	c := cfgBase("1.0", "MyKit")
	cmd := validScrapeCmd()
	cmd.Cmd = "echo hi"
	c.Commands = []Command{cmd}
	errs := Validate(c)
	if len(errs) == 0 {
		t.Fatal("want error when both command and scrape_url are set")
	}
}

func TestValidate_ScrapeCmd_MissingScrapeURL(t *testing.T) {
	c := cfgBase("1.0", "MyKit")
	c.Commands = []Command{{
		ID: "c1", Name: "C1", Phase: 1, Check: "echo skip",
		URLPattern: `https://example\.com/files/tool\.exe`,
	}}
	errs := Validate(c)
	if len(errs) == 0 {
		t.Fatal("want error when url_pattern is set but scrape_url is missing")
	}
}

func TestValidate_ScrapeCmd_MissingURLPattern(t *testing.T) {
	c := cfgBase("1.0", "MyKit")
	c.Commands = []Command{{
		ID: "c1", Name: "C1", Phase: 1, Check: "echo skip",
		ScrapeURL: "https://example.com/download",
	}}
	errs := Validate(c)
	if len(errs) == 0 {
		t.Fatal("want error when scrape_url is set but url_pattern is missing")
	}
}
