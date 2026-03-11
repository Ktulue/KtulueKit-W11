#Requires -Version 5.1
<#
.SYNOPSIS
    KtulueKit-W11 setup script. Installs Go if needed, builds the binary, and launches it.

.DESCRIPTION
    Run this from an admin PowerShell at the repo root to get KtulueKit running
    on a fresh or existing machine in one step.

    Usage:
        .\setup.ps1 [args passed through to ktuluekit.exe]

    Examples:
        .\setup.ps1
        .\setup.ps1 --dry-run
        .\setup.ps1 status

.NOTES
    Requirements:
      - Run as Administrator
      - Run from the repo root (where go.mod lives)
      - The repo must already be cloned
      - winget must be available (comes with Windows 11)
#>

param(
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$KtulueKitArgs
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

# ── Helper functions ────────────────────────────────────────────────────────

function Write-Step {
    param([string]$Message)
    Write-Host ""
    Write-Host "  $Message" -ForegroundColor Cyan
}

function Write-Success {
    param([string]$Message)
    Write-Host "  [OK] $Message" -ForegroundColor Green
}

function Write-Fail {
    param([string]$Message)
    Write-Host ""
    Write-Host "  [ERROR] $Message" -ForegroundColor Red
    Write-Host ""
    exit 1
}

# ── Step 1: Admin check ──────────────────────────────────────────────────────

Write-Host ""
Write-Host "KtulueKit-W11 Setup" -ForegroundColor White
Write-Host "──────────────────────────────────────────────────────" -ForegroundColor DarkGray

$principal = [Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Write-Fail "This script must be run as Administrator.`n  Right-click PowerShell and select 'Run as administrator', then try again."
}
Write-Success "Running as Administrator"

# ── Step 2: Verify repo root ─────────────────────────────────────────────────

if (-not (Test-Path "go.mod")) {
    Write-Fail "go.mod not found. Run this script from the KtulueKit-W11 repo root."
}
Write-Success "Repo root confirmed (go.mod found)"

# ── Step 3: Check / install Go ──────────────────────────────────────────────

Write-Step "Checking for Go..."

$goCmd = Get-Command go -ErrorAction SilentlyContinue
if ($goCmd) {
    $goVersion = & go version
    if ($LASTEXITCODE -ne 0) {
        Write-Fail "Go found on PATH but 'go version' failed. The installation may be corrupted. Try reinstalling Go manually."
    }
    Write-Success "Go already installed: $goVersion"
} else {
    Write-Step "Go not found. Installing via winget..."

    $wingetCmd = Get-Command winget -ErrorAction SilentlyContinue
    if (-not $wingetCmd) {
        Write-Fail "winget not found. Install the App Installer from the Microsoft Store, then re-run."
    }

    winget install -e --id GoLang.Go --accept-package-agreements --accept-source-agreements
    if ($LASTEXITCODE -ne 0) {
        Write-Fail "winget failed to install Go (exit code $LASTEXITCODE). Check the output above."
    }

    # Rebuild PATH from registry (Machine + User). Note: this replaces any session-local
    # PATH entries added by the current shell, but winget writes to Machine PATH so Go
    # will be available after this.
    Write-Step "Refreshing PATH..."
    $machinePath = [System.Environment]::GetEnvironmentVariable("Path", "Machine")
    $userPath    = [System.Environment]::GetEnvironmentVariable("Path", "User")
    $env:Path    = "$machinePath;$userPath"

    # Verify Go is now on PATH.
    $goCmd = Get-Command go -ErrorAction SilentlyContinue
    if (-not $goCmd) {
        Write-Fail "Go was installed but is not on PATH. Close and reopen your terminal, then re-run setup.ps1."
    }

    $goVersion = & go version
    if ($LASTEXITCODE -ne 0) {
        Write-Fail "Go was installed but 'go version' failed. Try closing and reopening the terminal."
    }
    Write-Success "Go installed: $goVersion"
}

# ── Step 4: Build ktuluekit.exe ──────────────────────────────────────────────

Write-Step "Building ktuluekit.exe..."

if (Test-Path "ktuluekit.exe") {
    Write-Success "ktuluekit.exe already exists — skipping build."
    Write-Host "  (Delete ktuluekit.exe and re-run to force a rebuild.)" -ForegroundColor DarkGray
} else {
    & go build -o ktuluekit.exe ./cmd/
    if ($LASTEXITCODE -ne 0) {
        Write-Fail "Build failed (exit code $LASTEXITCODE). Check the output above."
    }
    Write-Success "Build succeeded: ktuluekit.exe"
}

# ── Step 5: Launch ───────────────────────────────────────────────────────────

Write-Host ""
Write-Host "──────────────────────────────────────────────────────" -ForegroundColor DarkGray

if ($KtulueKitArgs.Count -gt 0) {
    Write-Host "  Launching: ktuluekit.exe $KtulueKitArgs" -ForegroundColor White
} else {
    Write-Host "  Launching: ktuluekit.exe" -ForegroundColor White
}
Write-Host "──────────────────────────────────────────────────────" -ForegroundColor DarkGray
Write-Host ""

& .\ktuluekit.exe @KtulueKitArgs
exit $LASTEXITCODE
