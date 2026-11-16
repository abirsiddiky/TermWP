#!/usr/bin/env bash
# Cross-platform build script for TermWP in Bash.
# Compiles termwp, termwp-setup, and termwp-uninstall binaries for
# Windows (amd64) and Linux (amd64, arm64).

set -euo pipefail

VERSION="${1:-1.0.0}"
DIST_DIR="${DIST_DIR:-dist}"
CLEAN=false

for arg in "$@"; do
    case "$arg" in
        --clean)
            CLEAN=true
            ;;
    esac
done

# Git commit info
GIT_COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo "release")"
BUILD_DATE="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
LDFLAGS="-s -w -X termwp/internal/cli.Version=${VERSION} -X termwp/internal/cli.GitCommit=${GIT_COMMIT} -X termwp/internal/cli.BuildDate=${BUILD_DATE}"

echo -e "\033[0;35m====================================================="
echo -e "  🛠️  TermWP Cross-Platform Build Pipeline"
echo -e "  Version: ${VERSION} | Commit: ${GIT_COMMIT} | Date: ${BUILD_DATE}"
echo -e "=====================================================\033[0m\n"

if [ "$CLEAN" = true ] && [ -d "$DIST_DIR" ]; then
    echo "Cleaning $DIST_DIR..."
    rm -rf "$DIST_DIR"
fi

mkdir -p "$DIST_DIR"

PLATFORMS=(
    "windows/amd64/.exe"
    "linux/amd64/"
    "linux/arm64/"
)

APPS=(
    "termwp:./cmd/termwp"
    "termwp-setup:./cmd/termwp-setup"
    "termwp-uninstall:./cmd/termwp-uninstall"
)

for item in "${PLATFORMS[@]}"; do
    IFS="/" read -r GOOS GOARCH EXT <<< "$item"
    echo -e "\n\033[1;33m-- Building for ${GOOS}/${GOARCH} --\033[0m"

    for app_item in "${APPS[@]}"; do
        IFS=":" read -r APP_NAME APP_PKG <<< "$app_item"
        OUT_NAME="${APP_NAME}-${GOOS}-${GOARCH}${EXT}"
        OUT_PATH="${DIST_DIR}/${OUT_NAME}"

        echo -e "  \033[0;36m➜\033[0m Compiling ${APP_NAME} -> ${OUT_NAME}..."
        CGO_ENABLED=0 GOOS="${GOOS}" GOARCH="${GOARCH}" go build -trimpath -ldflags "${LDFLAGS}" -o "${OUT_PATH}" "${APP_PKG}"

        if [ -f "${OUT_PATH}" ]; then
            SIZE="$(ls -lh "${OUT_PATH}" | awk '{print $5}')"
            echo -e "  \033[0;32m✔\033[0m Built ${OUT_NAME} (${SIZE})"
        else
            echo -e "  \033[0;31m✘\033[0m Failed building ${APP_NAME}"
            exit 1
        fi
    done
done

echo -e "\n\033[1;33m-- Generating SHA256 Checksums --\033[0m"
CHECKSUM_FILE="${DIST_DIR}/checksums.txt"
rm -f "${CHECKSUM_FILE}"

cd "${DIST_DIR}"
if command -v sha256sum >/dev/null 2>&1; then
    sha256sum * > "checksums.txt"
elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 * > "checksums.txt"
fi
cd - >/dev/null

echo -e "\n\033[0;32m✔ Build process completed successfully! Binaries located in ${DIST_DIR}\033[0m\n"
