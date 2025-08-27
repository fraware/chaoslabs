# ChaosLabs Development Environment Setup Script for Windows
# This script sets up the development environment on Windows systems

Write-Host "ChaosLabs Development Environment Setup" -ForegroundColor Cyan
Write-Host "======================================" -ForegroundColor Cyan

# Function to check if a command exists
function Test-Command {
    param([string]$Command)
    try {
        Get-Command $Command -ErrorAction Stop | Out-Null
        return $true
    }
    catch {
        return $false
    }
}

# Function to check if Docker is running
function Test-DockerRunning {
    try {
        docker info | Out-Null
        return $true
    }
    catch {
        return $false
    }
}

Write-Host "Checking prerequisites..." -ForegroundColor Yellow

# Check Docker
if (Test-Command "docker") {
    if (Test-DockerRunning) {
        Write-Host "✓ Docker is installed and running" -ForegroundColor Green
    } else {
        Write-Host "✗ Docker is installed but not running. Please start Docker Desktop." -ForegroundColor Red
        Write-Host "  Download from: https://desktop.docker.com/win/main/amd64/Docker%20Desktop%20Installer.exe" -ForegroundColor Yellow
        exit 1
    }
} else {
    Write-Host "✗ Docker not found. Please install Docker Desktop." -ForegroundColor Red
    Write-Host "  Download from: https://desktop.docker.com/win/main/amd64/Docker%20Desktop%20Installer.exe" -ForegroundColor Yellow
    exit 1
}

# Check Docker Compose
if (Test-Command "docker-compose") {
    Write-Host "✓ Docker Compose is installed" -ForegroundColor Green
} elseif (docker compose version 2>$null) {
    Write-Host "✓ Docker Compose (v2) is installed" -ForegroundColor Green
} else {
    Write-Host "✗ Docker Compose not found. Please install Docker Desktop (includes Compose)." -ForegroundColor Red
    exit 1
}

# Check Go
if (Test-Command "go") {
    $goVersion = go version
    Write-Host "✓ Go is installed: $goVersion" -ForegroundColor Green
} else {
    Write-Host "✗ Go not found. Please install Go 1.21+" -ForegroundColor Red
    Write-Host "  Download from: https://golang.org/dl/" -ForegroundColor Yellow
    exit 1
}

# Check Node.js
if (Test-Command "node") {
    $nodeVersion = node --version
    Write-Host "✓ Node.js is installed: $nodeVersion" -ForegroundColor Green
} else {
    Write-Host "✗ Node.js not found. Please install Node.js 18+" -ForegroundColor Red
    Write-Host "  Download from: https://nodejs.org/" -ForegroundColor Yellow
    exit 1
}

# Check npm
if (Test-Command "npm") {
    $npmVersion = npm --version
    Write-Host "✓ npm is installed: $npmVersion" -ForegroundColor Green
} else {
    Write-Host "✗ npm not found. Usually comes with Node.js." -ForegroundColor Red
    exit 1
}

# Check Git
if (Test-Command "git") {
    $gitVersion = git --version
    Write-Host "✓ Git is installed: $gitVersion" -ForegroundColor Green
} else {
    Write-Host "✗ Git not found. Please install Git." -ForegroundColor Red
    Write-Host "  Download from: https://git-scm.com/download/win" -ForegroundColor Yellow
    exit 1
}

# Check optional tools
Write-Host "`nChecking optional tools..." -ForegroundColor Yellow

# Check k6
if (Test-Command "k6") {
    $k6Version = k6 version
    Write-Host "✓ k6 is installed: $k6Version" -ForegroundColor Green
} else {
    Write-Host "○ k6 not found (optional). Install with: winget install k6" -ForegroundColor Yellow
    Write-Host "  Or download from: https://github.com/grafana/k6/releases" -ForegroundColor Yellow
}

# Check Make (if using Windows make alternative)
if (Test-Command "make") {
    Write-Host "✓ Make is installed" -ForegroundColor Green
} else {
    Write-Host "○ Make not found. Consider installing:" -ForegroundColor Yellow
    Write-Host "  - Chocolatey: choco install make" -ForegroundColor Yellow
    Write-Host "  - Scoop: scoop install make" -ForegroundColor Yellow
    Write-Host "  - Or use PowerShell scripts directly" -ForegroundColor Yellow
}

Write-Host "`nSetting up development directories..." -ForegroundColor Yellow

# Create necessary directories
$directories = @("bin", "coverage", "benchmarks", "profiles", "tmp", "logs")
foreach ($dir in $directories) {
    if (-not (Test-Path $dir)) {
        New-Item -ItemType Directory -Path $dir -Force | Out-Null
        Write-Host "✓ Created directory: $dir" -ForegroundColor Green
    } else {
        Write-Host "✓ Directory exists: $dir" -ForegroundColor Green
    }
}

Write-Host "`nInstalling Go tools..." -ForegroundColor Yellow

# Install Go development tools
$goTools = @(
    "github.com/golangci/golangci-lint/cmd/golangci-lint@latest",
    "github.com/go-delve/delve/cmd/dlv@latest", 
    "golang.org/x/tools/cmd/goimports@latest",
    "golang.org/x/vuln/cmd/govulncheck@latest",
    "github.com/air-verse/air@latest"
)

foreach ($tool in $goTools) {
    $toolName = ($tool -split "/")[-1] -replace "@.*", ""
    Write-Host "Installing $toolName..." -ForegroundColor Cyan
    go install $tool
    if ($LASTEXITCODE -eq 0) {
        Write-Host "✓ Installed $toolName" -ForegroundColor Green
    } else {
        Write-Host "✗ Failed to install $toolName" -ForegroundColor Red
    }
}

Write-Host "`nSetting up frontend dependencies..." -ForegroundColor Yellow

# Install frontend dependencies
$dashboardPath = Join-Path "dashboard-v2" "package.json"
if (Test-Path $dashboardPath) {
    Push-Location "dashboard-v2"
    Write-Host "Installing frontend dependencies..." -ForegroundColor Cyan
    npm install
    if ($LASTEXITCODE -eq 0) {
        Write-Host "✓ Frontend dependencies installed" -ForegroundColor Green
    } else {
        Write-Host "✗ Failed to install frontend dependencies" -ForegroundColor Red
    }
    Pop-Location
} else {
    Write-Host "○ No dashboard-v2/package.json found, skipping frontend setup" -ForegroundColor Yellow
}

Write-Host "`nSetting up Git hooks..." -ForegroundColor Yellow

# Set up Git hooks if .git directory exists
if (Test-Path ".git") {
    $preCommitContent = @'
#!/bin/sh
# ChaosLabs pre-commit hook
echo "Running pre-commit checks..."

# Run Go formatting
gofmt -w .
goimports -w .

# Run linting
golangci-lint run --config .golangci.yml

echo "Pre-commit checks completed!"
'@

    $hookPath = Join-Path ".git" "hooks" "pre-commit"
    $preCommitContent | Out-File -FilePath $hookPath -Encoding ascii
    Write-Host "✓ Git pre-commit hook installed" -ForegroundColor Green
} else {
    Write-Host "○ Not a Git repository, skipping Git hooks" -ForegroundColor Yellow
}

Write-Host "`nValidating setup..." -ForegroundColor Yellow

# Test basic Go build
Write-Host "Testing Go compilation..." -ForegroundColor Cyan
if (Test-Path "controller") {
    Push-Location "controller"
    go build -o "../tmp/controller-test.exe" .
    if ($LASTEXITCODE -eq 0) {
        Write-Host "✓ Controller builds successfully" -ForegroundColor Green
        Remove-Item "../tmp/controller-test.exe" -ErrorAction SilentlyContinue
    } else {
        Write-Host "✗ Controller build failed" -ForegroundColor Red
    }
    Pop-Location
}

# Test Docker connectivity
Write-Host "Testing Docker connectivity..." -ForegroundColor Cyan
docker run --rm hello-world | Out-Null
if ($LASTEXITCODE -eq 0) {
    Write-Host "✓ Docker is working correctly" -ForegroundColor Green
} else {
    Write-Host "✗ Docker test failed" -ForegroundColor Red
}

Write-Host "`nDevelopment Environment Setup Complete!" -ForegroundColor Green
Write-Host "=======================================" -ForegroundColor Green
Write-Host "`nNext steps:" -ForegroundColor Cyan
Write-Host "1. Run docker-compose -f infrastructure/docker-compose.dev.yml up to start the development environment" -ForegroundColor White
Write-Host "2. Open http://localhost:3000 for the dashboard" -ForegroundColor White
Write-Host "3. Open http://localhost:3001 for Grafana (admin/chaoslabs)" -ForegroundColor White
Write-Host "4. Open http://localhost:16686 for Jaeger tracing" -ForegroundColor White
Write-Host "`nUseful commands:" -ForegroundColor Cyan
Write-Host "  Make commands (if Make installed) or use PowerShell scripts directly" -ForegroundColor White
Write-Host "  docker-compose -f infrastructure/docker-compose.dev.yml logs -f  - View logs" -ForegroundColor White
