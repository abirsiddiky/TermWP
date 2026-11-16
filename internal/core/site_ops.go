package core

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"termwp/internal/config"
	"termwp/internal/runtime"
	"termwp/internal/system"
	"termwp/internal/ui"
)

// CreateSiteParams encapsulates input parameters for new site creation.
type CreateSiteParams struct {
	SiteName   string
	PHPVersion string
	SSL        bool
	Title      string
	Email      string
	User       string
	Password   string
	SkipWait   bool
}

// SanitizeDBIdentifier cleans a domain into a valid SQL database and user name.
func SanitizeDBIdentifier(name string) string {
	cleaned := strings.ReplaceAll(name, ".", "_")
	cleaned = strings.ReplaceAll(cleaned, "-", "_")
	if len(cleaned) > 28 {
		cleaned = cleaned[:28]
	}
	return strings.ToLower(cleaned)
}

// CreateSite executes the complete site creation pipeline with automatic rollback on error.
func CreateSite(ctx context.Context, p CreateSiteParams, tracker *ui.StepTracker) error {
	store := GetStateStore()
	key := strings.ToLower(p.SiteName)

	if _, err := store.GetSite(key); err == nil {
		return fmt.Errorf("site '%s' already exists", p.SiteName)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	stack, err := runtime.GetStackManager()
	if err != nil {
		return err
	}

	wpMgr := runtime.NewWordPressManager()
	rollback := NewRollbackStack()
	defer func() {
		// If rollback still has steps, execute them
		rollback.Execute(tracker)
	}()

	siteDir := config.SiteDir(p.SiteName)
	dbName := "wp_" + SanitizeDBIdentifier(p.SiteName)
	dbUser := "usr_" + SanitizeDBIdentifier(p.SiteName)
	dbPass := config.GenerateRandomPassword(20)

	// Step 1: Create directory & extract WordPress
	tracker.Start(fmt.Sprintf("Extracting WordPress into %s", p.SiteName))
	if err := wpMgr.ExtractToSite(siteDir, tracker); err != nil {
		tracker.Fail("Failed extracting WordPress")
		return err
	}
	rollback.Push("Remove site files", func() error {
		return os.RemoveAll(siteDir)
	})
	tracker.Success("WordPress files prepared")

	// Step 2: Ensure MariaDB is running & create database + user
	tracker.Start(fmt.Sprintf("Creating database '%s' and user '%s'", dbName, dbUser))
	if err := stack.MariaDB.CreateDatabaseAndUser(dbName, dbUser, dbPass); err != nil {
		tracker.Fail("Database creation failed")
		return err
	}
	rollback.Push("Drop database and user", func() error {
		return stack.MariaDB.DropDatabaseAndUser(dbName, dbUser)
	})
	tracker.Success(fmt.Sprintf("Database '%s' created", dbName))

	// Step 3: Generate wp-config.php
	tracker.Start("Generating wp-config.php")
	if err := wpMgr.GenerateWPConfig(siteDir, dbName, dbUser, dbPass, cfg.MariaDBPort); err != nil {
		tracker.Fail("Failed generating wp-config.php")
		return err
	}
	tracker.Success("wp-config.php generated")

	// Step 4: Generate SSL Certificates if requested
	if p.SSL {
		tracker.Start(fmt.Sprintf("Generating mkcert SSL certificate for '%s'", p.SiteName))
		if _, _, err := stack.Mkcert.GenerateSiteCert(p.SiteName); err != nil {
			tracker.Fail("SSL certificate generation failed")
			return err
		}
		rollback.Push("Remove SSL certificate", func() error {
			return stack.Mkcert.RemoveSiteCert(p.SiteName)
		})
		tracker.Success("SSL certificate generated")
	}

	// Step 5: Write Nginx virtual host configuration
	tracker.Start("Generating nginx virtual host configuration")
	siteModel := &Site{
		Name:       p.SiteName,
		Domain:     p.SiteName,
		PHPVersion: p.PHPVersion,
		SSL:        p.SSL,
		Status:     StatusOn,
		DBName:     dbName,
		DBUser:     dbUser,
		DBPass:     dbPass,
		SiteTitle:  p.Title,
		AdminUser:  p.User,
		AdminEmail: p.Email,
		CreatedAt:  time.Now(),
	}

	phpPort := stack.PHP.GetPort(p.PHPVersion)
	if err := stack.Nginx.WriteSiteConfig(p.SiteName, p.SiteName, p.SSL, phpPort); err != nil {
		tracker.Fail("Nginx config creation failed")
		return err
	}
	rollback.Push("Remove nginx config", func() error {
		return stack.Nginx.RemoveSiteConfig(p.SiteName)
	})
	tracker.Success("Nginx virtual host configured")

	// Step 6: Add hosts file entry
	tracker.Start(fmt.Sprintf("Adding %s to system hosts file", p.SiteName))
	if err := system.AddHostsEntry(p.SiteName, "127.0.0.1"); err != nil {
		tracker.Fail("Failed updating hosts file")
		return err
	}
	rollback.Push("Remove hosts entry", func() error {
		return system.RemoveHostsEntry(p.SiteName)
	})
	tracker.Success("Hosts file updated")

	// Step 7: Ensure stack services are running for this site
	tracker.Start("Starting required PHP and Nginx background services")
	if err := stack.EnsureStackRunning([]string{p.PHPVersion}); err != nil {
		tracker.Fail("Failed starting services")
		return err
	}
	tracker.Success("Stack services active")

	// Step 8: Run WordPress installation via WP-CLI
	tracker.Start("Executing WordPress core installation via WP-CLI")
	proto := "http"
	port := cfg.HTTPPort
	if p.SSL {
		proto = "https"
		port = cfg.HTTPSPort
	}
	siteURL := fmt.Sprintf("%s://%s", proto, p.SiteName)
	if (proto == "http" && port != 80) || (proto == "https" && port != 443) {
		siteURL = fmt.Sprintf("%s://%s:%d", proto, p.SiteName, port)
	}

	if err := stack.WPCLI.CoreInstall(siteDir, p.PHPVersion, siteURL, p.Title, p.User, p.Password, p.Email); err != nil {
		tracker.Fail("WP-CLI core install failed")
		return err
	}
	tracker.Success("WordPress installation complete")

	// Step 9: Save to state store
	if err := store.AddSite(siteModel); err != nil {
		tracker.Fail("Failed saving site state")
		return err
	}
	rollback.Push("Remove from state registry", func() error {
		return store.DeleteSite(p.SiteName)
	})

	// Transaction succeeded: discard rollback
	rollback.Discard()

	// Display beautiful summary card
	summaryData := ui.SiteSummaryData{
		SiteName:      p.SiteName,
		SiteURL:       siteURL,
		AdminURL:      siteURL + "/wp-admin",
		AdminUsername: p.User,
		AdminPassword: p.Password,
		AdminEmail:    p.Email,
		PHPVersion:    p.PHPVersion,
		SSL:           p.SSL,
		DatabaseName:  dbName,
		DatabaseUser:  dbUser,
		SitePath:      siteDir,
	}

	ui.ShowSummaryCardInteractive(summaryData, p.SkipWait)
	return nil
}

// DeleteSite completely removes a site and its database.
func DeleteSite(ctx context.Context, siteName string, tracker *ui.StepTracker) error {
	store := GetStateStore()
	site, err := store.GetSite(siteName)
	if err != nil {
		return err
	}

	stack, err := runtime.GetStackManager()
	if err != nil {
		return err
	}

	tracker.Start(fmt.Sprintf("Removing site '%s'", site.Name))

	// 1. Remove Nginx config
	_ = stack.Nginx.RemoveSiteConfig(site.Name)
	_ = stack.Nginx.RemoveSiteConfig(site.Name + ".disabled")

	// 2. Remove hosts entry
	_ = system.RemoveHostsEntry(site.Name)

	// 3. Remove SSL certificates if any
	_ = stack.Mkcert.RemoveSiteCert(site.Name)

	// 4. Drop database & user
	if site.DBName != "" {
		_ = stack.MariaDB.DropDatabaseAndUser(site.DBName, site.DBUser)
	}

	// 5. Delete document root
	siteDir := config.SiteDir(site.Name)
	_ = os.RemoveAll(siteDir)

	// 6. Delete from state store
	if err := store.DeleteSite(site.Name); err != nil {
		tracker.Fail("Failed removing site from state.json")
		return err
	}

	// 7. Reload Nginx
	_ = stack.Nginx.Reload()

	// 8. If no other active site uses this PHP version, stop it
	activeVersions, _ := store.GetActivePHPVersions()
	stillUsed := false
	for _, v := range activeVersions {
		if v == site.PHPVersion {
			stillUsed = true
			break
		}
	}
	if !stillUsed && site.PHPVersion != "" {
		_, _ = stack.PHP.StopPHP(site.PHPVersion, false)
	}

	tracker.Success(fmt.Sprintf("Site '%s' deleted successfully", site.Name))
	return nil
}

// TurnOffSite deactivates a site by disabling nginx conf and removing hosts entry.
func TurnOffSite(ctx context.Context, site *Site, tracker *ui.StepTracker) error {
	stack, err := runtime.GetStackManager()
	if err != nil {
		return err
	}

	store := GetStateStore()

	tracker.Start(fmt.Sprintf("Disabling nginx virtual host for '%s'", site.Name))
	if err := stack.Nginx.DisableSiteConfig(site.Name); err != nil {
		tracker.Fail("Failed disabling nginx config")
		return err
	}

	tracker.Start(fmt.Sprintf("Removing '%s' from hosts file", site.Name))
	if err := system.RemoveHostsEntry(site.Name); err != nil {
		tracker.Warn("Hosts entry could not be removed automatically")
	}

	if err := store.SetSiteStatus(site.Name, StatusOff); err != nil {
		tracker.Fail("Failed updating site status")
		return err
	}

	_ = stack.Nginx.Reload()

	// Stop PHP backend if no other active site uses it
	activeVersions, _ := store.GetActivePHPVersions()
	stillUsed := false
	for _, v := range activeVersions {
		if v == site.PHPVersion {
			stillUsed = true
			break
		}
	}
	if !stillUsed && site.PHPVersion != "" {
		_, _ = stack.PHP.StopPHP(site.PHPVersion, false)
	}

	tracker.Success(fmt.Sprintf("Site '%s' is now off", site.Name))
	return nil
}

// TurnOnSite restores an inactive site.
func TurnOnSite(ctx context.Context, site *Site, tracker *ui.StepTracker) error {
	stack, err := runtime.GetStackManager()
	if err != nil {
		return err
	}

	store := GetStateStore()

	tracker.Start(fmt.Sprintf("Restoring virtual host for '%s'", site.Name))
	if err := stack.Nginx.EnableSiteConfig(site.Name); err != nil {
		// If enabled file was missing, rewrite it
		phpPort := stack.PHP.GetPort(site.PHPVersion)
		if writeErr := stack.Nginx.WriteSiteConfig(site.Name, site.Domain, site.SSL, phpPort); writeErr != nil {
			tracker.Fail("Failed enabling virtual host")
			return writeErr
		}
	}

	tracker.Start(fmt.Sprintf("Adding '%s' to hosts file", site.Name))
	if err := system.AddHostsEntry(site.Name, "127.0.0.1"); err != nil {
		tracker.Fail("Failed updating hosts file")
		return err
	}

	if err := store.SetSiteStatus(site.Name, StatusOn); err != nil {
		tracker.Fail("Failed updating site status")
		return err
	}

	// Ensure background daemons needed are active
	if err := stack.EnsureStackRunning([]string{site.PHPVersion}); err != nil {
		tracker.Fail("Failed starting stack services")
		return err
	}

	_ = stack.Nginx.Reload()

	tracker.Success(fmt.Sprintf("Site '%s' is now on", site.Name))
	return nil
}
