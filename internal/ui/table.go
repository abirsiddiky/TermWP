package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"termwp/internal/config"
)

// ServiceStatus holds the live state of all TermWP background daemons.
type ServiceStatus struct {
	NginxRunning   bool            `json:"nginx_running"`
	MariaDBRunning bool            `json:"mariadb_running"`
	PHPRunning     map[string]bool `json:"php_running"`
}

// SiteRow represents a site item displayed in the show table.
type SiteRow struct {
	Name       string    `json:"name"`
	Domain     string    `json:"domain"`
	PHPVersion string    `json:"php_version"`
	SSL        bool      `json:"ssl"`
	Status     string    `json:"status"` // "on" or "off"
	DBName     string    `json:"db_name"`
	CreatedAt  time.Time `json:"created_at"`
}

// RenderServiceHeader returns a styled summary banner of active services.
func RenderServiceHeader(status ServiceStatus) string {
	renderState := func(running bool) string {
		if running {
			return SuccessStyle.Render("● running")
		}
		return MutedStyle.Render("○ stopped")
	}

	var phpParts []string
	versions := []string{"8.3", "8.4", "8.5"}
	for _, v := range versions {
		running := status.PHPRunning != nil && status.PHPRunning[v]
		phpParts = append(phpParts, fmt.Sprintf("PHP %s: %s", v, renderState(running)))
	}

	header := fmt.Sprintf(
		"SERVICES: nginx: %s | MariaDB: %s | %s",
		renderState(status.NginxRunning),
		renderState(status.MariaDBRunning),
		strings.Join(phpParts, " | "),
	)

	return lipgloss.NewStyle().
		MarginBottom(1).
		Foreground(ColorMuted).
		Render(header)
}

// RenderSitesTable renders the sites table using lipgloss.
func RenderSitesTable(sites []SiteRow, cfg *config.GlobalConfig) string {
	if len(sites) == 0 {
		return MutedStyle.Render("No WordPress sites found. Create your first site with:\n  termwp add mysite.local\n")
	}

	headers := []string{"SITE NAME", "URL", "PHP", "SSL", "STATUS", "DATABASE", "CREATED"}
	rows := make([][]string, 0, len(sites))

	for _, s := range sites {
		proto := "http"
		port := cfg.HTTPPort
		if s.SSL {
			proto = "https"
			port = cfg.HTTPSPort
		}

		urlStr := fmt.Sprintf("%s://%s", proto, s.Domain)
		if (proto == "http" && port != 80) || (proto == "https" && port != 443) {
			urlStr = fmt.Sprintf("%s://%s:%d", proto, s.Domain, port)
		}

		sslStr := "No"
		if s.SSL {
			sslStr = "Yes (mkcert)"
		}

		statusStr := "🔴 off"
		if s.Status == "on" {
			statusStr = "🟢 on"
		}

		createdStr := s.CreatedAt.Format("2006-01-02 15:04")
		if s.CreatedAt.IsZero() {
			createdStr = time.Now().Format("2006-01-02")
		}

		rows = append(rows, []string{
			s.Name,
			urlStr,
			s.PHPVersion,
			sslStr,
			statusStr,
			s.DBName,
			createdStr,
		})
	}

	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(ColorPrimary)).
		Headers(headers...).
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			switch {
			case row == table.HeaderRow:
				return lipgloss.NewStyle().
					Bold(true).
					Foreground(ColorSecondary).
					Padding(0, 1)
			case col == 0:
				return lipgloss.NewStyle().
					Bold(true).
					Foreground(ColorText).
					Padding(0, 1)
			case col == 1:
				return lipgloss.NewStyle().
					Foreground(ColorSecondary).
					Padding(0, 1)
			case col == 4: // Status column
				return lipgloss.NewStyle().
					Padding(0, 1)
			default:
				return lipgloss.NewStyle().
					Foreground(ColorText).
					Padding(0, 1)
			}
		})

	return t.Render()
}
