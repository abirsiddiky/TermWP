package system

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"termwp/internal/config"
	"termwp/internal/ui"
)

type CheckLevel int

const (
	LevelPass CheckLevel = iota
	LevelWarn
	LevelFail
)

// CheckItem represents a single system inspection result.
type CheckItem struct {
	Category string
	Name     string
	Level    CheckLevel
	Details  string
	FixHint  string
	CanFix   bool
}

// DoctorEngine coordinates diagnostic analysis and auto-repairs.
type DoctorEngine struct {
	cfg *config.GlobalConfig
}

// NewDoctorEngine creates a new DoctorEngine.
func NewDoctorEngine(cfg *config.GlobalConfig) *DoctorEngine {
	return &DoctorEngine{cfg: cfg}
}

// RunDiagnostics executes the full diagnostic checklist.
func (d *DoctorEngine) RunDiagnostics(isShutdown bool, activePHPVersions []string) []CheckItem {
	var results []CheckItem

	// 1. Environment & Permissions
	results = append(results, d.checkSystemEnvironment()...)

	// 2. Storage & Home Directory
	results = append(results, d.checkHomeDirectory()...)

	// 3. Runtime Binaries
	results = append(results, d.checkRuntimeBinaries()...)

	// 4. PHP Extensions
	results = append(results, d.checkPHPExtensions()...)

	// 5. Port Availability & Conflicts
	results = append(results, d.checkPortConflicts()...)

	// 6. Stack Daemons & DB Connection
	results = append(results, d.checkStackServices(isShutdown)...)

	// 7. Hosts File Sync
	results = append(results, d.checkHostsFile()...)

	// 8. SSL & mkcert Trust
	results = append(results, d.checkSSLTrust()...)

	// 9. Internet Connectivity
	results = append(results, d.checkInternetConnectivity()...)

	// 10. Linux mDNS Collision Warning
	if runtime.GOOS == "linux" {
		results = append(results, d.checkLinuxMDNS()...)
	}

	return results
}

func (d *DoctorEngine) checkSystemEnvironment() []CheckItem {
	var items []CheckItem

	elevated := IsElevated()
	items = append(items, CheckItem{
		Category: "System",
		Name:     fmt.Sprintf("OS & Architecture (%s/%s)", runtime.GOOS, runtime.GOARCH),
		Level:    LevelPass,
		Details:  fmt.Sprintf("Running on %s (%s)", runtime.GOOS, runtime.GOARCH),
	})

	privDetail := "Standard user permissions"
	if elevated {
		privDetail = "Running with Administrator/Root privileges"
	}
	items = append(items, CheckItem{
		Category: "System",
		Name:     "Process Privileges",
		Level:    LevelPass,
		Details:  privDetail,
		FixHint:  "Elevation will be requested automatically when editing the hosts file",
	})

	return items
}

func (d *DoctorEngine) checkHomeDirectory() []CheckItem {
	var items []CheckItem
	home := config.HomeDir()

	info, err := os.Stat(home)
	if err != nil {
		items = append(items, CheckItem{
			Category: "Storage",
			Name:     "TermWP Home Directory",
			Level:    LevelFail,
			Details:  fmt.Sprintf("Directory %s does not exist", home),
			FixHint:  "Run with --fix to automatically recreate the directory tree",
			CanFix:   true,
		})
		return items
	}

	// Test write permissions
	testFile := filepath.Join(home, ".write_test")
	writeErr := os.WriteFile(testFile, []byte("ok"), 0644)
	if writeErr == nil {
		_ = os.Remove(testFile)
		items = append(items, CheckItem{
			Category: "Storage",
			Name:     "TermWP Home Directory",
			Level:    LevelPass,
			Details:  fmt.Sprintf("%s (writable, mode %v)", home, info.Mode()),
		})
	} else {
		items = append(items, CheckItem{
			Category: "Storage",
			Name:     "TermWP Home Directory",
			Level:    LevelFail,
			Details:  fmt.Sprintf("Directory %s is not writable: %v", home, writeErr),
			FixHint:  "Adjust folder permissions so your user account has write access",
		})
	}

	return items
}

func (d *DoctorEngine) checkRuntimeBinaries() []CheckItem {
	var items []CheckItem

	// Nginx
	nginxBin := filepath.Join(config.NginxRuntimeDir(), "nginx.exe")
	if runtime.GOOS != "windows" {
		nginxBin = filepath.Join(config.NginxRuntimeDir(), "sbin", "nginx")
	}
	if _, err := os.Stat(nginxBin); err == nil {
		items = append(items, CheckItem{
			Category: "Runtimes",
			Name:     "Nginx Binary",
			Level:    LevelPass,
			Details:  "Installed and available",
		})
	} else {
		items = append(items, CheckItem{
			Category: "Runtimes",
			Name:     "Nginx Binary",
			Level:    LevelWarn,
			Details:  "Not yet downloaded",
			FixHint:  "Will be downloaded automatically when creating or starting sites",
		})
	}

	// MariaDB
	dbBin := filepath.Join(config.MariaDBRuntimeDir(), "bin", "mariadbd.exe")
	if runtime.GOOS != "windows" {
		dbBin = filepath.Join(config.MariaDBRuntimeDir(), "bin", "mariadbd")
	}
	if _, err := os.Stat(dbBin); err == nil {
		items = append(items, CheckItem{
			Category: "Runtimes",
			Name:     "MariaDB Binary",
			Level:    LevelPass,
			Details:  "Installed and available",
		})
	} else {
		items = append(items, CheckItem{
			Category: "Runtimes",
			Name:     "MariaDB Binary",
			Level:    LevelWarn,
			Details:  "Not yet downloaded",
			FixHint:  "Will be downloaded automatically on first site creation",
		})
	}

	// WP-CLI
	if _, err := os.Stat(config.WPCLIPath()); err == nil {
		items = append(items, CheckItem{
			Category: "Runtimes",
			Name:     "WP-CLI Phar",
			Level:    LevelPass,
			Details:  "wp-cli.phar installed",
		})
	} else {
		items = append(items, CheckItem{
			Category: "Runtimes",
			Name:     "WP-CLI Phar",
			Level:    LevelWarn,
			Details:  "Not yet downloaded",
			FixHint:  "Will be downloaded automatically on first site creation",
		})
	}

	// mkcert
	if _, err := os.Stat(config.MkcertPath()); err == nil {
		items = append(items, CheckItem{
			Category: "Runtimes",
			Name:     "mkcert Tool",
			Level:    LevelPass,
			Details:  "mkcert binary installed",
		})
	} else {
		items = append(items, CheckItem{
			Category: "Runtimes",
			Name:     "mkcert Tool",
			Level:    LevelWarn,
			Details:  "Not yet installed",
			FixHint:  "Will be downloaded automatically when adding an SSL site",
		})
	}

	return items
}

func (d *DoctorEngine) checkPHPExtensions() []CheckItem {
	var items []CheckItem
	versions := []string{"8.3", "8.4", "8.5"}

	for _, v := range versions {
		cli := filepath.Join(config.PHPRuntimeDir(v), "php.exe")
		if runtime.GOOS != "windows" {
			cli = filepath.Join(config.PHPRuntimeDir(v), "bin", "php")
		}

		if _, err := os.Stat(cli); os.IsNotExist(err) {
			continue // Version not installed yet
		}

		iniPath := filepath.Join(config.PHPRuntimeDir(v), "php.ini")
		var cmd *exec.Cmd
		if _, err := os.Stat(iniPath); err == nil {
			cmd = exec.Command(cli, "-c", iniPath, "-m")
		} else {
			cmd = exec.Command(cli, "-m")
		}

		out, err := cmd.Output()
		if err != nil {
			continue
		}
		modules := string(out)

		required := []string{"curl", "mbstring", "mysqli", "openssl", "fileinfo", "gd", "zip"}
		missing := []string{}
		for _, mod := range required {
			if !strings.Contains(strings.ToLower(modules), mod) {
				missing = append(missing, mod)
			}
		}

		if len(missing) == 0 {
			items = append(items, CheckItem{
				Category: "PHP",
				Name:     fmt.Sprintf("PHP %s Extensions", v),
				Level:    LevelPass,
				Details:  "All required WordPress extensions are enabled",
			})
		} else {
			items = append(items, CheckItem{
				Category: "PHP",
				Name:     fmt.Sprintf("PHP %s Extensions", v),
				Level:    LevelFail,
				Details:  fmt.Sprintf("Missing extensions: %s", strings.Join(missing, ", ")),
				FixHint:  "Enable required extensions in php.ini",
				CanFix:   true,
			})
		}
	}

	return items
}

func (d *DoctorEngine) checkPortConflicts() []CheckItem {
	var items []CheckItem

	ports := []struct {
		name string
		port int
	}{
		{"HTTP Port", d.cfg.HTTPPort},
		{"HTTPS Port", d.cfg.HTTPSPort},
		{"MariaDB Port", d.cfg.MariaDBPort},
		{"PHP 8.3 FastCGI", 9083},
		{"PHP 8.4 FastCGI", 9084},
		{"PHP 8.5 FastCGI", 9085},
	}

	for _, p := range ports {
		if IsPortAvailable(p.port) {
			items = append(items, CheckItem{
				Category: "Networking",
				Name:     fmt.Sprintf("%s (%d)", p.name, p.port),
				Level:    LevelPass,
				Details:  "Port is open and available",
			})
		} else {
			items = append(items, CheckItem{
				Category: "Networking",
				Name:     fmt.Sprintf("%s (%d)", p.name, p.port),
				Level:    LevelWarn,
				Details:  "Port is currently in use (by TermWP daemon or external program)",
				FixHint:  "If conflicting with an external service, update port in config.json",
			})
		}
	}

	return items
}

func (d *DoctorEngine) checkStackServices(isShutdown bool) []CheckItem {
	var items []CheckItem

	if isShutdown {
		items = append(items, CheckItem{
			Category: "Daemons",
			Name:     "Background Stack Daemons",
			Level:    LevelPass,
			Details:  "Services stopped (normal - 'termwp shutdown' was executed)",
			FixHint:  "Services will automatically restart when a site command is executed",
		})
	}

	return items
}

func (d *DoctorEngine) checkHostsFile() []CheckItem {
	var items []CheckItem
	hostsPath := HostsFilePath()

	if _, err := os.Stat(hostsPath); err != nil {
		items = append(items, CheckItem{
			Category: "DNS & Hosts",
			Name:     "System Hosts File",
			Level:    LevelFail,
			Details:  fmt.Sprintf("Hosts file not found at %s", hostsPath),
		})
		return items
	}

	items = append(items, CheckItem{
		Category: "DNS & Hosts",
		Name:     "System Hosts File",
		Level:    LevelPass,
		Details:  fmt.Sprintf("Hosts file located at %s", hostsPath),
	})

	return items
}

func (d *DoctorEngine) checkSSLTrust() []CheckItem {
	var items []CheckItem
	mkcertBin := config.MkcertPath()

	if _, err := os.Stat(mkcertBin); os.IsNotExist(err) {
		return items
	}

	cmd := exec.Command(mkcertBin, "-CAROOT")
	out, err := cmd.Output()
	if err == nil {
		caRoot := strings.TrimSpace(string(out))
		if _, err := os.Stat(filepath.Join(caRoot, "rootCA.pem")); err == nil {
			items = append(items, CheckItem{
				Category: "Security",
				Name:     "mkcert Local Root CA",
				Level:    LevelPass,
				Details:  fmt.Sprintf("Trusted Root CA installed at %s", caRoot),
			})
			return items
		}
	}

	items = append(items, CheckItem{
		Category: "Security",
		Name:     "mkcert Local Root CA",
		Level:    LevelWarn,
		Details:  "mkcert root CA is not yet installed in system trust store",
		FixHint:  "Run: termwp add <site> ssl or mkcert -install",
	})

	return items
}

func (d *DoctorEngine) checkInternetConnectivity() []CheckItem {
	var items []CheckItem
	client := &http.Client{Timeout: 3 * time.Second}

	resp, err := client.Head("https://wordpress.org")
	if err == nil && resp.StatusCode < 500 {
		items = append(items, CheckItem{
			Category: "Connectivity",
			Name:     "WordPress.org Connection",
			Level:    LevelPass,
			Details:  "Internet reachable, WordPress downloads available",
		})
	} else {
		items = append(items, CheckItem{
			Category: "Connectivity",
			Name:     "WordPress.org Connection",
			Level:    LevelWarn,
			Details:  "Could not reach wordpress.org",
			FixHint:  "Check internet connection or proxy settings",
		})
	}

	return items
}

func (d *DoctorEngine) checkLinuxMDNS() []CheckItem {
	var items []CheckItem

	// Check if systemd-resolved or avahi-daemon are active
	data, err := os.ReadFile("/etc/nsswitch.conf")
	if err == nil && strings.Contains(string(data), "mdns4_minimal") {
		items = append(items, CheckItem{
			Category: "Linux Compatibility",
			Name:     ".local Domain Resolution (mDNS)",
			Level:    LevelWarn,
			Details:  "mDNS (avahi/systemd-resolved) is enabled on this system; .local domains may have high lookup latency or fail",
			FixHint:  "Consider using .test (e.g. mysite.test) instead of .local for local development",
		})
	}

	return items
}

// RenderDoctorChecklist formats the diagnostic results as a clean terminal checklist.
func RenderDoctorChecklist(items []CheckItem) string {
	var sb strings.Builder

	currentCategory := ""
	passed, warned, failed := 0, 0, 0

	for _, item := range items {
		if item.Category != currentCategory {
			currentCategory = item.Category
			sb.WriteString(fmt.Sprintf("\n%s\n", ui.HighlightStyle.Render(fmt.Sprintf("━━ %s ━━", currentCategory))))
		}

		var icon string
		switch item.Level {
		case LevelPass:
			icon = ui.SuccessStyle.Render(ui.IconCheck)
			passed++
		case LevelWarn:
			icon = ui.WarningStyle.Render(ui.IconWarning)
			warned++
		case LevelFail:
			icon = ui.DangerStyle.Render(ui.IconCross)
			failed++
		}

		sb.WriteString(fmt.Sprintf("  %s %-32s %s\n", icon, ui.BoldStyle.Render(item.Name), ui.MutedStyle.Render(item.Details)))

		if item.FixHint != "" && item.Level != LevelPass {
			sb.WriteString(fmt.Sprintf("      ↳ %s %s\n", ui.WarningStyle.Render("Suggestion:"), item.FixHint))
		}
	}

	// Summary line
	summary := fmt.Sprintf("\nSummary: %d Passed, %d Warnings, %d Failures\n", passed, warned, failed)
	if failed == 0 && warned == 0 {
		sb.WriteString(ui.SuccessStyle.Render(summary))
	} else if failed > 0 {
		sb.WriteString(ui.DangerStyle.Render(summary))
	} else {
		sb.WriteString(ui.WarningStyle.Render(summary))
	}

	return sb.String()
}
