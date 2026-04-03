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
	tp, err := newOTLPTracerProvider(ctx, "chaoslabs-agent", "1.0.0")
	if err != nil {
		slog.Error("failed to initialize tracer", "err", err)
		os.Exit(1)
	}
	defer func() { _ = tp.Shutdown(context.Background()) }()
	otel.SetTracerProvider(tp)

	registerAgentHandlers()

	http.Handle("/metrics", promhttp.Handler())

	slog.Info("ChaosLab Agent listening", "addr", ":9090")
	if err := http.ListenAndServe(":9090", nil); err != nil {
		slog.Error("agent failed", "err", err)
		os.Exit(1)
	}
}
