package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// PreflightCheck represents a single preflight check
type PreflightCheck struct {
	Name        string
	Description string
	Status      CheckStatus
	Error       string
	Remediation string
	Required    bool
}

// CheckStatus represents the status of a preflight check
type CheckStatus string

const (
	CheckStatusPass CheckStatus = "PASS"
	CheckStatusFail CheckStatus = "FAIL"
	CheckStatusWarn CheckStatus = "WARN"
	CheckStatusSkip CheckStatus = "SKIP"
)

// PreflightResult holds the results of all preflight checks
type PreflightResult struct {
	Timestamp    time.Time
	Hostname     string
	OS           string
	Architecture string
	Checks       []PreflightCheck
	Summary      PreflightSummary
}

// PreflightSummary provides a summary of all checks
type PreflightSummary struct {
	TotalChecks   int
	PassedChecks  int
	FailedChecks  int
	WarningChecks int
	SkippedChecks int
	OverallStatus CheckStatus
}

// PreflightManager manages all preflight checks
type PreflightManager struct {
	checks []PreflightCheck
}

// NewPreflightManager creates a new preflight manager
func NewPreflightManager() *PreflightManager {
	return &PreflightManager{
		checks: make([]PreflightCheck, 0),
	}
}

// RunAllChecks executes all preflight checks
func (pm *PreflightManager) RunAllChecks() (*PreflightResult, error) {
	result := &PreflightResult{
		Timestamp:    time.Now(),
		Hostname:     getHostname(),
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
		Checks:       make([]PreflightCheck, 0),
	}

	// Run capability checks
	pm.runCapabilityChecks(result)

	// Run tool availability checks
	pm.runToolAvailabilityChecks(result)

	// Run container privilege checks
	pm.runContainerPrivilegeChecks(result)

	// Run system requirement checks
	pm.runSystemRequirementChecks(result)

	// Run network capability checks
	pm.runNetworkCapabilityChecks(result)

	// Calculate summary
	result.Summary = pm.calculateSummary(result.Checks)

	return result, nil
}

// runCapabilityChecks runs all capability-related checks
func (pm *PreflightManager) runCapabilityChecks(result *PreflightResult) {
	// Check CAP_NET_ADMIN capability
	pm.checkCapability(result, "CAP_NET_ADMIN", "Network administration capabilities", true,
		"Required for network fault injection (tc, ip commands)",
		"Run: sudo setcap cap_net_admin+ep /path/to/agent")

	// Check CAP_SYS_ADMIN capability
	pm.checkCapability(result, "CAP_SYS_ADMIN", "System administration capabilities", false,
		"Required for advanced system stress testing",
		"Run: sudo setcap cap_sys_admin+ep /path/to/agent")

	// Check CAP_SYS_RESOURCE capability
	pm.checkCapability(result, "CAP_SYS_RESOURCE", "System resource management capabilities", false,
		"Required for resource limits and cgroups",
		"Run: sudo setcap cap_sys_resource+ep /path/to/agent")
}

// runToolAvailabilityChecks runs all tool availability checks
func (pm *PreflightManager) runToolAvailabilityChecks(result *PreflightResult) {
	// Check tc (traffic control) availability
	pm.checkToolAvailability(result, "tc", "Traffic control utility", true,
		"Required for network fault injection",
		"Install: apt-get install iproute2 (Ubuntu) or yum install iproute-tc (CentOS)")

	// Check ip utility availability
	pm.checkToolAvailability(result, "ip", "IP utility", true,
		"Required for network interface management",
		"Install: apt-get install iproute2 (Ubuntu) or yum install iproute (CentOS)")

	// Check stress-ng availability
	pm.checkToolAvailability(result, "stress-ng", "Stress testing utility", false,
		"Required for system stress testing",
		"Install: apt-get install stress-ng (Ubuntu) or yum install stress-ng (CentOS)")

	// Check cgcreate availability (cgroups)
	pm.checkToolAvailability(result, "cgcreate", "Cgroup creation utility", false,
		"Required for resource isolation",
		"Install: apt-get install cgroup-tools (Ubuntu) or yum install libcgroup-tools (CentOS)")

	// Check ifconfig availability (fallback)
	pm.checkToolAvailability(result, "ifconfig", "Interface configuration utility", false,
		"Fallback for network interface management",
		"Install: apt-get install net-tools (Ubuntu) or yum install net-tools (CentOS)")
}

// runContainerPrivilegeChecks runs container-specific privilege checks
func (pm *PreflightManager) runContainerPrivilegeChecks(result *PreflightResult) {
	// Check if running in container
	if pm.isRunningInContainer() {
		pm.addCheck(result, PreflightCheck{
			Name:        "Container Privileges",
			Description: "Container privilege assessment",
			Status:      CheckStatusWarn,
			Required:    false,
			Remediation: "Ensure container has necessary capabilities and mounts",
		})

		// Check for privileged container
		if pm.isPrivilegedContainer() {
			pm.addCheck(result, PreflightCheck{
				Name:        "Privileged Container",
				Description: "Container running with privileged mode",
				Status:      CheckStatusPass,
				Required:    false,
				Remediation: "Container has elevated privileges",
			})
		} else {
			pm.addCheck(result, PreflightCheck{
				Name:        "Privileged Container",
				Description: "Container running with privileged mode",
				Status:      CheckStatusWarn,
				Required:    false,
				Remediation: "Consider running with --privileged or specific capabilities",
			})
		}

		// Check for necessary mounts
		pm.checkContainerMounts(result)
	} else {
		pm.addCheck(result, PreflightCheck{
			Name:        "Container Environment",
			Description: "Running on bare metal or VM",
			Status:      CheckStatusPass,
			Required:    false,
			Remediation: "No container-specific configuration needed",
		})
	}
}

// runSystemRequirementChecks runs system requirement checks
func (pm *PreflightManager) runSystemRequirementChecks(result *PreflightResult) {
	// Check kernel version
	pm.checkKernelVersion(result)

	// Check available memory
	pm.checkAvailableMemory(result)

	// Check available disk space
	pm.checkAvailableDiskSpace(result)

	// Check CPU cores
	pm.checkCPUInfo(result)
}

// runNetworkCapabilityChecks runs network-specific capability checks
func (pm *PreflightManager) runNetworkCapabilityChecks(result *PreflightResult) {
	// Check IFB module availability
	pm.checkKernelModule(result, "ifb", "Intermediate Functional Block device", false,
		"Required for ingress traffic shaping",
		"Load module: sudo modprobe ifb")

	// Check netem module availability
	pm.checkKernelModule(result, "sch_netem", "Network emulation scheduler", true,
		"Required for network fault injection",
		"Load module: sudo modprobe sch_netem")

	// Check network interface count
	pm.checkNetworkInterfaceCount(result)
}

// checkCapability checks if a specific capability is available
func (pm *PreflightManager) checkCapability(result *PreflightResult, capName, description string, required bool, remediation string) {
	cap, err := pm.getCapability(capName)
	if err != nil {
		pm.addCheck(result, PreflightCheck{
			Name:        capName,
			Description: description,
			Status:      CheckStatusFail,
			Error:       err.Error(),
			Required:    required,
			Remediation: remediation,
		})
		return
	}

	status := CheckStatusPass
	if !cap {
		status = CheckStatusFail
		if !required {
			status = CheckStatusWarn
		}
	}

	pm.addCheck(result, PreflightCheck{
		Name:        capName,
		Description: description,
		Status:      status,
		Required:    required,
		Remediation: remediation,
	})
}

// checkToolAvailability checks if a tool is available in PATH
func (pm *PreflightManager) checkToolAvailability(result *PreflightResult, toolName, description string, required bool, remediation string) {
	_, err := exec.LookPath(toolName)
	status := CheckStatusPass
	if err != nil {
		status = CheckStatusFail
		if !required {
			status = CheckStatusWarn
		}
	}

	pm.addCheck(result, PreflightCheck{
		Name:        toolName,
		Description: description,
		Status:      status,
		Error:       err.Error(),
		Required:    required,
		Remediation: remediation,
	})
}

// checkKernelModule checks if a kernel module is available
func (pm *PreflightManager) checkKernelModule(result *PreflightResult, moduleName, description string, required bool, remediation string) {
	available := pm.isKernelModuleAvailable(moduleName)
	status := CheckStatusPass
	if !available {
		status = CheckStatusFail
		if !required {
			status = CheckStatusWarn
		}
	}

	pm.addCheck(result, PreflightCheck{
		Name:        moduleName,
		Description: description,
		Status:      status,
		Required:    required,
		Remediation: remediation,
	})
}

// getCapability checks if a specific capability is available
func (pm *PreflightManager) getCapability(capName string) (bool, error) {
	// Parse capability name to number
	var capNum int
	switch capName {
	case "CAP_NET_ADMIN":
		capNum = unix.CAP_NET_ADMIN
	case "CAP_SYS_ADMIN":
		capNum = unix.CAP_SYS_ADMIN
	case "CAP_SYS_RESOURCE":
		capNum = unix.CAP_SYS_RESOURCE
	default:
		return false, fmt.Errorf("unknown capability: %s", capName)
	}

	// Check if process has the capability
	_, _, err := unix.Syscall(unix.SYS_CAPGET, 0, 0, 0)
	if err != 0 {
		// Capabilities not supported, try alternative method
		return pm.checkCapabilityAlternative(capNum), nil
	}

	// Use prctl to check capability
	var caps [2]uint32
	_, _, err = unix.Syscall(unix.SYS_PRCTL, unix.PR_CAPBSET_READ, uintptr(capNum), uintptr(0))
	if err != 0 {
		return false, fmt.Errorf("failed to check capability: %v", err)
	}

	return true, nil
}

// checkCapabilityAlternative provides an alternative method to check capabilities
func (pm *PreflightManager) checkCapabilityAlternative(capNum int) bool {
	// Try to execute a privileged operation
	switch capNum {
	case unix.CAP_NET_ADMIN:
		// Try to create a dummy qdisc (will fail but shows capability)
		cmd := exec.Command("tc", "qdisc", "add", "dev", "lo", "root", "netem", "delay", "1ms")
		_ = cmd.Run()
		return cmd.ProcessState.ExitCode() != 1 // Exit code 1 usually means permission denied
	case unix.CAP_SYS_ADMIN:
		// Try to read /proc/sys (usually requires SYS_ADMIN)
		_, err := os.ReadFile("/proc/sys/kernel/hostname")
		return err == nil
	default:
		return false
	}
}

// isRunningInContainer checks if the process is running in a container
func (pm *PreflightManager) isRunningInContainer() bool {
	// Check for common container indicators
	indicators := []string{
		"/.dockerenv",
		"/proc/1/cgroup",
		"/proc/1/root",
	}

	for _, indicator := range indicators {
		if _, err := os.Stat(indicator); err == nil {
			return true
		}
	}

	// Check cgroup for container indicators
	if pm.hasContainerCgroup() {
		return true
	}

	return false
}

// isPrivilegedContainer checks if the container is running with privileged mode
func (pm *PreflightManager) isPrivilegedContainer() bool {
	// Check if we can access host devices
	if _, err := os.Stat("/dev/net/tun"); err == nil {
		return true
	}

	// Check if we can access host proc
	if _, err := os.Stat("/proc/sys"); err == nil {
		return true
	}

	return false
}

// hasContainerCgroup checks cgroup for container indicators
func (pm *PreflightManager) hasContainerCgroup() bool {
	file, err := os.Open("/proc/1/cgroup")
	if err != nil {
		return false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "docker") || strings.Contains(line, "kubepods") {
			return true
		}
	}
	return false
}

// checkContainerMounts checks if necessary mounts are available in container
func (pm *PreflightManager) checkContainerMounts(result *PreflightResult) {
	requiredMounts := []string{
		"/proc",
		"/sys",
		"/dev",
	}

	for _, mount := range requiredMounts {
		if _, err := os.Stat(mount); err == nil {
			pm.addCheck(result, PreflightCheck{
				Name:        "Mount: " + mount,
				Description: "Required mount point available",
				Status:      CheckStatusPass,
				Required:    true,
				Remediation: "Mount point is available",
			})
		} else {
			pm.addCheck(result, PreflightCheck{
				Name:        "Mount: " + mount,
				Description: "Required mount point missing",
				Status:      CheckStatusFail,
				Error:       err.Error(),
				Required:    true,
				Remediation: "Ensure container has access to " + mount,
			})
		}
	}
}

// isKernelModuleAvailable checks if a kernel module is available
func (pm *PreflightManager) isKernelModuleAvailable(moduleName string) bool {
	// Check if module is loaded
	cmd := exec.Command("lsmod")
	output, err := cmd.Output()
	if err == nil {
		if strings.Contains(string(output), moduleName) {
			return true
		}
	}

	// Check if module can be loaded
	cmd = exec.Command("modprobe", "-n", moduleName)
	err = cmd.Run()
	return err == nil
}

// checkKernelVersion checks the kernel version
func (pm *PreflightManager) checkKernelVersion(result *PreflightResult) {
	cmd := exec.Command("uname", "-r")
	output, err := cmd.Output()
	if err != nil {
		pm.addCheck(result, PreflightCheck{
			Name:        "Kernel Version",
			Description: "Kernel version check",
			Status:      CheckStatusFail,
			Error:       err.Error(),
			Required:    true,
			Remediation: "Ensure uname command is available",
		})
		return
	}

	kernelVersion := strings.TrimSpace(string(output))
	pm.addCheck(result, PreflightCheck{
		Name:        "Kernel Version",
		Description: "Kernel version: " + kernelVersion,
		Status:      CheckStatusPass,
		Required:    false,
		Remediation: "Kernel version is available",
	})
}

// checkAvailableMemory checks available memory
func (pm *PreflightManager) checkAvailableMemory(result *PreflightResult) {
	// Read /proc/meminfo
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		pm.addCheck(result, PreflightCheck{
			Name:        "Available Memory",
			Description: "Memory availability check",
			Status:      CheckStatusFail,
			Error:       err.Error(),
			Required:    false,
			Remediation: "Ensure /proc filesystem is mounted",
		})
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var memTotal, memAvailable int64

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				memTotal, _ = strconv.ParseInt(fields[1], 10, 64)
			}
		} else if strings.HasPrefix(line, "MemAvailable:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				memAvailable, _ = strconv.ParseInt(fields[1], 10, 64)
			}
		}
	}

	if memTotal > 0 && memAvailable > 0 {
		availableGB := float64(memAvailable) / 1024 / 1024
		status := CheckStatusPass
		if availableGB < 1.0 {
			status = CheckStatusWarn
		}

		pm.addCheck(result, PreflightCheck{
			Name:        "Available Memory",
			Description: fmt.Sprintf("Available memory: %.2f GB", availableGB),
			Status:      status,
			Required:    false,
			Remediation: "Ensure sufficient memory for experiments",
		})
	}
}

// checkAvailableDiskSpace checks available disk space
func (pm *PreflightManager) checkAvailableDiskSpace(result *PreflightResult) {
	var stat unix.Statfs_t
	err := unix.Statfs(".", &stat)
	if err != nil {
		pm.addCheck(result, PreflightCheck{
			Name:        "Available Disk Space",
			Description: "Disk space check",
			Status:      CheckStatusFail,
			Error:       err.Error(),
			Required:    false,
			Remediation: "Ensure filesystem is accessible",
		})
		return
	}

	availableGB := float64(stat.Bavail*uint64(stat.Bsize)) / 1024 / 1024 / 1024
	status := CheckStatusPass
	if availableGB < 1.0 {
		status = CheckStatusWarn
	}

	pm.addCheck(result, PreflightCheck{
		Name:        "Available Disk Space",
		Description: fmt.Sprintf("Available disk space: %.2f GB", availableGB),
		Status:      status,
		Required:    false,
		Remediation: "Ensure sufficient disk space for logs and experiments",
	})
}

// checkCPUInfo checks CPU information
func (pm *PreflightManager) checkCPUInfo(result *PreflightResult) {
	file, err := os.Open("/proc/cpuinfo")
	if err != nil {
		pm.addCheck(result, PreflightCheck{
			Name:        "CPU Information",
			Description: "CPU information check",
			Status:      CheckStatusFail,
			Error:       err.Error(),
			Required:    false,
			Remediation: "Ensure /proc filesystem is mounted",
		})
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var cpuCount int
	var cpuModel string

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "processor") {
			cpuCount++
		} else if strings.Contains(line, "model name") {
			fields := strings.Split(line, ":")
			if len(fields) >= 2 {
				cpuModel = strings.TrimSpace(fields[1])
			}
		}
	}

	if cpuCount > 0 {
		pm.addCheck(result, PreflightCheck{
			Name:        "CPU Information",
			Description: fmt.Sprintf("CPU cores: %d, Model: %s", cpuCount, cpuModel),
			Status:      CheckStatusPass,
			Required:    false,
			Remediation: "CPU information is available",
		})
	}
}

// checkNetworkInterfaceCount checks the number of network interfaces
func (pm *PreflightManager) checkNetworkInterfaceCount(result *PreflightResult) {
	cmd := exec.Command("ip", "link", "show")
	output, err := cmd.Output()
	if err != nil {
		pm.addCheck(result, PreflightCheck{
			Name:        "Network Interfaces",
			Description: "Network interface count check",
			Status:      CheckStatusFail,
			Error:       err.Error(),
			Required:    false,
			Remediation: "Ensure ip command is available",
		})
		return
	}

	lines := strings.Split(string(output), "\n")
	var interfaceCount int
	for _, line := range lines {
		if strings.Contains(line, ":") && !strings.Contains(line, "lo:") {
			interfaceCount++
		}
	}

	status := CheckStatusPass
	if interfaceCount < 2 {
		status = CheckStatusWarn
	}

	pm.addCheck(result, PreflightCheck{
		Name:        "Network Interfaces",
		Description: fmt.Sprintf("Network interfaces: %d", interfaceCount),
		Status:      status,
		Required:    false,
		Remediation: "Ensure sufficient network interfaces for experiments",
	})
}

// addCheck adds a check to the result
func (pm *PreflightManager) addCheck(result *PreflightResult, check PreflightCheck) {
	result.Checks = append(result.Checks, check)
}

// calculateSummary calculates the summary of all checks
func (pm *PreflightManager) calculateSummary(checks []PreflightCheck) PreflightSummary {
	summary := PreflightSummary{
		TotalChecks: len(checks),
	}

	for _, check := range checks {
		switch check.Status {
		case CheckStatusPass:
			summary.PassedChecks++
		case CheckStatusFail:
			summary.FailedChecks++
		case CheckStatusWarn:
			summary.WarningChecks++
		case CheckStatusSkip:
			summary.SkippedChecks++
		}
	}

	// Determine overall status
	if summary.FailedChecks > 0 {
		summary.OverallStatus = CheckStatusFail
	} else if summary.WarningChecks > 0 {
		summary.OverallStatus = CheckStatusWarn
	} else {
		summary.OverallStatus = CheckStatusPass
	}

	return summary
}

// PrintResults prints the preflight check results in a formatted way
func (pr *PreflightResult) PrintResults() {
	fmt.Printf("\n=== CHAOSLABS AGENT PREFLIGHT CHECK RESULTS ===\n")
	fmt.Printf("Timestamp: %s\n", pr.Timestamp.Format(time.RFC3339))
	fmt.Printf("Hostname: %s\n", pr.Hostname)
	fmt.Printf("OS: %s\n", pr.OS)
	fmt.Printf("Architecture: %s\n", pr.Architecture)
	fmt.Printf("\n")

	// Print each check
	for _, check := range pr.Checks {
		statusIcon := "✅"
		switch check.Status {
		case CheckStatusPass:
			statusIcon = "✅"
		case CheckStatusFail:
			statusIcon = "❌"
		case CheckStatusWarn:
			statusIcon = "⚠️"
		case CheckStatusSkip:
			statusIcon = "⏭️"
		}

		fmt.Printf("%s %s: %s\n", statusIcon, check.Name, check.Description)
		if check.Status == CheckStatusFail || check.Status == CheckStatusWarn {
			if check.Error != "" {
				fmt.Printf("   Error: %s\n", check.Error)
			}
			fmt.Printf("   Remediation: %s\n", check.Remediation)
		}
		fmt.Printf("\n")
	}

	// Print summary
	fmt.Printf("=== SUMMARY ===\n")
	fmt.Printf("Total Checks: %d\n", pr.Summary.TotalChecks)
	fmt.Printf("Passed: %d ✅\n", pr.Summary.PassedChecks)
	fmt.Printf("Failed: %d ❌\n", pr.Summary.FailedChecks)
	fmt.Printf("Warnings: %d ⚠️\n", pr.Summary.WarningChecks)
	fmt.Printf("Skipped: %d ⏭️\n", pr.Summary.SkippedChecks)
	fmt.Printf("Overall Status: %s\n", pr.Summary.OverallStatus)

	if pr.Summary.OverallStatus == CheckStatusFail {
		fmt.Printf("\n❌ PREFLIGHT CHECKS FAILED - Agent may not function properly\n")
		fmt.Printf("Please address the failed checks before running experiments.\n")
	} else if pr.Summary.OverallStatus == CheckStatusWarn {
		fmt.Printf("\n⚠️  PREFLIGHT CHECKS PASSED WITH WARNINGS\n")
		fmt.Printf("Agent should function but some features may be limited.\n")
	} else {
		fmt.Printf("\n✅ PREFLIGHT CHECKS PASSED - Agent is ready for experiments\n")
	}
}

// getHostname gets the hostname
func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}

func main() {
	var (
		outputFormat = flag.String("format", "text", "Output format: text, json, or json-pretty")
		outputFile   = flag.String("output", "", "Output file path (optional)")
		checkOnly    = flag.Bool("check-only", false, "Only run checks, don't print results")
	)
	flag.Parse()

	// Create preflight manager
	preflightManager := NewPreflightManager()

	// Run all checks
	result, err := preflightManager.RunAllChecks()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running preflight checks: %v\n", err)
		os.Exit(1)
	}

	// Print results if not check-only mode
	if !*checkOnly {
		switch *outputFormat {
		case "text":
			result.PrintResults()
		case "json":
			outputJSON(result, false, *outputFile)
		case "json-pretty":
			outputJSON(result, true, *outputFile)
		default:
			fmt.Fprintf(os.Stderr, "Unknown output format: %s\n", *outputFormat)
			os.Exit(1)
		}
	}

	// Exit with appropriate code based on results
	if result.Summary.OverallStatus == CheckStatusFail {
		os.Exit(1)
	} else if result.Summary.OverallStatus == CheckStatusWarn {
		os.Exit(2)
	}
}

// outputJSON outputs the result in JSON format
func outputJSON(result *PreflightResult, pretty bool, outputFile string) {
	var data []byte
	var err error

	if pretty {
		data, err = json.MarshalIndent(result, "", "  ")
	} else {
		data, err = json.Marshal(result)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
		os.Exit(1)
	}

	if outputFile != "" {
		// Ensure directory exists
		dir := filepath.Dir(outputFile)
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
			os.Exit(1)
		}

		// Write to file
		if err := os.WriteFile(outputFile, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Results written to: %s\n", outputFile)
	} else {
		// Write to stdout
		fmt.Println(string(data))
	}
}
