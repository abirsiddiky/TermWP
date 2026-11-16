package runtime

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"termwp/internal/config"
	"termwp/internal/ui"
)

// WordPressManager handles downloading WordPress core and generating wp-config.php.
type WordPressManager struct{}

// NewWordPressManager creates an initialized WordPressManager.
func NewWordPressManager() *WordPressManager {
	return &WordPressManager{}
}

// GenerateSalt creates a 64-character secure random salt for WordPress keys.
func GenerateSalt() string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()-_ []{}<>~`+=,.;:/?|"
	bytes := make([]byte, 64)
	if _, err := rand.Read(bytes); err != nil {
		return config.GenerateRandomPassword(64)
	}
	var sb strings.Builder
	for _, b := range bytes {
		sb.WriteByte(chars[int(b)%len(chars)])
	}
	return sb.String()
}

// EnsureWordPressArchive downloads and caches the latest WordPress zip/tarball.
func (w *WordPressManager) EnsureWordPressArchive(tracker *ui.StepTracker) (string, error) {
	src := config.GetWordPressSource(runtime.GOOS == "windows")
	cachePath := filepath.Join(config.CacheDir(), src.Filename)

	if _, err := os.Stat(cachePath); err == nil {
		return cachePath, nil // Already cached
	}

	downloader := NewDownloader(tracker)
	if err := downloader.DownloadFile(src.URL, cachePath); err != nil {
		return "", fmt.Errorf("failed downloading WordPress core: %w", err)
	}

	return cachePath, nil
}

// ExtractToSite extracts WordPress core into the target site directory.
func (w *WordPressManager) ExtractToSite(siteDir string, tracker *ui.StepTracker) error {
	cachePath, err := w.EnsureWordPressArchive(tracker)
	if err != nil {
		return err
	}

	if tracker != nil {
		tracker.Start("Extracting WordPress files")
	}

	if err := os.MkdirAll(siteDir, 0755); err != nil {
		return err
	}

	src := config.GetWordPressSource(runtime.GOOS == "windows")
	if src.ArchiveType == config.ArchiveZip {
		if err := ExtractZip(cachePath, siteDir, 1); err != nil {
			return fmt.Errorf("failed extracting WordPress zip: %w", err)
		}
	} else {
		if err := ExtractTarGz(cachePath, siteDir, 1); err != nil {
			return fmt.Errorf("failed extracting WordPress tar.gz: %w", err)
		}
	}

	if tracker != nil {
		tracker.Success("WordPress files extracted")
	}

	return nil
}

// GenerateWPConfig writes an isolated wp-config.php file for the site.
func (w *WordPressManager) GenerateWPConfig(siteDir, dbName, dbUser, dbPass string, dbPort int) error {
	configPath := filepath.Join(siteDir, "wp-config.php")

	escapeSalt := func(s string) string {
		return strings.ReplaceAll(s, `'`, `\'`)
	}

	dbHost := fmt.Sprintf("127.0.0.1:%d", dbPort)

	content := fmt.Sprintf(`<?php
/**
 * TermWP generated wp-config.php
 */

define( 'DB_NAME',     '%s' );
define( 'DB_USER',     '%s' );
define( 'DB_PASSWORD', '%s' );
define( 'DB_HOST',     '%s' );
define( 'DB_CHARSET',  'utf8mb4' );
define( 'DB_COLLATE',  '' );

/**#@+
 * Authentication Unique Keys and Salts.
 */
define( 'AUTH_KEY',         '%s' );
define( 'SECURE_AUTH_KEY',  '%s' );
define( 'LOGGED_IN_KEY',    '%s' );
define( 'NONCE_KEY',        '%s' );
define( 'AUTH_SALT',        '%s' );
define( 'SECURE_AUTH_SALT', '%s' );
define( 'LOGGED_IN_SALT',   '%s' );
define( 'NONCE_SALT',       '%s' );
/**#@-*/

$table_prefix = 'wp_';

define( 'WP_DEBUG', true );
define( 'WP_DEBUG_LOG', false );
define( 'WP_DEBUG_DISPLAY', true );
define( 'SCRIPT_DEBUG', true );

define( 'FS_METHOD', 'direct' );

/* That's all, stop editing! Happy publishing. */

if ( ! defined( 'ABSPATH' ) ) {
	define( 'ABSPATH', __DIR__ . '/' );
}

require_once ABSPATH . 'wp-settings.php';
`,
		dbName, dbUser, dbPass, dbHost,
		escapeSalt(GenerateSalt()),
		escapeSalt(GenerateSalt()),
		escapeSalt(GenerateSalt()),
		escapeSalt(GenerateSalt()),
		escapeSalt(GenerateSalt()),
		escapeSalt(GenerateSalt()),
		escapeSalt(GenerateSalt()),
		escapeSalt(GenerateSalt()),
	)

	return os.WriteFile(configPath, []byte(content), 0644)
}
