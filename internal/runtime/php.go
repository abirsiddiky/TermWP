package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"termwp/internal/config"
	"termwp/internal/system"
	"termwp/internal/ui"
)

// PHPManager handles PHP installations, configurations, and FastCGI/FPM daemon processes.
type PHPManager struct {
	cfg *config.GlobalConfig
}

// NewPHPManager creates an initialized PHPManager.
func NewPHPManager(cfg *config.GlobalConfig) *PHPManager {
	return &PHPManager{cfg: cfg}
}

// GetPort returns the assigned FastCGI port for a PHP version (e.g. 9084 for 8.4).
func (m *PHPManager) GetPort(version string) int {
	if m.cfg != nil && m.cfg.PHPPorts != nil {
		if p, ok := m.cfg.PHPPorts[version]; ok && p > 0 {
			return p
		}
	}
	switch version {
	case "8.3":
		return 9083
	case "8.4":
		return 9084
	case "8.5":
		return 9085
	default:
		return 9084
	}
}

// GetCLIPath returns the executable path for running PHP CLI scripts (like WP-CLI).
func (m *PHPManager) GetCLIPath(version string) string {
	base := config.PHPRuntimeDir(version)
	if runtime.GOOS == "windows" {
		return filepath.Join(base, "php.exe")
	}
	// On Linux static-php-cli, binary is at bin/php or php
	candidate := filepath.Join(base, "bin", "php")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return filepath.Join(base, "php")
}

// GetDaemonPath returns the FastCGI / FPM executable.
func (m *PHPManager) GetDaemonPath(version string) string {
	base := config.PHPRuntimeDir(version)
	if runtime.GOOS == "windows" {
		return filepath.Join(base, "php-cgi.exe")
	}
	// On Linux, look for sbin/php-fpm or bin/php-fpm or php-fpm
	candidates := []string{
		filepath.Join(base, "sbin", "php-fpm"),
		filepath.Join(base, "bin", "php-fpm"),
		filepath.Join(base, "php-fpm"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return candidates[0]
}

// PIDFilePath returns the path to the PID file for this PHP version.
func (m *PHPManager) PIDFilePath(version string) string {
	return filepath.Join(config.LogsDir(), fmt.Sprintf("php_%s.pid", version))
}

// LogFilePath returns the path to the PHP error log file.
func (m *PHPManager) LogFilePath(version string) string {
	return filepath.Join(config.LogsDir(), fmt.Sprintf("php_%s_error.log", version))
}

// IniFilePath returns the path to php.ini for this version.
func (m *PHPManager) IniFilePath(version string) string {
	return filepath.Join(config.PHPRuntimeDir(version), "php.ini")
}

// EnsurePHPInstalled verifies if PHP binaries exist or downloads them.
func (m *PHPManager) EnsurePHPInstalled(version string, tracker *ui.StepTracker) error {
	cli := m.GetCLIPath(version)
	if _, err := os.Stat(cli); err == nil {
		return m.EnsurePHPIni(version)
	}

	src, err := config.GetPHPSource(version, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}

	dest := config.PHPRuntimeDir(version)
	downloader := NewDownloader(tracker)
	if err := downloader.FetchAndInstall(src, dest); err != nil {
		return fmt.Errorf("failed installing PHP %s: %w", version, err)
	}

	return m.EnsurePHPIni(version)
}

// EnsurePHPIni generates an optimized development php.ini for WordPress.
func (m *PHPManager) EnsurePHPIni(version string) error {
	iniPath := m.IniFilePath(version)
	errorLogPath := filepath.ToSlash(m.LogFilePath(version))

	var sb strings.Builder
	sb.WriteString("; TermWP generated development php.ini for WordPress\n")
	sb.WriteString("[PHP]\n")
	sb.WriteString("memory_limit = 512M\n")
	sb.WriteString("upload_max_filesize = 256M\n")
	sb.WriteString("post_max_size = 256M\n")
	sb.WriteString("max_execution_time = 300\n")
	sb.WriteString("max_input_time = 300\n")
	sb.WriteString("display_errors = On\n")
	sb.WriteString("display_startup_errors = On\n")
	sb.WriteString("log_errors = On\n")
	sb.WriteString(fmt.Sprintf("error_log = \"%s\"\n", errorLogPath))
	sb.WriteString("error_reporting = E_ALL & ~E_DEPRECATED & ~E_STRICT\n")
	sb.WriteString("date.timezone = UTC\n")
	sb.WriteString("cgi.fix_pathinfo = 0\n")

	if runtime.GOOS == "windows" {
		extDir := filepath.ToSlash(filepath.Join(config.PHPRuntimeDir(version), "ext"))
		sb.WriteString(fmt.Sprintf("extension_dir = \"%s\"\n", extDir))
		// Required WordPress extensions
		extensions := []string{
			"curl",
			"mbstring",
			"mysqli",
			"openssl",
			"fileinfo",
			"exif",
			"gd",
			"zip",
			"intl",
			"pdo_mysql",
		}
		for _, ext := range extensions {
			sb.WriteString(fmt.Sprintf("extension=%s\n", ext))
		}
		sb.WriteString("zend_extension=opcache\n")
	}

	sb.WriteString("\n[opcache]\n")
	sb.WriteString("opcache.enable = 1\n")
	sb.WriteString("opcache.enable_cli = 0\n")
	sb.WriteString("opcache.memory_consumption = 128\n")
	sb.WriteString("opcache.interned_strings_buffer = 8\n")
	sb.WriteString("opcache.max_accelerated_files = 10000\n")
	sb.WriteString("opcache.revalidate_freq = 2\n")

	return os.WriteFile(iniPath, []byte(sb.String()), 0644)
}

// EnsureLinuxFpmConf generates a php-fpm.conf configuration file on Linux.
func (m *PHPManager) EnsureLinuxFpmConf(version string) (string, error) {
	confPath := filepath.Join(config.PHPRuntimeDir(version), "php-fpm.conf")
	port := m.GetPort(version)
	pidPath := filepath.ToSlash(m.PIDFilePath(version))
	errorLogPath := filepath.ToSlash(m.LogFilePath(version))

	content := fmt.Sprintf(`[global]
pid = %s
error_log = %s
daemonize = no

[www]
listen = 127.0.0.1:%d
listen.backlog = 511
pm = dynamic
pm.max_children = 20
pm.start_servers = 2
pm.min_spare_servers = 1
pm.max_spare_servers = 3
clear_env = no
catch_workers_output = yes
`, pidPath, errorLogPath, port)

	if err := os.WriteFile(confPath, []byte(content), 0644); err != nil {
		return "", err
	}
	return confPath, nil
}

// IsPHPRunning checks if the PHP FastCGI/FPM daemon is currently running.
func (m *PHPManager) IsPHPRunning(version string) bool {
	pid := system.ReadPIDFile(m.PIDFilePath(version))
	if pid <= 0 {
		return false
	}
	return system.IsProcessRunning(pid)
}

// StartPHP starts the FastCGI/FPM backend process for the given PHP version.
func (m *PHPManager) StartPHP(version string) error {
	if m.IsPHPRunning(version) {
		return nil
	}

	if err := m.EnsurePHPInstalled(version, nil); err != nil {
		return err
	}

	daemonPath := m.GetDaemonPath(version)
	iniPath := m.IniFilePath(version)
	port := m.GetPort(version)
	logPath := m.LogFilePath(version)

	var args []string
	if runtime.GOOS == "windows" {
		// Set PHP_FCGI_CHILDREN and MAX_REQUESTS
		_ = os.Setenv("PHP_FCGI_CHILDREN", "4")
		_ = os.Setenv("PHP_FCGI_MAX_REQUESTS", "1000")
		args = []string{"-b", fmt.Sprintf("127.0.0.1:%d", port), "-c", iniPath}
	} else {
		confPath, err := m.EnsureLinuxFpmConf(version)
		if err != nil {
			return err
		}
		args = []string{"-y", confPath, "-c", iniPath}
	}

	proc, err := system.SpawnBackgroundProcess(daemonPath, args, config.PHPRuntimeDir(version), logPath, logPath)
	if err != nil {
		return fmt.Errorf("failed starting PHP %s daemon: %w", version, err)
	}

	if err := system.WritePIDFile(m.PIDFilePath(version), proc.Pid); err != nil {
		return fmt.Errorf("failed recording PHP %s PID: %w", version, err)
	}

	// Wait up to 3s for port to start listening
	if !system.WaitForPort(port, 3*time.Second) {
		// If port didn't bind, check if process died
		if !system.IsProcessRunning(proc.Pid) {
			return fmt.Errorf("PHP %s daemon exited prematurely, check logs at %s", version, logPath)
		}
	}

	return nil
}

// StopPHP stops the PHP backend process.
func (m *PHPManager) StopPHP(version string, force bool) (bool, error) {
	pidFile := m.PIDFilePath(version)
	pid := system.ReadPIDFile(pidFile)
	if pid <= 0 || !system.IsProcessRunning(pid) {
		system.RemovePIDFile(pidFile)
		return false, nil // Already stopped
	}

	if force {
		_ = system.KillProcessTree(pid)
		system.RemovePIDFile(pidFile)
		return true, nil
	}

	stopped, err := system.StopDaemon(pidFile, func() error {
		// Send SIGTERM or kill process tree
		return system.KillProcessTree(pid)
	}, 3*time.Second)

	return stopped, err
}
