<#
.SYNOPSIS
    Cross-platform build script for TermWP in PowerShell.
.DESCRIPTION
    Compiles termwp, termwp-setup, and termwp-uninstall binaries for
    Windows (amd64) and Linux (amd64, arm64). Strips debug symbols and
    injects version metadata via ldflags.
#>

[CmdletBinding()]
param(
    [string]$Target = "all",
    [string]$Version = "1.0.0",
    [string]$DistDir = "dist",
    [switch]$Clean
)

$ErrorActionPreference = 'Stop'
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8

$GitCommit = "dev"
try {
    $GitCommit = (git rev-parse --short HEAD 2>$null)
    if (-not $GitCommit) { $GitCommit = "release" }
} catch {
    $GitCommit = "release"
}

$BuildDate = (Get-Date -Format "yyyy-MM-ddTHH:mm:ssZ")
$LDFlags = "-s -w -X termwp/internal/cli.Version=$Version -X termwp/internal/cli.GitCommit=$GitCommit -X termwp/internal/cli.BuildDate=$BuildDate"

Write-Host "=====================================================" -ForegroundColor Magenta
Write-Host "  🛠️  TermWP Cross-Platform Build Pipeline" -ForegroundColor Magenta
Write-Host "  Version: $Version | Commit: $GitCommit | Date: $BuildDate" -ForegroundColor Cyan
Write-Host "=====================================================" -ForegroundColor Magenta

if ($Clean -and (Test-Path $DistDir)) {
    Write-Host "Cleaning dist directory..." -ForegroundColor Yellow
    Remove-Item -Recurse -Force $DistDir
}

if (-not (Test-Path $DistDir)) {
    New-Item -ItemType Directory -Path $DistDir -Force | Out-Null
}

$Platforms = @(
    @{ OS = "windows"; Arch = "amd64"; Ext = ".exe" },
    @{ OS = "linux";   Arch = "amd64"; Ext = "" },
    @{ OS = "linux";   Arch = "arm64"; Ext = "" }
)

$Apps = @(
    @{ Name = "termwp";           Pkg = "./cmd/termwp" },
    @{ Name = "termwp-setup";     Pkg = "./cmd/termwp-setup" },
    @{ Name = "termwp-uninstall"; Pkg = "./cmd/termwp-uninstall" }
)

foreach ($p in $Platforms) {
    $platformKey = "$($p.OS)-$($p.Arch)"
    if ($Target -ne "all" -and $Target -ne $platformKey) {
        continue
    }

    Write-Host "`n-- Building for $($p.OS)/$($p.Arch) --" -ForegroundColor Yellow

    $env:GOOS = $p.OS
    $env:GOARCH = $p.Arch
    $env:CGO_ENABLED = "0"

    foreach ($app in $Apps) {
        $outName = "$($app.Name)-$($p.OS)-$($p.Arch)$($p.Ext)"
        $outPath = Join-Path $DistDir $outName

        Write-Host "  >> Compiling $($app.Name) -> $outName..." -ForegroundColor Cyan
        go build -trimpath -ldflags $LDFlags -o $outPath $app.Pkg

        if (Test-Path $outPath) {
            $size = (Get-Item $outPath).Length / 1MB
            Write-Host ("  [OK] Built {0} ({1:N2} MB)" -f $outName, $size) -ForegroundColor Green
        } else {
            Write-Host "  [FAIL] Failed building $($app.Name)" -ForegroundColor Red
            exit 1
        }
    }
}

# Reset environment
$env:GOOS = ""
$env:GOARCH = ""
$env:CGO_ENABLED = ""

# Generate SHA256 Checksums
Write-Host "`n-- Generating SHA256 Checksums --" -ForegroundColor Yellow
$ChecksumFile = Join-Path $DistDir "checksums.txt"
if (Test-Path $ChecksumFile) { Remove-Item $ChecksumFile }

Get-ChildItem -Path $DistDir -File | Where-Object { $_.Name -ne "checksums.txt" } | ForEach-Object {
    $hash = (Get-FileHash -Path $_.FullName -Algorithm SHA256).Hash.ToLower()
    $line = "$hash  $($_.Name)"
    Add-Content -Path $ChecksumFile -Value $line
    Write-Host "  $line" -ForegroundColor Gray
}

Write-Host "`nBuild process completed successfully! Binaries located in $DistDir`n" -ForegroundColor Green
