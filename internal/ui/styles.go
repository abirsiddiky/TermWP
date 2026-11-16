package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	// NoColor can be set to true globally via --no-color flag
	NoColor = false

	// Palette definitions
	ColorPrimary   = lipgloss.Color("#7D56F4") // Indigo purple
	ColorSecondary = lipgloss.Color("#00D2D3") // Cyan
	ColorSuccess   = lipgloss.Color("#04B575") // Green
	ColorWarning   = lipgloss.Color("#FFB800") // Amber
	ColorDanger    = lipgloss.Color("#FF4757") // Coral red
	ColorMuted     = lipgloss.Color("#6B7280") // Gray
	ColorText      = lipgloss.Color("#F8F9FA") // Off-white
	ColorDarkBg    = lipgloss.Color("#1E1E2E") // Dark surface

	// Text styles
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	BoldStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorText)

	SuccessStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorSuccess)

	WarningStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorWarning)

	DangerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorDanger)

	MutedStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	HighlightStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorSecondary)

	// Status Badges
	StatusOnBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorSuccess).
			SetString("🟢 on")

	StatusOffBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorDanger).
			SetString("🔴 off")

	// Service running badges
	ServiceRunningBadge = lipgloss.NewStyle().
				Foreground(ColorSuccess).
				SetString("running")

	ServiceStoppedBadge = lipgloss.NewStyle().
				Foreground(ColorMuted).
				SetString("stopped")

	// Box / Card container style
	CardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(1, 2).
			Margin(1, 0)
)

// SetNoColor disables all colors for output.
func SetNoColor(disabled bool) {
	NoColor = disabled
	if disabled {
		TitleStyle = lipgloss.NewStyle().Bold(true)
		SubtitleStyle = lipgloss.NewStyle()
		BoldStyle = lipgloss.NewStyle().Bold(true)
		SuccessStyle = lipgloss.NewStyle()
		WarningStyle = lipgloss.NewStyle()
		DangerStyle = lipgloss.NewStyle()
		MutedStyle = lipgloss.NewStyle()
		HighlightStyle = lipgloss.NewStyle()
		CardStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2).Margin(1, 0)
		StatusOnBadge = lipgloss.NewStyle().SetString("[ON]")
		StatusOffBadge = lipgloss.NewStyle().SetString("[OFF]")
	}
}

// Icon markers
const (
	IconCheck   = "✔"
	IconWarning = "⚠"
	IconCross   = "✘"
	IconArrow   = "➜"
	IconBullet  = "•"
)

// FormatSuccess formats a line with a green checkmark
func FormatSuccess(msg string) string {
	if NoColor {
		return fmt.Sprintf("%s %s", IconCheck, msg)
	}
	return fmt.Sprintf("%s %s", SuccessStyle.Render(IconCheck), msg)
}

// FormatWarning formats a line with a warning icon
func FormatWarning(msg string) string {
	if NoColor {
		return fmt.Sprintf("%s %s", IconWarning, msg)
	}
	return fmt.Sprintf("%s %s", WarningStyle.Render(IconWarning), msg)
}

// FormatError formats a line with an error icon
func FormatError(msg string) string {
	if NoColor {
		return fmt.Sprintf("%s %s", IconCross, msg)
	}
	return fmt.Sprintf("%s %s", DangerStyle.Render(IconCross), msg)
}

// FormatInfo formats a line with an arrow icon
func FormatInfo(msg string) string {
	if NoColor {
		return fmt.Sprintf("%s %s", IconArrow, msg)
	}
	return fmt.Sprintf("%s %s", HighlightStyle.Render(IconArrow), msg)
}
