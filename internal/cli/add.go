package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"termwp/internal/config"
	"termwp/internal/core"
	"termwp/internal/ui"
)

var (
	addTitleFlag string
	addEmailFlag string
	addUserFlag  string
	addPassFlag  string
	addYesFlag   bool
)

var addCmd = &cobra.Command{
	Use:   "add <site.local> [php-version] [ssl]",
	Short: "Create a new WordPress site or add SSL to an existing site",
	Long: `Creates a new local WordPress development site with isolated database,
nginx virtual host, local hosts entry, and optional mkcert SSL.

Syntax variants:
  termwp add mysite.local
  termwp add mysite.local php8.3
  termwp add mysite.local ssl
  termwp add mysite.local php8.4 ssl
  termwp add mysite.local ssl php8.5
  termwp add ssl mysite.local (adds SSL to an existing site)`,
	DisableFlagParsing: false,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		addArgs, err := ParseAddArgs(args, cfg.DefaultPHP)
		if err != nil {
			return err
		}

		if addArgs.IsAddExisting {
			// Sub-workflow: termwp add ssl <site.local>
			return executeAddSSL(cmd.Context(), addArgs.SiteName)
		}

		// Sub-workflow: termwp add <site.local> [php] [ssl]
		return executeCreateSite(cmd.Context(), addArgs)
	},
}

func init() {
	addCmd.Flags().StringVar(&addTitleFlag, "title", "", "WordPress Site Title")
	addCmd.Flags().StringVar(&addEmailFlag, "email", "admin@example.local", "WordPress Admin Email")
	addCmd.Flags().StringVar(&addUserFlag, "user", "admin", "WordPress Admin Username")
	addCmd.Flags().StringVar(&addPassFlag, "pass", "", "WordPress Admin Password (auto-generated if empty)")
	addCmd.Flags().BoolVarP(&addYesFlag, "yes", "y", false, "Skip interactive prompts and accept defaults")
}

// executeAddSSL is the handler for `termwp add ssl <site>`
func executeAddSSL(ctx context.Context, siteName string) error {
	tracker := ui.NewStepTracker(false, verboseFlag)
	return core.AddSSLToSite(ctx, siteName, tracker)
}

// executeCreateSite is the handler for new site creation
func executeCreateSite(ctx context.Context, args *AddArgs) error {
	promptRes, err := ui.PromptSiteDetails(args.SiteName, addTitleFlag, addEmailFlag, addUserFlag, addPassFlag, addYesFlag)
	if err != nil {
		return err
	}

	tracker := ui.NewStepTracker(false, verboseFlag)

	params := core.CreateSiteParams{
		SiteName:   args.SiteName,
		PHPVersion: args.PHPVersion,
		SSL:        args.SSL,
		Title:      promptRes.Title,
		Email:      promptRes.Email,
		User:       promptRes.User,
		Password:   promptRes.Password,
		SkipWait:   addYesFlag,
	}

	return core.CreateSite(ctx, params, tracker)
}
