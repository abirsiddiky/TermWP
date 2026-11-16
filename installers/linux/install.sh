#!/usr/bin/env bash
# TermWP Linux Automated Installer & Upgrader
# Installs the isolated, portable TermWP stack for local WordPress development:
# Nginx, PHP 8.3/8.4/8.5, MariaDB, WP-CLI, and mkcert SSL.

set -euo pipefail

# ANSI Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# Helper Logging
info()    { echo -e "  ${CYAN}➜${NC} $1..."; }
success() { echo -e "  ${GREEN}✔${NC} $1"; }
warning() { echo -e "  ${YELLOW}⚠${NC} $1"; }
error()   { echo -e "  ${RED}✘${NC} $1"; }

echo -e "${MAGENTA}${BOLD}=====================================================${NC}"
echo -e "${MAGENTA}${BOLD}  ⚡ TermWP Linux Stack Installer                   ${NC}"
echo -e "${MAGENTA}${BOLD}  Terminal-based Isolated WordPress Development      ${NC}"
echo -e "${MAGENTA}${BOLD}=====================================================${NC}\n"

# 1. Architecture Detection
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
if [ "$OS" != "linux" ]; then
    error "This installer only supports Linux. Detected: $OS"
    exit 1
fi

ARCH_RAW="$(uname -m)"
case "$ARCH_RAW" in
    x86_64|amd64)
        ARCH="amd64"
        PHP_ARCH="x86_64"
        MARIADB_ARCH="x86_64"
        ;;
    aarch64|arm64)
        ARCH="arm64"
        PHP_ARCH="aarch64"
        MARIADB_ARCH="aarch64"
        ;;
    *)
        error "Unsupported processor architecture: $ARCH_RAW. TermWP supports x86_64 and aarch64/arm64."
        exit 1
        ;;
esac

success "Detected Linux ($ARCH)"

# 2. Directory Layout
INSTALL_DIR="${TERMWP_HOME:-$HOME/.termwp}"
BIN_DIR="$INSTALL_DIR/bin"
RUNTIME_DIR="$INSTALL_DIR/runtime"
CONF_DIR="$INSTALL_DIR/conf"
CERTS_DIR="$INSTALL_DIR/certs"
SITES_DIR="$INSTALL_DIR/sites"
DATA_DIR="$INSTALL_DIR/data"
LOGS_DIR="$INSTALL_DIR/logs"
CACHE_DIR="$RUNTIME_DIR/cache"
TOOLS_DIR="$RUNTIME_DIR/tools"

info "Preparing TermWP home directory structure in $INSTALL_DIR"
mkdir -p "$INSTALL_DIR" "$BIN_DIR" "$RUNTIME_DIR" "$CONF_DIR" "$CERTS_DIR" "$SITES_DIR" "$DATA_DIR" "$LOGS_DIR" "$CACHE_DIR" "$TOOLS_DIR"
mkdir -p "$RUNTIME_DIR/nginx" "$RUNTIME_DIR/mariadb" "$RUNTIME_DIR/php/8.3" "$RUNTIME_DIR/php/8.4" "$RUNTIME_DIR/php/8.5"
mkdir -p "$CONF_DIR/nginx/sites" "$DATA_DIR/mariadb" "$DATA_DIR/backups"
success "Directory structure prepared"

# 3. Stop running services if upgrading
TERMWP_BIN="$BIN_DIR/termwp"
if [ -f "$TERMWP_BIN" ] && [ -x "$TERMWP_BIN" ]; then
    info "Terminating running TermWP background processes"
    "$TERMWP_BIN" shutdown --force --quiet 2>/dev/null || true
    success "Previous processes stopped"
fi

# 4. Install termwp binary
info "Installing termwp CLI binary"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" 2>/dev/null && pwd || echo "")"
LOCAL_BUILD="$SCRIPT_DIR/../../termwp"

if [ -f "$LOCAL_BUILD" ]; then
    cp "$LOCAL_BUILD" "$TERMWP_BIN"
    chmod +x "$TERMWP_BIN"
    success "Installed termwp from local build"
elif [ ! -f "$TERMWP_BIN" ]; then
    RELEASE_URL="https://github.com/termwp-binaries/releases/releases/latest/download/termwp-linux-$ARCH"
    info "Downloading termwp binary from release repository"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$RELEASE_URL" -o "$TERMWP_BIN"
    elif command -v wget >/dev/null 2>&1; then
        wget -qO "$TERMWP_BIN" "$RELEASE_URL"
    else
        error "Neither curl nor wget found. Please install curl or wget."
        exit 1
    fi
    chmod +x "$TERMWP_BIN"
    success "Downloaded and installed termwp binary"
fi

# 5. Component Downloader Helper
download_and_extract() {
    local name="$1"
    local url="$2"
    local filename="$3"
    local target_dir="$4"
    local strip_dirs="${5:-0}"

    local cached_file="$CACHE_DIR/$filename"
    if [ ! -f "$cached_file" ]; then
        info "Downloading $name ($filename)"
        if command -v curl >/dev/null 2>&1; then
            curl -fsSL --continue-at - "$url" -o "$cached_file" || curl -fsSL "$url" -o "$cached_file"
        else
            wget -c -q "$url" -O "$cached_file" || wget -q "$url" -O "$cached_file"
        fi
        success "Downloaded $name"
    else
        success "Using cached $name"
    fi

    if [[ "$filename" == *.tar.gz ]] || [[ "$filename" == *.tgz ]]; then
        info "Extracting $name"
        mkdir -p "$target_dir"
        if [ "$strip_dirs" -gt 0 ]; then
            tar -xzf "$cached_file" -C "$target_dir" --strip-components="$strip_dirs"
        else
            tar -xzf "$cached_file" -C "$target_dir"
        fi
        success "Extracted $name"
    elif [[ "$filename" == *.zip ]]; then
        info "Extracting $name"
        mkdir -p "$target_dir"
        if command -v unzip >/dev/null 2>&1; then
            unzip -q -o "$cached_file" -d "$target_dir"
        else
            python3 -m zipfile -e "$cached_file" "$target_dir" 2>/dev/null || true
        fi
        success "Extracted $name"
    else
        # Direct executable or phar
        local dest_file="$target_dir/$filename"
        cp "$cached_file" "$dest_file"
        chmod +x "$dest_file"
        success "Installed $name"
    fi
}

# 6. Download Runtime Stack
echo -e "\n${YELLOW}${BOLD}-- Downloading and Installing Runtime Stack --${NC}"

# Nginx
download_and_extract "Nginx 1.26.3" \
    "https://github.com/termwp-binaries/nginx-portable/releases/download/v1.26.3/nginx-1.26.3-linux-$ARCH.tar.gz" \
    "nginx-1.26.3-linux-$ARCH.tar.gz" \
    "$RUNTIME_DIR/nginx" 1

# MariaDB LTS
download_and_extract "MariaDB 11.4.5" \
    "https://archive.mariadb.org/mariadb-11.4.5/bintar-linux-systemd-$MARIADB_ARCH/mariadb-11.4.5-linux-systemd-$MARIADB_ARCH.tar.gz" \
    "mariadb-11.4.5-linux-systemd-$MARIADB_ARCH.tar.gz" \
    "$RUNTIME_DIR/mariadb" 1

# PHP 8.3
download_and_extract "PHP 8.3.17" \
    "https://dl.static-php.dev/static-php-cli/bulk/php-8.3.17-fpm-linux-$PHP_ARCH.tar.gz" \
    "php-8.3.17-fpm-linux-$PHP_ARCH.tar.gz" \
    "$RUNTIME_DIR/php/8.3" 0

# PHP 8.4
download_and_extract "PHP 8.4.4" \
    "https://dl.static-php.dev/static-php-cli/bulk/php-8.4.4-fpm-linux-$PHP_ARCH.tar.gz" \
    "php-8.4.4-fpm-linux-$PHP_ARCH.tar.gz" \
    "$RUNTIME_DIR/php/8.4" 0

# PHP 8.5
download_and_extract "PHP 8.5-dev" \
    "https://dl.static-php.dev/static-php-cli/bulk/php-8.5.0-dev-fpm-linux-$PHP_ARCH.tar.gz" \
    "php-8.5.0-dev-fpm-linux-$PHP_ARCH.tar.gz" \
    "$RUNTIME_DIR/php/8.5" 0

# WP-CLI
download_and_extract "WP-CLI" \
    "https://raw.githubusercontent.com/wp-cli/builds/gh-pages/phar/wp-cli.phar" \
    "wp-cli.phar" \
    "$TOOLS_DIR" 0

# mkcert
download_and_extract "mkcert" \
    "https://github.com/FiloSottile/mkcert/releases/download/v1.4.4/mkcert-v1.4.4-linux-$ARCH" \
    "mkcert" \
    "$TOOLS_DIR" 0

# Ensure executables permissions
find "$RUNTIME_DIR" -type f \( -name "nginx" -o -name "mariadbd" -o -name "mariadb-admin" -o -name "mysqld" -o -name "php" -o -name "php-fpm" -o -name "mkcert" -o -name "*.phar" \) -exec chmod +x {} + 2>/dev/null || true

# 7. Non-root port capability for Nginx
NGINX_EXE="$RUNTIME_DIR/nginx/sbin/nginx"
if [ ! -f "$NGINX_EXE" ]; then
    NGINX_EXE="$RUNTIME_DIR/nginx/nginx"
fi

if [ -f "$NGINX_EXE" ] && command -v setcap >/dev/null 2>&1; then
    info "Setting capability for non-root low port binding (80/443) on Nginx"
    if [ "$(id -u)" -eq 0 ]; then
        setcap 'cap_net_bind_service=+ep' "$NGINX_EXE" 2>/dev/null || true
        success "Applied cap_net_bind_service capability"
    elif command -v sudo >/dev/null 2>&1; then
        sudo setcap 'cap_net_bind_service=+ep' "$NGINX_EXE" 2>/dev/null && success "Applied cap_net_bind_service capability via sudo" || warning "Could not apply setcap; ports 8080/8443 will be used if 80/443 require root"
    fi
fi

# 8. Symlink into PATH
echo -e "\n${YELLOW}${BOLD}-- Configuring User Environment PATH --${NC}"
LOCAL_BIN="$HOME/.local/bin"
mkdir -p "$LOCAL_BIN"
ln -sf "$TERMWP_BIN" "$LOCAL_BIN/termwp"
success "Symlinked termwp to $LOCAL_BIN/termwp"

# Check if ~/.local/bin is in PATH
if [[ ":$PATH:" != *":$LOCAL_BIN:"* ]]; then
    SHELL_NAME="$(basename "${SHELL:-bash}")"
    RC_FILE="$HOME/.bashrc"
    if [ "$SHELL_NAME" = "zsh" ] && [ -f "$HOME/.zshrc" ]; then
        RC_FILE="$HOME/.zshrc"
    fi

    if [ -f "$RC_FILE" ]; then
        if ! grep -q 'export PATH="$HOME/.local/bin:$PATH"' "$RC_FILE"; then
            info "Adding $LOCAL_BIN to $RC_FILE"
            echo -e '\n# TermWP CLI path\nexport PATH="$HOME/.local/bin:$PATH"' >> "$RC_FILE"
            success "Added PATH export to $RC_FILE"
        fi
    fi
else
    success "$LOCAL_BIN is already in your PATH"
fi

# 9. Run TermWP Doctor with auto-fix
echo -e "\n${YELLOW}${BOLD}-- Executing System Diagnosis --${NC}"
"$TERMWP_BIN" doctor --fix || true

echo -e "\n${GREEN}${BOLD}=====================================================${NC}"
echo -e "${GREEN}${BOLD}  ✔ TermWP installation successfully completed!       ${NC}"
echo -e "${GREEN}${BOLD}  Open a NEW shell (or source $RC_FILE) and run:     ${NC}"
echo -e "      ${BOLD}termwp show${NC}"
echo -e "      ${BOLD}termwp add mysite.local${NC}"
echo -e "${GREEN}${BOLD}=====================================================${NC}"
