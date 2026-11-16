package ui

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/huh"
	"termwp/internal/config"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

// SitePromptResult holds the collected WordPress install details.
type SitePromptResult struct {
	Title    string
	Email    string
	User     string
	Password string
}

// ValidateEmail verifies email address format.
func ValidateEmail(email string) error {
	trimmed := strings.TrimSpace(email)
	if trimmed == "" {
		return errors.New("email address is required")
	}
	if !emailRegex.MatchString(trimmed) {
		return errors.New("please enter a valid email address (e.g. admin@example.com)")
	}
	return nil
}

// PromptSiteDetails prompts the user for WordPress configuration or applies flag defaults.
func PromptSiteDetails(siteName string, titleFlag, emailFlag, userFlag, passFlag string, yesFlag bool) (*SitePromptResult, error) {
	// Auto-generate strong default password if not specified
	defaultPass := passFlag
	if defaultPass == "" {
		defaultPass = config.GenerateRandomPassword(16)
	}

	defaultTitle := titleFlag
	if defaultTitle == "" {
		// Capitalize domain prefix (e.g. mysite.local -> Mysite)
		parts := strings.Split(siteName, ".")
		defaultTitle = strings.Title(strings.ReplaceAll(parts[0], "-", " "))
	}

	defaultEmail := emailFlag
	if defaultEmail == "" {
		defaultEmail = "admin@example.local"
	}

	defaultUser := userFlag
	if defaultUser == "" {
		defaultUser = "admin"
	}

	// Skip prompts if --yes flag was provided or non-interactive
	if yesFlag {
		return &SitePromptResult{
			Title:    defaultTitle,
			Email:    defaultEmail,
			User:     defaultUser,
			Password: defaultPass,
		}, nil
	}

	var title string = defaultTitle
	var email string = defaultEmail
	var user string = defaultUser
	var password string = defaultPass
	var confirm bool = true

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("WordPress Site Title").
				Value(&title),

			huh.NewInput().
				Title("Admin Email Address").
				Value(&email).
				Validate(ValidateEmail),

			huh.NewInput().
				Title("Admin Username").
				Value(&user).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return errors.New("username cannot be empty")
					}
					return nil
				}),

			huh.NewInput().
				Title("Admin Password (pre-filled with strong random)").
				EchoMode(huh.EchoModePassword).
				Value(&password).
				Validate(func(s string) error {
					if len(strings.TrimSpace(s)) < 6 {
						return errors.New("password must be at least 6 characters")
					}
					return nil
				}),

			huh.NewConfirm().
				Title("Confirm and proceed with installation?").
				Value(&confirm),
		),
	)

	if err := form.Run(); err != nil {
		return nil, fmt.Errorf("prompt cancelled: %w", err)
	}

	if !confirm {
		return nil, errors.New("site installation cancelled by user")
	}

	return &SitePromptResult{
		Title:    strings.TrimSpace(title),
		Email:    strings.TrimSpace(email),
		User:     strings.TrimSpace(user),
		Password: strings.TrimSpace(password),
	}, nil
}
