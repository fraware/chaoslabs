package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	ctx := context.Background()
	tp, err := newOTLPTracerProvider(ctx, "chaoslabs-controller", "1.0.0")
	if err != nil {
		slog.Error("failed to initialize tracer", "err", err)
		os.Exit(1)
	}
	defer func() { _ = tp.Shutdown(context.Background()) }()
	otel.SetTracerProvider(tp)

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
	handler := RequestIDMiddleware(
		CORSMiddleware(
			SecurityHeadersMiddleware(
				rateLimitMiddleware.Middleware(
					validationMiddleware.Middleware(
						ConditionalGetMiddleware(mux),
					),
				),
			),
		),
	)

	// Expose Prometheus metrics endpoint directly (bypass rate limiting)
	http.Handle("/metrics", promhttp.Handler())

	// Apply middleware to all other routes
	http.Handle("/", handler)

	slog.Info("ChaosLab Controller listening", "addr", ":8080")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		slog.Error("controller failed", "err", err)
		os.Exit(1)
	}
}
