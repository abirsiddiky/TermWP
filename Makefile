# TermWP Makefile
# Targets for testing, building, cross-compiling, and installing TermWP

SHELL := /bin/bash
VERSION ?= 1.0.0
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "release")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -s -w -X termwp/internal/cli.Version=$(VERSION) \
                 -X termwp/internal/cli.GitCommit=$(GIT_COMMIT) \
                 -X termwp/internal/cli.BuildDate=$(BUILD_DATE)

DIST_DIR := dist
BIN_DIR := bin

.PHONY: all build test cross-compile clean install windows linux-amd64 linux-arm64

all: test build

## build: Build termwp CLI binary for the host operating system
build:
	@mkdir -p $(BIN_DIR)
	@echo "➜ Compiling termwp CLI..."
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/termwp ./cmd/termwp
	@echo "➜ Compiling termwp-setup..."
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/termwp-setup ./cmd/termwp-setup
	@echo "➜ Compiling termwp-uninstall..."
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/termwp-uninstall ./cmd/termwp-uninstall
	@echo "✔ Built all binaries in $(BIN_DIR)/"

## test: Run complete unit test suite across all packages
test:
	@echo "➜ Running test suite..."
	go test -v -race ./...

## test-fast: Run test suite without race detector
test-fast:
	@echo "➜ Running fast test suite..."
	go test -v ./...

## cross-compile: Compile for Windows amd64, Linux amd64, and Linux arm64
cross-compile:
	@mkdir -p $(DIST_DIR)
	@echo "➜ Building Windows (amd64)..."
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/termwp-windows-amd64.exe ./cmd/termwp
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/termwp-setup-windows-amd64.exe ./cmd/termwp-setup
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/termwp-uninstall-windows-amd64.exe ./cmd/termwp-uninstall
	@echo "➜ Building Linux (amd64)..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/termwp-linux-amd64 ./cmd/termwp
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/termwp-setup-linux-amd64 ./cmd/termwp-setup
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/termwp-uninstall-linux-amd64 ./cmd/termwp-uninstall
	@echo "➜ Building Linux (arm64)..."
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/termwp-linux-arm64 ./cmd/termwp
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/termwp-setup-linux-arm64 ./cmd/termwp-setup
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/termwp-uninstall-linux-arm64 ./cmd/termwp-uninstall
	@echo "➜ Generating SHA256 checksums..."
	@cd $(DIST_DIR) && (sha256sum * 2>/dev/null || shasum -a 256 *) > checksums.txt
	@echo "✔ Cross-compilation complete! Artifacts in $(DIST_DIR)/"

## windows: Compile Windows amd64 binaries
windows:
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/termwp-windows-amd64.exe ./cmd/termwp

## linux-amd64: Compile Linux amd64 binaries
linux-amd64:
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/termwp-linux-amd64 ./cmd/termwp

## linux-arm64: Compile Linux arm64 binaries
linux-arm64:
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/termwp-linux-arm64 ./cmd/termwp

## install: Install termwp to user local bin directory
install: build
	@mkdir -p $$HOME/.local/bin
	@cp $(BIN_DIR)/termwp $$HOME/.local/bin/termwp
	@chmod +x $$HOME/.local/bin/termwp
	@echo "✔ Installed termwp to $$HOME/.local/bin/termwp"

## clean: Remove build artifacts and temporary files
clean:
	@rm -rf $(BIN_DIR) $(DIST_DIR)
	@echo "✔ Cleaned build directories"
