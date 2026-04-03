# ChaosLabs Quality Check Script (Windows)
# Runs comprehensive quality checks on the codebase

Write-Host "ChaosLabs Quality Checks" -ForegroundColor Cyan
Write-Host "========================" -ForegroundColor Cyan

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

# Track overall status
$overallSuccess = $true
$checkResults = @()

# Function to add check result
function Add-CheckResult {
    param(
        [string]$Check,
        [bool]$Success,
        [string]$Message = ""
    )
    
    $script:checkResults += [PSCustomObject]@{
        Check = $Check
        Success = $Success
        Message = $Message
    }
    
    if (-not $Success) {
        $script:overallSuccess = $false
    }
}

Write-Host "`n1. Go Code Quality Checks" -ForegroundColor Yellow
Write-Host "=========================" -ForegroundColor Yellow

# Go formatting check
Write-Host "Checking Go code formatting..." -ForegroundColor Cyan
if (Test-Command "gofmt") {
    $unformattedFiles = gofmt -l . 2>$null
    if ($unformattedFiles) {
        Add-CheckResult "Go Formatting" $false "Files need formatting: $($unformattedFiles -join ', ')"
        Write-Host "✗ Go formatting issues found" -ForegroundColor Red
        Write-Host "  Run 'gofmt -w .' to fix" -ForegroundColor Yellow
    } else {
        Add-CheckResult "Go Formatting" $true
        Write-Host "✓ Go code is properly formatted" -ForegroundColor Green
    }
} else {
    Add-CheckResult "Go Formatting" $false "gofmt not available"
    Write-Host "○ gofmt not found, skipping formatting check" -ForegroundColor Yellow
}

# Go vet check
Write-Host "Running go vet..." -ForegroundColor Cyan
if (Test-Command "go") {
    go vet ./... 2>$null
    if ($LASTEXITCODE -eq 0) {
        Add-CheckResult "Go Vet" $true
        Write-Host "✓ go vet passed" -ForegroundColor Green
    } else {
        Add-CheckResult "Go Vet" $false "go vet found issues"
        Write-Host "✗ go vet found issues" -ForegroundColor Red
        Write-Host "  Run 'go vet ./...' to see details" -ForegroundColor Yellow
    }
} else {
    Add-CheckResult "Go Vet" $false "go not available"
    Write-Host "○ go not found, skipping vet check" -ForegroundColor Yellow
}

# Go linting
Write-Host "Running golangci-lint..." -ForegroundColor Cyan
if (Test-Command "golangci-lint") {
    golangci-lint run --config .golangci.yml 2>$null
    if ($LASTEXITCODE -eq 0) {
        Add-CheckResult "Go Lint" $true
        Write-Host "✓ golangci-lint passed" -ForegroundColor Green
    } else {
        Add-CheckResult "Go Lint" $false "golangci-lint found issues"
        Write-Host "✗ golangci-lint found issues" -ForegroundColor Red
        Write-Host "  Run 'golangci-lint run' to see details" -ForegroundColor Yellow
    }
} else {
    Add-CheckResult "Go Lint" $false "golangci-lint not available"
    Write-Host "○ golangci-lint not found, skipping lint check" -ForegroundColor Yellow
}

Write-Host "`n2. Frontend Code Quality Checks" -ForegroundColor Yellow
Write-Host "===============================" -ForegroundColor Yellow

$dashboardPath = Join-Path "dashboard-v2" "package.json"
if (Test-Path $dashboardPath) {
    Push-Location "dashboard-v2"
    
    # TypeScript check
    Write-Host "Checking TypeScript..." -ForegroundColor Cyan
    if (Test-Command "npm") {
        npm run type-check 2>$null
        if ($LASTEXITCODE -eq 0) {
            Add-CheckResult "TypeScript" $true
            Write-Host "✓ TypeScript check passed" -ForegroundColor Green
        } else {
            Add-CheckResult "TypeScript" $false "TypeScript errors found"
            Write-Host "✗ TypeScript errors found" -ForegroundColor Red
            Write-Host "  Run 'cd dashboard-v2 && npm run type-check' to see details" -ForegroundColor Yellow
        }
        
        # ESLint check
        Write-Host "Running ESLint..." -ForegroundColor Cyan
        npm run lint 2>$null
        if ($LASTEXITCODE -eq 0) {
            Add-CheckResult "ESLint" $true
            Write-Host "✓ ESLint passed" -ForegroundColor Green
        } else {
            Add-CheckResult "ESLint" $false "ESLint issues found"
            Write-Host "✗ ESLint issues found" -ForegroundColor Red
            Write-Host "  Run 'cd dashboard-v2 && npm run lint' to see details" -ForegroundColor Yellow
        }
    } else {
        Add-CheckResult "Frontend Checks" $false "npm not available"
        Write-Host "○ npm not found, skipping frontend checks" -ForegroundColor Yellow
    }
    
    Pop-Location
} else {
    Add-CheckResult "Frontend Checks" $false "No dashboard-v2 directory found"
    Write-Host "○ No dashboard-v2 directory found, skipping frontend checks" -ForegroundColor Yellow
}

Write-Host "`n3. Build Verification" -ForegroundColor Yellow
Write-Host "=====================" -ForegroundColor Yellow

# Test Go builds
Write-Host "Testing Go builds..." -ForegroundColor Cyan
$components = @("controller", "agent", "cli")
$buildSuccess = $true

foreach ($component in $components) {
    if (Test-Path $component) {
        Push-Location $component
        Write-Host "  Building $component..." -ForegroundColor Gray
        go build -o "../tmp/test-$component.exe" . 2>$null
        if ($LASTEXITCODE -eq 0) {
            Write-Host "  ✓ $component builds successfully" -ForegroundColor Green
            Remove-Item "../tmp/test-$component.exe" -ErrorAction SilentlyContinue
        } else {
            Write-Host "  ✗ $component build failed" -ForegroundColor Red
            $buildSuccess = $false
        }
        Pop-Location
    } else {
        Write-Host "  ○ $component directory not found" -ForegroundColor Yellow
    }
}

Add-CheckResult "Go Builds" $buildSuccess

Write-Host "`n4. Docker Configuration" -ForegroundColor Yellow
Write-Host "=======================" -ForegroundColor Yellow

# Check Docker files
Write-Host "Checking Docker configuration..." -ForegroundColor Cyan
$dockerFiles = @(
    "infrastructure/Dockerfile.controller.optimized",
    "infrastructure/compose/docker-compose.yml",
    "infrastructure/docker-compose.dev.yml"
)

$dockerConfigOk = $true
foreach ($file in $dockerFiles) {
    if (Test-Path $file) {
        Write-Host "  ✓ $file exists" -ForegroundColor Green
    } else {
        Write-Host "  ✗ $file missing" -ForegroundColor Red
        $dockerConfigOk = $false
    }
}

Add-CheckResult "Docker Configuration" $dockerConfigOk

# Generate summary report
Write-Host "`nQuality Check Summary" -ForegroundColor Cyan
Write-Host "====================" -ForegroundColor Cyan

$passedChecks = ($checkResults | Where-Object { $_.Success }).Count
$totalChecks = $checkResults.Count

Write-Host "Checks passed: $passedChecks / $totalChecks" -ForegroundColor White

Write-Host "`nDetailed Results:" -ForegroundColor White
foreach ($result in $checkResults) {
    if ($result.Success) {
        Write-Host "  ✓ $($result.Check)" -ForegroundColor Green
    } else {
        Write-Host "  ✗ $($result.Check)" -ForegroundColor Red
        if ($result.Message) {
            Write-Host "    $($result.Message)" -ForegroundColor Yellow
        }
    }
}

if ($overallSuccess) {
    Write-Host "`n🎉 All quality checks passed!" -ForegroundColor Green
    Write-Host "Your code is ready for commit/deployment." -ForegroundColor Green
    exit 0
} else {
    Write-Host "`n⚠️  Some quality checks failed." -ForegroundColor Red
    Write-Host "Please fix the issues above before committing." -ForegroundColor Red
    Write-Host "`nQuick fixes:" -ForegroundColor Cyan
    Write-Host "  Format code and run basic checks" -ForegroundColor White
    exit 1
}
