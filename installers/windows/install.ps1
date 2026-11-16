<#
.SYNOPSIS
    TermWP Windows Automated Installer & Upgrader
.DESCRIPTION
    Installs the isolated, portable TermWP stack for local WordPress development:
    Nginx, PHP 8.3/8.4/8.5, MariaDB, WP-CLI, and mkcert SSL.
#>

[CmdletBinding()]
param(
    [string]$InstallDir = "$env:LOCALAPPDATA\TermWP",
    [switch]$SkipDoctor,
    [switch]$Force
)

$ErrorActionPreference = 'Stop'
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8

# Styling Helpers
function Write-Step { param([string]$Msg) Write-Host "  ➜ $Msg..." -ForegroundColor Cyan }
function Write-Success { param([string]$Msg) Write-Host "  ✔ $Msg" -ForegroundColor Green }
function Write-Warning { param([string]$Msg) Write-Host "  ⚠ $Msg" -ForegroundColor Yellow }
function Write-ErrorMsg { param([string]$Msg) Write-Host "  ✘ $Msg" -ForegroundColor Red }

Write-Host @"
=====================================================
  ⚡ TermWP Windows Stack Installer
  Terminal-based Isolated WordPress Development
=====================================================
"@ -ForegroundColor Magenta

# 1. Prepare Directory Structure
Write-Step "Creating TermWP home directory structure in $InstallDir"
$BinDir     = Join-Path $InstallDir "bin"
$RuntimeDir = Join-Path $InstallDir "runtime"
$ConfDir    = Join-Path $InstallDir "conf"
$CertsDir   = Join-Path $InstallDir "certs"
$SitesDir   = Join-Path $InstallDir "sites"
$DataDir    = Join-Path $InstallDir "data"
$LogsDir    = Join-Path $InstallDir "logs"
$CacheDir   = Join-Path $RuntimeDir "cache"
$ToolsDir   = Join-Path $RuntimeDir "tools"

@($InstallDir, $BinDir, $RuntimeDir, $ConfDir, $CertsDir, $SitesDir, $DataDir, $LogsDir, $CacheDir, $ToolsDir,
  (Join-Path $RuntimeDir "nginx"), (Join-Path $RuntimeDir "mariadb"),
  (Join-Path $RuntimeDir "php\8.3"), (Join-Path $RuntimeDir "php\8.4"), (Join-Path $RuntimeDir "php\8.5"),
  (Join-Path $ConfDir "nginx\sites"), (Join-Path $DataDir "mariadb"), (Join-Path $DataDir "backups")) | ForEach-Object {
    if (-not (Test-Path $_)) {
        New-Item -ItemType Directory -Path $_ -Force | Out-Null
    }
}
Write-Success "Directory structure prepared"

# 2. Shutdown running services before upgrading
$TermwpExe = Join-Path $BinDir "termwp.exe"
if (Test-Path $TermwpExe) {
    Write-Step "Terminating running TermWP background processes"
    try {
        & $TermwpExe shutdown --force --quiet 2>$null
    } catch {
        # Ignore errors if not running
    }
    Write-Success "Previous processes stopped"
}

# 3. Install termwp binary
Write-Step "Installing termwp CLI binary"
# If installer script is executed from build directory, copy local binary
$LocalBuild = Join-Path $PSScriptRoot "..\..\termwp.exe"
if (Test-Path $LocalBuild) {
    Copy-Item $LocalBuild -Destination $TermwpExe -Force
    Write-Success "Installed termwp.exe from local build"
} elseif (-not (Test-Path $TermwpExe)) {
    # If downloading release from repository
    $ReleaseUrl = "https://github.com/termwp-binaries/releases/releases/latest/download/termwp-windows-amd64.exe"
    Invoke-WebRequest -Uri $ReleaseUrl -OutFile $TermwpExe -UseBasicParsing
    Write-Success "Downloaded termwp.exe"
}

# 4. Helper for downloading and extracting
function Install-Component {
    param(
        [string]$Name,
        [string]$Url,
        [string]$OutFile,
        [string]$TargetDir,
        [int]$StripDirs = 0
    )

    $CachedFile = Join-Path $CacheDir $OutFile
    if (-not (Test-Path $CachedFile)) {
        Write-Step "Downloading $Name ($OutFile)"
        Start-BitsTransfer -Source $Url -Destination $CachedFile -DisplayName "TermWP: $Name" -ErrorAction SilentlyContinue
        if (-not (Test-Path $CachedFile)) {
            Invoke-WebRequest -Uri $Url -OutFile $CachedFile -UseBasicParsing
        }
        Write-Success "Downloaded $Name"
    } else {
        Write-Success "Using cached $Name"
    }

    if ($OutFile.EndsWith(".zip")) {
        Write-Step "Extracting $Name"
        Expand-Archive -Path $CachedFile -DestinationPath $TargetDir -Force
        # If stripDirs == 1, move contents up one level
        if ($StripDirs -eq 1) {
            $SubDirs = Get-ChildItem -Path $TargetDir -Directory
            if ($SubDirs.Count -eq 1) {
                $SingleDir = $SubDirs[0].FullName
                Get-ChildItem -Path $SingleDir | Move-Item -Destination $TargetDir -Force
                Remove-Item $SingleDir -Force -Recurse
            }
        }
        Write-Success "Extracted $Name"
    } elseif ($OutFile.EndsWith(".exe") -or $OutFile.EndsWith(".phar")) {
        Copy-Item -Path $CachedFile -Destination (Join-Path $TargetDir (Split-Path $CachedFile -Leaf)) -Force
        Write-Success "Installed $Name"
    }
}

# 5. Download Runtime Stack
Write-Host "`n-- Downloading and Installing Runtime Stack --" -ForegroundColor Yellow

# Nginx
Install-Component "Nginx 1.26.3" "https://nginx.org/download/nginx-1.26.3.zip" "nginx-1.26.3.zip" (Join-Path $RuntimeDir "nginx") 1

# MariaDB LTS
Install-Component "MariaDB 11.4.5" "https://archive.mariadb.org/mariadb-11.4.5/winx64-packages/mariadb-11.4.5-winx64.zip" "mariadb-11.4.5-winx64.zip" (Join-Path $RuntimeDir "mariadb") 1

# PHP 8.3
Install-Component "PHP 8.3.17" "https://windows.php.net/downloads/releases/archives/php-8.3.17-nts-Win32-vs16-x64.zip" "php-8.3.17-nts.zip" (Join-Path $RuntimeDir "php\8.3") 0

# PHP 8.4
Install-Component "PHP 8.4.4" "https://windows.php.net/downloads/releases/archives/php-8.4.4-nts-Win32-vs17-x64.zip" "php-8.4.4-nts.zip" (Join-Path $RuntimeDir "php\8.4") 0

# PHP 8.5
Install-Component "PHP 8.5-dev" "https://windows.php.net/downloads/snaps/master/latest/php-master-nts-windows-vs17-x64.zip" "php-8.5-dev.zip" (Join-Path $RuntimeDir "php\8.5") 0

# WP-CLI
Install-Component "WP-CLI" "https://raw.githubusercontent.com/wp-cli/builds/gh-pages/phar/wp-cli.phar" "wp-cli.phar" $ToolsDir 0

# mkcert
Install-Component "mkcert" "https://github.com/FiloSottile/mkcert/releases/download/v1.4.4/mkcert-v1.4.4-windows-amd64.exe" "mkcert.exe" $ToolsDir 0

# 6. Add bin directory to User PATH
Write-Host "`n-- Configuring User Environment PATH --" -ForegroundColor Yellow
$UserPath = [Environment]::GetEnvironmentVariable("Path", [EnvironmentVariableTarget]::User)
if ($UserPath -notlike "*$BinDir*") {
    Write-Step "Adding $BinDir to User PATH"
    $NewPath = "$BinDir;$UserPath"
    [Environment]::SetEnvironmentVariable("Path", $NewPath, [EnvironmentVariableTarget]::User)
    $env:Path = "$BinDir;$env:Path"
    
    # Broadcast WM_SETTINGCHANGE so other applications see the change immediately
    Add-Type -Namespace Win32 -Name NativeMethods -MemberDefinition @'
        [System.Runtime.InteropServices.DllImport("user32.dll", SetLastError = true, CharSet = System.Runtime.InteropServices.CharSet.Auto)]
        public static extern IntPtr SendMessageTimeout(IntPtr hWnd, uint Msg, UIntPtr wParam, string lParam, uint fuFlags, uint uTimeout, out UIntPtr lpdwResult);
'@
    $result = [UIntPtr]::Zero
    [Win32.NativeMethods]::SendMessageTimeout([IntPtr]0xffff, 0x001A, [UIntPtr]::Zero, "Environment", 2, 5000, [ref]$result) | Out-Null
    Write-Success "PATH updated and environment broadcasted"
} else {
    Write-Success "User PATH already includes $BinDir"
}

# 7. Run TermWP Doctor
if (-not $SkipDoctor) {
    Write-Host "`n-- Executing System Diagnosis --" -ForegroundColor Yellow
    & $TermwpExe doctor --fix
}

Write-Host @"

=====================================================
  ✔ TermWP installation successfully completed!
  Open a NEW terminal (PowerShell or cmd) and run:
      termwp show
      termwp add mysite.local
=====================================================
"@ -ForegroundColor Green
