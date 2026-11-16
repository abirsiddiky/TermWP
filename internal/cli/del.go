package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"termwp/internal/core"
	"termwp/internal/ui"
)

var (
	delYesFlag bool
)

var delCmd = &cobra.Command{
	Use:   "del <site.local>",
	Short: "Delete a WordPress site and its database completely",
	Long: `Permanently deletes a WordPress site, including document root files,
MariaDB database and database user, nginx virtual host configuration,
SSL certificate, and local hosts file entry.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		siteName := strings.ToLower(args[0])
		if err := ValidateSiteName(siteName); err != nil {
			return err
		}

		store := core.GetStateStore()
		site, err := store.GetSite(siteName)
		if err != nil {
			return fmt.Errorf("site '%s' does not exist in registry", siteName)
		}

		if !delYesFlag {
			fmt.Printf("%s This will permanently delete site files, database (%s), and certificates.\n",
				ui.WarningStyle.Render(ui.IconWarning), site.DBName)
			fmt.Printf("Type '%s' to confirm deletion: ", siteName)

			reader := bufio.NewReader(os.Stdin)
			confirmation, _ := reader.ReadString('\n')
			confirmation = strings.TrimSpace(confirmation)

			if confirmation != siteName {
				fmt.Printf("%s Deletion canceled.\n", ui.MutedStyle.Render(ui.IconBullet))
				return nil
			}
		}

		return executeDeleteSite(cmd.Context(), siteName)
	},
}

func init() {
	delCmd.Flags().BoolVarP(&delYesFlag, "yes", "y", false, "Confirm deletion without interactive prompt")
}

func executeDeleteSite(ctx context.Context, siteName string) error {
	tracker := ui.NewStepTracker(false, verboseFlag)
	return core.DeleteSite(ctx, siteName, tracker)
}
