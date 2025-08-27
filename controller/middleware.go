package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/time/rate"
)

// ValidationMiddleware provides strict schema validation for API requests
type ValidationMiddleware struct {
	validator *validator.Validate
}

// RateLimitMiddleware provides per-key and role-based rate limiting
type RateLimitMiddleware struct {
	limiters map[string]*RateLimiter
	mu       sync.RWMutex
	config   *RateLimitConfig
}

// RateLimitConfig defines rate limiting rules
type RateLimitConfig struct {
	GlobalRPS     int           `json:"global_rps"`
	DefaultRPS    int           `json:"default_rps"`
	BurstSize     int           `json:"burst_size"`
	RoleRPS       map[string]int `json:"role_rps"`
	KeyRPS        map[string]int `json:"key_rps"`
	CleanupPeriod time.Duration `json:"cleanup_period"`
}

// RateLimiter wraps rate.Limiter with additional metadata
type RateLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
	apiKey   string
	role     string
}

// Prometheus metrics for API hardening
var (
	apiRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_requests_total",
			Help: "Total number of API requests",
		},
		[]string{"method", "endpoint", "status", "api_key", "role"},
	)
	
	apiRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "api_request_duration_seconds",
			Help:    "API request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint", "api_key", "role"},
	)
	
	rateLimitHits = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rate_limit_hits_total",
			Help: "Total number of rate limit hits",
		},
		[]string{"api_key", "role", "limit_type"},
	)
	
	validationErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "validation_errors_total",
			Help: "Total number of validation errors",
		},
		[]string{"field", "error_type"},
	)
)

func init() {
	prometheus.MustRegister(apiRequestsTotal)
	prometheus.MustRegister(apiRequestDuration)
	prometheus.MustRegister(rateLimitHits)
	prometheus.MustRegister(validationErrors)
}

// NewValidationMiddleware creates a new validation middleware
func NewValidationMiddleware() *ValidationMiddleware {
	v := validator.New()
	
	// Register custom validators
	v.RegisterValidation("experiment_type", validateExperimentType)
	v.RegisterValidation("positive_duration", validatePositiveDuration)
	
	return &ValidationMiddleware{
		validator: v,
	}
}

// validateExperimentType validates experiment type values
func validateExperimentType(fl validator.FieldLevel) bool {
	validTypes := []string{"network_latency", "network_loss", "cpu_stress", "memory_stress", "process_kill"}
	expType := fl.Field().String()
	
	for _, valid := range validTypes {
		if expType == valid {
			return true
		}
	}
	return false
}

// validatePositiveDuration validates that duration is positive
func validatePositiveDuration(fl validator.FieldLevel) bool {
	duration := fl.Field().Int()
	return duration > 0
}

// Validate validates request payload and returns structured errors
func (vm *ValidationMiddleware) Validate(v interface{}) *ValidationErrorResponse {
	err := vm.validator.Struct(v)
	if err == nil {
		return nil
	}
	
	var errors []ValidationError
	for _, err := range err.(validator.ValidationErrors) {
		field := strings.ToLower(err.Field())
		tag := err.Tag()
		
		validationErrors.WithLabelValues(field, tag).Inc()
		
		errors = append(errors, ValidationError{
			Field:   field,
			Tag:     tag,
			Value:   fmt.Sprintf("%v", err.Value()),
			Message: getValidationMessage(err),
		})
	}
	
	return &ValidationErrorResponse{
		Error:   "validation_failed",
		Message: "Request validation failed",
		Details: errors,
	}
}

// ValidationError represents a single validation error
type ValidationError struct {
	Field   string `json:"field"`
	Tag     string `json:"tag"`
	Value   string `json:"value"`
	Message string `json:"message"`
}

// ValidationErrorResponse represents validation error response
type ValidationErrorResponse struct {
	Error   string            `json:"error"`
	Message string            `json:"message"`
	Details []ValidationError `json:"details"`
}

// getValidationMessage returns human-readable validation messages
func getValidationMessage(err validator.FieldError) string {
	switch err.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", err.Field())
	case "min":
		return fmt.Sprintf("%s must be at least %s", err.Field(), err.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s", err.Field(), err.Param())
	case "experiment_type":
		return fmt.Sprintf("%s must be one of: network_latency, network_loss, cpu_stress, memory_stress, process_kill", err.Field())
	case "positive_duration":
		return fmt.Sprintf("%s must be a positive number", err.Field())
	default:
		return fmt.Sprintf("%s is invalid", err.Field())
	}
}

// NewRateLimitMiddleware creates a new rate limit middleware
func NewRateLimitMiddleware(config *RateLimitConfig) *RateLimitMiddleware {
	if config == nil {
		config = &RateLimitConfig{
			GlobalRPS:     1000,
			DefaultRPS:    100,
			BurstSize:     10,
			RoleRPS:       make(map[string]int),
			KeyRPS:        make(map[string]int),
			CleanupPeriod: 10 * time.Minute,
		}
		
		// Default role-based limits
		config.RoleRPS["admin"] = 1000
		config.RoleRPS["user"] = 100
		config.RoleRPS["readonly"] = 50
	}
	
	rlm := &RateLimitMiddleware{
		limiters: make(map[string]*RateLimiter),
		config:   config,
	}
	
	// Start cleanup routine
	go rlm.cleanupRoutine()
	
	return rlm
}

// GetLimiter gets or creates a rate limiter for the given key and role
func (rlm *RateLimitMiddleware) GetLimiter(apiKey, role string) *RateLimiter {
	rlm.mu.Lock()
	defer rlm.mu.Unlock()
	
	key := fmt.Sprintf("%s:%s", apiKey, role)
	
	if limiter, exists := rlm.limiters[key]; exists {
		limiter.lastSeen = time.Now()
		return limiter
	}
	
	// Determine rate limit based on key or role
	rps := rlm.config.DefaultRPS
	
	if keyRPS, exists := rlm.config.KeyRPS[apiKey]; exists {
		rps = keyRPS
	} else if roleRPS, exists := rlm.config.RoleRPS[role]; exists {
		rps = roleRPS
	}
	
	limiter := &RateLimiter{
		limiter:  rate.NewLimiter(rate.Limit(rps), rlm.config.BurstSize),
		lastSeen: time.Now(),
		apiKey:   apiKey,
		role:     role,
	}
	
	rlm.limiters[key] = limiter
	return limiter
}

// cleanupRoutine removes stale rate limiters
func (rlm *RateLimitMiddleware) cleanupRoutine() {
	ticker := time.NewTicker(rlm.config.CleanupPeriod)
	defer ticker.Stop()
	
	for range ticker.C {
		rlm.cleanup()
	}
}

// cleanup removes rate limiters that haven't been used recently
func (rlm *RateLimitMiddleware) cleanup() {
	rlm.mu.Lock()
	defer rlm.mu.Unlock()
	
	cutoff := time.Now().Add(-2 * rlm.config.CleanupPeriod)
	
	for key, limiter := range rlm.limiters {
		if limiter.lastSeen.Before(cutoff) {
			delete(rlm.limiters, key)
		}
	}
}

// extractAPIKey extracts API key from request
func extractAPIKey(r *http.Request) string {
	// Try Authorization header first
	if auth := r.Header.Get("Authorization"); auth != "" {
		if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
			return strings.TrimSpace(auth[7:])
		}
	}
	
	// Try X-API-Key header
	if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
		return apiKey
	}
	
	// Try query parameter
	return r.URL.Query().Get("api_key")
}

// extractRole extracts user role from request (placeholder - implement based on your auth system)
func extractRole(r *http.Request, apiKey string) string {
	// Placeholder implementation - replace with actual role resolution
	role := r.Header.Get("X-User-Role")
	if role == "" {
		// Default role mapping based on API key patterns
		if strings.HasPrefix(apiKey, "admin_") {
			return "admin"
		} else if strings.HasPrefix(apiKey, "readonly_") {
			return "readonly"
		}
		return "user"
	}
	return role
}

// RateLimitMiddleware HTTP middleware
func (rlm *RateLimitMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		apiKey := extractAPIKey(r)
		if apiKey == "" {
			apiKey = "anonymous"
		}
		
		role := extractRole(r, apiKey)
		limiter := rlm.GetLimiter(apiKey, role)
		
		// Check rate limit
		if !limiter.limiter.Allow() {
			rateLimitHits.WithLabelValues(apiKey, role, "per_key").Inc()
			
			// Calculate retry-after based on rate limit
			retryAfter := int(time.Second / time.Duration(limiter.limiter.Limit()))
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%.0f", float64(limiter.limiter.Limit())))
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Second).Unix(), 10))
			
			http.Error(w, `{"error":"rate_limit_exceeded","message":"Rate limit exceeded. Please retry after the specified time.","retry_after_seconds":`+strconv.Itoa(retryAfter)+`}`, http.StatusTooManyRequests)
			
			apiRequestsTotal.WithLabelValues(r.Method, r.URL.Path, "429", apiKey, role).Inc()
			return
		}
		
		// Add rate limit headers
		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%.0f", float64(limiter.limiter.Limit())))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", limiter.limiter.Tokens()))
		
		// Wrap response writer to capture status
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		
		next.ServeHTTP(wrapped, r)
		
		// Record metrics
		duration := time.Since(start).Seconds()
		apiRequestsTotal.WithLabelValues(r.Method, r.URL.Path, strconv.Itoa(wrapped.statusCode), apiKey, role).Inc()
		apiRequestDuration.WithLabelValues(r.Method, r.URL.Path, apiKey, role).Observe(duration)
	})
}

// ValidationMiddleware HTTP middleware
func (vm *ValidationMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only validate POST and PUT requests with JSON content
		if (r.Method == http.MethodPost || r.Method == http.MethodPut) && 
		   strings.Contains(r.Header.Get("Content-Type"), "application/json") {
			
			// This will be handled by individual handlers that need validation
			// We just add the validator to the request context
			ctx := context.WithValue(r.Context(), "validator", vm)
			r = r.WithContext(ctx)
		}
		
		next.ServeHTTP(w, r)
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// ConditionalGetMiddleware implements ETag support for caching
func ConditionalGetMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only apply to GET requests
		if r.Method != http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}
		
		// For history endpoints, generate ETag based on last modified time
		if strings.Contains(r.URL.Path, "/experiments") {
			// Generate a simple ETag based on current time and request parameters
			etag := fmt.Sprintf(`"experiments-%d"`, time.Now().Unix()/60) // 1-minute granularity
			
			w.Header().Set("ETag", etag)
			w.Header().Set("Cache-Control", "public, max-age=60")
			
			// Check If-None-Match header
			if match := r.Header.Get("If-None-Match"); match != "" {
				if match == etag {
					w.WriteHeader(http.StatusNotModified)
					return
				}
			}
		}
		
		next.ServeHTTP(w, r)
	})
}

// CORSMiddleware handles CORS headers
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key, X-User-Role")
		w.Header().Set("Access-Control-Expose-Headers", "X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset, Retry-After")
		
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// SecurityHeadersMiddleware adds security headers
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		w.Header().Set("Content-Security-Policy", "default-src 'self'")
		
		next.ServeHTTP(w, r)
	})
}