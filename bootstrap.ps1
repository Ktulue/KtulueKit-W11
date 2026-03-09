# KtulueKit-W11 Bootstrap
# Run this once from an admin PowerShell or cmd.exe to install Go and build the tool.
# After this, run: .\ktuluekit.exe --dry-run

#Requires -RunAsAdministrator

Write-Host "KtulueKit-W11 Bootstrap" -ForegroundColor Cyan
Write-Host "========================" -ForegroundColor Cyan

# 1. Install Go via winget
Write-Host "`nStep 1: Installing Go..." -ForegroundColor Yellow
winget install -e --id GoLang.Go --accept-package-agreements --accept-source-agreements --disable-interactivity

# 2. Refresh PATH so go.exe is available in this session
Write-Host "`nStep 2: Refreshing PATH..." -ForegroundColor Yellow
$env:Path = [System.Environment]::GetEnvironmentVariable("Path","Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path","User")

# 3. Verify Go is available
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Host "ERROR: 'go' not found after install. Try opening a new terminal and re-running." -ForegroundColor Red
    exit 1
}
Write-Host "Go version: $(go version)" -ForegroundColor Green

# 4. Download dependencies
Write-Host "`nStep 3: Downloading Go dependencies..." -ForegroundColor Yellow
go mod download

# 5. Build the binary
Write-Host "`nStep 4: Building ktuluekit.exe..." -ForegroundColor Yellow
go build -o ktuluekit.exe ./cmd/

if (Test-Path "ktuluekit.exe") {
    Write-Host "`nBuild successful! Run:" -ForegroundColor Green
    Write-Host "  .\ktuluekit.exe --dry-run       # preview what will be installed" -ForegroundColor White
    Write-Host "  .\ktuluekit.exe                 # run the full install" -ForegroundColor White
} else {
    Write-Host "ERROR: Build failed. Check output above." -ForegroundColor Red
    exit 1
}
