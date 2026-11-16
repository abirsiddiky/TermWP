# ⚡ TermWP

> **Terminal-based Isolated WordPress Development Stack**  
> Fast, portable, zero-system-pollution local WordPress development for **Windows** and **Linux** — like Laravel Herd or Local WP, but 100% terminal-native.

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Cross Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20Linux-lightgrey.svg)](#architecture)

---

## 🌟 Highlights

- **Zero System Pollution**: No `apt`, `choco`, `brew`, or Docker required. Everything lives completely inside one isolated directory (`%LOCALAPPDATA%\TermWP` on Windows, `~/.termwp` on Linux).
- **Multi-Version PHP Side-by-Side**: Run PHP 8.3, 8.4, and 8.5 concurrently. TermWP dynamically launches a PHP backend only when an active site requires it.
- **Single Nginx Reverse Proxy**: One unified, blazing-fast Nginx instance serves all sites with clean permalinks, FastCGI caching, and HTTP/2 SSL.
- **Isolated MariaDB LTS**: One shared portable MariaDB 11.4 instance with isolated databases and randomly generated credentials per site.
- **Trusted Local SSL (mkcert)**: Local CA automatically installed into Windows Cert Store and Linux system trust store; zero browser SSL warning screens.
- **Atomic Hosts File Management**: Cleanly adds and removes domain mappings inside dedicated `# TermWP BEGIN` ... `# TermWP END` blocks with automatic elevation fallback.
- **Interactive TUI & Scriptable CLI**: Interactive prompts powered by `charmbracelet/huh` and gorgeous `lipgloss` summary cards with clipboard copying. Use `--yes` to script everything.
- **Atomic Rollback Stack**: Transactional LIFO undo engine ensures that if database creation, WP-CLI download, or config writing fails halfway, every created file, DB, hosts entry, and cert is cleanly rolled back.
- **Pure Go Multi-File Log Tailer**: Live streaming log multiplexer combining WordPress `debug.log`, PHP error logs, and Nginx access/error logs with colored source tags on both Windows and Linux without relying on Unix `tail`.
- **System Doctor & Self-Healer**: Diagnostic suite (`termwp doctor --fix`) checking runtimes, port conflicts, PHP extensions, hosts synchronisation, and Linux mDNS collisions.

---

## 🏗️ Architecture

```
                   ┌────────────────────────────────────────┐
                   │          User Web Browser              │
                   └───────────────────┬────────────────────┘
                                       │ http(s)://mysite.local
                                       ▼
                   ┌────────────────────────────────────────┐
                   │      Nginx Master Reverse Proxy        │
                   │        Ports 80/443 (or 8080/8443)     │
                   └──────┬──────────────────────┬──────────┘
                          │                      │
        FastCGI (PHP 8.3) │                      │ FastCGI (PHP 8.4)
        Port 9083         │                      │ Port 9084
                          ▼                      ▼
               ┌────────────────────┐ ┌────────────────────┐
               │ PHP-FPM / php-cgi  │ │ PHP-FPM / php-cgi  │
               │      PHP 8.3       │ │      PHP 8.4       │
               └──────────┬─────────┘ └──────────┬─────────┘
                          │                      │
                          └──────────┬───────────┘
                                     │ MySQL Connection (Port 3306)
                                     ▼
                   ┌────────────────────────────────────────┐
                   │          MariaDB 11.4 LTS              │
                   │      Site-isolated DBs & Users         │
                   └────────────────────────────────────────┘
```

---

## 📂 Directory Layout

All binaries, runtimes, configurations, site files, and databases are strictly confined to the TermWP root directory:

- **Windows**: `C:\Users\<user>\AppData\Local\TermWP\`
- **Linux**: `~/.termwp/`

```
<TermWP Home>/
├── bin/
│   └── termwp                   # Main CLI/TUI executable
├── runtime/
│   ├── nginx/                   # Portable Nginx distribution
│   ├── mariadb/                 # Portable MariaDB LTS distribution
│   ├── php/
│   │   ├── 8.3/                 # PHP 8.3 (FastCGI / FPM + extensions)
│   │   ├── 8.4/                 # PHP 8.4 (FastCGI / FPM + extensions)
│   │   └── 8.5/                 # PHP 8.5 dev (FastCGI / FPM + extensions)
│   ├── tools/
│   │   ├── wp-cli.phar          # WP-CLI binary
│   │   └── mkcert               # Local certificate authority tool
│   └── cache/                   # Download cache for archives & WordPress core
├── sites/
│   └── <sitename>/              # Document root containing WordPress core
├── conf/
│   └── nginx/
│       ├── nginx.conf           # Master Nginx configuration
│       └── sites/
│           ├── <sitename>.conf  # Active site virtual host
│           └── <sitename>.conf.disabled
├── certs/
│   ├── <sitename>.pem           # TLS certificate
│   └── <sitename>-key.pem       # TLS private key
├── data/
│   ├── mariadb/                 # MariaDB data directory
│   └── backups/                 # Automatic SQL dumps before updates
├── logs/
│   ├── nginx-error.log          # Global Nginx error log
│   ├── mariadb.log              # MariaDB error log
│   ├── <sitename>-access.log    # Per-site access log
│   ├── <sitename>-error.log     # Per-site web server error log
│   └── php-<ver>.log            # PHP error logs
├── state.json                   # Site registry & daemon state
├── state.lock                   # File lock ensuring concurrent process safety
└── config.json                  # Global port bindings & generated DB password
```

---

## 🚀 Installation

### Windows (PowerShell)

Run the automated installer script:

```powershell
# Automated PowerShell Installer
irm https://raw.githubusercontent.com/termwp/termwp/main/installers/windows/install.ps1 | iex
```

Or run the precompiled native installer:
```powershell
.\termwp-setup.exe
```

### Linux (Bash)

Supports Debian, Ubuntu, Fedora, Arch, Alpine, and CentOS on `amd64` and `arm64`:

```bash
# Automated Bash Installer
curl -fsSL https://raw.githubusercontent.com/termwp/termwp/main/installers/linux/install.sh | bash
```

Or run the precompiled native installer:
```bash
chmod +x ./termwp-setup
./termwp-setup
```

---

## 💻 CLI Commands

### 1. `termwp add <site.local> [php-version] [ssl]`
Create and launch a brand new WordPress site.

```bash
# Interactive mode (prompts for title, admin email, user, and password)
termwp add blog.local

# Specify PHP version and SSL in any order
termwp add shop.local php8.3 ssl
termwp add api.test ssl php8.5

# Non-interactive / scripted creation with automatic strong password
termwp add client.local ssl --title="Client Portal" --email="admin@client.local" --yes
```

#### What happens under the hood:
1. Validates domain name and verifies it does not already exist.
2. Downloads and caches WordPress core archive (`latest.zip` / `latest.tar.gz`).
3. Creates dedicated database (`wp_<site>`) and user with cryptographically secure credentials.
4. Generates `wp-config.php` with unique 64-character security salts.
5. Emits Nginx virtual host configuration with WordPress rewrite rules.
6. Adds entry to the system hosts file inside `# TermWP BEGIN` ... `# TermWP END`.
7. (If SSL enabled) Generates local trusted TLS certificate via `mkcert`.
8. Starts or reloads Nginx, MariaDB, and the target PHP backend.
9. Executes `wp core install` using WP-CLI.
10. Displays a Lipgloss summary card with clickable links and clipboard copy prompt.
11. *Automatic rollback on failure:* If any step fails, all changes are cleanly reverted.

---

### 2. `termwp add ssl <site.local>`
Upgrade an existing HTTP site to trusted HTTPS:

```bash
termwp add ssl blog.local
```
Generates certificate, updates virtual host to listen on port 443 with HTTP/2 and HTTP->HTTPS redirect, updates WordPress `home` and `siteurl` via WP-CLI, and reloads Nginx.

---

### 3. `termwp show`
Display environment status and registered sites:

```bash
termwp show

# Machine-readable output for scripts
termwp show --json
```

**Output Preview:**
```
  Nginx: running (port 80, 443) | MariaDB: running (port 3306) | PHP: 8.4 (running)

  SITE           URL                        PHP   SSL   STATUS   DATABASE       CREATED
  blog.local     https://blog.local         8.4   ✔     🟢 on    wp_blog        2026-11-16
  store.test     http://store.test          8.3   -     🟢 on    wp_store       2026-11-16
  demo.local     https://demo.local         8.5   ✔     🔴 off   wp_demo        2026-11-15
```

---

### 4. `termwp off <site.local>` / `termwp on <site.local>`
Toggle sites without deleting files or databases:

```bash
# Deactivate site (disables vhost, unbinds hosts entry, frees PHP backend if unused)
termwp off blog.local

# Reactivate site (restores vhost, updates hosts entry, ensures PHP runtime is started)
termwp on blog.local
```

---

### 5. `termwp clone <source.local> to <new.local>`
Clone a full WordPress site including all uploads, themes, plugins, and database:

```bash
termwp clone blog.local to staging.local
```
- Performs a filesystem clone of `sites/<source>/` to `sites/<new>/`.
- Dumps database using `mariadb-dump` and imports into a new database (`wp_<new>`).
- Generates a new `wp-config.php` with independent database credentials.
- Runs WP-CLI `search-replace` across all database tables (handling serialized arrays and objects, and both HTTP and HTTPS URLs).
- Configures virtual hosts, SSL certificates, and hosts file entries.

---

### 6. `termwp update <site.local>`
Safely update WordPress core, themes, and plugins:

```bash
# Update WordPress core and database schema
termwp update blog.local

# Update core + all plugins
termwp update blog.local --plugins

# Update core + themes
termwp update blog.local --themes

# Update everything (core, DB schema, plugins, and themes)
termwp update blog.local --all
```
*Safety Feature:* Automatically takes a full SQL database backup into `data/backups/<site>/<timestamp>.sql` before executing updates.

---

### 7. `termwp logs <site.local>`
View and tail site logs in real time:

```bash
# View last 50 lines merged from all logs
termwp logs blog.local

# Live tail (follow mode)
termwp logs blog.local -f

# Filter to specific subsystem
termwp logs blog.local --wp -f        # WordPress debug.log only
termwp logs blog.local --php -f       # PHP error log only
termwp logs blog.local --nginx -f     # Nginx access & error logs only
termwp logs blog.local -n 100         # Last 100 lines
```

---

### 8. `termwp doctor [--fix]`
Comprehensive system health diagnostic engine:

```bash
termwp doctor

# Auto-repair detected discrepancies
termwp doctor --fix
```

Checks:
- System OS, architecture, disk space, and elevation status
- Runtime binaries presence and versions (Nginx, PHP 8.3/8.4/8.5, MariaDB, WP-CLI, mkcert)
- Mandatory WordPress PHP extensions (`mysqli`, `curl`, `mbstring`, `gd`, `zip`, `xml`, `intl`, `openssl`, etc.)
- Port availability and port collision detection (80, 443, 3306, 9083-9085)
- Stack daemon process states (respects deliberate shutdown)
- Hosts file synchronization (identifies orphan or missing domain entries)
- Site virtual hosts syntax validation (`nginx -t`)
- SSL certificate validity and expiry dates
- `mkcert` root CA trust status
- Internet connectivity to download mirrors
- Linux `.local` mDNS warning (advises `.test` if systemd-resolved conflicts)

---

### 9. `termwp shutdown [--force]`
Gracefully terminates all TermWP daemons in strict order:

```bash
termwp shutdown

# Force termination
termwp shutdown --force
```
**Shutdown Sequence:**
1. Nginx (`nginx -s quit`, up to 5s, then force termination)
2. Active PHP backends (PHP 8.3, 8.4, 8.5)
3. MariaDB (`mariadb-admin shutdown`, up to 15s, then force termination)
4. Flags `state.Shutdown = true` so `termwp doctor` recognizes normal stopped status.

---

### 10. `termwp del <site.local>`
Completely and safely remove a site:

```bash
termwp del blog.local

# Skip confirmation prompt
termwp del blog.local --yes
```
Removes database and database user, site files, Nginx virtual host, SSL certificates, hosts file entries, and registry entry. Stops the PHP backend if no other active site uses that version.

---

## ⚙️ Configuration

Global settings are stored in `<TermWP Home>/config.json`:

```json
{
  "mariadb_root_password": "<randomly_generated_24_char_password>",
  "http_port": 80,
  "https_port": 443,
  "mariadb_port": 3306,
  "default_php": "8.4",
  "php_ports": {
    "8.3": 9083,
    "8.4": 9084,
    "8.5": 9085
  }
}
```

### Download Sources
All portable runtime URLs and checksums are centralized in [`internal/config/sources.go`](internal/config/sources.go). To update PHP versions, MariaDB versions, or Nginx releases, edit that single file.

---

## 🗑️ Uninstallation

### Windows
```powershell
# PowerShell Uninstaller
.\installers\windows\uninstall.ps1

# Preserving site files and databases:
.\installers\windows\uninstall.ps1 -KeepData
```
Or use the compiled native uninstaller:
```powershell
.\termwp-uninstall.exe --keep-data
```

### Linux
```bash
# Bash Uninstaller
./installers/linux/uninstall.sh

# Preserving site files and databases:
./installers/linux/uninstall.sh --keep-data
```
Or use the compiled native uninstaller:
```bash
./termwp-uninstall --keep-data
```

---

## 🛠️ Building from Source

### Prerequisites
- **Go 1.22+** (pure Go, CGO disabled)
- Git

### Build Targets

Using `make`:
```bash
# Build CLI binaries for current host
make build

# Run unit tests
make test

# Cross-compile for Windows amd64, Linux amd64, Linux arm64
make cross-compile
```

Using PowerShell (Windows):
```powershell
.\scripts\build.ps1 -Clean
```

Using Bash (Linux):
```bash
./scripts/build.sh --clean
```

---

## 📄 License

Distributed under the **MIT License**. See [`LICENSE`](LICENSE) for details.
