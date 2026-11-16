package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"termwp/internal/config"
	"termwp/internal/ui"
)

var (
	// Version is injected at build time via ldflags
	Version   = "1.0.0"
	GitCommit = "dev"
	BuildDate = "unknown"

	verboseFlag bool
	noColorFlag bool
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "termwp",
	Short: "TermWP - Terminal-based isolated WordPress development stack",
	Long: `TermWP is a complete, production-grade cross-platform CLI/TUI application
that manages local WordPress development sites with an isolated portable stack:
nginx, multi-version PHP (8.3/8.4/8.5), MariaDB, WP-CLI, and mkcert SSL.`,
	Version: Version,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if noColorFlag {
			ui.SetNoColor(true)
		}
		// Ensure base directory tree exists on every invocation
		_ = config.EnsureDirectories()
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command and exits on error.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "\n%s %v\n\n", ui.DangerStyle.Render(ui.IconCross), err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&verboseFlag, "verbose", false, "Enable verbose output with detailed logs")
	rootCmd.PersistentFlags().BoolVar(&noColorFlag, "no-color", false, "Disable colored terminal output")

	// Custom version template
	rootCmd.SetVersionTemplate(fmt.Sprintf("TermWP v%s (commit: %s, built: %s)\n", Version, GitCommit, BuildDate))

	// Register subcommands
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(delCmd)
	rootCmd.AddCommand(onCmd)
	rootCmd.AddCommand(offCmd)
	rootCmd.AddCommand(showCmd)
	rootCmd.AddCommand(cloneCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(shutdownCmd)
	rootCmd.AddCommand(startCmd)
}
