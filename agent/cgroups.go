package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// CgroupManager manages cgroups v2 for experiment resource constraints
type CgroupManager struct {
	mu           sync.RWMutex
	cgroupsPath  string
	experiments  map[string]*CgroupState
	cleanupTimer *time.Timer
}

// CgroupState tracks the state of a cgroup for an experiment
type CgroupState struct {
	ExperimentID string
	Path         string
	Config       *CgroupConfig
	Created      time.Time
	TTL          time.Duration
}

// CgroupConfig defines resource limits for a cgroup
type CgroupConfig struct {
	CPUQuota     int64   `json:"cpu_quota"`     // CPU quota in microseconds (e.g., 100000 = 1 core)
	CPUMax       float64 `json:"cpu_max"`       // CPU max percentage (e.g., 50.0 = 50%)
	MemoryHigh   int64   `json:"memory_high"`   // Memory high limit in bytes
	MemoryMax    int64   `json:"memory_max"`    // Memory max limit in bytes
	MemorySwap   int64   `json:"memory_swap"`   // Memory swap limit in bytes
	PidsMax      int64   `json:"pids_max"`      // Maximum number of processes
	IOWeight     int64   `json:"io_weight"`     // IO weight (1-1000)
	NetworkClass int64   `json:"network_class"` // Network class (1-7)
}

// NewCgroupManager creates a new cgroup manager
func NewCgroupManager() *CgroupManager {
	cgm := &CgroupManager{
		cgroupsPath: "/sys/fs/cgroup",
		experiments: make(map[string]*CgroupState),
	}

	// Start cleanup goroutine
	go cgm.cleanupRoutine()

	return cgm
}

// CreateCgroup creates a new cgroup for an experiment
func (cgm *CgroupManager) CreateCgroup(expID string, config *CgroupConfig, ttl time.Duration) error {
	cgm.mu.Lock()
	defer cgm.mu.Unlock()

	// Validate cgroups v2 availability
	if err := cgm.validateCgroupsV2(); err != nil {
		return fmt.Errorf("cgroups v2 validation failed: %w", err)
	}

	// Create cgroup path
	cgroupPath := filepath.Join(cgm.cgroupsPath, "chaoslabs", expID)
	if err := os.MkdirAll(cgroupPath, 0755); err != nil {
		return fmt.Errorf("failed to create cgroup directory: %w", err)
	}

	// Apply resource limits
	if err := cgm.applyResourceLimits(cgroupPath, config); err != nil {
		// Cleanup on failure
		os.RemoveAll(cgroupPath)
		return fmt.Errorf("failed to apply resource limits: %w", err)
	}

	// Record cgroup state
	cgroupState := &CgroupState{
		ExperimentID: expID,
		Path:         cgroupPath,
		Config:       config,
		Created:      time.Now(),
		TTL:          ttl,
	}
	cgm.experiments[expID] = cgroupState

	// Set cleanup timer
	if ttl > 0 {
		time.AfterFunc(ttl, func() {
			cgm.DestroyCgroup(expID)
		})
	}

	log.Printf("[Cgroups] Created cgroup for experiment %s: %s", expID, cgroupPath)
	return nil
}

// DestroyCgroup destroys a cgroup and cleans up resources
func (cgm *CgroupManager) DestroyCgroup(expID string) error {
	cgm.mu.Lock()
	defer cgm.mu.Unlock()

	cgroupState, exists := cgm.experiments[expID]
	if !exists {
		return fmt.Errorf("cgroup for experiment %s not found", expID)
	}

	// Kill all processes in the cgroup
	if err := cgm.killProcessesInCgroup(cgroupState.Path); err != nil {
		log.Printf("[Cgroups] Warning: failed to kill processes in cgroup %s: %v", expID, err)
	}

	// Remove the cgroup directory
	if err := os.RemoveAll(cgroupState.Path); err != nil {
		return fmt.Errorf("failed to remove cgroup directory: %w", err)
	}

	// Remove from tracking
	delete(cgm.experiments, expID)

	log.Printf("[Cgroups] Destroyed cgroup for experiment %s", expID)
	return nil
}

// AddProcess adds a process to a cgroup
func (cgm *CgroupManager) AddProcess(expID string, pid int) error {
	cgm.mu.RLock()
	defer cgm.mu.RUnlock()

	cgroupState, exists := cgm.experiments[expID]
	if !exists {
		return fmt.Errorf("cgroup for experiment %s not found", expID)
	}

	// Write PID to cgroup.procs
	procsFile := filepath.Join(cgroupState.Path, "cgroup.procs")
	pidStr := strconv.Itoa(pid)

	if err := os.WriteFile(procsFile, []byte(pidStr), 0644); err != nil {
		return fmt.Errorf("failed to add process %d to cgroup: %w", pid, err)
	}

	log.Printf("[Cgroups] Added process %d to cgroup %s", pid, expID)
	return nil
}

// GetCgroupStatus returns the current status of all cgroups
func (cgm *CgroupManager) GetCgroupStatus() map[string]interface{} {
	cgm.mu.RLock()
	defer cgm.mu.RUnlock()

	status := make(map[string]interface{})

	for expID, cgroupState := range cgm.experiments {
		expStatus := map[string]interface{}{
			"path":         cgroupState.Path,
			"created":      cgroupState.Created,
			"ttl":          cgroupState.TTL,
			"config":       cgroupState.Config,
			"processes":    0,
			"cpu_usage":    0.0,
			"memory_usage": 0,
		}

		// Get current resource usage
		if usage, err := cgm.getResourceUsage(cgroupState.Path); err == nil {
			expStatus["processes"] = usage.ProcessCount
			expStatus["cpu_usage"] = usage.CPUUsage
			expStatus["memory_usage"] = usage.MemoryUsage
		}

		status[expID] = expStatus
	}

	return status
}

// validateCgroupsV2 checks if cgroups v2 is available and properly configured
func (cgm *CgroupManager) validateCgroupsV2() error {
	// Check if cgroups v2 filesystem is mounted
	if _, err := os.Stat(filepath.Join(cgm.cgroupsPath, "cgroup.controllers")); os.IsNotExist(err) {
		return fmt.Errorf("cgroups v2 not available at %s", cgm.cgroupsPath)
	}

	// Check if we can create directories
	testPath := filepath.Join(cgm.cgroupsPath, "chaoslabs", "test")
	if err := os.MkdirAll(testPath, 0755); err != nil {
		return fmt.Errorf("cannot create cgroup directories: %w", err)
	}
	defer os.RemoveAll(testPath)

	// Check if we can write to cgroup.procs
	testProcsFile := filepath.Join(testPath, "cgroup.procs")
	if err := os.WriteFile(testProcsFile, []byte("0"), 0644); err != nil {
		return fmt.Errorf("cannot write to cgroup.procs: %w", err)
	}

	return nil
}

// applyResourceLimits applies resource limits to a cgroup
func (cgm *CgroupManager) applyResourceLimits(cgroupPath string, config *CgroupConfig) error {
	// Apply CPU limits
	if config.CPUQuota > 0 {
		cpuMaxFile := filepath.Join(cgroupPath, "cpu.max")
		cpuMaxValue := fmt.Sprintf("%d 100000", config.CPUQuota)
		if err := os.WriteFile(cpuMaxFile, []byte(cpuMaxValue), 0644); err != nil {
			return fmt.Errorf("failed to set CPU quota: %w", err)
		}
	}

	if config.CPUMax > 0 {
		cpuMaxFile := filepath.Join(cgroupPath, "cpu.max")
		cpuMaxValue := fmt.Sprintf("%.0f 100000", config.CPUMax*1000)
		if err := os.WriteFile(cpuMaxFile, []byte(cpuMaxValue), 0644); err != nil {
			return fmt.Errorf("failed to set CPU max: %w", err)
		}
	}

	// Apply memory limits
	if config.MemoryHigh > 0 {
		memoryHighFile := filepath.Join(cgroupPath, "memory.high")
		if err := os.WriteFile(memoryHighFile, []byte(strconv.FormatInt(config.MemoryHigh, 10)), 0644); err != nil {
			return fmt.Errorf("failed to set memory.high: %w", err)
		}
	}

	if config.MemoryMax > 0 {
		memoryMaxFile := filepath.Join(cgroupPath, "memory.max")
		if err := os.WriteFile(memoryMaxFile, []byte(strconv.FormatInt(config.MemoryMax, 10)), 0644); err != nil {
			return fmt.Errorf("failed to set memory.max: %w", err)
		}
	}

	if config.MemorySwap > 0 {
		memorySwapFile := filepath.Join(cgroupPath, "memory.swap.max")
		if err := os.WriteFile(memorySwapFile, []byte(strconv.FormatInt(config.MemorySwap, 10)), 0644); err != nil {
			return fmt.Errorf("failed to set memory.swap.max: %w", err)
		}
	}

	// Apply process limits
	if config.PidsMax > 0 {
		pidsMaxFile := filepath.Join(cgroupPath, "pids.max")
		if err := os.WriteFile(pidsMaxFile, []byte(strconv.FormatInt(config.PidsMax, 10)), 0644); err != nil {
			return fmt.Errorf("failed to set pids.max: %w", err)
		}
	}

	// Apply IO limits
	if config.IOWeight > 0 {
		ioWeightFile := filepath.Join(cgroupPath, "io.weight")
		if err := os.WriteFile(ioWeightFile, []byte(strconv.FormatInt(config.IOWeight, 10)), 0644); err != nil {
			return fmt.Errorf("failed to set io.weight: %w", err)
		}
	}

	return nil
}

// killProcessesInCgroup kills all processes in a cgroup
func (cgm *CgroupManager) killProcessesInCgroup(cgroupPath string) error {
	procsFile := filepath.Join(cgroupPath, "cgroup.procs")

	// Read PIDs from cgroup.procs
	data, err := os.ReadFile(procsFile)
	if err != nil {
		return fmt.Errorf("failed to read cgroup.procs: %w", err)
	}

	pids := strings.Fields(string(data))
	for _, pidStr := range pids {
		if pid, err := strconv.Atoi(pidStr); err == nil && pid > 1 {
			// Send SIGTERM first
			if err := exec.Command("kill", "-TERM", strconv.Itoa(pid)).Run(); err == nil {
				// Wait a bit, then send SIGKILL if still running
				time.Sleep(100 * time.Millisecond)
				exec.Command("kill", "-KILL", strconv.Itoa(pid)).Run()
			}
		}
	}

	return nil
}

// ResourceUsage represents current resource usage in a cgroup
type ResourceUsage struct {
	ProcessCount int     `json:"process_count"`
	CPUUsage     float64 `json:"cpu_usage"`
	MemoryUsage  int64   `json:"memory_usage"`
}

// getResourceUsage gets current resource usage from a cgroup
func (cgm *CgroupManager) getResourceUsage(cgroupPath string) (*ResourceUsage, error) {
	usage := &ResourceUsage{}

	// Get process count
	if data, err := os.ReadFile(filepath.Join(cgroupPath, "cgroup.procs")); err == nil {
		usage.ProcessCount = len(strings.Fields(string(data)))
	}

	// Get CPU usage
	if data, err := os.ReadFile(filepath.Join(cgroupPath, "cpu.stat")); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "usage_usec") {
				if fields := strings.Fields(line); len(fields) >= 2 {
					if usageUsec, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
						usage.CPUUsage = float64(usageUsec) / 1000000.0 // Convert to seconds
					}
				}
			}
		}
	}

	// Get memory usage
	if data, err := os.ReadFile(filepath.Join(cgroupPath, "memory.current")); err == nil {
		if memoryBytes, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64); err == nil {
			usage.MemoryUsage = memoryBytes
		}
	}

	return usage, nil
}

// cleanupRoutine periodically cleans up expired cgroups
func (cgm *CgroupManager) cleanupRoutine() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		cgm.mu.Lock()
		now := time.Now()
		var expiredCgroups []string

		for expID, cgroupState := range cgm.experiments {
			if cgroupState.TTL > 0 && now.Sub(cgroupState.Created) > cgroupState.TTL {
				expiredCgroups = append(expiredCgroups, expID)
			}
		}

		cgm.mu.Unlock()

		// Destroy expired cgroups
		for _, expID := range expiredCgroups {
			cgm.DestroyCgroup(expID)
		}
	}
}

// ExportState exports the current cgroup state for persistence
func (cgm *CgroupManager) ExportState() ([]byte, error) {
	cgm.mu.RLock()
	defer cgm.mu.RUnlock()

	state := struct {
		Experiments map[string]*CgroupState `json:"experiments"`
		Timestamp   time.Time               `json:"timestamp"`
	}{
		Experiments: cgm.experiments,
		Timestamp:   time.Now(),
	}

	return json.MarshalIndent(state, "", "  ")
}

// ImportState imports cgroup state from persistence
func (cgm *CgroupManager) ImportState(data []byte) error {
	var state struct {
		Experiments map[string]*CgroupState `json:"experiments"`
		Timestamp   time.Time               `json:"timestamp"`
	}

	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to unmarshal state: %w", err)
	}

	cgm.mu.Lock()
	defer cgm.mu.Unlock()

	cgm.experiments = state.Experiments

	// Restart cleanup routine
	if cgm.cleanupTimer != nil {
		cgm.cleanupTimer.Stop()
	}
	go cgm.cleanupRoutine()

	return nil
}

// GetDefaultConfig returns a safe default configuration for experiments
func (cgm *CgroupManager) GetDefaultConfig() *CgroupConfig {
	return &CgroupConfig{
		CPUQuota:     50000,             // 0.5 cores
		CPUMax:       25.0,              // 25% CPU max
		MemoryHigh:   100 * 1024 * 1024, // 100MB
		MemoryMax:    200 * 1024 * 1024, // 200MB
		MemorySwap:   50 * 1024 * 1024,  // 50MB swap
		PidsMax:      10,                // Max 10 processes
		IOWeight:     100,               // Normal IO weight
		NetworkClass: 1,                 // Best effort
	}
}

// GetHighLoadConfig returns a configuration for high-load experiments
func (cgm *CgroupManager) GetHighLoadConfig() *CgroupConfig {
	return &CgroupConfig{
		CPUQuota:     200000,                 // 2 cores
		CPUMax:       50.0,                   // 50% CPU max
		MemoryHigh:   500 * 1024 * 1024,      // 500MB
		MemoryMax:    1 * 1024 * 1024 * 1024, // 1GB
		MemorySwap:   200 * 1024 * 1024,      // 200MB swap
		PidsMax:      50,                     // Max 50 processes
		IOWeight:     200,                    // Higher IO weight
		NetworkClass: 2,                      // Interactive
	}
}
