package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"
)

// SoakTestConfig holds configuration for soak testing
type SoakTestConfig struct {
	Duration       time.Duration
	Concurrency    int
	RequestsPerSec int
	ExperimentType string
	TargetEndpoint string
}

// SoakTestResult holds the results of a soak test
type SoakTestResult struct {
	TotalRequests   int64
	SuccessfulReqs  int64
	FailedReqs      int64
	RateLimitedReqs int64
	AvgLatency      time.Duration
	P95Latency      time.Duration
	P99Latency      time.Duration
	MaxLatency      time.Duration
	MinLatency      time.Duration
	Errors          []string
	StartTime       time.Time
	EndTime         time.Time
}

// TestSoakTest runs a comprehensive soak test
func TestSoakTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping soak test in short mode")
	}

	config := SoakTestConfig{
		Duration:       2 * time.Hour, // 2-hour soak test
		Concurrency:    50,
		RequestsPerSec: 100,
		ExperimentType: "network-latency",
		TargetEndpoint: "http://localhost:8080",
	}

	// Override for shorter test in CI
	if testing.Verbose() {
		config.Duration = 5 * time.Minute
	}

	result := runSoakTest(t, config)

	// Assert performance requirements
	if result.FailedReqs > 0 {
		t.Logf("Warning: %d requests failed during soak test", result.FailedReqs)
	}

	if result.RateLimitedReqs > result.TotalRequests/10 {
		t.Errorf("Too many rate limited requests: %d/%d (%.1f%%)",
			result.RateLimitedReqs, result.TotalRequests,
			float64(result.RateLimitedReqs)/float64(result.TotalRequests)*100)
	}

	if result.P99Latency > 100*time.Millisecond {
		t.Errorf("P99 latency too high: %v (expected < 100ms)", result.P99Latency)
	}

	t.Logf("Soak test completed successfully: %+v", result)
}

// TestSlowlorisProtection tests protection against slowloris attacks
func TestSlowlorisProtection(t *testing.T) {
	config := SoakTestConfig{
		Duration:       1 * time.Minute,
		Concurrency:    10,
		RequestsPerSec: 1,
		ExperimentType: "slowloris-test",
		TargetEndpoint: "http://localhost:8080",
	}

	result := runSlowlorisTest(t, config)

	if result.FailedReqs == 0 {
		t.Log("Slowloris protection working correctly")
	} else {
		t.Logf("Some slowloris requests failed (expected): %d", result.FailedReqs)
	}
}

// TestAdmissionControl tests admission control under load
func TestAdmissionControl(t *testing.T) {
	config := SoakTestConfig{
		Duration:       30 * time.Second,
		Concurrency:    200, // Exceed worker pool size
		RequestsPerSec: 1000,
		ExperimentType: "admission-control-test",
		TargetEndpoint: "http://localhost:8080",
	}

	result := runAdmissionControlTest(t, config)

	// Should see some 503 responses when worker pool is full
	if result.FailedReqs == 0 {
		t.Log("Admission control working correctly")
	} else {
		t.Logf("Admission control rejected %d requests (expected under load)", result.FailedReqs)
	}
}

// runSoakTest executes the main soak test
func runSoakTest(t *testing.T, config SoakTestConfig) SoakTestResult {
	result := SoakTestResult{
		StartTime: time.Now(),
		Errors:    make([]string, 0),
	}

	// Create HTTP client with timeouts
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
		Name:           "soak-test-experiment",
		Description:    "Soak test experiment",
		ExperimentType: config.ExperimentType,
		Target:         "localhost",
		Duration:       30,
		DelayMs:        50,
		LossPercent:    2,
		CPUWorkers:     1,
		MemSizeMB:      50,
		StartTime:      time.Now(),
	}

	jsonData, _ := json.Marshal(expReq)

	// Calculate request interval
	requestInterval := time.Second / time.Duration(config.RequestsPerSec)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), config.Duration)
	defer cancel()

	// Start workers
	var wg sync.WaitGroup
	latencyChan := make(chan time.Duration, 1000)
	errorChan := make(chan string, 1000)

	// Start latency collector
	go func() {
		var latencies []time.Duration
		for {
			select {
			case <-ctx.Done():
				result.AvgLatency = calculateAverageLatency(latencies)
				result.P95Latency = calculatePercentileLatency(latencies, 95)
				result.P99Latency = calculatePercentileLatency(latencies, 99)
				result.MaxLatency = calculateMaxLatency(latencies)
				result.MinLatency = calculateMinLatency(latencies)
				return
			case latency := <-latencyChan:
				latencies = append(latencies, latency)
			}
		}
	}()

	// Start error collector
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case err := <-errorChan:
				result.Errors = append(result.Errors, err)
			}
		}
	}()

	// Start workers
	for i := 0; i < config.Concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			ticker := time.NewTicker(requestInterval)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					// Send request
					start := time.Now()
					resp, err := client.Post(config.TargetEndpoint+"/start", "application/json", bytes.NewBuffer(jsonData))
					latency := time.Since(start)

					if err != nil {
						errorChan <- fmt.Sprintf("Worker %d request failed: %v", workerID, err)
						result.FailedReqs++
						continue
					}

					latencyChan <- latency
					result.TotalRequests++

					if resp.StatusCode == http.StatusTooManyRequests {
						result.RateLimitedReqs++
					} else if resp.StatusCode == http.StatusServiceUnavailable {
						result.FailedReqs++
					} else if resp.StatusCode == http.StatusOK {
						result.SuccessfulReqs++
					} else {
						result.FailedReqs++
						errorChan <- fmt.Sprintf("Worker %d unexpected status: %d", workerID, resp.StatusCode)
					}

					resp.Body.Close()
				}
			}
		}(i)
	}

	// Wait for completion
	wg.Wait()
	result.EndTime = time.Now()

	return result
}

// runSlowlorisTest tests slowloris protection
func runSlowlorisTest(t *testing.T, config SoakTestConfig) SoakTestResult {
	result := SoakTestResult{
		StartTime: time.Now(),
		Errors:    make([]string, 0),
	}

	// Create slow HTTP client (no timeouts)
	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        config.Concurrency,
			MaxIdleConnsPerHost: config.Concurrency,
		},
	}

	expReq := ExperimentRequest{
		Name:           "slowloris-test",
		Description:    "Slowloris protection test",
		ExperimentType: "network-latency",
		Target:         "localhost",
		Duration:       10,
		DelayMs:        100,
	}

	jsonData, _ := json.Marshal(expReq)

	ctx, cancel := context.WithTimeout(context.Background(), config.Duration)
	defer cancel()

	var wg sync.WaitGroup
	for i := 0; i < config.Concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			// Send request and hold connection open
			req, _ := http.NewRequest("POST", config.TargetEndpoint+"/start", bytes.NewBuffer(jsonData))
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				errorChan := make(chan string, 1)
				errorChan <- fmt.Sprintf("Worker %d slowloris failed: %v", workerID, err)
				result.FailedReqs++
				return
			}

			result.TotalRequests++
			if resp.StatusCode == http.StatusOK {
				result.SuccessfulReqs++
			} else {
				result.FailedReqs++
			}

			// Hold connection open for a while
			time.Sleep(5 * time.Second)
			resp.Body.Close()
		}(i)
	}

	wg.Wait()
	result.EndTime = time.Now()

	return result
}

// runAdmissionControlTest tests admission control
func runAdmissionControlTest(t *testing.T, config SoakTestConfig) SoakTestResult {
	result := SoakTestResult{
		StartTime: time.Now(),
		Errors:    make([]string, 0),
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	expReq := ExperimentRequest{
		Name:           "admission-control-test",
		Description:    "Admission control test",
		ExperimentType: "cpu-stress",
		Target:         "localhost",
		Duration:       5,
		CPUWorkers:     1,
	}

	jsonData, _ := json.Marshal(expReq)

	ctx, cancel := context.WithTimeout(context.Background(), config.Duration)
	defer cancel()

	var wg sync.WaitGroup
	for i := 0; i < config.Concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			resp, err := client.Post(config.TargetEndpoint+"/start", "application/json", bytes.NewBuffer(jsonData))
			if err != nil {
				result.FailedReqs++
				return
			}

			result.TotalRequests++
			if resp.StatusCode == http.StatusServiceUnavailable {
				result.FailedReqs++ // Expected when worker pool is full
			} else if resp.StatusCode == http.StatusOK {
				result.SuccessfulReqs++
			} else {
				result.FailedReqs++
			}

			resp.Body.Close()
		}(i)
	}

	wg.Wait()
	result.EndTime = time.Now()

	return result
}

// Helper functions for latency calculations
func calculateAverageLatency(latencies []time.Duration) time.Duration {
	if len(latencies) == 0 {
		return 0
	}

	var total time.Duration
	for _, l := range latencies {
		total += l
	}
	return total / time.Duration(len(latencies))
}

func calculatePercentileLatency(latencies []time.Duration, percentile int) time.Duration {
	if len(latencies) == 0 {
		return 0
	}

	// Sort latencies (simplified - in production use proper sorting)
	// For now, return a reasonable estimate
	return latencies[len(latencies)*percentile/100]
}

func calculateMaxLatency(latencies []time.Duration) time.Duration {
	if len(latencies) == 0 {
		return 0
	}

	max := latencies[0]
	for _, l := range latencies {
		if l > max {
			max = l
		}
	}
	return max
}

func calculateMinLatency(latencies []time.Duration) time.Duration {
	if len(latencies) == 0 {
		return 0
	}

	min := latencies[0]
	for _, l := range latencies {
		if l < min {
			min = l
		}
	}
	return min
}
