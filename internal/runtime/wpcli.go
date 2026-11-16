package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"termwp/internal/config"
	"termwp/internal/ui"
)

// WPCLIManager executes WordPress administration commands via wp-cli.phar.
type WPCLIManager struct {
	phpMgr *PHPManager
}

// NewWPCLIManager creates an initialized WPCLIManager.
func NewWPCLIManager(phpMgr *PHPManager) *WPCLIManager {
	return &WPCLIManager{phpMgr: phpMgr}
}

// EnsureInstalled downloads wp-cli.phar if not already cached/installed.
func (w *WPCLIManager) EnsureInstalled(tracker *ui.StepTracker) error {
	dest := config.WPCLIPath()
	if _, err := os.Stat(dest); err == nil {
		return nil
	}

	src := config.GetWPCLISource()
	downloader := NewDownloader(tracker)
	return downloader.FetchAndInstall(src, config.ToolsDir())
}

// Run executes a WP-CLI command against a site directory using the site's PHP version.
func (w *WPCLIManager) Run(siteDir, phpVersion string, args ...string) (string, error) {
	if err := w.EnsureInstalled(nil); err != nil {
		return "", err
	}

	phpBin := w.phpMgr.GetCLIPath(phpVersion)
	iniPath := w.phpMgr.IniFilePath(phpVersion)
	pharPath := config.WPCLIPath()

	cmdArgs := []string{
		"-c", iniPath,
		pharPath,
		fmt.Sprintf("--path=%s", siteDir),
		"--allow-root",
	}
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.Command(phpBin, cmdArgs...)
	cmd.Dir = siteDir

	output, err := cmd.CombinedOutput()
	outStr := strings.TrimSpace(string(output))
	if err != nil {
		return outStr, fmt.Errorf("wp-cli error: %s (%w)", outStr, err)
	}
	return outStr, nil
}

// CoreInstall runs `wp core install`.
func (w *WPCLIManager) CoreInstall(siteDir, phpVersion, url, title, adminUser, adminPass, adminEmail string) error {
	_, err := w.Run(siteDir, phpVersion,
		"core", "install",
		fmt.Sprintf("--url=%s", url),
		fmt.Sprintf("--title=%s", title),
		fmt.Sprintf("--admin_user=%s", adminUser),
		fmt.Sprintf("--admin_password=%s", adminPass),
		fmt.Sprintf("--admin_email=%s", adminEmail),
		"--skip-email",
	)
	return err
}

// CoreUpdate updates WordPress core to the latest version.
func (w *WPCLIManager) CoreUpdate(siteDir, phpVersion string) error {
	_, err := w.Run(siteDir, phpVersion, "core", "update")
	return err
}

// CoreUpdateDB updates the WordPress database schema after a core update.
func (w *WPCLIManager) CoreUpdateDB(siteDir, phpVersion string) error {
	_, err := w.Run(siteDir, phpVersion, "core", "update-db")
	return err
}

// SearchReplace performs search-replace across all database tables including serialized strings.
func (w *WPCLIManager) SearchReplace(siteDir, phpVersion, oldDomain, newDomain string) error {
	// First replace full https URLs
	_, err := w.Run(siteDir, phpVersion, "search-replace",
		fmt.Sprintf("https://%s", oldDomain),
		fmt.Sprintf("https://%s", newDomain),
		"--all-tables",
	)
	if err != nil {
		return err
	}

	// Then replace http URLs
	_, err = w.Run(siteDir, phpVersion, "search-replace",
		fmt.Sprintf("http://%s", oldDomain),
		fmt.Sprintf("http://%s", newDomain),
		"--all-tables",
	)
	if err != nil {
		return err
	}

	// Finally replace bare domain names
	_, err = w.Run(siteDir, phpVersion, "search-replace", oldDomain, newDomain, "--all-tables")
	return err
}

// PluginUpdate updates plugins (--all or specified plugin).
func (w *WPCLIManager) PluginUpdate(siteDir, phpVersion string) error {
	_, err := w.Run(siteDir, phpVersion, "plugin", "update", "--all")
	return err
}

// ThemeUpdate updates themes (--all).
func (w *WPCLIManager) ThemeUpdate(siteDir, phpVersion string) error {
	_, err := w.Run(siteDir, phpVersion, "theme", "update", "--all")
	return err
}

// GetVersion returns the installed WordPress core version string.
func (w *WPCLIManager) GetVersion(siteDir, phpVersion string) (string, error) {
	out, err := w.Run(siteDir, phpVersion, "core", "version")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// UpdateOption sets a WordPress option value.
func (w *WPCLIManager) UpdateOption(siteDir, phpVersion, optionName, value string) error {
	_, err := w.Run(siteDir, phpVersion, "option", "update", optionName, value)
	return err
}
