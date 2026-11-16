#!/usr/bin/env bash
# TermWP Linux Automated Uninstaller
# Safely tears down background processes, removes local SSL CA trust,
# cleans up /etc/hosts file entries, removes symlinks, and deletes ~/.termwp.

set -euo pipefail

# ANSI Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

info()    { echo -e "  ${CYAN}➜${NC} $1..."; }
success() { echo -e "  ${GREEN}✔${NC} $1"; }
warning() { echo -e "  ${YELLOW}⚠${NC} $1"; }
error()   { echo -e "  ${RED}✘${NC} $1"; }

INSTALL_DIR="${TERMWP_HOME:-$HOME/.termwp}"
KEEP_DATA=false
FORCE=false

# Argument parsing
for arg in "$@"; do
    case "$arg" in
        --keep-data)
            KEEP_DATA=true
            ;;
        --force|-f)
            FORCE=true
            ;;
        *)
            ;;
    esac
done

echo -e "${RED}${BOLD}=====================================================${NC}"
echo -e "${RED}${BOLD}  🗑️  TermWP Linux Stack Uninstaller                 ${NC}"
echo -e "${RED}${BOLD}  Terminal-based Isolated WordPress Development      ${NC}"
echo -e "${RED}${BOLD}=====================================================${NC}\n"

if [ ! -d "$INSTALL_DIR" ]; then
    warning "TermWP directory not found at $INSTALL_DIR. Nothing to uninstall."
    exit 0
fi

if [ "$FORCE" = false ]; then
    read -rp "Are you sure you want to completely uninstall TermWP? (y/N): " confirm
    case "$confirm" in
        [yY][eE][sS]|[yY])
            ;;
        *)
            warning "Uninstallation aborted."
            exit 0
            ;;
    esac
fi

TERMWP_BIN="$INSTALL_DIR/bin/termwp"
MKCERT_BIN="$INSTALL_DIR/runtime/tools/mkcert"

# 1. Stop all TermWP services
info "Stopping all running background services"
if [ -x "$TERMWP_BIN" ]; then
    "$TERMWP_BIN" shutdown --force --quiet 2>/dev/null || true
fi

# Kill any orphaned processes running from INSTALL_DIR
if command -v pgrep >/dev/null 2>&1; then
    pgrep -f "$INSTALL_DIR" | while read -r pid; do
        if [ "$pid" != "$$" ]; then
            kill -9 "$pid" 2>/dev/null || true
        fi
    done
fi
success "All background services stopped"

# 2. Uninstall mkcert Root CA
if [ -x "$MKCERT_BIN" ]; then
    info "Uninstalling local trusted SSL Root CA"
    "$MKCERT_BIN" -uninstall 2>/dev/null || true
    success "mkcert Root CA uninstalled"
fi

# 3. Clean /etc/hosts entries
info "Cleaning /etc/hosts file entries"
HOSTS_FILE="/etc/hosts"
if [ -f "$HOSTS_FILE" ]; then
    if grep -q "# TermWP BEGIN" "$HOSTS_FILE"; then
        if [ "$(id -u)" -eq 0 ]; then
            sed -i '/# TermWP BEGIN/,/# TermWP END/d' "$HOSTS_FILE"
            success "Removed TermWP block from $HOSTS_FILE"
        elif command -v sudo >/dev/null 2>&1; then
            info "Requesting sudo to remove TermWP block from $HOSTS_FILE"
            sudo sed -i '/# TermWP BEGIN/,/# TermWP END/d' "$HOSTS_FILE"
            success "Removed TermWP block from $HOSTS_FILE via sudo"
        else
            warning "Could not remove hosts entries: root or sudo required. Please edit $HOSTS_FILE manually."
        fi
    else
        success "No TermWP block found in $HOSTS_FILE"
    fi
fi

# 4. Remove Symlinks
info "Removing symlinks"
rm -f "$HOME/.local/bin/termwp"
if [ -w "/usr/local/bin/termwp" ]; then
    rm -f "/usr/local/bin/termwp"
elif [ -f "/usr/local/bin/termwp" ] && command -v sudo >/dev/null 2>&1; then
    sudo rm -f "/usr/local/bin/termwp" 2>/dev/null || true
fi
success "Symlinks removed"

# 5. Handle Site and DB Data Backup
if [ "$KEEP_DATA" = true ]; then
    BACKUP_TARGET="$HOME/TermWP_Backup_$(date +%Y%m%d_%H%M%S)"
    info "Archiving site files and database data to $BACKUP_TARGET"
    mkdir -p "$BACKUP_TARGET"
    if [ -d "$INSTALL_DIR/sites" ]; then cp -r "$INSTALL_DIR/sites" "$BACKUP_TARGET/"; fi
    if [ -d "$INSTALL_DIR/data" ];  then cp -r "$INSTALL_DIR/data" "$BACKUP_TARGET/"; fi
    success "Backup preserved at $BACKUP_TARGET"
fi

# 6. Remove Installation Directory
info "Deleting TermWP directory at $INSTALL_DIR"
rm -rf "$INSTALL_DIR"
success "TermWP directory removed"

echo -e "\n${GREEN}${BOLD}=====================================================${NC}"
echo -e "${GREEN}${BOLD}  ✔ TermWP has been completely uninstalled.           ${NC}"
echo -e "${GREEN}${BOLD}=====================================================${NC}"
