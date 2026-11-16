package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"termwp/internal/core"
	"termwp/internal/runtime"
)

var (
	shutdownForceFlag bool
	shutdownQuietFlag bool
)

var shutdownCmd = &cobra.Command{
	Use:   "shutdown",
	Short: "Gracefully stop all TermWP background stack services",
	Long: `Stops all running background services (nginx, PHP backends, and MariaDB) in proper order:
  1. nginx (waits up to 5s, then force kills)
  2. Running PHP backends (8.3, 8.4, 8.5)
  3. MariaDB (flushes buffers via mariadb-admin shutdown, waits up to 15s)

Site status in state.json is preserved (sites marked 'on' stay 'on').
The next command requiring the stack will restart all necessary services automatically.

Flags:
  termwp shutdown
  termwp shutdown --force
  termwp shutdown --quiet`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return executeShutdown(shutdownForceFlag, shutdownQuietFlag)
	},
}

func init() {
	shutdownCmd.Flags().BoolVar(&shutdownForceFlag, "force", false, "Force kill services immediately without graceful stop")
	shutdownCmd.Flags().BoolVarP(&shutdownQuietFlag, "quiet", "q", false, "Suppress output except errors")
}

func executeShutdown(force, quiet bool) error {
	store := core.GetStateStore()
	// Record explicit shutdown flag in state
	_ = store.SetShutdown(true)

	stack, err := runtime.GetStackManager()
	if err != nil {
		return fmt.Errorf("failed to initialize stack manager: %w", err)
	}

	return stack.ShutdownStack(force, quiet)
}
