package core

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"termwp/internal/config"
	"termwp/internal/runtime"
	"termwp/internal/system"
	"termwp/internal/ui"
)

// AddSSLToSite enables SSL on an existing WordPress site.
func AddSSLToSite(ctx context.Context, siteName string, tracker *ui.StepTracker) error {
	store := GetStateStore()
	site, err := store.GetSite(siteName)
	if err != nil {
		return fmt.Errorf("site '%s' not found: %w", siteName, err)
	}

	if site.SSL {
		if tracker != nil {
			tracker.Info(fmt.Sprintf("Site '%s' already has SSL enabled.", site.Name))
		}
		return nil
	}

	stack, err := runtime.GetStackManager()
	if err != nil {
		return err
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	// 1. Generate SSL Certificate
	if tracker != nil {
		tracker.Start(fmt.Sprintf("Generating mkcert certificate for '%s'", site.Domain))
	}
	if _, _, err := stack.Mkcert.GenerateSiteCert(site.Domain); err != nil {
		if tracker != nil {
			tracker.Fail("Certificate generation failed")
		}
		return err
	}
	if tracker != nil {
		tracker.Success("Certificate generated")
	}

	// 2. Update Nginx configuration
	if tracker != nil {
		tracker.Start("Updating nginx virtual host with SSL directives")
	}
	site.SSL = true
	phpPort := stack.PHP.GetPort(site.PHPVersion)
	if err := stack.Nginx.WriteSiteConfig(site.Name, site.Domain, true, phpPort); err != nil {
		if tracker != nil {
			tracker.Fail("Failed updating nginx configuration")
		}
		return err
	}
	if tracker != nil {
		tracker.Success("Nginx configuration updated")
	}

	// 3. Update WordPress URLs via WP-CLI
	siteDir := config.SiteDir(site.Name)

	// Ensure MariaDB and PHP are running for WP-CLI configuration updates
	if err := stack.EnsureStackRunning([]string{site.PHPVersion}); err != nil {
		tracker.Fail("Failed starting required stack services for SSL update")
		return err
	}
	httpsPort := cfg.HTTPSPort
	newURL := fmt.Sprintf("https://%s", site.Domain)
	if httpsPort != 443 {
		newURL = fmt.Sprintf("https://%s:%d", site.Domain, httpsPort)
	}

	if tracker != nil {
		tracker.Start("Updating WordPress siteurl and home to HTTPS")
	}
	_ = stack.WPCLI.UpdateOption(siteDir, site.PHPVersion, "siteurl", newURL)
	_ = stack.WPCLI.UpdateOption(siteDir, site.PHPVersion, "home", newURL)

	// Replace internal http URLs to https
	_ = stack.WPCLI.SearchReplace(siteDir, site.PHPVersion, fmt.Sprintf("http://%s", site.Domain), newURL)
	if tracker != nil {
		tracker.Success("WordPress URLs converted to HTTPS")
	}

	// 4. Update state store
	_ = store.UpdateSite(site)

	// 5. Reload Nginx
	_ = stack.Nginx.Reload()

	if tracker != nil {
		tracker.Success(fmt.Sprintf("SSL enabled for '%s'. Reachable at %s", site.Name, newURL))
	}

	return nil
}

// CopyDir recursively copies a directory tree from src to dst.
func CopyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := CopyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := CopyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// CopyFile copies a single file from src to dst.
func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

// CloneSite duplicates an existing WordPress site to a new domain.
func CloneSite(ctx context.Context, sourceSite *Site, targetName string, tracker *ui.StepTracker) error {
	store := GetStateStore()
	targetKey := strings.ToLower(targetName)

	if _, err := store.GetSite(targetKey); err == nil {
		return fmt.Errorf("target site '%s' already exists", targetName)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	stack, err := runtime.GetStackManager()
	if err != nil {
		return err
	}

	// Ensure MariaDB, PHP, and Nginx are running for the clone operation
	if err := stack.EnsureStackRunning([]string{sourceSite.PHPVersion}); err != nil {
		tracker.Fail("Failed starting required stack services for clone")
		return err
	}

	rollback := NewRollbackStack()
	defer func() {
		rollback.Execute(tracker)
	}()

	sourceDir := config.SiteDir(sourceSite.Name)
	targetDir := config.SiteDir(targetName)
	targetDB := "wp_" + SanitizeDBIdentifier(targetName)
	targetUser := "usr_" + SanitizeDBIdentifier(targetName)
	targetPass := config.GenerateRandomPassword(20)

	// Step 1: Export source database
	tracker.Start(fmt.Sprintf("Exporting database from '%s'", sourceSite.Name))
	tmpDumpPath := filepath.Join(config.CacheDir(), fmt.Sprintf("clone_dump_%d.sql", time.Now().UnixNano()))
	if err := stack.MariaDB.ExportDatabase(sourceSite.DBName, tmpDumpPath); err != nil {
		tracker.Fail("Database export failed")
		return err
	}
	defer os.Remove(tmpDumpPath)
	tracker.Success("Database exported")

	// Step 2: Copy site files
	tracker.Start(fmt.Sprintf("Copying files to '%s'", targetName))
	if err := CopyDir(sourceDir, targetDir); err != nil {
		tracker.Fail("File copy failed")
		return err
	}
	rollback.Push("Delete cloned files", func() error {
		return os.RemoveAll(targetDir)
	})
	tracker.Success("Files copied")

	// Step 3: Create target database and user
	tracker.Start(fmt.Sprintf("Creating target database '%s'", targetDB))
	if err := stack.MariaDB.CreateDatabaseAndUser(targetDB, targetUser, targetPass); err != nil {
		tracker.Fail("Target database creation failed")
		return err
	}
	rollback.Push("Drop target database", func() error {
		return stack.MariaDB.DropDatabaseAndUser(targetDB, targetUser)
	})
	tracker.Success("Target database created")

	// Step 4: Import dump into target database
	tracker.Start("Importing database data into target")
	if err := stack.MariaDB.ImportDatabase(targetDB, tmpDumpPath); err != nil {
		tracker.Fail("Database import failed")
		return err
	}
	tracker.Success("Database imported")

	// Step 5: Generate new wp-config.php with isolated credentials and unique salts
	tracker.Start("Generating target wp-config.php")
	wpMgr := runtime.NewWordPressManager()
	if err := wpMgr.GenerateWPConfig(targetDir, targetDB, targetUser, targetPass, cfg.MariaDBPort); err != nil {
		tracker.Fail("Failed writing target wp-config.php")
		return err
	}
	tracker.Success("wp-config.php generated")

	// Step 6: Search & Replace URLs across all database tables
	tracker.Start(fmt.Sprintf("Replacing URLs '%s' -> '%s'", sourceSite.Domain, targetName))
	if err := stack.WPCLI.SearchReplace(targetDir, sourceSite.PHPVersion, sourceSite.Domain, targetName); err != nil {
		tracker.Fail("wp search-replace failed")
		return err
	}
	tracker.Success("Database URLs updated")

	// Step 7: SSL certificate if source had SSL
	if sourceSite.SSL {
		tracker.Start(fmt.Sprintf("Generating SSL certificate for '%s'", targetName))
		if _, _, err := stack.Mkcert.GenerateSiteCert(targetName); err != nil {
			tracker.Fail("SSL generation failed")
			return err
		}
		rollback.Push("Remove target SSL cert", func() error {
			return stack.Mkcert.RemoveSiteCert(targetName)
		})
		tracker.Success("SSL certificate generated")
	}

	// Step 8: Virtual host configuration
	tracker.Start("Configuring target nginx virtual host")
	targetSiteModel := &Site{
		Name:       targetName,
		Domain:     targetName,
		PHPVersion: sourceSite.PHPVersion,
		SSL:        sourceSite.SSL,
		Enabled:    sourceSite.Enabled,
		DBName:     targetDB,
		DBUser:     targetUser,
		DBPass:     targetPass,
		SiteTitle:  sourceSite.SiteTitle + " (Clone)",
		AdminUser:  sourceSite.AdminUser,
		AdminEmail: sourceSite.AdminEmail,
		CreatedAt:  time.Now(),
	}

	phpPort := stack.PHP.GetPort(sourceSite.PHPVersion)
	if err := stack.Nginx.WriteSiteConfig(targetName, targetName, sourceSite.SSL, phpPort); err != nil {
		tracker.Fail("Target virtual host creation failed")
		return err
	}
	rollback.Push("Remove target nginx config", func() error {
		return stack.Nginx.RemoveSiteConfig(targetName)
	})
	tracker.Success("Nginx virtual host created")

	// Step 9: Hosts file entry
	tracker.Start(fmt.Sprintf("Adding '%s' to hosts file", targetName))
	if err := system.AddHostsEntry(targetName, "127.0.0.1"); err != nil {
		tracker.Fail("Failed updating hosts file")
		return err
	}
	rollback.Push("Remove target hosts entry", func() error {
		return system.RemoveHostsEntry(targetName)
	})
	tracker.Success("Hosts file updated")

	// Step 10: Register in state store
	if err := store.AddSite(targetSiteModel); err != nil {
		tracker.Fail("Failed recording target site in state")
		return err
	}
	rollback.Push("Delete target from state registry", func() error {
		return store.DeleteSite(targetName)
	})

	// Step 11: Reload Nginx
	_ = stack.Nginx.Reload()

	// Discard rollback stack on success
	rollback.Discard()

	proto := "http"
	port := cfg.HTTPPort
	if targetSiteModel.SSL {
		proto = "https"
		port = cfg.HTTPSPort
	}
	targetURL := fmt.Sprintf("%s://%s", proto, targetName)
	if (proto == "http" && port != 80) || (proto == "https" && port != 443) {
		targetURL = fmt.Sprintf("%s://%s:%d", proto, targetName, port)
	}

	tracker.Success(fmt.Sprintf("Site successfully cloned to '%s'", targetName))

	// Display summary card
	summaryData := ui.SiteSummaryData{
		SiteName:      targetName,
		SiteURL:       targetURL,
		AdminURL:      targetURL + "/wp-admin",
		AdminUsername: targetSiteModel.AdminUser,
		AdminPassword: "(same as source)",
		AdminEmail:    targetSiteModel.AdminEmail,
		PHPVersion:    targetSiteModel.PHPVersion,
		SSL:           targetSiteModel.SSL,
		DatabaseName:  targetDB,
		DatabaseUser:  targetUser,
		SitePath:      targetDir,
	}

	ui.ShowSummaryCardInteractive(summaryData, false)
	return nil
}

// UpdateSite updates WordPress core, plugins, or themes with automatic pre-update backup.
func UpdateSite(ctx context.Context, site *Site, plugins, themes, all bool, tracker *ui.StepTracker) error {
	stack, err := runtime.GetStackManager()
	if err != nil {
		return err
	}

	siteDir := config.SiteDir(site.Name)

	// Ensure MariaDB and PHP are running for update and backup
	if err := stack.EnsureStackRunning([]string{site.PHPVersion}); err != nil {
		tracker.Fail("Failed starting required stack services for update")
		return err
	}

	// Step 1: Automatic safety backup of database
	backupDir := config.SiteBackupsDir(site.Name)
	_ = os.MkdirAll(backupDir, 0755)
	timestamp := time.Now().Format("20060102_150405")
	backupFile := filepath.Join(backupDir, fmt.Sprintf("pre_update_%s.sql", timestamp))

	tracker.Start(fmt.Sprintf("Taking safety database backup to %s", filepath.Base(backupFile)))
	if err := stack.MariaDB.ExportDatabase(site.DBName, backupFile); err != nil {
		tracker.Fail("Pre-update backup failed. Aborting update for safety.")
		return err
	}
	tracker.Success(fmt.Sprintf("Database backup saved (%s)", filepath.Base(backupFile)))

	// Step 2: Get current WordPress version
	oldVersion, _ := stack.WPCLI.GetVersion(siteDir, site.PHPVersion)

	// Update Core if neither specific flag was requested, or if --all was requested
	updateCore := all || (!plugins && !themes)

	if updateCore {
		tracker.Start("Updating WordPress core and database schema")
		if err := stack.WPCLI.CoreUpdate(siteDir, site.PHPVersion); err != nil {
			tracker.Warn(fmt.Sprintf("Core update message: %v", err))
		}
		_ = stack.WPCLI.CoreUpdateDB(siteDir, site.PHPVersion)
		newVersion, _ := stack.WPCLI.GetVersion(siteDir, site.PHPVersion)

		if oldVersion != "" && newVersion != "" {
			tracker.Success(fmt.Sprintf("WordPress Core: %s ➜ %s", oldVersion, newVersion))
		} else {
			tracker.Success("WordPress Core updated")
		}
	}

	if plugins || all {
		tracker.Start("Updating WordPress plugins")
		if err := stack.WPCLI.PluginUpdate(siteDir, site.PHPVersion); err != nil {
			tracker.Warn(fmt.Sprintf("Plugin update note: %v", err))
		} else {
			tracker.Success("Plugins updated")
		}
	}

	if themes || all {
		tracker.Start("Updating WordPress themes")
		if err := stack.WPCLI.ThemeUpdate(siteDir, site.PHPVersion); err != nil {
			tracker.Warn(fmt.Sprintf("Theme update note: %v", err))
		} else {
			tracker.Success("Themes updated")
		}
	}

	tracker.Success(fmt.Sprintf("Update operations complete for '%s'", site.Name))
	return nil
}
