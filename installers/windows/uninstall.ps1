<#
.SYNOPSIS
    TermWP Windows Automated Uninstaller
.DESCRIPTION
    Safely tears down background processes, removes local SSL CA trust,
    cleans up hosts file entries, removes TermWP from PATH, and deletes
    the isolated stack directory.
#>

[CmdletBinding()]
param(
    [string]$InstallDir = "$env:LOCALAPPDATA\TermWP",
    [switch]$KeepData,
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
  🗑️  TermWP Windows Stack Uninstaller
  Terminal-based Isolated WordPress Development
=====================================================
"@ -ForegroundColor Red

if (-not (Test-Path $InstallDir)) {
    Write-Warning "TermWP installation directory not found at $InstallDir. Nothing to uninstall."
    exit 0
}

if (-not $Force) {
    $confirm = Read-Host "Are you sure you want to completely uninstall TermWP? (y/N)"
    if ($confirm -notmatch '^(y|yes)$') {
        Write-Warning "Uninstallation aborted."
        exit 0
    }
}

$BinDir   = Join-Path $InstallDir "bin"
$ToolsDir = Join-Path $InstallDir "runtime\tools"
$TermwpExe = Join-Path $BinDir "termwp.exe"
$MkcertExe = Join-Path $ToolsDir "mkcert.exe"

# 1. Stop all TermWP services
Write-Step "Stopping all running background services"
if (Test-Path $TermwpExe) {
    try {
        & $TermwpExe shutdown --force --quiet 2>$null
    } catch {
        # Continue even if shutdown error occurs
    }
}

# Kill any orphaned processes running from $InstallDir
Get-Process | Where-Object {
    try { $_.Path -and $_.Path.StartsWith($InstallDir, [System.StringComparison]::OrdinalIgnoreCase) } catch { $false }
} | ForEach-Object {
    try { Stop-Process -Id $_.Id -Force -ErrorAction SilentlyContinue } catch {}
}
Write-Success "All background services stopped"

# 2. Uninstall mkcert root CA if present
if (Test-Path $MkcertExe) {
    Write-Step "Uninstalling local trusted SSL Root CA"
    try {
        & $MkcertExe -uninstall 2>$null
        Write-Success "mkcert Root CA uninstalled"
    } catch {
        Write-Warning "Could not uninstall mkcert CA: $_"
    }
}

# 3. Clean hosts file entries
Write-Step "Cleaning hosts file entries"
$HostsPath = "$env:SystemRoot\System32\drivers\etc\hosts"
try {
    if (Test-Path $HostsPath) {
        $content = Get-Content -Path $HostsPath -Raw -ErrorAction Stop
        if ($content -match "# TermWP BEGIN[\s\S]*?# TermWP END") {
            # Try writing directly; if permission denied, run elevated PowerShell
            try {
                $cleaned = $content -replace "(?ms)\r?\n?# TermWP BEGIN[\s\S]*?# TermWP END", ""
                [System.IO.File]::WriteAllText($HostsPath, $cleaned, [System.Text.Encoding]::UTF8)
                Write-Success "Cleaned hosts file entries"
            } catch {
                Write-Step "Elevating privilege to remove hosts file entries"
                $scriptBlock = @"
`$content = Get-Content -Path '$HostsPath' -Raw
`$cleaned = `$content -replace '(?ms)\r?\n?# TermWP BEGIN[\s\S]*?# TermWP END', ''
[System.IO.File]::WriteAllText('$HostsPath', `$cleaned, [System.Text.Encoding]::UTF8)
"@
                $encoded = [Convert]::ToBase64String([System.Text.Encoding]::Unicode.GetBytes($scriptBlock))
                Start-Process -FilePath "powershell.exe" -ArgumentList "-NoProfile", "-EncodedCommand", $encoded -Verb RunAs -Wait
                Write-Success "Cleaned hosts file entries via elevated command"
            }
        } else {
            Write-Success "No TermWP block found in hosts file"
        }
    }
} catch {
    Write-Warning "Could not update hosts file automatically: $_"
}

# 4. Remove bin directory from User PATH
Write-Step "Removing TermWP from User Environment PATH"
$UserPath = [Environment]::GetEnvironmentVariable("Path", [EnvironmentVariableTarget]::User)
if ($UserPath -like "*$BinDir*") {
    $pathParts = $UserPath -split ";" | Where-Object { $_ -ne "" -and $_ -ne $BinDir }
    $NewPath = $pathParts -join ";"
    [Environment]::SetEnvironmentVariable("Path", $NewPath, [EnvironmentVariableTarget]::User)
    
    # Broadcast WM_SETTINGCHANGE
    Add-Type -Namespace Win32 -Name NativeMethods -MemberDefinition @'
        [System.Runtime.InteropServices.DllImport("user32.dll", SetLastError = true, CharSet = System.Runtime.InteropServices.CharSet.Auto)]
        public static extern IntPtr SendMessageTimeout(IntPtr hWnd, uint Msg, UIntPtr wParam, string lParam, uint fuFlags, uint uTimeout, out UIntPtr lpdwResult);
'@
    $result = [UIntPtr]::Zero
    [Win32.NativeMethods]::SendMessageTimeout([IntPtr]0xffff, 0x001A, [UIntPtr]::Zero, "Environment", 2, 5000, [ref]$result) | Out-Null
    Write-Success "Removed $BinDir from User PATH"
} else {
    Write-Success "User PATH did not contain $BinDir"
}

# 5. Handle Site and DB Data Backup
$SitesDir = Join-Path $InstallDir "sites"
$DataDir  = Join-Path $InstallDir "data"
if ($KeepData) {
    $BackupTarget = Join-Path $env:USERPROFILE "TermWP_Backup_$(Get-Date -Format 'yyyyMMdd_HHmmss')"
    Write-Step "Archiving site files and database data to $BackupTarget"
    New-Item -ItemType Directory -Path $BackupTarget -Force | Out-Null
    if (Test-Path $SitesDir) { Copy-Item -Path $SitesDir -Destination (Join-Path $BackupTarget "sites") -Recurse -Force }
    if (Test-Path $DataDir)  { Copy-Item -Path $DataDir -Destination (Join-Path $BackupTarget "data") -Recurse -Force }
    Write-Success "Backup preserved at $BackupTarget"
}

# 6. Remove Installation Directory
Write-Step "Deleting TermWP directory at $InstallDir"
Start-Sleep -Seconds 1 # allow file handles to release
try {
    Remove-Item -Path $InstallDir -Recurse -Force -ErrorAction Stop
    Write-Success "TermWP directory removed"
} catch {
    Write-Warning "Some files could not be removed immediately: $_"
    Write-Warning "You may manually delete remaining files in: $InstallDir"
}

Write-Host @"

=====================================================
  ✔ TermWP has been completely uninstalled.
=====================================================
"@ -ForegroundColor Green
