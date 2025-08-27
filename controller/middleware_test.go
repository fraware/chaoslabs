package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestValidationMiddleware(t *testing.T) {
	vm := NewValidationMiddleware()

	tests := []struct {
		name     string
		request  ExperimentRequest
		valid    bool
		errorField string
	}{
		{
			name: "valid request",
			request: ExperimentRequest{
				Name:           "test-experiment",
				Description:    "Test description",
				ExperimentType: "network_latency",
				Target:         "test-target",
				Duration:       30,
				DelayMs:        100,
				LossPercent:    5,
				CPUWorkers:     2,
				MemSizeMB:      512,
				AgentCount:     1,
			},
			valid: true,
		},
		{
			name: "missing required name",
			request: ExperimentRequest{
				ExperimentType: "network_latency",
				Target:         "test-target",
				Duration:       30,
			},
			valid: false,
			errorField: "name",
		},
		{
			name: "invalid experiment type",
			request: ExperimentRequest{
				Name:           "test-experiment",
				ExperimentType: "invalid_type",
				Target:         "test-target",
				Duration:       30,
			},
			valid: false,
			errorField: "experiment_type",
		},
		{
			name: "duration too high",
			request: ExperimentRequest{
				Name:           "test-experiment",
				ExperimentType: "network_latency",
				Target:         "test-target",
				Duration:       5000, // exceeds max of 3600
			},
			valid: false,
			errorField: "duration",
		},
		{
			name: "negative duration",
			request: ExperimentRequest{
				Name:           "test-experiment",
				ExperimentType: "network_latency",
				Target:         "test-target",
				Duration:       -1,
			},
			valid: false,
			errorField: "duration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validationErr := vm.Validate(tt.request)
			
			if tt.valid && validationErr != nil {
				t.Errorf("Expected valid request, got validation error: %+v", validationErr)
			}
			
			if !tt.valid && validationErr == nil {
				t.Errorf("Expected validation error, got none")
			}
			
			if !tt.valid && validationErr != nil {
				found := false
				for _, err := range validationErr.Details {
					if err.Field == tt.errorField {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected error on field '%s', got errors: %+v", tt.errorField, validationErr.Details)
				}
			}
		})
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	config := &RateLimitConfig{
		GlobalRPS:     10,
		DefaultRPS:    2,
		BurstSize:     2,
		RoleRPS:       map[string]int{"admin": 5},
		KeyRPS:        map[string]int{"test-key": 3},
		CleanupPeriod: time.Minute,
	}
	
	rlm := NewRateLimitMiddleware(config)
	
	// Test basic rate limiting
	limiter := rlm.GetLimiter("test-user", "user")
	
	// Should allow first few requests up to burst
	for i := 0; i < config.BurstSize; i++ {
		if !limiter.limiter.Allow() {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}
	
	// Should deny the next request
	if limiter.limiter.Allow() {
		t.Error("Request should be rate limited")
	}
	
	// Test role-based limits
	adminLimiter := rlm.GetLimiter("admin-user", "admin")
	if adminLimiter.limiter.Limit() != 5 {
		t.Errorf("Expected admin rate limit of 5, got %f", float64(adminLimiter.limiter.Limit()))
	}
	
	// Test key-based limits
	keyLimiter := rlm.GetLimiter("test-key", "user")
	if keyLimiter.limiter.Limit() != 3 {
		t.Errorf("Expected key-based rate limit of 3, got %f", float64(keyLimiter.limiter.Limit()))
	}
}

func TestRateLimitHTTPMiddleware(t *testing.T) {
	config := &RateLimitConfig{
		GlobalRPS:     10,
		DefaultRPS:    1,
		BurstSize:     1,
		CleanupPeriod: time.Minute,
	}
	
	rlm := NewRateLimitMiddleware(config)
	
	handler := rlm.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	
	// First request should pass
	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.Header.Set("X-API-Key", "test-key")
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)
	
	if w1.Code != http.StatusOK {
		t.Errorf("First request should pass, got status %d", w1.Code)
	}
	
	// Second request should be rate limited
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.Header.Set("X-API-Key", "test-key")
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	
	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("Second request should be rate limited, got status %d", w2.Code)
	}
	
	// Check rate limit headers
	if w2.Header().Get("Retry-After") == "" {
		t.Error("Rate limited response should include Retry-After header")
	}
	
	if w2.Header().Get("X-RateLimit-Limit") == "" {
		t.Error("Response should include X-RateLimit-Limit header")
	}
}

func TestHealthEndpoints(t *testing.T) {
	hc := NewHealthChecker("test-version")
	
	// Test healthz endpoint
	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	hc.HealthzHandler(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Health check should return 200, got %d", w.Code)
	}
	
	var health HealthStatus
	if err := json.Unmarshal(w.Body.Bytes(), &health); err != nil {
		t.Errorf("Failed to parse health response: %v", err)
	}
	
	if health.Version != "test-version" {
		t.Errorf("Expected version 'test-version', got '%s'", health.Version)
	}
	
	// Test readyz endpoint
	req2 := httptest.NewRequest("GET", "/readyz", nil)
	w2 := httptest.NewRecorder()
	hc.ReadyzHandler(w2, req2)
	
	if w2.Code != http.StatusOK {
		t.Errorf("Readiness check should return 200, got %d", w2.Code)
	}
	
	var readiness ReadinessStatus
	if err := json.Unmarshal(w2.Body.Bytes(), &readiness); err != nil {
		t.Errorf("Failed to parse readiness response: %v", err)
	}
}

func TestValidationIntegration(t *testing.T) {
	// Test full integration with validation middleware
	vm := NewValidationMiddleware()
	
	handler := vm.Middleware(http.HandlerFunc(startExperimentHandler))
	
	// Test valid request
	validReq := ExperimentRequest{
		Name:           "test-experiment",
		ExperimentType: "network_latency",
		Target:         "test-target",
		Duration:       30,
		AgentCount:     1,
	}
	
	reqBody, _ := json.Marshal(validReq)
	req := httptest.NewRequest("POST", "/start", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	
	handler.ServeHTTP(w, req)
	
	if w.Code == http.StatusBadRequest {
		t.Errorf("Valid request should not return 400, body: %s", w.Body.String())
	}
	
	// Test invalid request
	invalidReq := ExperimentRequest{
		Name:           "", // Missing required field
		ExperimentType: "invalid_type",
		Target:         "test-target",
		Duration:       -1, // Invalid duration
	}
	
	reqBody2, _ := json.Marshal(invalidReq)
	req2 := httptest.NewRequest("POST", "/start", bytes.NewReader(reqBody2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	
	handler.ServeHTTP(w2, req2)
	
	if w2.Code != http.StatusBadRequest {
		t.Errorf("Invalid request should return 400, got %d", w2.Code)
	}
}