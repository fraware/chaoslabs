package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// HealthChecker manages health and readiness checks
type HealthChecker struct {
	mu           sync.RWMutex
	dependencies map[string]HealthDependency
	startTime    time.Time
	version      string
}

// HealthDependency represents a dependency health check
type HealthDependency struct {
	Name      string        `json:"name"`
	Status    string        `json:"status"`
	LastCheck time.Time     `json:"last_check"`
	Latency   time.Duration `json:"latency"`
	Error     string        `json:"error,omitempty"`
	CheckFunc func() error  `json:"-"`
	Timeout   time.Duration `json:"-"`
	Interval  time.Duration `json:"-"`
	Critical  bool          `json:"critical"`
}

// HealthStatus represents overall health status
type HealthStatus struct {
	Status       string                      `json:"status"`
	Timestamp    time.Time                   `json:"timestamp"`
	Version      string                      `json:"version"`
	Uptime       string                      `json:"uptime"`
	Dependencies map[string]HealthDependency `json:"dependencies"`
	System       SystemInfo                  `json:"system"`
	Metrics      HealthMetrics               `json:"metrics"`
}

// SystemInfo contains system information
type SystemInfo struct {
	Hostname     string     `json:"hostname"`
	Platform     string     `json:"platform"`
	Architecture string     `json:"architecture"`
	GoVersion    string     `json:"go_version"`
	Goroutines   int        `json:"goroutines"`
	Memory       MemoryInfo `json:"memory"`
}

// MemoryInfo contains memory usage information
type MemoryInfo struct {
	Allocated  uint64 `json:"allocated_bytes"`
	TotalAlloc uint64 `json:"total_alloc_bytes"`
	System     uint64 `json:"system_bytes"`
	GCRuns     uint32 `json:"gc_runs"`
}

// HealthMetrics contains application metrics
type HealthMetrics struct {
	RequestsTotal       int64   `json:"requests_total"`
	RequestsPerSecond   float64 `json:"requests_per_second"`
	AverageResponseTime float64 `json:"avg_response_time_ms"`
	ErrorRate           float64 `json:"error_rate_percent"`
}

// ReadinessStatus represents readiness check result
type ReadinessStatus struct {
	Ready        bool                        `json:"ready"`
	Timestamp    time.Time                   `json:"timestamp"`
	Dependencies map[string]HealthDependency `json:"dependencies"`
	Reason       string                      `json:"reason,omitempty"`
}

// Prometheus metrics for health monitoring
var (
	healthCheckDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "health_check_duration_seconds",
			Help: "Health check duration in seconds",
		},
		[]string{"dependency", "status"},
	)

	healthCheckStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "health_check_status",
			Help: "Health check status (1 for healthy, 0 for unhealthy)",
		},
		[]string{"dependency"},
	)

	applicationUptime = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "application_uptime_seconds",
			Help: "Application uptime in seconds",
		},
	)
)

func init() {
	prometheus.MustRegister(healthCheckDuration)
	prometheus.MustRegister(healthCheckStatus)
	prometheus.MustRegister(applicationUptime)
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(version string) *HealthChecker {
	hc := &HealthChecker{
		dependencies: make(map[string]HealthDependency),
		startTime:    time.Now(),
		version:      version,
	}

	// Register default dependencies
	hc.RegisterDependency("database", HealthDependency{
		Name:      "database",
		CheckFunc: hc.checkDatabase,
		Timeout:   5 * time.Second,
		Interval:  30 * time.Second,
		Critical:  true,
	})

	hc.RegisterDependency("agents", HealthDependency{
		Name:      "agents",
		CheckFunc: hc.checkAgents,
		Timeout:   3 * time.Second,
		Interval:  15 * time.Second,
		Critical:  false,
	})

	hc.RegisterDependency("jaeger", HealthDependency{
		Name:      "jaeger",
		CheckFunc: hc.checkJaeger,
		Timeout:   3 * time.Second,
		Interval:  60 * time.Second,
		Critical:  false,
	})

	// Start periodic health checks
	go hc.startPeriodicChecks()

	// Update uptime metric periodically
	go hc.updateUptimeMetric()

	return hc
}

// RegisterDependency registers a new dependency for health checking
func (hc *HealthChecker) RegisterDependency(name string, dep HealthDependency) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	dep.Name = name
	dep.Status = "unknown"
	dep.LastCheck = time.Now()
	hc.dependencies[name] = dep
}

// startPeriodicChecks starts periodic health checks for all dependencies
func (hc *HealthChecker) startPeriodicChecks() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		hc.mu.RLock()
		deps := make(map[string]HealthDependency)
		for k, v := range hc.dependencies {
			deps[k] = v
		}
		hc.mu.RUnlock()

		for name, dep := range deps {
			if time.Since(dep.LastCheck) >= dep.Interval {
				go hc.checkDependency(name, dep)
			}
		}
	}
}

// checkDependency performs a health check for a specific dependency
func (hc *HealthChecker) checkDependency(name string, dep HealthDependency) {
	start := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), dep.Timeout)
	defer cancel()

	var err error
	done := make(chan error, 1)

	go func() {
		done <- dep.CheckFunc()
	}()

	select {
	case err = <-done:
	case <-ctx.Done():
		err = ctx.Err()
	}

	duration := time.Since(start)
	status := "healthy"
	errorMsg := ""

	if err != nil {
		status = "unhealthy"
		errorMsg = err.Error()
	}

	// Update dependency status
	hc.mu.Lock()
	updatedDep := hc.dependencies[name]
	updatedDep.Status = status
	updatedDep.LastCheck = time.Now()
	updatedDep.Latency = duration
	updatedDep.Error = errorMsg
	hc.dependencies[name] = updatedDep
	hc.mu.Unlock()

	// Update Prometheus metrics
	healthCheckDuration.WithLabelValues(name, status).Observe(duration.Seconds())
	if status == "healthy" {
		healthCheckStatus.WithLabelValues(name).Set(1)
	} else {
		healthCheckStatus.WithLabelValues(name).Set(0)
	}
}

// updateUptimeMetric updates the uptime Prometheus metric
func (hc *HealthChecker) updateUptimeMetric() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		uptime := time.Since(hc.startTime).Seconds()
		applicationUptime.Set(uptime)
	}
}

// GetHealthStatus returns the current health status
func (hc *HealthChecker) GetHealthStatus() HealthStatus {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	status := "healthy"

	// Check if any critical dependencies are unhealthy
	for _, dep := range hc.dependencies {
		if dep.Critical && dep.Status != "healthy" {
			status = "unhealthy"
			break
		}
	}

	// Get system information
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	systemInfo := SystemInfo{
		Platform:     runtime.GOOS,
		Architecture: runtime.GOARCH,
		GoVersion:    runtime.Version(),
		Goroutines:   runtime.NumGoroutine(),
		Memory: MemoryInfo{
			Allocated:  m.Alloc,
			TotalAlloc: m.TotalAlloc,
			System:     m.Sys,
			GCRuns:     m.NumGC,
		},
	}

	// Calculate metrics (placeholder - implement based on your metrics collection)
	metrics := HealthMetrics{
		RequestsTotal:       0, // TODO: Get from prometheus metrics
		RequestsPerSecond:   0, // TODO: Calculate from metrics
		AverageResponseTime: 0, // TODO: Calculate from metrics
		ErrorRate:           0, // TODO: Calculate from metrics
	}

	return HealthStatus{
		Status:       status,
		Timestamp:    time.Now(),
		Version:      hc.version,
		Uptime:       time.Since(hc.startTime).String(),
		Dependencies: hc.dependencies,
		System:       systemInfo,
		Metrics:      metrics,
	}
}

// GetReadinessStatus returns the readiness status
func (hc *HealthChecker) GetReadinessStatus() ReadinessStatus {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	ready := true
	reason := ""

	// Check critical dependencies
	for _, dep := range hc.dependencies {
		if dep.Critical && dep.Status != "healthy" {
			ready = false
			if reason == "" {
				reason = fmt.Sprintf("Critical dependency '%s' is %s", dep.Name, dep.Status)
			}
		}
	}

	return ReadinessStatus{
		Ready:        ready,
		Timestamp:    time.Now(),
		Dependencies: hc.dependencies,
		Reason:       reason,
	}
}

// Health check functions
func (hc *HealthChecker) checkDatabase() error {
	// Placeholder for database health check
	// In a real implementation, you would check your actual database connection
	return nil
}

func (hc *HealthChecker) checkAgents() error {
	// Check if agents are reachable
	agentEndpoints := getAgentEndpoints()
	if len(agentEndpoints) == 0 {
		return fmt.Errorf("no agent endpoints configured")
	}

	// Try to reach at least one agent
	for _, endpoint := range agentEndpoints {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		req, err := http.NewRequestWithContext(ctx, "GET", endpoint+"/health", nil)
		if err != nil {
			cancel()
			continue
		}

		resp, err := http.DefaultClient.Do(req)
		cancel()

		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
	}

	return fmt.Errorf("no healthy agents found")
}

func (hc *HealthChecker) checkJaeger() error {
	// Check Jaeger collector health
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", "http://jaeger-collector:14269/health", nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jaeger collector returned status %d", resp.StatusCode)
	}

	return nil
}

// HTTP Handlers

// HealthzHandler handles /healthz endpoint
func (hc *HealthChecker) HealthzHandler(w http.ResponseWriter, r *http.Request) {
	status := hc.GetHealthStatus()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	if status.Status == "healthy" {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	json.NewEncoder(w).Encode(status)
}

// ReadyzHandler handles /readyz endpoint
func (hc *HealthChecker) ReadyzHandler(w http.ResponseWriter, r *http.Request) {
	status := hc.GetReadinessStatus()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	if status.Ready {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	json.NewEncoder(w).Encode(status)
}

// MetricsHandler is already provided by Prometheus, but we can extend it
func (hc *HealthChecker) MetricsHandler(w http.ResponseWriter, r *http.Request) {
	// Add custom business metrics
	customMetrics := map[string]interface{}{
		"experiments_active": 0, // TODO: Get from experiment manager
		"experiments_total":  0, // TODO: Get from experiment manager
		"agents_connected":   0, // TODO: Get from agent manager
		"uptime_seconds":     time.Since(hc.startTime).Seconds(),
		"version":            hc.version,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"custom_metrics":      customMetrics,
		"prometheus_endpoint": "/metrics",
		"note":                "For detailed metrics, use the /metrics endpoint with a Prometheus-compatible client",
	})
}
