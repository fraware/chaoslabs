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
	// Initialize distributed tracing.
	tp, err := initTracer()
	if err != nil {
		log.Fatalf("failed to initialize tracer: %v", err)
	}
	defer func() { _ = tp.Shutdown(context.Background()) }()

	registerAgentHandlers()

	// Expose Prometheus metrics endpoint.
	http.Handle("/metrics", promhttp.Handler())

	log.Println("ChaosLab Agent running on :9090")
	if err := http.ListenAndServe(":9090", nil); err != nil {
		log.Fatalf("Agent failed to start: %v", err)
	}
}

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
