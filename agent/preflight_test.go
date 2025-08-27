package main

import (
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestNewPreflightManager(t *testing.T) {
	pm := NewPreflightManager()
	if pm == nil {
		t.Fatal("NewPreflightManager returned nil")
	}
	if pm.checks == nil {
		t.Fatal("checks slice not initialized")
	}
}

func TestRunAllChecks(t *testing.T) {
	pm := NewPreflightManager()
	result, err := pm.RunAllChecks()

	if err != nil {
		t.Fatalf("RunAllChecks failed: %v", err)
	}

	if result == nil {
		t.Fatal("RunAllChecks returned nil result")
	}

	// Check basic fields
	if result.Timestamp.IsZero() {
		t.Error("Timestamp not set")
	}
	if result.Hostname == "" {
		t.Error("Hostname not set")
	}
	if result.OS == "" {
		t.Error("OS not set")
	}
	if result.Architecture == "" {
		t.Error("Architecture not set")
	}

	// Check that we have checks
	if len(result.Checks) == 0 {
		t.Error("No checks were run")
	}

	// Check summary
	if result.Summary.TotalChecks == 0 {
		t.Error("Summary total checks not set")
	}
}

func TestCheckCapability(t *testing.T) {
	pm := NewPreflightManager()
	result := &PreflightResult{
		Checks: make([]PreflightCheck, 0),
	}

	// Test required capability
	pm.checkCapability(result, "CAP_NET_ADMIN", "Test description", true, "Test remediation")

	if len(result.Checks) != 1 {
		t.Errorf("Expected 1 check, got %d", len(result.Checks))
	}

	check := result.Checks[0]
	if check.Name != "CAP_NET_ADMIN" {
		t.Errorf("Expected name CAP_NET_ADMIN, got %s", check.Name)
	}
	if check.Description != "Test description" {
		t.Errorf("Expected description 'Test description', got %s", check.Description)
	}
	if check.Required != true {
		t.Errorf("Expected required true, got %v", check.Required)
	}
	if check.Remediation != "Test remediation" {
		t.Errorf("Expected remediation 'Test remediation', got %s", check.Remediation)
	}
}

func TestCheckToolAvailability(t *testing.T) {
	pm := NewPreflightManager()
	result := &PreflightResult{
		Checks: make([]PreflightCheck, 0),
	}

	// Test with a tool that should exist (ls)
	pm.checkToolAvailability(result, "ls", "Test description", true, "Test remediation")

	if len(result.Checks) != 1 {
		t.Errorf("Expected 1 check, got %d", len(result.Checks))
	}

	check := result.Checks[0]
	if check.Name != "ls" {
		t.Errorf("Expected name 'ls', got %s", check.Name)
	}

	// Test with a tool that shouldn't exist
	pm.checkToolAvailability(result, "nonexistent_tool_12345", "Test description", false, "Test remediation")

	if len(result.Checks) != 2 {
		t.Errorf("Expected 2 checks, got %d", len(result.Checks))
	}

	check2 := result.Checks[1]
	if check2.Name != "nonexistent_tool_12345" {
		t.Errorf("Expected name 'nonexistent_tool_12345', got %s", check2.Name)
	}
	if check2.Status != CheckStatusWarn {
		t.Errorf("Expected status WARN for non-required tool, got %s", check2.Status)
	}
}

func TestCheckKernelModule(t *testing.T) {
	pm := NewPreflightManager()
	result := &PreflightResult{
		Checks: make([]PreflightCheck, 0),
	}

	// Test with a common kernel module
	pm.checkKernelModule(result, "sch_netem", "Test description", true, "Test remediation")

	if len(result.Checks) != 1 {
		t.Errorf("Expected 1 check, got %d", len(result.Checks))
	}

	check := result.Checks[0]
	if check.Name != "sch_netem" {
		t.Errorf("Expected name 'sch_netem', got %s", check.Name)
	}
}

func TestIsRunningInContainer(t *testing.T) {
	pm := NewPreflightManager()

	// This test will depend on the environment
	// We just check that it doesn't panic
	_ = pm.isRunningInContainer()
}

func TestIsPrivilegedContainer(t *testing.T) {
	pm := NewPreflightManager()

	// This test will depend on the environment
	// We just check that it doesn't panic
	_ = pm.isPrivilegedContainer()
}

func TestHasContainerCgroup(t *testing.T) {
	pm := NewPreflightManager()

	// This test will depend on the environment
	// We just check that it doesn't panic
	_ = pm.hasContainerCgroup()
}

func TestIsKernelModuleAvailable(t *testing.T) {
	pm := NewPreflightManager()

	// Test with a common module
	available := pm.isKernelModuleAvailable("sch_netem")
	// We can't guarantee the result, but it shouldn't panic
	_ = available
}

func TestCheckKernelVersion(t *testing.T) {
	pm := NewPreflightManager()
	result := &PreflightResult{
		Checks: make([]PreflightCheck, 0),
	}

	pm.checkKernelVersion(result)

	if len(result.Checks) != 1 {
		t.Errorf("Expected 1 check, got %d", len(result.Checks))
	}

	check := result.Checks[0]
	if check.Name != "Kernel Version" {
		t.Errorf("Expected name 'Kernel Version', got %s", check.Name)
	}
}

func TestCheckAvailableMemory(t *testing.T) {
	pm := NewPreflightManager()
	result := &PreflightResult{
		Checks: make([]PreflightCheck, 0),
	}

	pm.checkAvailableMemory(result)

	if len(result.Checks) != 1 {
		t.Errorf("Expected 1 check, got %d", len(result.Checks))
	}

	check := result.Checks[0]
	if check.Name != "Available Memory" {
		t.Errorf("Expected name 'Available Memory', got %s", check.Name)
	}
}

func TestCheckAvailableDiskSpace(t *testing.T) {
	pm := NewPreflightManager()
	result := &PreflightResult{
		Checks: make([]PreflightCheck, 0),
	}

	pm.checkAvailableDiskSpace(result)

	if len(result.Checks) != 1 {
		t.Errorf("Expected 1 check, got %d", len(result.Checks))
	}

	check := result.Checks[0]
	if check.Name != "Available Disk Space" {
		t.Errorf("Expected name 'Available Disk Space', got %s", check.Name)
	}
}

func TestCheckCPUInfo(t *testing.T) {
	pm := NewPreflightManager()
	result := &PreflightResult{
		Checks: make([]PreflightCheck, 0),
	}

	pm.checkCPUInfo(result)

	if len(result.Checks) != 1 {
		t.Errorf("Expected 1 check, got %d", len(result.Checks))
	}

	check := result.Checks[0]
	if check.Name != "CPU Information" {
		t.Errorf("Expected name 'CPU Information', got %s", check.Name)
	}
}

func TestCheckNetworkInterfaceCount(t *testing.T) {
	pm := NewPreflightManager()
	result := &PreflightResult{
		Checks: make([]PreflightCheck, 0),
	}

	pm.checkNetworkInterfaceCount(result)

	if len(result.Checks) != 1 {
		t.Errorf("Expected 1 check, got %d", len(result.Checks))
	}

	check := result.Checks[0]
	if check.Name != "Network Interfaces" {
		t.Errorf("Expected name 'Network Interfaces', got %s", check.Name)
	}
}

func TestAddCheck(t *testing.T) {
	pm := NewPreflightManager()
	result := &PreflightResult{
		Checks: make([]PreflightCheck, 0),
	}

	check := PreflightCheck{
		Name:        "Test Check",
		Description: "Test Description",
		Status:      CheckStatusPass,
		Required:    true,
	}

	pm.addCheck(result, check)

	if len(result.Checks) != 1 {
		t.Errorf("Expected 1 check, got %d", len(result.Checks))
	}

	if result.Checks[0].Name != "Test Check" {
		t.Errorf("Expected name 'Test Check', got %s", result.Checks[0].Name)
	}
}

func TestCalculateSummary(t *testing.T) {
	checks := []PreflightCheck{
		{Status: CheckStatusPass},
		{Status: CheckStatusPass},
		{Status: CheckStatusFail},
		{Status: CheckStatusWarn},
		{Status: CheckStatusSkip},
	}

	pm := NewPreflightManager()
	summary := pm.calculateSummary(checks)

	if summary.TotalChecks != 5 {
		t.Errorf("Expected total checks 5, got %d", summary.TotalChecks)
	}
	if summary.PassedChecks != 2 {
		t.Errorf("Expected passed checks 2, got %d", summary.PassedChecks)
	}
	if summary.FailedChecks != 1 {
		t.Errorf("Expected failed checks 1, got %d", summary.FailedChecks)
	}
	if summary.WarningChecks != 1 {
		t.Errorf("Expected warning checks 1, got %d", summary.WarningChecks)
	}
	if summary.SkippedChecks != 1 {
		t.Errorf("Expected skipped checks 1, got %d", summary.SkippedChecks)
	}

	// Test overall status logic
	if summary.OverallStatus != CheckStatusFail {
		t.Errorf("Expected overall status FAIL, got %s", summary.OverallStatus)
	}

	// Test with only warnings
	checksOnlyWarnings := []PreflightCheck{
		{Status: CheckStatusPass},
		{Status: CheckStatusWarn},
	}
	summaryWarnings := pm.calculateSummary(checksOnlyWarnings)
	if summaryWarnings.OverallStatus != CheckStatusWarn {
		t.Errorf("Expected overall status WARN, got %s", summaryWarnings.OverallStatus)
	}

	// Test with only passes
	checksOnlyPass := []PreflightCheck{
		{Status: CheckStatusPass},
		{Status: CheckStatusPass},
	}
	summaryPass := pm.calculateSummary(checksOnlyPass)
	if summaryPass.OverallStatus != CheckStatusPass {
		t.Errorf("Expected overall status PASS, got %s", summaryPass.OverallStatus)
	}
}

func TestGetHostname(t *testing.T) {
	hostname := getHostname()
	if hostname == "" {
		t.Error("Hostname should not be empty")
	}
	if hostname == "unknown" {
		t.Error("Hostname should not be 'unknown'")
	}
}

func TestPreflightResultPrintResults(t *testing.T) {
	result := &PreflightResult{
		Timestamp:    time.Now(),
		Hostname:     "test-host",
		OS:           "linux",
		Architecture: "amd64",
		Checks: []PreflightCheck{
			{
				Name:        "Test Check 1",
				Description: "Test Description 1",
				Status:      CheckStatusPass,
				Required:    true,
			},
			{
				Name:        "Test Check 2",
				Description: "Test Description 2",
				Status:      CheckStatusFail,
				Error:       "Test Error",
				Required:    true,
				Remediation: "Test Remediation",
			},
		},
		Summary: PreflightSummary{
			TotalChecks:   2,
			PassedChecks:  1,
			FailedChecks:  1,
			WarningChecks: 0,
			SkippedChecks: 0,
			OverallStatus: CheckStatusFail,
		},
	}

	// This test just ensures the method doesn't panic
	// In a real test, we might capture stdout and verify the output
	result.PrintResults()
}

func TestCheckCapabilityAlternative(t *testing.T) {
	pm := NewPreflightManager()

	// Test with CAP_NET_ADMIN
	result := pm.checkCapabilityAlternative(unix.CAP_NET_ADMIN)
	// We can't guarantee the result, but it shouldn't panic
	_ = result

	// Test with CAP_SYS_ADMIN
	result2 := pm.checkCapabilityAlternative(unix.CAP_SYS_ADMIN)
	// We can't guarantee the result, but it shouldn't panic
	_ = result2

	// Test with unknown capability
	result3 := pm.checkCapabilityAlternative(999999)
	if result3 != false {
		t.Errorf("Expected false for unknown capability, got %v", result3)
	}
}

func TestGetCapability(t *testing.T) {
	pm := NewPreflightManager()

	// Test with valid capability names
	capNames := []string{"CAP_NET_ADMIN", "CAP_SYS_ADMIN", "CAP_SYS_RESOURCE"}

	for _, capName := range capNames {
		_, err := pm.getCapability(capName)
		// We can't guarantee the result, but it shouldn't panic
		_ = err
	}

	// Test with invalid capability name
	_, err := pm.getCapability("INVALID_CAP")
	if err == nil {
		t.Error("Expected error for invalid capability name")
	}
}

func TestCheckContainerMounts(t *testing.T) {
	pm := NewPreflightManager()
	result := &PreflightResult{
		Checks: make([]PreflightCheck, 0),
	}

	pm.checkContainerMounts(result)

	// Should have 3 mount checks
	if len(result.Checks) != 3 {
		t.Errorf("Expected 3 mount checks, got %d", len(result.Checks))
	}

	// Check that all required mounts are present
	expectedMounts := []string{"/proc", "/sys", "/dev"}
	for _, mount := range expectedMounts {
		found := false
		for _, check := range result.Checks {
			if check.Name == "Mount: "+mount {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected mount check for %s", mount)
		}
	}
}

// Benchmark tests
func BenchmarkRunAllChecks(b *testing.B) {
	pm := NewPreflightManager()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := pm.RunAllChecks()
		if err != nil {
			b.Fatalf("RunAllChecks failed: %v", err)
		}
	}
}

func BenchmarkCheckCapability(b *testing.B) {
	pm := NewPreflightManager()
	result := &PreflightResult{
		Checks: make([]PreflightCheck, 0),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pm.checkCapability(result, "CAP_NET_ADMIN", "Test description", true, "Test remediation")
	}
}

func BenchmarkCheckToolAvailability(b *testing.B) {
	pm := NewPreflightManager()
	result := &PreflightResult{
		Checks: make([]PreflightCheck, 0),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pm.checkToolAvailability(result, "ls", "Test description", true, "Test remediation")
	}
}
