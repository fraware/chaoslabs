# ChaosLabs Cache Warming Script (Windows)
Write-Host "ChaosLabs Cache Warming" -ForegroundColor Cyan
Write-Host "=======================" -ForegroundColor Cyan

# Function to check if a command exists
function Test-Command {
    param([string]$Command)
    try {
        Get-Command $Command -ErrorAction Stop | Out-Null
        return $true
    } catch {
        return $false
    }
}

Write-Host "Warming up development caches..." -ForegroundColor Yellow

# Warm up Go module cache
Write-Host "`n1. Warming Go module cache..." -ForegroundColor Cyan
if (Test-Command "go") {
    Write-Host "  - Downloading Go dependencies..." -ForegroundColor White
    go mod download
    if ($LASTEXITCODE -eq 0) {
        Write-Host "  ✓ Go modules cached successfully" -ForegroundColor Green
    } else {
        Write-Host "  ✗ Failed to cache Go modules" -ForegroundColor Red
    }
} else {
    Write-Host "  ○ Go not found, skipping Go cache warming" -ForegroundColor Yellow
}

# Warm up Node.js/npm cache
Write-Host "`n2. Warming Node.js cache..." -ForegroundColor Cyan
if (Test-Command "npm") {
    if (Test-Path "dashboard-v2\package.json") {
        Push-Location "dashboard-v2"
        Write-Host "  - Installing Node.js dependencies..." -ForegroundColor White
        npm ci --silent --prefer-offline
        if ($LASTEXITCODE -eq 0) {
            Write-Host "  ✓ Node modules cached successfully" -ForegroundColor Green
        } else {
            Write-Host "  ✗ Failed to cache Node modules" -ForegroundColor Red
        }
        Pop-Location
    } else {
        Write-Host "  ○ No package.json found, skipping Node cache" -ForegroundColor Yellow
    }
} else {
    Write-Host "  ○ npm not found, skipping Node cache warming" -ForegroundColor Yellow
}

# Create build cache directories
Write-Host "`n3. Preparing build cache directories..." -ForegroundColor Cyan
$cacheDirs = @("tmp", "bin", "coverage", "profiles", ".cache")

foreach ($dir in $cacheDirs) {
    if (-not (Test-Path $dir)) {
        New-Item -ItemType Directory -Path $dir -Force | Out-Null
        Write-Host "  ✓ Created $dir" -ForegroundColor Green
    } else {
        Write-Host "  ✓ $dir exists" -ForegroundColor Green
    }
}

Write-Host "`nCache Warming Complete!" -ForegroundColor Green
Write-Host "=======================" -ForegroundColor Green
