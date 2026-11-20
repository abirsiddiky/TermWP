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
		Enabled:    true,
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

	// Clear shutdown flag
	_ = store.SetShutdown(false)

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

	// 7. Reload Nginx if running
	if stack.Nginx.IsRunning() {
		_ = stack.Nginx.Reload()
	}

	// 8. If no other active site uses this PHP version, stop it
	activeVersions, _ := store.GetActivePHPVersions()
	stillUsed := false
	for _, v := range activeVersions {
		if v == site.PHPVersion {
			stillUsed = true
			break
		}
	}
	if !stillUsed && site.PHPVersion != "" && stack.PHP.IsPHPRunning(site.PHPVersion) {
		_, _ = stack.PHP.StopPHP(site.PHPVersion, false)
	}

	tracker.Success(fmt.Sprintf("Site '%s' deleted successfully", site.Name))
	return nil
}

// TurnOffSite deactivates a site by disabling nginx conf and removing hosts entry.
// If services are stopped, off works instantly without starting background services.
func TurnOffSite(ctx context.Context, site *Site, tracker *ui.StepTracker) error {
	stack, err := runtime.GetStackManager()
	if err != nil {
		return err
	}

	store := GetStateStore()

	tracker.Start(fmt.Sprintf("Disabling nginx virtual host for '%s'", site.Name))
	_ = stack.Nginx.DisableSiteConfig(site.Name)

	tracker.Start(fmt.Sprintf("Removing '%s' from hosts file", site.Name))
	if err := system.RemoveHostsEntry(site.Name); err != nil {
		tracker.Warn("Hosts entry could not be removed automatically")
	}

	if err := store.SetSiteEnabled(site.Name, false); err != nil {
		tracker.Fail("Failed updating site status")
		return err
	}

	// Only reload Nginx if Nginx is actually running
	if stack.Nginx.IsRunning() {
		_ = stack.Nginx.Reload()
	}

	// Stop PHP backend if no other active site uses it and it is currently running
	activeVersions, _ := store.GetActivePHPVersions()
	stillUsed := false
	for _, v := range activeVersions {
		if v == site.PHPVersion {
			stillUsed = true
			break
		}
	}
	if !stillUsed && site.PHPVersion != "" && stack.PHP.IsPHPRunning(site.PHPVersion) {
		_, _ = stack.PHP.StopPHP(site.PHPVersion, false)
	}

	tracker.Success(fmt.Sprintf("Site '%s' is now off", site.Name))
	return nil
}

// TurnOnSite restores an inactive site, starts needed services, and verifies it responds.
func TurnOnSite(ctx context.Context, site *Site, tracker *ui.StepTracker) error {
	stack, err := runtime.GetStackManager()
	if err != nil {
		return err
	}

	store := GetStateStore()

	// 1. Mark enabled in state
	if err := store.SetSiteEnabled(site.Name, true); err != nil {
		tracker.Fail("Failed updating site state")
		return err
	}
	site.Enabled = true

	// Clear shutdown flag
	_ = store.SetShutdown(false)

	// 2. Restore Nginx configuration
	tracker.Start(fmt.Sprintf("Restoring virtual host for '%s'", site.Name))
	if err := stack.Nginx.EnableSiteConfig(site.Name); err != nil {
		phpPort := stack.PHP.GetPort(site.PHPVersion)
		if writeErr := stack.Nginx.WriteSiteConfig(site.Name, site.Domain, site.SSL, phpPort); writeErr != nil {
			tracker.Fail("Failed enabling virtual host")
			return writeErr
		}
	}

	// 3. Sync hosts entry
	tracker.Start(fmt.Sprintf("Adding '%s' to hosts file", site.Name))
	if err := system.AddHostsEntry(site.Name, "127.0.0.1"); err != nil {
		tracker.Fail("Failed updating hosts file")
		return err
	}

	// 4. Ensure background services needed by this site are running
	// (Only starts MariaDB, Nginx, and this site's PHP version)
	tracker.Start(fmt.Sprintf("Starting services (MariaDB, PHP %s, nginx)", site.PHPVersion))
	if err := stack.EnsureStackRunning([]string{site.PHPVersion}); err != nil {
		tracker.Fail("Failed starting stack services")
		return err
	}

	// 5. Reload Nginx
	if err := stack.Nginx.Reload(); err != nil {
		tracker.Warn(fmt.Sprintf("Nginx reload warning: %v", err))
	}

	// 6. Verify the site responds
	tracker.Start("Verifying site responsiveness")
	cfg, _ := config.LoadConfig()
	var liveStatus SiteLiveStatus
	for i := 0; i < 6; i++ {
		liveStatus = CheckSiteStatus(ctx, site, cfg, stack)
		if liveStatus.State == StatusRunning {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	siteURL := fmt.Sprintf("http://%s", site.Domain)
	if site.SSL {
		siteURL = fmt.Sprintf("https://%s", site.Domain)
	}
	if cfg != nil {
		if !site.SSL && cfg.HTTPPort != 80 {
			siteURL = fmt.Sprintf("http://%s:%d", site.Domain, cfg.HTTPPort)
		} else if site.SSL && cfg.HTTPSPort != 443 {
			siteURL = fmt.Sprintf("https://%s:%d", site.Domain, cfg.HTTPSPort)
		}
	}

	if liveStatus.State == StatusRunning {
		tracker.Success(fmt.Sprintf("Site '%s' started. Reachable at %s", site.Name, siteURL))
	} else {
		tracker.Warn(fmt.Sprintf("Site '%s' started, but probe reported: %s (%s)", site.Name, liveStatus.State, liveStatus.Detail))
	}

	return nil
}
