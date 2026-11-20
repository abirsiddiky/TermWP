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
active PHP version, SSL status, live runtime state, database name, and creation timestamp.
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

		// Parallel live runtime status check for all sites
		liveStatuses := core.CheckAllSitesStatusParallel(cmd.Context(), sites, cfg, stack)

		if showJSONFlag {
			type SiteJSONOutput struct {
				Name         string `json:"name"`
				Domain       string `json:"domain"`
				PHPVersion   string `json:"php_version"`
				SSL          bool   `json:"ssl"`
				Enabled      bool   `json:"enabled"`
				Status       string `json:"status"`
				StatusDetail string `json:"status_detail,omitempty"`
				DBName       string `json:"db_name"`
				DBUser       string `json:"db_user"`
				CreatedAt    string `json:"created_at"`
			}
			type JSONOutput struct {
				Sites    []SiteJSONOutput     `json:"sites"`
				Config   *config.GlobalConfig `json:"config"`
				Services ui.ServiceStatus     `json:"services"`
			}

			jsonSites := make([]SiteJSONOutput, len(sites))
			for i, s := range sites {
				st := liveStatuses[s.Name]
				jsonSites[i] = SiteJSONOutput{
					Name:         s.Name,
					Domain:       s.Domain,
					PHPVersion:   s.PHPVersion,
					SSL:          s.SSL,
					Enabled:      s.Enabled,
					Status:       st.State,
					StatusDetail: st.Detail,
					DBName:       s.DBName,
					DBUser:       s.DBUser,
					CreatedAt:    s.CreatedAt.Format("2006-01-02 15:04"),
				}
			}

			out := JSONOutput{
				Sites:    jsonSites,
				Config:   cfg,
				Services: svcStatus,
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(out)
		}

		// Print header banner with live service states
		fmt.Println(ui.RenderServiceHeader(svcStatus))

		// Convert sites to ui.SiteRow with live status
		rows := make([]ui.SiteRow, len(sites))
		for i, s := range sites {
			st := liveStatuses[s.Name]
			rows[i] = ui.SiteRow{
				Name:         s.Name,
				Domain:       s.Domain,
				PHPVersion:   s.PHPVersion,
				SSL:          s.SSL,
				Enabled:      s.Enabled,
				Status:       st.State,
				StatusDetail: st.Detail,
				DBName:       s.DBName,
				CreatedAt:    s.CreatedAt,
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
