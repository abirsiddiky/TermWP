package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"termwp/internal/config"
	"termwp/internal/core"
	"termwp/internal/runtime"
	"termwp/internal/ui"
)

var (
	showJSONFlag bool
)

var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Display all registered WordPress sites and stack services",
	Long: `Prints a styled table of all registered WordPress sites, including their domains,
active PHP version, SSL status, running state, database name, and creation timestamp.
Also displays a top banner with the live running status of background services.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		store := core.GetStateStore()
		sites, err := store.ListSites()
		if err != nil {
			return fmt.Errorf("failed to retrieve sites: %w", err)
		}

		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		stack, _ := runtime.GetStackManager()
		var svcStatus ui.ServiceStatus
		if stack != nil {
			svcStatus = stack.CheckServices()
		}

		if showJSONFlag {
			type JSONOutput struct {
				Sites    []*core.Site         `json:"sites"`
				Config   *config.GlobalConfig `json:"config"`
				Services ui.ServiceStatus     `json:"services"`
			}
			out := JSONOutput{
				Sites:    sites,
				Config:   cfg,
				Services: svcStatus,
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(out)
		}

		// Print header banner with live service states
		fmt.Println(ui.RenderServiceHeader(svcStatus))

		// Convert sites to ui.SiteRow
		rows := make([]ui.SiteRow, len(sites))
		for i, s := range sites {
			rows[i] = ui.SiteRow{
				Name:       s.Name,
				Domain:     s.Domain,
				PHPVersion: s.PHPVersion,
				SSL:        s.SSL,
				Status:     s.Status,
				DBName:     s.DBName,
				CreatedAt:  s.CreatedAt,
			}
		}

		// Print sites table
		fmt.Println(ui.RenderSitesTable(rows, cfg))
		return nil
	},
}

func init() {
	showCmd.Flags().BoolVar(&showJSONFlag, "json", false, "Output site list and service status in JSON format")
}
