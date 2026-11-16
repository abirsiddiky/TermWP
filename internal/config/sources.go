package config

import (
	"fmt"
	"runtime"
)

// ArchiveType represents the packaging format of the download.
type ArchiveType string

const (
	ArchiveZip    ArchiveType = "zip"
	ArchiveTarGz  ArchiveType = "tar.gz"
	ArchiveBinary ArchiveType = "binary"
)

// DownloadSource describes a downloadable portable runtime component.
type DownloadSource struct {
	Name        string      `json:"name"`
	Version     string      `json:"version"`
	URL         string      `json:"url"`
	SHA256      string      `json:"sha256,omitempty"`
	Filename    string      `json:"filename"`
	ArchiveType ArchiveType `json:"archive_type"`
	StripDirs   int         `json:"strip_dirs,omitempty"`
}

// Registry containing all download sources for Windows and Linux (amd64/arm64).
// All URLs and versions are defined here for single-point maintenance.
var (
	// WordPress Core
	WordPressSourceZip = DownloadSource{
		Name:        "wordpress",
		Version:     "latest",
		URL:         "https://wordpress.org/latest.zip",
		Filename:    "wordpress-latest.zip",
		ArchiveType: ArchiveZip,
		StripDirs:   1, // extracts to wordpress/
	}

	WordPressSourceTarGz = DownloadSource{
		Name:        "wordpress",
		Version:     "latest",
		URL:         "https://wordpress.org/latest.tar.gz",
		Filename:    "wordpress-latest.tar.gz",
		ArchiveType: ArchiveTarGz,
		StripDirs:   1,
	}

	// WP-CLI phar
	WPCLISource = DownloadSource{
		Name:        "wp-cli",
		Version:     "latest",
		URL:         "https://raw.githubusercontent.com/wp-cli/builds/gh-pages/phar/wp-cli.phar",
		Filename:    "wp-cli.phar",
		ArchiveType: ArchiveBinary,
	}

	// mkcert releases
	MkcertSources = map[string]DownloadSource{
		"windows/amd64": {
			Name:        "mkcert",
			Version:     "v1.4.4",
			URL:         "https://github.com/FiloSottile/mkcert/releases/download/v1.4.4/mkcert-v1.4.4-windows-amd64.exe",
			Filename:    "mkcert.exe",
			ArchiveType: ArchiveBinary,
		},
		"linux/amd64": {
			Name:        "mkcert",
			Version:     "v1.4.4",
			URL:         "https://github.com/FiloSottile/mkcert/releases/download/v1.4.4/mkcert-v1.4.4-linux-amd64",
			Filename:    "mkcert",
			ArchiveType: ArchiveBinary,
		},
		"linux/arm64": {
			Name:        "mkcert",
			Version:     "v1.4.4",
			URL:         "https://github.com/FiloSottile/mkcert/releases/download/v1.4.4/mkcert-v1.4.4-linux-arm64",
			Filename:    "mkcert",
			ArchiveType: ArchiveBinary,
		},
	}

	// Nginx releases
	NginxSources = map[string]DownloadSource{
		"windows/amd64": {
			Name:        "nginx",
			Version:     "1.26.3",
			URL:         "https://nginx.org/download/nginx-1.26.3.zip",
			Filename:    "nginx-1.26.3.zip",
			ArchiveType: ArchiveZip,
			StripDirs:   1,
		},
		"linux/amd64": {
			Name:        "nginx",
			Version:     "1.26.3",
			URL:         "https://github.com/termwp-binaries/nginx-portable/releases/download/v1.26.3/nginx-1.26.3-linux-amd64.tar.gz",
			Filename:    "nginx-1.26.3-linux-amd64.tar.gz",
			ArchiveType: ArchiveTarGz,
			StripDirs:   1,
		},
		"linux/arm64": {
			Name:        "nginx",
			Version:     "1.26.3",
			URL:         "https://github.com/termwp-binaries/nginx-portable/releases/download/v1.26.3/nginx-1.26.3-linux-arm64.tar.gz",
			Filename:    "nginx-1.26.3-linux-arm64.tar.gz",
			ArchiveType: ArchiveTarGz,
			StripDirs:   1,
		},
	}

	// MariaDB releases (LTS 11.4)
	MariaDBSources = map[string]DownloadSource{
		"windows/amd64": {
			Name:        "mariadb",
			Version:     "11.4.5",
			URL:         "https://archive.mariadb.org/mariadb-11.4.5/winx64-packages/mariadb-11.4.5-winx64.zip",
			Filename:    "mariadb-11.4.5-winx64.zip",
			ArchiveType: ArchiveZip,
			StripDirs:   1,
		},
		"linux/amd64": {
			Name:        "mariadb",
			Version:     "11.4.5",
			URL:         "https://archive.mariadb.org/mariadb-11.4.5/bintar-linux-systemd-x86_64/mariadb-11.4.5-linux-systemd-x86_64.tar.gz",
			Filename:    "mariadb-11.4.5-linux-systemd-x86_64.tar.gz",
			ArchiveType: ArchiveTarGz,
			StripDirs:   1,
		},
		"linux/arm64": {
			Name:        "mariadb",
			Version:     "11.4.5",
			URL:         "https://archive.mariadb.org/mariadb-11.4.5/bintar-linux-systemd-aarch64/mariadb-11.4.5-linux-systemd-aarch64.tar.gz",
			Filename:    "mariadb-11.4.5-linux-systemd-aarch64.tar.gz",
			ArchiveType: ArchiveTarGz,
			StripDirs:   1,
		},
	}

	// PHP builds (8.3, 8.4, 8.5)
	// Windows uses official NTS x64 builds; Linux uses static-php-cli / dl.static-php.dev FPM builds with all required WP extensions.
	PHPSources = map[string]map[string]DownloadSource{
		"8.3": {
			"windows/amd64": {
				Name:        "php-8.3",
				Version:     "8.3.17",
				URL:         "https://windows.php.net/downloads/releases/archives/php-8.3.17-nts-Win32-vs16-x64.zip",
				Filename:    "php-8.3.17-nts-Win32-vs16-x64.zip",
				ArchiveType: ArchiveZip,
				StripDirs:   0,
			},
			"linux/amd64": {
				Name:        "php-8.3",
				Version:     "8.3.17",
				URL:         "https://dl.static-php.dev/static-php-cli/bulk/php-8.3.17-fpm-linux-x86_64.tar.gz",
				Filename:    "php-8.3.17-fpm-linux-x86_64.tar.gz",
				ArchiveType: ArchiveTarGz,
				StripDirs:   0,
			},
			"linux/arm64": {
				Name:        "php-8.3",
				Version:     "8.3.17",
				URL:         "https://dl.static-php.dev/static-php-cli/bulk/php-8.3.17-fpm-linux-aarch64.tar.gz",
				Filename:    "php-8.3.17-fpm-linux-aarch64.tar.gz",
				ArchiveType: ArchiveTarGz,
				StripDirs:   0,
			},
		},
		"8.4": {
			"windows/amd64": {
				Name:        "php-8.4",
				Version:     "8.4.4",
				URL:         "https://windows.php.net/downloads/releases/archives/php-8.4.4-nts-Win32-vs17-x64.zip",
				Filename:    "php-8.4.4-nts-Win32-vs17-x64.zip",
				ArchiveType: ArchiveZip,
				StripDirs:   0,
			},
			"linux/amd64": {
				Name:        "php-8.4",
				Version:     "8.4.4",
				URL:         "https://dl.static-php.dev/static-php-cli/bulk/php-8.4.4-fpm-linux-x86_64.tar.gz",
				Filename:    "php-8.4.4-fpm-linux-x86_64.tar.gz",
				ArchiveType: ArchiveTarGz,
				StripDirs:   0,
			},
			"linux/arm64": {
				Name:        "php-8.4",
				Version:     "8.4.4",
				URL:         "https://dl.static-php.dev/static-php-cli/bulk/php-8.4.4-fpm-linux-aarch64.tar.gz",
				Filename:    "php-8.4.4-fpm-linux-aarch64.tar.gz",
				ArchiveType: ArchiveTarGz,
				StripDirs:   0,
			},
		},
		"8.5": {
			"windows/amd64": {
				Name:        "php-8.5",
				Version:     "8.5.0-dev",
				URL:         "https://windows.php.net/downloads/snaps/master/latest/php-master-nts-windows-vs17-x64.zip",
				Filename:    "php-8.5-dev-nts-Win32-vs17-x64.zip",
				ArchiveType: ArchiveZip,
				StripDirs:   0,
			},
			"linux/amd64": {
				Name:        "php-8.5",
				Version:     "8.5.0-dev",
				URL:         "https://dl.static-php.dev/static-php-cli/bulk/php-8.5.0-dev-fpm-linux-x86_64.tar.gz",
				Filename:    "php-8.5.0-dev-fpm-linux-x86_64.tar.gz",
				ArchiveType: ArchiveTarGz,
				StripDirs:   0,
			},
			"linux/arm64": {
				Name:        "php-8.5",
				Version:     "8.5.0-dev",
				URL:         "https://dl.static-php.dev/static-php-cli/bulk/php-8.5.0-dev-fpm-linux-aarch64.tar.gz",
				Filename:    "php-8.5.0-dev-fpm-linux-aarch64.tar.gz",
				ArchiveType: ArchiveTarGz,
				StripDirs:   0,
			},
		},
	}
)

// CurrentPlatformKey returns os/arch key e.g. "windows/amd64" or "linux/amd64".
func CurrentPlatformKey() string {
	return runtime.GOOS + "/" + runtime.GOARCH
}

// GetNginxSource returns the nginx download source for the target platform.
func GetNginxSource(goos, goarch string) (*DownloadSource, error) {
	key := goos + "/" + goarch
	src, ok := NginxSources[key]
	if !ok {
		return nil, fmt.Errorf("unsupported platform for nginx: %s", key)
	}
	return &src, nil
}

// GetMariaDBSource returns the MariaDB download source for the target platform.
func GetMariaDBSource(goos, goarch string) (*DownloadSource, error) {
	key := goos + "/" + goarch
	src, ok := MariaDBSources[key]
	if !ok {
		return nil, fmt.Errorf("unsupported platform for mariadb: %s", key)
	}
	return &src, nil
}

// GetPHPSource returns the PHP download source for the given version and target platform.
func GetPHPSource(version, goos, goarch string) (*DownloadSource, error) {
	versions, ok := PHPSources[version]
	if !ok {
		return nil, fmt.Errorf("unsupported PHP version: %s (supported: 8.3, 8.4, 8.5)", version)
	}
	key := goos + "/" + goarch
	src, ok := versions[key]
	if !ok {
		return nil, fmt.Errorf("unsupported platform for PHP %s: %s", version, key)
	}
	return &src, nil
}

// GetMkcertSource returns the mkcert binary download source.
func GetMkcertSource(goos, goarch string) (*DownloadSource, error) {
	key := goos + "/" + goarch
	src, ok := MkcertSources[key]
	if !ok {
		return nil, fmt.Errorf("unsupported platform for mkcert: %s", key)
	}
	return &src, nil
}

// GetWPCLISource returns the WP-CLI download source.
func GetWPCLISource() *DownloadSource {
	return &WPCLISource
}

// GetWordPressSource returns the WordPress archive download source.
func GetWordPressSource(preferZip bool) *DownloadSource {
	if preferZip || runtime.GOOS == "windows" {
		return &WordPressSourceZip
	}
	return &WordPressSourceTarGz
}
