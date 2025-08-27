# Windows Development Setup Guide

This guide helps you set up the ChaosLabs development environment on Windows 10/11.

## Prerequisites

### Required Software

1. **Git for Windows**
   ```powershell
   # Using winget (Windows Package Manager)
   winget install Git.Git
   
   # Or download from: https://git-scm.com/download/win
   ```

2. **Go 1.21+**
   ```powershell
   # Using winget
   winget install GoLang.Go
   
   # Or download from: https://golang.org/dl/
   ```

3. **Node.js 18+**
   ```powershell
   # Using winget
   winget install OpenJS.NodeJS
   
   # Or download from: https://nodejs.org/
   ```

4. **Docker Desktop**
   ```powershell
   # Using winget
   winget install Docker.DockerDesktop
   
   # Or download from: https://desktop.docker.com/win/main/amd64/Docker%20Desktop%20Installer.exe
   ```

### Optional but Recommended

5. **Make for Windows**
   ```powershell
   # Using Chocolatey
   choco install make
   
   # Using Scoop
   scoop install make
   
   # Or use PowerShell scripts directly (see below)
   ```

6. **k6 Load Testing Tool**
   ```powershell
   # Using winget
   winget install k6
   
   # Or download from: https://github.com/grafana/k6/releases
   ```

7. **Windows Subsystem for Linux (WSL)** - Alternative option
   ```powershell
   wsl --install
   ```

## Quick Setup

### Method 1: Using Package Managers

#### Install Chocolatey (if not already installed)
```powershell
Set-ExecutionPolicy Bypass -Scope Process -Force; [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072; iex ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))
```

#### Install all dependencies
```powershell
# Core dependencies
choco install git golang nodejs docker-desktop make -y

# Optional tools
choco install k6 -y
```

#### Install Winget (alternative package manager)
```powershell
# Install from Microsoft Store or GitHub releases
# Then install dependencies:
winget install Git.Git GoLang.Go OpenJS.NodeJS Docker.DockerDesktop
```

### Method 2: Manual Installation

1. Download and install each tool from their official websites
2. Ensure all tools are added to your PATH
3. Restart your terminal/PowerShell

## Project Setup

### Clone and Setup
```powershell
# Clone the repository
git clone https://github.com/your-org/chaoslabs.git
cd chaoslabs

# Run the setup script
powershell -ExecutionPolicy Bypass -File infrastructure/devtools/scripts/dev-setup.ps1

# Alternative: Use Make if installed
make setup
```

### Verify Installation
```powershell
# Check versions
go version
node --version
npm --version
docker --version
git --version

# Test Docker
docker run hello-world
```

## Development Workflow

### Using PowerShell Scripts (Recommended for Windows)

```powershell
# Setup development environment
.\infrastructure\devtools\scripts\dev-setup.ps1

# Start development environment
docker-compose -f infrastructure/docker-compose.dev.yml up

# Run quality checks
.\scripts\check-all.ps1

# Generate performance report
.\infrastructure\performance-report.ps1

# Warm up caches
.\infrastructure\cache-warming.ps1
```

### Using Make (if installed)

```powershell
# Show all available commands
make help

# Setup development environment
make setup

# Start development environment
make dev

# Build all components
make build

# Run tests
make test

# Run quality checks
make check-all

# Generate performance report
make perf-report
```

### Manual Commands

```powershell
# Build Go components
cd controller
go build -o ../bin/controller.exe .
cd ../agent
go build -o ../bin/agent.exe .
cd ../cli
go build -o ../bin/chaoslabs-cli.exe .

# Build frontend
cd dashboard-v2
npm install
npm run build

# Run tests
cd controller
go test ./...
cd ../agent
go test ./...
cd ../cli
go test ./...
cd ../dashboard-v2
npm test
```

## Common Windows Issues and Solutions

### 1. PowerShell Execution Policy

If you get execution policy errors:
```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
```

### 2. Docker Desktop Issues

- Ensure Hyper-V is enabled
- Make sure Docker Desktop is running
- Check Windows features: "Containers" and "Hyper-V"

```powershell
# Enable Windows features
Enable-WindowsOptionalFeature -Online -FeatureName containers -All
Enable-WindowsOptionalFeature -Online -FeatureName Microsoft-Hyper-V -All
```

### 3. Path Issues

Ensure all tools are in your PATH:
```powershell
# Check current PATH
$env:PATH -split ';'

# Add to PATH if needed (example for Go)
$env:PATH += ";C:\Program Files\Go\bin"
```

### 4. Long Path Support

Enable long path support for Git:
```powershell
git config --global core.longpaths true
```

### 5. Line Ending Issues

Configure Git for Windows line endings:
```powershell
git config --global core.autocrlf true
```

## Alternative: WSL2 Development

If you prefer a Linux-like environment:

```powershell
# Install WSL2
wsl --install

# Install Ubuntu
wsl --install -d Ubuntu

# Switch to WSL2
wsl

# Then follow the Linux setup instructions inside WSL
```

## IDE Setup

### Visual Studio Code
```powershell
# Install VS Code
winget install Microsoft.VisualStudioCode

# Recommended extensions
code --install-extension golang.go
code --install-extension bradlc.vscode-tailwindcss
code --install-extension ms-vscode.vscode-typescript-next
code --install-extension ms-vscode-remote.remote-containers
```

### GoLand (JetBrains)
- Download from: https://www.jetbrains.com/go/
- Configure Go SDK and modules

## Performance Tips

1. **Use SSD storage** - Significantly improves Docker and build performance
2. **Increase Docker memory** - Allocate 4GB+ RAM to Docker Desktop
3. **Exclude from Windows Defender** - Add project directory to exclusions
4. **Use PowerShell Core** - Install PowerShell 7+ for better performance

```powershell
# Install PowerShell Core
winget install Microsoft.PowerShell
```

## Troubleshooting

### Common Commands for Debugging

```powershell
# Check Docker status
docker info
docker ps

# Check services
docker-compose -f infrastructure/docker-compose.dev.yml ps

# View logs
docker-compose -f infrastructure/docker-compose.dev.yml logs

# Reset Docker
docker system prune -a

# Clean project
Remove-Item -Recurse -Force bin, tmp, coverage
```

### Environment Variables

```powershell
# Set Go environment
$env:GOPROXY = "https://proxy.golang.org,direct"
$env:GOSUMDB = "sum.golang.org"

# Set Docker BuildKit
$env:DOCKER_BUILDKIT = "1"
$env:COMPOSE_DOCKER_CLI_BUILD = "1"
```

## Getting Help

- **Project Issues**: Open an issue on GitHub
- **Windows-specific help**: Check the [Windows Development Guide](./windows-dev-guide.md)
- **Docker Desktop help**: [Docker Desktop for Windows documentation](https://docs.docker.com/desktop/windows/)
- **Go on Windows**: [Go Windows Installation Guide](https://golang.org/doc/install)

## Next Steps

After setup:
1. Read the [Development Guide](./development.md)
2. Check out the [Architecture Overview](./architecture.md)
3. Review the [Contributing Guidelines](../CONTRIBUTING.md)
4. Start with `make help` or `.\scripts\check-all.ps1`
