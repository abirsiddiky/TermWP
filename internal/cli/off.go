package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"termwp/internal/core"
	"termwp/internal/ui"
)

var offCmd = &cobra.Command{
	Use:   "off <site.local>",
	Short: "Turn off a site and disable its nginx routing",
	Long: `Disables the site's nginx virtual host, removes its entry from the hosts file,
and marks the site status as 'off' in state.json. Files and databases remain untouched.
If no other active site requires a running PHP version or stack daemon, unused services can shut down.`,
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

		if site.Status == core.StatusOff {
			fmt.Println(ui.FormatInfo(fmt.Sprintf("Site '%s' is already off.", siteName)))
			return nil
		}

		return executeTurnOffSite(cmd.Context(), site)
	},
}

func executeTurnOffSite(ctx context.Context, site *core.Site) error {
	tracker := ui.NewStepTracker(false, verboseFlag)
	return core.TurnOffSite(ctx, site, tracker)
}
