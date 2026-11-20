package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"termwp/internal/config"
	"termwp/internal/core"
	"termwp/internal/runtime"
	"termwp/internal/ui"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start stack services and verify all enabled WordPress sites",
	Long: `Starts MariaDB, nginx, and the PHP version backends needed by all enabled sites.
Verifies each enabled site responds over HTTP/HTTPS, and prints a summary of site states.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return executeStartServices(cmd.Context())
	},
}

func executeStartServices(ctx context.Context) error {
	store := core.GetStateStore()
	enabledSites, err := store.GetEnabledSites()
	if err != nil {
		return fmt.Errorf("failed to query site state: %w", err)
	}

	if len(enabledSites) == 0 {
		fmt.Println(ui.FormatInfo("No enabled sites configured. Start a site with: termwp on <site.local>"))
		return nil
	}

	tracker := ui.NewStepTracker(false, verboseFlag)
	tracker.Start("Starting stack services for enabled sites")

	// Clear shutdown flag when started
	_ = store.SetShutdown(false)

	stack, err := runtime.GetStackManager()
	if err != nil {
		tracker.Fail("Failed initializing stack")
		return err
	}

	cfg, _ := config.LoadConfig()

	activeVersions, _ := store.GetActivePHPVersions()
	if err := stack.EnsureStackRunning(activeVersions); err != nil {
		tracker.Fail("Failed starting stack services")
		return err
	}

	tracker.Success("All required stack services are running.")

	// Verify each enabled site responds
	statuses := core.CheckAllSitesStatusParallel(ctx, enabledSites, cfg, stack)

	fmt.Println("\nSite status:")
	for _, s := range enabledSites {
		st := statuses[s.Name]
		proto := "http"
		port := cfg.HTTPPort
		if s.SSL {
			proto = "https"
			port = cfg.HTTPSPort
		}
		urlStr := fmt.Sprintf("%s://%s", proto, s.Domain)
		if (!s.SSL && port != 80) || (s.SSL && port != 443) {
			urlStr = fmt.Sprintf("%s://%s:%d", proto, s.Domain, port)
		}

		formattedStatus := ui.FormatSiteStatus(st.State, st.Detail)
		fmt.Printf("  • %-20s (%s) -> %s\n", s.Name, urlStr, formattedStatus)
	}
	fmt.Println()

	return nil
}
