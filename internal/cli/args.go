package cli

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// siteNameRegex enforces: lowercase letters, numbers, hyphens, and dots with valid domain structure.
	siteNameRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)
)

// AddArgs represents parsed arguments for `termwp add`.
type AddArgs struct {
	SiteName      string
	PHPVersion    string
	SSL           bool
	IsAddExisting bool // True if invoked as `termwp add ssl <site>`
}

// CloneArgs represents parsed arguments for `termwp clone <source> to <target>`.
type CloneArgs struct {
	SourceSite string
	TargetSite string
}

// ValidateSiteName checks that a domain name complies with TermWP specifications.
func ValidateSiteName(name string) error {
	trimmed := strings.ToLower(strings.TrimSpace(name))
	if trimmed == "" {
		return fmt.Errorf("site name cannot be empty")
	}

	if strings.Contains(name, " ") {
		return fmt.Errorf("site name cannot contain whitespace: '%s'", name)
	}

	// Must contain at least one dot (domain + TLD)
	if !strings.Contains(trimmed, ".") {
		return fmt.Errorf("site name must include a domain extension (e.g. '%s.local' or '%s.test')", trimmed, trimmed)
	}

	if !siteNameRegex.MatchString(trimmed) {
		return fmt.Errorf("invalid site name '%s': must contain only lowercase letters, digits, dots, and hyphens", name)
	}

	parts := strings.Split(trimmed, ".")
	tld := parts[len(parts)-1]
	if len(tld) < 2 {
		return fmt.Errorf("invalid TLD in '%s'", name)
	}

	reserved := map[string]bool{
		"localhost": true,
		"termwp":    true,
		"nginx":     true,
		"mariadb":   true,
		"mysql":     true,
	}

	if reserved[parts[0]] || reserved[trimmed] {
		return fmt.Errorf("'%s' is a reserved system name and cannot be used as a site domain", name)
	}

	return nil
}

// NormalizePHPVersion cleans and validates a PHP version string ("php8.4", "8.4", "8.4.0" -> "8.4").
func NormalizePHPVersion(v string) (string, error) {
	cleaned := strings.ToLower(strings.TrimSpace(v))
	cleaned = strings.TrimPrefix(cleaned, "php")

	switch cleaned {
	case "8.3", "8.3.x":
		return "8.3", nil
	case "8.4", "8.4.x":
		return "8.4", nil
	case "8.5", "8.5.x":
		return "8.5", nil
	default:
		return "", fmt.Errorf("unsupported PHP version '%s' (supported: 8.3, 8.4, 8.5)", v)
	}
}

// IsPHPVersionToken checks if a token looks like a PHP version argument.
func IsPHPVersionToken(token string) bool {
	t := strings.ToLower(strings.TrimSpace(token))
	t = strings.TrimPrefix(t, "php")
	return t == "8.3" || t == "8.4" || t == "8.5"
}

// ParseAddArgs parses `termwp add` arguments:
// Handles both:
// 1) termwp add <site.local> [php8.3|php8.4|php8.5] [ssl]  (in ANY order after site name)
// 2) termwp add ssl <site.local>
func ParseAddArgs(args []string, defaultPHP string) (*AddArgs, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("site name is required. Usage: termwp add <site.local> [php-version] [ssl]")
	}

	if defaultPHP == "" {
		defaultPHP = "8.4"
	}

	// Case 1: `termwp add ssl <site.local>`
	if strings.ToLower(args[0]) == "ssl" {
		if len(args) < 2 {
			return nil, fmt.Errorf("missing site name. Usage: termwp add ssl <site.local>")
		}
		site := strings.ToLower(args[1])
		if err := ValidateSiteName(site); err != nil {
			return nil, err
		}
		return &AddArgs{
			SiteName:      site,
			PHPVersion:    defaultPHP,
			SSL:           true,
			IsAddExisting: true,
		}, nil
	}

	// Case 2: `termwp add <site.local> [php...] [ssl...]`
	site := strings.ToLower(args[0])
	if err := ValidateSiteName(site); err != nil {
		return nil, err
	}

	phpVersion := defaultPHP
	ssl := false

	// Remaining tokens in ANY order
	for _, token := range args[1:] {
		lowerToken := strings.ToLower(token)
		if lowerToken == "ssl" {
			ssl = true
		} else if IsPHPVersionToken(lowerToken) {
			norm, err := NormalizePHPVersion(lowerToken)
			if err != nil {
				return nil, err
			}
			phpVersion = norm
		} else {
			return nil, fmt.Errorf("unexpected argument '%s'. Usage: termwp add <site.local> [php8.3|php8.4|php8.5] [ssl]", token)
		}
	}

	return &AddArgs{
		SiteName:      site,
		PHPVersion:    phpVersion,
		SSL:           ssl,
		IsAddExisting: false,
	}, nil
}

// ParseCloneArgs parses `termwp clone <source> to <target>`.
func ParseCloneArgs(args []string) (*CloneArgs, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf("invalid clone arguments. Usage: termwp clone <source.local> to <new.local>")
	}

	source := strings.ToLower(args[0])
	toKeyword := strings.ToLower(args[1])
	target := strings.ToLower(args[2])

	if toKeyword != "to" {
		return nil, fmt.Errorf("expected 'to' keyword. Usage: termwp clone <source.local> to <new.local>")
	}

	if err := ValidateSiteName(source); err != nil {
		return nil, fmt.Errorf("invalid source site: %w", err)
	}

	if err := ValidateSiteName(target); err != nil {
		return nil, fmt.Errorf("invalid target site: %w", err)
	}

	if source == target {
		return nil, fmt.Errorf("source and target site names cannot be the same")
	}

	return &CloneArgs{
		SourceSite: source,
		TargetSite: target,
	}, nil
}
