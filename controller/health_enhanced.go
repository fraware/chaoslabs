package main

import (
	"encoding/json"
	"net/http"
	"runtime"
	"time"
)

// EnhancedHealthStatus provides comprehensive health information
type EnhancedHealthStatus struct {
	Status      string                 `json:"status"`
	Timestamp   time.Time              `json:"timestamp"`
	Version     string                 `json:"version"`
	Uptime      string                 `json:"uptime"`
	StartTime   time.Time              `json:"start_time"`
	Components  map[string]interface{} `json:"components"`
	Preflight   *PreflightHealth       `json:"preflight,omitempty"`
	System      *SystemHealth          `json:"system,omitempty"`
	Experiments *ExperimentHealth      `json:"experiments,omitempty"`
}

// PreflightHealth represents the health status of preflight checks
type PreflightHealth struct {
	Status          string    `json:"status"`
	LastCheck       time.Time `json:"last_check"`
	TotalChecks     int       `json:"total_checks"`
	PassedChecks    int       `json:"passed_checks"`
	FailedChecks    int       `json:"failed_checks"`
	WarningChecks   int       `json:"warning_checks"`
	SuccessRate     float64   `json:"success_rate"`
	LastFailure     string    `json:"last_failure,omitempty"`
	Recommendations []string  `json:"recommendations,omitempty"`
}

// SystemHealth represents system resource health
type SystemHealth struct {
	CPU        *ResourceHealth `json:"cpu,omitempty"`
	Memory     *ResourceHealth `json:"memory,omitempty"`
	Disk       *ResourceHealth `json:"disk,omitempty"`
	Network    *ResourceHealth `json:"network,omitempty"`
	Processes  *ResourceHealth `json:"processes,omitempty"`
	Containers *ResourceHealth `json:"containers,omitempty"`
}

// ResourceHealth represents the health of a specific resource
type ResourceHealth struct {
	Status      string    `json:"status"`
	Usage       float64   `json:"usage_percent"`
	Available   float64   `json:"available"`
	Threshold   float64   `json:"threshold"`
	LastUpdated time.Time `json:"last_updated"`
}

// ExperimentHealth represents experiment execution health
type ExperimentHealth struct {
	ActiveExperiments int     `json:"active_experiments"`
	TotalStarted      int     `json:"total_started"`
	TotalCompleted    int     `json:"total_completed"`
	TotalFailed       int     `json:"total_failed"`
	SuccessRate       float64 `json:"success_rate"`
	AverageDuration   float64 `json:"average_duration_seconds"`
	LastExperiment    string  `json:"last_experiment,omitempty"`
}

// EnhancedHealthHandler provides comprehensive health status
func EnhancedHealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	health := &EnhancedHealthStatus{
		Status:    "healthy",
		Timestamp: time.Now().UTC(),
		Version:   "1.0.0",
		StartTime: getStartTime(),
		Components: map[string]interface{}{
			"controller": map[string]interface{}{
				"status": "healthy",
				"port":   8080,
			},
			"http": map[string]interface{}{
				"status": "healthy",
				"endpoints": []string{
					"/start",
					"/stop",
					"/experiments",
					"/health",
					"/health/enhanced",
					"/health/socketio",
					"/ready",
					"/metrics",
				},
			},
		},
	}

	// Calculate uptime
	if !health.StartTime.IsZero() {
		health.Uptime = time.Since(health.StartTime).String()
	}

	// Add preflight health if available
	if preflightHealth := getPreflightHealth(); preflightHealth != nil {
		health.Preflight = preflightHealth
	}

	// Add system health
	health.System = getSystemHealth()

	// Add experiment health
	health.OverallStatus()

	// Set appropriate HTTP status
	if health.Status == "healthy" {
		w.WriteHeader(http.StatusOK)
	} else if health.Status == "degraded" {
		w.WriteHeader(http.StatusOK) // Still OK but with warnings
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	json.NewEncoder(w).Encode(health)
}

// getStartTime returns the application start time
func getStartTime() time.Time {
	// In a real implementation, this would be stored when the application starts
	// For now, we'll use a placeholder
	return time.Now().Add(-time.Hour) // Simulate 1 hour uptime
}

// getPreflightHealth retrieves preflight check health information
func getPreflightHealth() *PreflightHealth {
	// In a real implementation, this would query the preflight system
	// For now, we'll return simulated data
	return &PreflightHealth{
		Status:        "healthy",
		LastCheck:     time.Now().Add(-5 * time.Minute),
		TotalChecks:   25,
		PassedChecks:  23,
		FailedChecks:  1,
		WarningChecks: 1,
		SuccessRate:   92.0,
		LastFailure:   "CAP_SYS_ADMIN capability missing",
		Recommendations: []string{
			"Run container with --privileged flag",
			"Add CAP_SYS_ADMIN capability",
			"Ensure tc command is available",
		},
	}
}

// getSystemHealth retrieves system resource health information
func getSystemHealth() *SystemHealth {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Calculate memory usage percentage
	memUsage := float64(m.Alloc) / float64(m.Sys) * 100

	// Simulate other resource health
	cpuHealth := &ResourceHealth{
		Status:      "healthy",
		Usage:       45.2,
		Available:   54.8,
		Threshold:   80.0,
		LastUpdated: time.Now(),
	}

	memHealth := &ResourceHealth{
		Status:      "healthy",
		Usage:       memUsage,
		Available:   100 - memUsage,
		Threshold:   85.0,
		LastUpdated: time.Now(),
	}

	diskHealth := &ResourceHealth{
		Status:      "healthy",
		Usage:       62.1,
		Available:   37.9,
		Threshold:   90.0,
		LastUpdated: time.Now(),
	}

	networkHealth := &ResourceHealth{
		Status:      "healthy",
		Usage:       23.4,
		Available:   76.6,
		Threshold:   80.0,
		LastUpdated: time.Now(),
	}

	processesHealth := &ResourceHealth{
		Status:      "healthy",
		Usage:       12.8,
		Available:   87.2,
		Threshold:   90.0,
		LastUpdated: time.Now(),
	}

	containersHealth := &ResourceHealth{
		Status:      "healthy",
		Usage:       8.5,
		Available:   91.5,
		Threshold:   95.0,
		LastUpdated: time.Now(),
	}

	return &SystemHealth{
		CPU:        cpuHealth,
		Memory:     memHealth,
		Disk:       diskHealth,
		Network:    networkHealth,
		Processes:  processesHealth,
		Containers: containersHealth,
	}
}

// getExperimentHealth retrieves experiment execution health information
func getExperimentHealth() *ExperimentHealth {
	// In a real implementation, this would query the experiment metrics
	// For now, we'll return simulated data
	return &ExperimentHealth{
		ActiveExperiments: 3,
		TotalStarted:      15,
		TotalCompleted:    12,
		TotalFailed:       2,
		SuccessRate:       85.7,
		AverageDuration:   45.2,
		LastExperiment:    "network-latency-test",
	}
}

// OverallStatus determines the overall health status based on all components
func (h *EnhancedHealthStatus) OverallStatus() {
	// Check if any critical components are unhealthy
	if h.Preflight != nil && h.Preflight.Status == "unhealthy" {
		h.Status = "unhealthy"
		return
	}

	// Check system resources
	if h.System != nil {
		if h.System.CPU != nil && h.System.CPU.Usage > h.System.CPU.Threshold {
			h.Status = "degraded"
		}
		if h.System.Memory != nil && h.System.Memory.Usage > h.System.Memory.Threshold {
			h.Status = "degraded"
		}
		if h.System.Disk != nil && h.System.Disk.Usage > h.System.Disk.Threshold {
			h.Status = "degraded"
		}
	}

	// Check experiment health
	if h.Experiments != nil && h.Experiments.SuccessRate < 80.0 {
		h.Status = "degraded"
	}

	// If no issues found, status remains "healthy"
}

// PreflightHealthHandler provides detailed preflight health information
func PreflightHealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	preflightHealth := getPreflightHealth()
	if preflightHealth == nil {
		http.Error(w, "Preflight health information not available", http.StatusServiceUnavailable)
		return
	}

	// Set appropriate HTTP status
	if preflightHealth.Status == "healthy" {
		w.WriteHeader(http.StatusOK)
	} else if preflightHealth.Status == "degraded" {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	json.NewEncoder(w).Encode(preflightHealth)
}

// SystemHealthHandler provides detailed system health information
func SystemHealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	systemHealth := getSystemHealth()
	if systemHealth == nil {
		http.Error(w, "System health information not available", http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(systemHealth)
}

// ExperimentHealthHandler provides detailed experiment health information
func ExperimentHealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	experimentHealth := getExperimentHealth()
	if experimentHealth == nil {
		http.Error(w, "Experiment health information not available", http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(experimentHealth)
}

// HealthSummaryHandler provides a summary of all health components
func HealthSummaryHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	summary := map[string]interface{}{
		"overall_status": "healthy",
		"timestamp":      time.Now().UTC(),
		"components": map[string]string{
			"controller":  "healthy",
			"preflight":   "healthy",
			"system":      "healthy",
			"experiments": "healthy",
		},
		"checks": map[string]interface{}{
			"total":   4,
			"healthy": 4,
			"failed":  0,
		},
		"last_check": time.Now().Add(-1 * time.Minute),
		"uptime":     time.Since(getStartTime()).String(),
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(summary)
}

