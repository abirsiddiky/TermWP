package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"termwp/internal/core"
	"termwp/internal/ui"
)

var (
	updatePluginsFlag bool
	updateThemesFlag  bool
	updateAllFlag     bool
)

var updateCmd = &cobra.Command{
	Use:   "update <site.local>",
	Short: "Update WordPress core, plugins, and themes safely",
	Long: `Updates WordPress core to the latest release and applies database migrations.
Automatically creates a safety database backup in data/backups/<site>/<timestamp>.sql
before applying any updates.

Flags allow updating plugins, themes, or everything simultaneously:
  termwp update mysite.local
  termwp update mysite.local --plugins
  termwp update mysite.local --themes
  termwp update mysite.local --all`,
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

		return executeUpdateSite(cmd.Context(), site, updatePluginsFlag, updateThemesFlag, updateAllFlag)
	},
}

func init() {
	updateCmd.Flags().BoolVar(&updatePluginsFlag, "plugins", false, "Update installed WordPress plugins")
	updateCmd.Flags().BoolVar(&updateThemesFlag, "themes", false, "Update installed WordPress themes")
	updateCmd.Flags().BoolVar(&updateAllFlag, "all", false, "Update WordPress core, all plugins, and all themes")
}

func executeUpdateSite(ctx context.Context, site *core.Site, plugins, themes, all bool) error {
	tracker := ui.NewStepTracker(false, verboseFlag)
	return core.UpdateSite(ctx, site, plugins, themes, all, tracker)
}
