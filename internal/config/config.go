package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// AppName is the application name.
const AppName = "TermWP"

// GlobalConfig represents persistent global settings for TermWP.
type GlobalConfig struct {
	MariaDBRootPassword string         `json:"mariadb_root_password"`
	HTTPPort            int            `json:"http_port"`
	HTTPSPort           int            `json:"https_port"`
	MariaDBPort         int            `json:"mariadb_port"`
	DefaultPHP          string         `json:"default_php"`
	PHPPorts            map[string]int `json:"php_ports"`
}

var (
	homeDirOnce sync.Once
	cachedHome  string
)

// GenerateRandomPassword produces a secure hex string of given length.
func GenerateRandomPassword(length int) string {
	bytes := make([]byte, length/2+1)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback deterministic if entropy failure (extremely rare)
		return "twp_pass_" + fmt.Sprintf("%d", os.Getpid())
	}
	s := hex.EncodeToString(bytes)
	if len(s) > length {
		return s[:length]
	}
	return s
}

// HomeDir returns the root TermWP directory for the current OS.
// Windows: C:\Users\<user>\AppData\Local\TermWP\
// Linux:   ~/.termwp/
// Can be overridden by the TERMWP_HOME environment variable.
func HomeDir() string {
	homeDirOnce.Do(func() {
		if env := os.Getenv("TERMWP_HOME"); env != "" {
			cachedHome = filepath.Clean(env)
			return
		}

		if runtime.GOOS == "windows" {
			localAppData := os.Getenv("LOCALAPPDATA")
			if localAppData != "" {
				cachedHome = filepath.Join(localAppData, AppName)
				return
			}
			userProfile := os.Getenv("USERPROFILE")
			if userProfile != "" {
				cachedHome = filepath.Join(userProfile, "AppData", "Local", AppName)
				return
			}
		}

		userHome, err := os.UserHomeDir()
		if err != nil {
			cachedHome = filepath.Join(os.TempDir(), ".termwp")
			return
		}

		if runtime.GOOS == "windows" {
			cachedHome = filepath.Join(userHome, "AppData", "Local", AppName)
		} else {
			cachedHome = filepath.Join(userHome, ".termwp")
		}
	})
	return cachedHome
}

// ResetHomeDirCache resets the cached home dir (primarily for unit testing).
func ResetHomeDirCache() {
	cachedHome = ""
	homeDirOnce = sync.Once{}
}

// Directory path helpers
func BinDir() string             { return filepath.Join(HomeDir(), "bin") }
func RuntimeDir() string         { return filepath.Join(HomeDir(), "runtime") }
func NginxRuntimeDir() string    { return filepath.Join(RuntimeDir(), "nginx") }
func PHPBaseDir() string         { return filepath.Join(RuntimeDir(), "php") }
func PHPRuntimeDir(v string) string { return filepath.Join(PHPBaseDir(), v) }
func MariaDBRuntimeDir() string  { return filepath.Join(RuntimeDir(), "mariadb") }
func ToolsDir() string           { return filepath.Join(RuntimeDir(), "tools") }
func CacheDir() string           { return filepath.Join(RuntimeDir(), "cache") }
func SitesDir() string           { return filepath.Join(HomeDir(), "sites") }
func SiteDir(name string) string { return filepath.Join(SitesDir(), strings.ToLower(name)) }
func ConfDir() string            { return filepath.Join(HomeDir(), "conf") }
func NginxConfDir() string       { return filepath.Join(ConfDir(), "nginx") }
func NginxSitesConfDir() string  { return filepath.Join(NginxConfDir(), "sites") }
func CertsDir() string           { return filepath.Join(HomeDir(), "certs") }
func DataDir() string            { return filepath.Join(HomeDir(), "data") }
func MariaDBDataDir() string     { return filepath.Join(DataDir(), "mariadb") }
func BackupsDir() string         { return filepath.Join(DataDir(), "backups") }
func SiteBackupsDir(name string) string { return filepath.Join(BackupsDir(), strings.ToLower(name)) }
func LogsDir() string            { return filepath.Join(HomeDir(), "logs") }
func StateFilePath() string      { return filepath.Join(HomeDir(), "state.json") }
func StateLockPath() string      { return filepath.Join(HomeDir(), "state.lock") }
func ConfigFilePath() string     { return filepath.Join(HomeDir(), "config.json") }

// Tool binary paths
func WPCLIPath() string {
	return filepath.Join(ToolsDir(), "wp-cli.phar")
}

func MkcertPath() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(ToolsDir(), "mkcert.exe")
	}
	return filepath.Join(ToolsDir(), "mkcert")
}

// EnsureDirectories ensures all primary directories in TermWP home exist.
func EnsureDirectories() error {
	dirs := []string{
		HomeDir(),
		BinDir(),
		RuntimeDir(),
		NginxRuntimeDir(),
		PHPBaseDir(),
		PHPRuntimeDir("8.3"),
		PHPRuntimeDir("8.4"),
		PHPRuntimeDir("8.5"),
		MariaDBRuntimeDir(),
		ToolsDir(),
		CacheDir(),
		SitesDir(),
		ConfDir(),
		NginxConfDir(),
		NginxSitesConfDir(),
		CertsDir(),
		DataDir(),
		MariaDBDataDir(),
		BackupsDir(),
		LogsDir(),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return nil
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *GlobalConfig {
	return &GlobalConfig{
		MariaDBRootPassword: GenerateRandomPassword(24),
		HTTPPort:            80,
		HTTPSPort:           443,
		MariaDBPort:         3306,
		DefaultPHP:          "8.4",
		PHPPorts: map[string]int{
			"8.3": 9083,
			"8.4": 9084,
			"8.5": 9085,
		},
	}
}

// LoadConfig loads config.json or initializes a new one if missing.
func LoadConfig() (*GlobalConfig, error) {
	if err := EnsureDirectories(); err != nil {
		return nil, err
	}

	path := ConfigFilePath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		cfg := DefaultConfig()
		if err := SaveConfig(cfg); err != nil {
			return nil, err
		}
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config.json: %w", err)
	}

	var cfg GlobalConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config.json: %w", err)
	}

	// Fill missing fields with defaults
	if cfg.DefaultPHP == "" {
		cfg.DefaultPHP = "8.4"
	}
	if cfg.HTTPPort == 0 {
		cfg.HTTPPort = 80
	}
	if cfg.HTTPSPort == 0 {
		cfg.HTTPSPort = 443
	}
	if cfg.MariaDBPort == 0 {
		cfg.MariaDBPort = 3306
	}
	if cfg.MariaDBRootPassword == "" {
		cfg.MariaDBRootPassword = GenerateRandomPassword(24)
		_ = SaveConfig(&cfg)
	}
	if cfg.PHPPorts == nil {
		cfg.PHPPorts = map[string]int{
			"8.3": 9083,
			"8.4": 9084,
			"8.5": 9085,
		}
	}

	return &cfg, nil
}

// SaveConfig writes the configuration to config.json atomically.
func SaveConfig(cfg *GlobalConfig) error {
	if err := EnsureDirectories(); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize config: %w", err)
	}

	tmpPath := ConfigFilePath() + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write tmp config: %w", err)
	}

	if err := os.Rename(tmpPath, ConfigFilePath()); err != nil {
		// On Windows, Rename fails if dest exists: remove and rename
		_ = os.Remove(ConfigFilePath())
		if err := os.Rename(tmpPath, ConfigFilePath()); err != nil {
			return fmt.Errorf("failed to save config.json: %w", err)
		}
	}

	return nil
}
