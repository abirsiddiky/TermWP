package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"termwp/internal/config"
	"termwp/internal/core"
	"termwp/internal/system"
	"termwp/internal/ui"
)

var (
	logsFollowFlag bool
	logsLinesFlag  int
	logsPHPFlag    bool
	logsNginxFlag  bool
	logsWPFlag     bool
)

var logsCmd = &cobra.Command{
	Use:   "logs <site.local>",
	Short: "Stream or view aggregated logs for a WordPress site",
	Long: `Displays aggregated or specific logs for a WordPress site:
- WordPress debug.log (prompts to enable WP_DEBUG_LOG if inactive)
- PHP error log for the site's PHP version
- Nginx access and error logs dedicated to this site

Features pure Go live-tailing (--follow) compatible with Windows, macOS, and Linux:
  termwp logs mysite.local
  termwp logs mysite.local -f
  termwp logs mysite.local --nginx
  termwp logs mysite.local --php
  termwp logs mysite.local --wp -n 100`,
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

		return executeStreamLogs(cmd.Context(), site, logsFollowFlag, logsLinesFlag, logsPHPFlag, logsNginxFlag, logsWPFlag)
	},
}

func init() {
	logsCmd.Flags().BoolVarP(&logsFollowFlag, "follow", "f", false, "Stream new log lines continuously")
	logsCmd.Flags().IntVarP(&logsLinesFlag, "lines", "n", 50, "Number of initial lines to display")
	logsCmd.Flags().BoolVar(&logsPHPFlag, "php", false, "Display only PHP logs")
	logsCmd.Flags().BoolVar(&logsNginxFlag, "nginx", false, "Display only Nginx logs")
	logsCmd.Flags().BoolVar(&logsWPFlag, "wp", false, "Display only WordPress debug logs")
}

func executeStreamLogs(ctx context.Context, site *core.Site, follow bool, lines int, phpOnly, nginxOnly, wpOnly bool) error {
	siteDir := config.SiteDir(site.Name)
	wpLogPath := filepath.Join(siteDir, "wp-content", "debug.log")
	phpLogPath := filepath.Join(config.LogsDir(), fmt.Sprintf("php_%s_error.log", site.PHPVersion))
	nginxAccessLog := filepath.Join(config.LogsDir(), fmt.Sprintf("%s_access.log", site.Name))
	nginxErrorLog := filepath.Join(config.LogsDir(), fmt.Sprintf("%s_error.log", site.Name))

	// If WordPress logs are requested (or default all) and debug log is not enabled, enable it
	if (wpOnly || (!phpOnly && !nginxOnly)) && !system.IsWPDebugLogEnabled(siteDir) {
		fmt.Println(ui.FormatInfo("Enabling WP_DEBUG_LOG in wp-config.php for debug logging..."))
		_ = system.EnableWPDebugLog(siteDir)
	}

	var targets []system.LogTarget

	includeAll := !phpOnly && !nginxOnly && !wpOnly

	if includeAll || nginxOnly {
		targets = append(targets, system.LogTarget{
			Path:  nginxAccessLog,
			Label: "nginx-access",
			Style: lipgloss.NewStyle().Foreground(ui.ColorSecondary).Bold(true),
		})
		targets = append(targets, system.LogTarget{
			Path:  nginxErrorLog,
			Label: "nginx-error",
			Style: lipgloss.NewStyle().Foreground(ui.ColorDanger).Bold(true),
		})
	}

	if includeAll || phpOnly {
		targets = append(targets, system.LogTarget{
			Path:  phpLogPath,
			Label: fmt.Sprintf("php-%s", site.PHPVersion),
			Style: lipgloss.NewStyle().Foreground(ui.ColorWarning).Bold(true),
		})
	}

	if includeAll || wpOnly {
		targets = append(targets, system.LogTarget{
			Path:  wpLogPath,
			Label: "wp-debug",
			Style: lipgloss.NewStyle().Foreground(ui.ColorSuccess).Bold(true),
		})
	}

	fmt.Printf("%s Monitoring logs for '%s' (PHP %s, %d sources)...\n\n",
		ui.HighlightStyle.Render(ui.IconArrow), site.Name, site.PHPVersion, len(targets))

	return system.StreamLogs(ctx, targets, lines, follow)
}
