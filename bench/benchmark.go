package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// BenchmarkConfig holds configuration for benchmarking
type BenchmarkConfig struct {
	ControllerURL  string        `json:"controller_url"`
	AgentURL       string        `json:"agent_url"`
	Duration       time.Duration `json:"duration"`
	Concurrency    int           `json:"concurrency"`
	ExperimentType string        `json:"experiment_type"`
}

// BenchmarkResult holds the results of a benchmark run
type BenchmarkResult struct {
	Timestamp     time.Time       `json:"timestamp"`
	Config        BenchmarkConfig `json:"config"`
	HTTPLatency   LatencyStats    `json:"http_latency"`
	CPUUsage      ResourceStats   `json:"cpu_usage"`
	MemoryUsage   ResourceStats   `json:"memory_usage"`
	KernelMetrics KernelStats     `json:"kernel_metrics"`
	Errors        []string        `json:"errors"`
}

// LatencyStats holds HTTP latency statistics
type LatencyStats struct {
	P50    time.Duration `json:"p50"`
	P95    time.Duration `json:"p95"`
	P99    time.Duration `json:"p99"`
	Mean   time.Duration `json:"mean"`
	Min    time.Duration `json:"min"`
	Max    time.Duration `json:"max"`
	Count  int           `json:"count"`
	StdDev time.Duration `json:"std_dev"`
}

// ResourceStats holds resource usage statistics
type ResourceStats struct {
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	Mean   float64 `json:"mean"`
	P95    float64 `json:"p95"`
	P99    float64 `json:"p99"`
	StdDev float64 `json:"std_dev"`
}

// KernelStats holds kernel-level metrics
type KernelStats struct {
	TCPRetransmits  int64     `json:"tcp_retransmits"`
	TCPDrops        int64     `json:"tcp_drops"`
	NetDevDrops     int64     `json:"netdev_drops"`
	CPUInterrupts   float64   `json:"cpu_interrupts"`
	ContextSwitches int64     `json:"context_switches"`
	PageFaults      int64     `json:"page_faults"`
	LoadAverage     []float64 `json:"load_average"`
}

// ExperimentRequest represents a chaos experiment
type ExperimentRequest struct {
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	ExperimentType string    `json:"experiment_type"`
	Target         string    `json:"target"`
	Duration       int       `json:"duration"`
	DelayMs        int       `json:"delay_ms"`
	LossPercent    int       `json:"loss_percent"`
	CPUWorkers     int       `json:"cpu_workers"`
	MemSizeMB      int       `json:"mem_size_mb"`
	StartTime      time.Time `json:"start_time"`
}

// Metrics collector for real-time monitoring
type MetricsCollector struct {
	latencies []time.Duration
	cpuUsage  []float64
	memUsage  []float64
	mu        sync.RWMutex
}

func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		latencies: make([]time.Duration, 0, 10000),
		cpuUsage:  make([]float64, 0, 10000),
		memUsage:  make([]float64, 0, 10000),
	}
}

func (mc *MetricsCollector) AddLatency(latency time.Duration) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.latencies = append(mc.latencies, latency)
}

func (mc *MetricsCollector) AddCPUUsage(cpu float64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.cpuUsage = append(mc.cpuUsage, cpu)
}

func (mc *MetricsCollector) AddMemUsage(mem float64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.memUsage = append(mc.memUsage, mem)
}

func (mc *MetricsCollector) GetLatencies() []time.Duration {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	result := make([]time.Duration, len(mc.latencies))
	copy(result, mc.latencies)
	return result
}

func (mc *MetricsCollector) GetCPUUsage() []float64 {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	result := make([]float64, len(mc.cpuUsage))
	copy(result, mc.cpuUsage)
	return result
}

func (mc *MetricsCollector) GetMemUsage() []float64 {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	result := make([]float64, len(mc.memUsage))
	copy(result, mc.memUsage)
	return result
}

func main() {
	config := BenchmarkConfig{
		ControllerURL:  "http://localhost:8080",
		AgentURL:       "http://localhost:9090",
		Duration:       5 * time.Minute,
		Concurrency:    10,
		ExperimentType: "network-latency",
	}

	// Parse command line flags
	if len(os.Args) > 1 {
		config.ControllerURL = os.Args[1]
	}
	if len(os.Args) > 2 {
		config.AgentURL = os.Args[2]
	}

	log.Printf("Starting benchmark with config: %+v", config)

	// Run benchmark
	result := runBenchmark(config)

	// Save results
	saveResults(result)

	// Print summary
	printSummary(result)
}

func runBenchmark(config BenchmarkConfig) BenchmarkResult {
	result := BenchmarkResult{
		Timestamp: time.Now(),
		Config:    config,
		Errors:    make([]string, 0),
	}

	// Create metrics collector
	collector := NewMetricsCollector()

	// Start monitoring goroutines
	ctx, cancel := context.WithTimeout(context.Background(), config.Duration)
	defer cancel()

	var wg sync.WaitGroup

	// Start resource monitoring
	wg.Add(1)
	go func() {
		defer wg.Done()
		monitorResources(ctx, collector)
	}()

	// Start HTTP load testing
	wg.Add(1)
	go func() {
		defer wg.Done()
		runHTTPLoadTest(ctx, config, collector)
	}()

	// Wait for completion
	wg.Wait()

	// Collect kernel metrics
	result.KernelMetrics = collectKernelMetrics()

	// Calculate statistics
	result.HTTPLatency = calculateLatencyStats(collector.GetLatencies())
	result.CPUUsage = calculateResourceStats(collector.GetCPUUsage())
	result.MemoryUsage = calculateResourceStats(collector.GetMemUsage())

	return result
}

func runHTTPLoadTest(ctx context.Context, config BenchmarkConfig, collector *MetricsCollector) {
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        config.Concurrency,
			MaxIdleConnsPerHost: config.Concurrency,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	// Create experiment request
	expReq := ExperimentRequest{
		Name:           "benchmark-experiment",
		Description:    "Benchmark load test",
		ExperimentType: config.ExperimentType,
		Target:         "localhost",
		Duration:       30,
		DelayMs:        100,
		LossPercent:    5,
		CPUWorkers:     2,
		MemSizeMB:      100,
		StartTime:      time.Now(),
	}

	jsonData, _ := json.Marshal(expReq)

	// Calculate request interval for target RPS
	requestInterval := time.Second / time.Duration(config.Concurrency*2) // 2 RPS per worker

	ticker := time.NewTicker(requestInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Send request to controller
			start := time.Now()
			resp, err := client.Post(config.ControllerURL+"/start", "application/json", strings.NewReader(string(jsonData)))
			latency := time.Since(start)

			if err != nil {
				log.Printf("HTTP request failed: %v", err)
				continue
			}
			resp.Body.Close()

			collector.AddLatency(latency)
		}
	}
}

func monitorResources(ctx context.Context, collector *MetricsCollector) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Monitor controller CPU and memory
			cpu, mem := getProcessStats("controller")
			collector.AddCPUUsage(cpu)
			collector.AddMemUsage(mem)

			// Monitor agent CPU and memory
			cpu, mem = getProcessStats("agent")
			collector.AddCPUUsage(cpu)
			collector.AddMemUsage(mem)
		}
	}
}

func getProcessStats(processName string) (cpu, mem float64) {
	// Get CPU usage
	cmd := exec.Command("ps", "-p", getPID(processName), "-o", "%cpu,%mem", "--no-headers")
	output, err := cmd.Output()
	if err != nil {
		return 0, 0
	}

	fields := strings.Fields(string(output))
	if len(fields) >= 2 {
		cpu, _ = strconv.ParseFloat(fields[0], 64)
		mem, _ = strconv.ParseFloat(fields[1], 64)
	}

	return cpu, mem
}

func getPID(processName string) string {
	cmd := exec.Command("pgrep", processName)
	output, err := cmd.Output()
	if err != nil {
		return "1"
	}
	return strings.TrimSpace(string(output))
}

func collectKernelMetrics() KernelStats {
	stats := KernelStats{}

	// Get TCP retransmits
	cmd := exec.Command("cat", "/proc/net/netstat")
	output, err := cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, "TcpRetransSegs") {
				fields := strings.Fields(line)
				if len(fields) > 1 {
					stats.TCPRetransmits, _ = strconv.ParseInt(fields[1], 10, 64)
				}
			}
		}
	}

	// Get CPU interrupts
	cmd = exec.Command("cat", "/proc/stat")
	output, err = cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "intr") {
				fields := strings.Fields(line)
				if len(fields) > 1 {
					stats.CPUInterrupts, _ = strconv.ParseFloat(fields[1], 64)
				}
			}
		}
	}

	// Get context switches
	cmd = exec.Command("cat", "/proc/stat")
	output, err = cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "ctxt") {
				fields := strings.Fields(line)
				if len(fields) > 1 {
					stats.ContextSwitches, _ = strconv.ParseInt(fields[1], 10, 64)
				}
			}
		}
	}

	// Get page faults
	cmd = exec.Command("cat", "/proc/vmstat")
	output, err = cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "pgfault") {
				fields := strings.Fields(line)
				if len(fields) > 1 {
					stats.PageFaults, _ = strconv.ParseInt(fields[1], 10, 64)
				}
			}
		}
	}

	// Get load average
	if loadAvg, err := os.ReadFile("/proc/loadavg"); err == nil {
		fields := strings.Fields(string(loadAvg))
		if len(fields) >= 3 {
			for i := 0; i < 3; i++ {
				if load, err := strconv.ParseFloat(fields[i], 64); err == nil {
					stats.LoadAverage = append(stats.LoadAverage, load)
				}
			}
		}
	}

	return stats
}

func generatePlots(result BenchmarkResult, outputDir string) {
	// Generate simple text-based plots for now
	// In a full implementation, you'd use a plotting library like gonum/plot

	// Create latency distribution plot
	latencyPlot := fmt.Sprintf(`HTTP Latency Distribution
P50: %v
P95: %v
P99: %v
Mean: %v
StdDev: %v
`, result.HTTPLatency.P50, result.HTTPLatency.P95, result.HTTPLatency.P99,
		result.HTTPLatency.Mean, result.HTTPLatency.StdDev)

	latencyFile := fmt.Sprintf("%s/latency_distribution.txt", outputDir)
	os.WriteFile(latencyFile, []byte(latencyPlot), 0644)

	// Create resource usage plot
	resourcePlot := fmt.Sprintf(`Resource Usage Summary
CPU Usage:
  P95: %.2f%%
  P99: %.2f%%
  Mean: %.2f%%
  StdDev: %.2f%%

Memory Usage:
  P95: %.2f%%
  P99: %.2f%%
  Mean: %.2f%%
  StdDev: %.2f%%
`, result.CPUUsage.P95, result.CPUUsage.P99, result.CPUUsage.Mean, result.CPUUsage.StdDev,
		result.MemoryUsage.P95, result.MemoryUsage.P99, result.MemoryUsage.Mean, result.MemoryUsage.StdDev)

	resourceFile := fmt.Sprintf("%s/resource_usage.txt", outputDir)
	os.WriteFile(resourceFile, []byte(resourcePlot), 0644)

	log.Printf("Plots generated in %s", outputDir)
}

func calculateLatencyStats(latencies []time.Duration) LatencyStats {
	if len(latencies) == 0 {
		return LatencyStats{}
	}

	// Sort latencies for percentile calculation
	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	stats := LatencyStats{
		Count: len(sorted),
		Min:   sorted[0],
		Max:   sorted[len(sorted)-1],
	}

	// Calculate mean
	var total time.Duration
	for _, l := range sorted {
		total += l
	}
	stats.Mean = total / time.Duration(len(sorted))

	// Calculate standard deviation
	var varianceSum time.Duration
	for _, l := range sorted {
		diff := l - stats.Mean
		varianceSum += diff * diff
	}
	variance := varianceSum / time.Duration(len(sorted))
	stats.StdDev = time.Duration(math.Sqrt(float64(variance)))

	// Calculate percentiles
	stats.P50 = calculatePercentile(sorted, 50)
	stats.P95 = calculatePercentile(sorted, 95)
	stats.P99 = calculatePercentile(sorted, 99)

	return stats
}

func calculatePercentile(sorted []time.Duration, percentile int) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	index := (percentile * len(sorted)) / 100
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}

func calculateResourceStats(values []float64) ResourceStats {
	if len(values) == 0 {
		return ResourceStats{}
	}

	// Sort values for percentile calculation
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	stats := ResourceStats{
		Min: sorted[0],
		Max: sorted[len(sorted)-1],
	}

	// Calculate mean
	var total float64
	for _, v := range sorted {
		total += v
	}
	stats.Mean = total / float64(len(sorted))

	// Calculate standard deviation
	var varianceSum float64
	for _, v := range sorted {
		diff := v - stats.Mean
		varianceSum += diff * diff
	}
	variance := varianceSum / float64(len(sorted))
	stats.StdDev = math.Sqrt(variance)

	// Calculate percentiles
	stats.P95 = calculateFloatPercentile(sorted, 95)
	stats.P99 = calculateFloatPercentile(sorted, 99)

	return stats
}

func calculateFloatPercentile(sorted []float64, percentile int) float64 {
	if len(sorted) == 0 {
		return 0
	}
	index := (percentile * len(sorted)) / 100
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}

func saveResults(result BenchmarkResult) {
	// Save to JSON file
	data, _ := json.MarshalIndent(result, "", "  ")
	filename := fmt.Sprintf("benchmark_%s.json", time.Now().Format("20060102_150405"))
	os.WriteFile(filename, data, 0644)
	log.Printf("Results saved to %s", filename)
}

func printSummary(result BenchmarkResult) {
	fmt.Printf("\n=== Benchmark Summary ===\n")
	fmt.Printf("HTTP Latency P50: %v\n", result.HTTPLatency.P50)
	fmt.Printf("HTTP Latency P95: %v\n", result.HTTPLatency.P95)
	fmt.Printf("HTTP Latency P99: %v\n", result.HTTPLatency.P99)
	fmt.Printf("HTTP Latency Mean: %v\n", result.HTTPLatency.Mean)
	fmt.Printf("HTTP Latency Count: %d\n", result.HTTPLatency.Count)
	fmt.Printf("CPU Usage P95: %.2f%%\n", result.CPUUsage.P95)
	fmt.Printf("Memory Usage P95: %.2f%%\n", result.MemoryUsage.P95)
	fmt.Printf("TCP Retransmits: %d\n", result.KernelMetrics.TCPRetransmits)
	fmt.Printf("CPU Interrupts: %.0f\n", result.KernelMetrics.CPUInterrupts)
}
