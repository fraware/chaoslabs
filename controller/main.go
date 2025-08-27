package main

import (
	"context"
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func main() {
	// Initialize distributed tracing via Jaeger.
	tp, err := initTracer()
	if err != nil {
		log.Fatalf("failed to initialize tracer: %v", err)
	}
	// Ensure tracer provider shuts down when the application exits.
	defer func() { _ = tp.Shutdown(context.Background()) }()

	// Initialize health checker
	healthChecker := NewHealthChecker("1.0.0")

	// Initialize middleware
	validationMiddleware := NewValidationMiddleware()
	rateLimitMiddleware := NewRateLimitMiddleware(nil) // Use default config

	// Create a new ServeMux for better control over routing
	mux := http.NewServeMux()

	// Register application endpoints with middleware chain
	registerHandlers(mux, healthChecker)

	// Apply middleware chain
	handler := CORSMiddleware(
		SecurityHeadersMiddleware(
			rateLimitMiddleware.Middleware(
				validationMiddleware.Middleware(
					ConditionalGetMiddleware(mux),
				),
			),
		),
	)

	// Expose Prometheus metrics endpoint directly (bypass rate limiting)
	http.Handle("/metrics", promhttp.Handler())

	// Apply middleware to all other routes
	http.Handle("/", handler)

	log.Println("ChaosLab Controller running on :8080")
	log.Println("Endpoints:")
	log.Println("  POST /start - Start chaos experiment")
	log.Println("  POST /stop - Stop chaos experiment")
	log.Println("  GET /experiments - List experiments")
	log.Println("  GET /healthz - Health check")
	log.Println("  GET /readyz - Readiness check")
	log.Println("  GET /metrics - Prometheus metrics")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Controller failed to start: %v", err)
	}
}

// initTracer sets up an OpenTelemetry tracer provider with Jaeger exporter.
func initTracer() (*sdktrace.TracerProvider, error) {
	exp, err := jaeger.New(jaeger.WithCollectorEndpoint(
		jaeger.WithEndpoint("http://jaeger-collector:14268/api/traces"),
	))
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)
	return tp, nil
}
