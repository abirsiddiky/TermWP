package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"termwp/internal/core"
	"termwp/internal/ui"
)

var cloneCmd = &cobra.Command{
	Use:   "clone <source.local> to <new.local>",
	Short: "Clone an existing WordPress site to a new domain",
	Long: `Duplicates a WordPress site completely: copies document root files,
exports and imports the database into a new schema, generates an isolated wp-config.php,
runs 'wp search-replace' to update URLs across all tables and serialized options,
creates an nginx configuration, generates new SSL certificates if applicable,
and updates the local hosts file.

The literal word 'to' is required:
  termwp clone mysite.local to mysite2.local`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cloneArgs, err := ParseCloneArgs(args)
		if err != nil {
			return err
		}

		store := core.GetStateStore()
		sourceSite, err := store.GetSite(cloneArgs.SourceSite)
		if err != nil {
			return fmt.Errorf("source site '%s' not found: %w", cloneArgs.SourceSite, err)
		}

		if _, err := store.GetSite(cloneArgs.TargetSite); err == nil {
			return fmt.Errorf("target site '%s' already exists", cloneArgs.TargetSite)
		}

		return executeCloneSite(cmd.Context(), sourceSite, cloneArgs.TargetSite)
	},
}

func executeCloneSite(ctx context.Context, source *core.Site, targetName string) error {
	tracker := ui.NewStepTracker(false, verboseFlag)
	return core.CloneSite(ctx, source, targetName, tracker)
}
