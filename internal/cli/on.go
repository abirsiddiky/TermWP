package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"termwp/internal/core"
	"termwp/internal/ui"
)

var onCmd = &cobra.Command{
	Use:   "on <site.local>",
	Short: "Turn on an existing site and start its backend services",
	Long: `Enables the nginx virtual host configuration, re-adds the local hosts entry,
starts required PHP and MariaDB background services, and reloads nginx.
Idempotent: turning on an already active site simply confirms its status.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		siteName := strings.ToLower(args[0])
		if err := ValidateSiteName(siteName); err != nil {
			return err
		}

		store := core.GetStateStore()
		site, err := store.GetSite(siteName)
		if err != nil {
			return fmt.Errorf("site '%s' not found", siteName)
		}

		if site.Status == core.StatusOn {
			fmt.Println(ui.FormatInfo(fmt.Sprintf("Site '%s' is already on.", siteName)))
			return nil
		}

		return executeTurnOnSite(cmd.Context(), site)
	},
}

func executeTurnOnSite(ctx context.Context, site *core.Site) error {
	tracker := ui.NewStepTracker(false, verboseFlag)
	return core.TurnOnSite(ctx, site, tracker)
}
