package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"termwp/internal/core"
	"termwp/internal/runtime"
	"termwp/internal/ui"
)

var startCmd = &cobra.Command{
	Use:    "start",
	Short:  "Start all stack services needed for active sites",
	Hidden: true, // Hidden utility command per requirements
	Long: `Starts nginx, MariaDB, and required PHP backends for all currently active ('on') sites.
Typically invoked automatically by commands like add, on, clone, or doctor, but can be called manually.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return executeStartServices()
	},
}

func executeStartServices() error {
	store := core.GetStateStore()
	hasActive, err := store.HasActiveSites()
	if err != nil {
		return fmt.Errorf("failed to query site state: %w", err)
	}

	if !hasActive {
		fmt.Println(ui.FormatInfo("No active sites configured. Services not needed."))
		return nil
	}

	tracker := ui.NewStepTracker(false, verboseFlag)
	tracker.Start("Starting stack services for active sites")

	// Clear shutdown flag when started
	_ = store.SetShutdown(false)

	stack, err := runtime.GetStackManager()
	if err != nil {
		tracker.Fail("Failed initializing stack")
		return err
	}

	activeVersions, _ := store.GetActivePHPVersions()
	if err := stack.EnsureStackRunning(activeVersions); err != nil {
		tracker.Fail("Failed starting stack services")
		return err
	}

	tracker.Success("All required stack services are running.")
	return nil
}
