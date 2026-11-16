package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/lipgloss"
)

// SiteSummaryData holds data to be displayed in the summary card.
type SiteSummaryData struct {
	SiteName      string
	SiteURL       string
	AdminURL      string
	AdminUsername string
	AdminPassword string
	AdminEmail    string
	PHPVersion    string
	SSL           bool
	DatabaseName  string
	DatabaseUser  string
	SitePath      string
}

// RenderSummaryCard generates the styled string representation of the summary card.
func RenderSummaryCard(data SiteSummaryData) string {
	width := 64

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		Align(lipgloss.Center).
		Width(width)

	sectionHeaderStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorSecondary).
		MarginTop(1)

	labelStyle := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Width(18)

	valStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorText)

	linkStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorSecondary).
		Underline(true)

	sslLabel := "Disabled (HTTP)"
	if data.SSL {
		sslLabel = "Enabled (HTTPS - mkcert local CA)"
	}

	var sb strings.Builder

	// Title
	sb.WriteString(titleStyle.Render("⚡ TermWP WordPress Site Ready") + "\n")
	sb.WriteString(lipgloss.NewStyle().Foreground(ColorMuted).Align(lipgloss.Center).Width(width).Render("Your isolated local environment is running") + "\n")

	// Section: Access
	sb.WriteString(sectionHeaderStyle.Render("🌐 Web Access") + "\n")
	sb.WriteString(fmt.Sprintf("  %s %s\n", labelStyle.Render("Site URL:"), linkStyle.Render(data.SiteURL)))
	sb.WriteString(fmt.Sprintf("  %s %s\n", labelStyle.Render("Admin Dashboard:"), linkStyle.Render(data.AdminURL)))

	// Section: Admin
	sb.WriteString(sectionHeaderStyle.Render("🔑 Administrator Credentials") + "\n")
	sb.WriteString(fmt.Sprintf("  %s %s\n", labelStyle.Render("Username:"), valStyle.Render(data.AdminUsername)))
	sb.WriteString(fmt.Sprintf("  %s %s\n", labelStyle.Render("Password:"), SuccessStyle.Render(data.AdminPassword)))
	if data.AdminEmail != "" {
		sb.WriteString(fmt.Sprintf("  %s %s\n", labelStyle.Render("Email:"), valStyle.Render(data.AdminEmail)))
	}

	// Section: Stack & Storage
	sb.WriteString(sectionHeaderStyle.Render("⚙️ Environment & Database") + "\n")
	sb.WriteString(fmt.Sprintf("  %s %s\n", labelStyle.Render("PHP Version:"), valStyle.Render(data.PHPVersion)))
	sb.WriteString(fmt.Sprintf("  %s %s\n", labelStyle.Render("SSL:"), valStyle.Render(sslLabel)))
	sb.WriteString(fmt.Sprintf("  %s %s\n", labelStyle.Render("Database:"), valStyle.Render(data.DatabaseName)))
	sb.WriteString(fmt.Sprintf("  %s %s\n", labelStyle.Render("Document Root:"), MutedStyle.Render(data.SitePath)))

	content := sb.String()

	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(1, 2).
		Width(width + 4).
		Render(content)

	return card
}

// ShowSummaryCardInteractive prints the summary card and waits for user interaction.
func ShowSummaryCardInteractive(data SiteSummaryData, skipWait bool) {
	fmt.Println(RenderSummaryCard(data))

	if skipWait {
		return
	}

	canCopy := !clipboard.Unsupported
	var promptMsg string
	if canCopy {
		promptMsg = lipgloss.NewStyle().Foreground(ColorMuted).Render("Press [c] to copy credentials to clipboard, or [Enter] to exit: ")
	} else {
		promptMsg = lipgloss.NewStyle().Foreground(ColorMuted).Render("Press [Enter] to exit...")
	}

	fmt.Print(promptMsg)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if canCopy && (input == "c" || input == "C") {
		creds := fmt.Sprintf(
			"Site URL: %s\nAdmin URL: %s\nUsername: %s\nPassword: %s\n",
			data.SiteURL,
			data.AdminURL,
			data.AdminUsername,
			data.AdminPassword,
		)
		if err := clipboard.WriteAll(creds); err == nil {
			fmt.Printf("  %s Credentials copied to clipboard!\n", SuccessStyle.Render(IconCheck))
		} else {
			fmt.Printf("  %s Could not copy to clipboard.\n", WarningStyle.Render(IconWarning))
		}
	}
}
