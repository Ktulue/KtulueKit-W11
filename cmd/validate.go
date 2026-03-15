package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
)

func runValidate(cmd *cobra.Command, args []string) error {
	resolved, cleanup, err := resolveConfigPaths(configPaths)
	if err != nil {
		return err
	}
	defer cleanup()

	cfg, err := config.LoadAll(resolved)
	if err != nil {
		return fmt.Errorf("config parse error: %w", err)
	}

	displayPaths := configPaths
	if len(displayPaths) == 0 {
		displayPaths = []string{config.DefaultConfigPath}
	}
	fmt.Printf("Validating config: %v\n", displayPaths)

	errs := config.Validate(cfg)
	if len(errs) == 0 {
		total := len(cfg.Packages) + len(cfg.Commands) + len(cfg.Extensions)
		fmt.Printf("  OK — no errors found (%d packages + %d commands + %d extensions = %d items validated)\n",
			len(cfg.Packages), len(cfg.Commands), len(cfg.Extensions), total)
		return nil
	}

	for _, e := range errs {
		fmt.Printf("  %sERROR%s  %-30s  %s\n", colorRed, colorReset, e.Field, e.Message)
	}
	return fmt.Errorf("%d error(s) found — fix the above before running", len(errs))
}
