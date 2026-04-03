package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// ObservabilityManager manages OpenTelemetry instrumentation
type ObservabilityManager struct {
	tracer        trace.Tracer
	meter         metric.Meter
	exporter      *prometheus.Exporter
	traceProvider *sdktrace.TracerProvider
	Metrics       *Metrics
}

// Metrics holds all the metrics for the controller
type Metrics struct {
	// Experiment metrics
	ExperimentsStarted   metric.Int64Counter
	ExperimentsCompleted metric.Int64Counter
	ExperimentsFailed    metric.Int64Counter
	ExperimentsActive    metric.Int64UpDownCounter
	ExperimentDuration   metric.Float64Histogram

	// HTTP metrics
	HTTPRequestsTotal    metric.Int64Counter
	HTTPRequestDuration  metric.Float64Histogram
	HTTPRequestsInFlight metric.Int64UpDownCounter

	// Agent communication metrics
	AgentRequestsTotal   metric.Int64Counter
	AgentRequestDuration metric.Float64Histogram
	AgentRequestFailures metric.Int64Counter

	// System metrics
	ActiveConnections     metric.Int64UpDownCounter
	WorkerPoolUtilization metric.Float64Gauge
	MemoryUsage           metric.Float64Gauge
	CPUUsage              metric.Float64Gauge

	// Event bus metrics
	EventPublishDuration    metric.Float64Histogram
	EventProcessingDuration metric.Float64Histogram
	EventProcessingFailures metric.Int64Counter
}

// NewObservabilityManager creates a new observability manager
func NewObservabilityManager() (*ObservabilityManager, error) {
	ctx := context.Background()
	tp, err := newOTLPTracerProvider(ctx, "chaoslabs-controller", "1.0.0")
	if err != nil {
		return nil, fmt.Errorf("otlp tracer: %w", err)
	}

	otel.SetTracerProvider(tp)

	// Create meter
	meter := otel.GetMeterProvider().Meter("chaoslabs-controller")

	// Create Prometheus exporter
	exporter, err := prometheus.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create Prometheus exporter: %w", err)
	}

	// Note: In newer OpenTelemetry versions, the metrics exporter is handled differently

	om := &ObservabilityManager{
		tracer:        tp.Tracer("chaoslabs-controller"),
		meter:         meter,
		exporter:      exporter,
		traceProvider: tp,
		Metrics:       &Metrics{},
	}

	// Initialize metrics
	if err := om.initMetrics(); err != nil {
		return nil, fmt.Errorf("failed to initialize metrics: %w", err)
	}

	return om, nil
}

// initMetrics initializes all metrics
func (om *ObservabilityManager) initMetrics() error {
	var err error

	// Experiment metrics
	om.Metrics.ExperimentsStarted, err = om.meter.Int64Counter(
		"experiments_started_total",
		metric.WithDescription("Total number of experiments started"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create experiments_started_total metric: %w", err)
	}

	om.Metrics.ExperimentsCompleted, err = om.meter.Int64Counter(
		"experiments_completed_total",
		metric.WithDescription("Total number of experiments completed successfully"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create experiments_completed_total metric: %w", err)
	}

	om.Metrics.ExperimentsFailed, err = om.meter.Int64Counter(
		"experiments_failed_total",
		metric.WithDescription("Total number of experiments that failed"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create experiments_failed_total metric: %w", err)
	}

	om.Metrics.ExperimentsActive, err = om.meter.Int64UpDownCounter(
		"experiments_active",
		metric.WithDescription("Current number of active experiments"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create experiments_active metric: %w", err)
	}

	om.Metrics.ExperimentDuration, err = om.meter.Float64Histogram(
		"experiment_duration_seconds",
		metric.WithDescription("Duration of experiments in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.1, 0.5, 1.0, 5.0, 10.0, 30.0, 60.0, 300.0),
	)
	if err != nil {
		return fmt.Errorf("failed to create experiment_duration_seconds metric: %w", err)
	}

	// HTTP metrics
	om.Metrics.HTTPRequestsTotal, err = om.meter.Int64Counter(
		"http_requests_total",
		metric.WithDescription("Total number of HTTP requests"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create http_requests_total metric: %w", err)
	}

	om.Metrics.HTTPRequestDuration, err = om.meter.Float64Histogram(
		"http_request_duration_seconds",
		metric.WithDescription("HTTP request duration in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.01, 0.05, 0.1, 0.5, 1.0, 5.0, 10.0),
	)
	if err != nil {
		return fmt.Errorf("failed to create http_request_duration_seconds metric: %w", err)
	}

	om.Metrics.HTTPRequestsInFlight, err = om.meter.Int64UpDownCounter(
		"http_requests_in_flight",
		metric.WithDescription("Current number of HTTP requests being processed"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create http_requests_in_flight metric: %w", err)
	}

	// Agent communication metrics
	om.Metrics.AgentRequestsTotal, err = om.meter.Int64Counter(
		"agent_requests_total",
		metric.WithDescription("Total number of requests to agents"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create agent_requests_total metric: %w", err)
	}

	om.Metrics.AgentRequestDuration, err = om.meter.Float64Histogram(
		"agent_request_duration_seconds",
		metric.WithDescription("Duration of agent requests in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.01, 0.05, 0.1, 0.5, 1.0, 5.0),
	)
	if err != nil {
		return fmt.Errorf("failed to create agent_request_duration_seconds metric: %w", err)
	}

	om.Metrics.AgentRequestFailures, err = om.meter.Int64Counter(
		"agent_request_failures_total",
		metric.WithDescription("Total number of failed agent requests"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create agent_request_failures_total metric: %w", err)
	}

	// System metrics
	om.Metrics.ActiveConnections, err = om.meter.Int64UpDownCounter(
		"active_connections",
		metric.WithDescription("Current number of active connections"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create active_connections metric: %w", err)
	}

	om.Metrics.WorkerPoolUtilization, err = om.meter.Float64Gauge(
		"worker_pool_utilization",
		metric.WithDescription("Current worker pool utilization percentage"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create worker_pool_utilization metric: %w", err)
	}

	om.Metrics.MemoryUsage, err = om.meter.Float64Gauge(
		"memory_usage_bytes",
		metric.WithDescription("Current memory usage in bytes"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return fmt.Errorf("failed to create memory_usage_bytes metric: %w", err)
	}

	om.Metrics.CPUUsage, err = om.meter.Float64Gauge(
		"cpu_usage_percentage",
		metric.WithDescription("Current CPU usage percentage"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create cpu_usage_percentage metric: %w", err)
	}

	// Event bus metrics
	om.Metrics.EventPublishDuration, err = om.meter.Float64Histogram(
		"event_publish_duration_seconds",
		metric.WithDescription("Duration of event publishing in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.001, 0.005, 0.01, 0.05, 0.1, 0.5),
	)
	if err != nil {
		return fmt.Errorf("failed to create event_publish_duration_seconds metric: %w", err)
	}

	om.Metrics.EventProcessingDuration, err = om.meter.Float64Histogram(
		"event_processing_duration_seconds",
		metric.WithDescription("Duration of event processing in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.001, 0.005, 0.01, 0.05, 0.1, 0.5),
	)
	if err != nil {
		return fmt.Errorf("failed to create event_processing_duration_seconds metric: %w", err)
	}

	om.Metrics.EventProcessingFailures, err = om.meter.Int64Counter(
		"event_processing_failures_total",
		metric.WithDescription("Total number of event processing failures"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create event_processing_failures_total metric: %w", err)
	}

	return nil
}

// StartExperimentSpan creates a span for starting an experiment
func (om *ObservabilityManager) StartExperimentSpan(ctx context.Context, experimentID, experimentType, target string) (context.Context, trace.Span) {
	attrs := []attribute.KeyValue{
		attribute.String("experiment.id", experimentID),
		attribute.String("experiment.type", experimentType),
		attribute.String("experiment.target", target),
		attribute.String("operation", "start_experiment"),
	}

	return om.tracer.Start(ctx, "start_experiment", trace.WithAttributes(attrs...))
}

// StopExperimentSpan creates a span for stopping an experiment
func (om *ObservabilityManager) StopExperimentSpan(ctx context.Context, experimentID string) (context.Context, trace.Span) {
	attrs := []attribute.KeyValue{
		attribute.String("experiment.id", experimentID),
		attribute.String("operation", "stop_experiment"),
	}

	return om.tracer.Start(ctx, "stop_experiment", trace.WithAttributes(attrs...))
}

// AgentRequestSpan creates a span for agent communication
func (om *ObservabilityManager) AgentRequestSpan(ctx context.Context, agentID, operation string) (context.Context, trace.Span) {
	attrs := []attribute.KeyValue{
		attribute.String("agent.id", agentID),
		attribute.String("operation", operation),
		attribute.String("component", "agent_communication"),
	}

	return om.tracer.Start(ctx, "agent_request", trace.WithAttributes(attrs...))
}

// RecordExperimentStarted records metrics for a started experiment
func (om *ObservabilityManager) RecordExperimentStarted(experimentType, target string) {
	attrs := []attribute.KeyValue{
		attribute.String("experiment_type", experimentType),
		attribute.String("target", target),
	}

	om.Metrics.ExperimentsStarted.Add(context.Background(), 1, metric.WithAttributes(attrs...))
	om.Metrics.ExperimentsActive.Add(context.Background(), 1, metric.WithAttributes(attrs...))
}

// RecordExperimentCompleted records metrics for a completed experiment
func (om *ObservabilityManager) RecordExperimentCompleted(experimentType, target string, duration time.Duration) {
	attrs := []attribute.KeyValue{
		attribute.String("experiment_type", experimentType),
		attribute.String("target", target),
	}

	om.Metrics.ExperimentsCompleted.Add(context.Background(), 1, metric.WithAttributes(attrs...))
	om.Metrics.ExperimentsActive.Add(context.Background(), -1, metric.WithAttributes(attrs...))
	om.Metrics.ExperimentDuration.Record(context.Background(), duration.Seconds(), metric.WithAttributes(attrs...))
}

// RecordExperimentFailed records metrics for a failed experiment
func (om *ObservabilityManager) RecordExperimentFailed(experimentType, target string, errorType string) {
	attrs := []attribute.KeyValue{
		attribute.String("experiment_type", experimentType),
		attribute.String("target", target),
		attribute.String("error_type", errorType),
	}

	om.Metrics.ExperimentsFailed.Add(context.Background(), 1, metric.WithAttributes(attrs...))
	om.Metrics.ExperimentsActive.Add(context.Background(), -1, metric.WithAttributes(attrs...))
}

// RecordHTTPRequest records metrics for an HTTP request
func (om *ObservabilityManager) RecordHTTPRequest(method, path string, statusCode int, duration time.Duration) {
	attrs := []attribute.KeyValue{
		attribute.String("method", method),
		attribute.String("path", path),
		attribute.Int("status_code", statusCode),
	}

	om.Metrics.HTTPRequestsTotal.Add(context.Background(), 1, metric.WithAttributes(attrs...))
	om.Metrics.HTTPRequestDuration.Record(context.Background(), duration.Seconds(), metric.WithAttributes(attrs...))
}

// RecordAgentRequest records metrics for an agent request
func (om *ObservabilityManager) RecordAgentRequest(agentID, operation string, duration time.Duration, success bool) {
	attrs := []attribute.KeyValue{
		attribute.String("agent_id", agentID),
		attribute.String("operation", operation),
		attribute.Bool("success", success),
	}

	om.Metrics.AgentRequestsTotal.Add(context.Background(), 1, metric.WithAttributes(attrs...))
	om.Metrics.AgentRequestDuration.Record(context.Background(), duration.Seconds(), metric.WithAttributes(attrs...))

	if !success {
		om.Metrics.AgentRequestFailures.Add(context.Background(), 1, metric.WithAttributes(attrs...))
	}
}

// UpdateSystemMetrics updates system-level metrics
func (om *ObservabilityManager) UpdateSystemMetrics(activeConnections int64, workerPoolUtilization, memoryUsage, cpuUsage float64) {
	om.Metrics.ActiveConnections.Add(context.Background(), activeConnections)
	om.Metrics.WorkerPoolUtilization.Record(context.Background(), workerPoolUtilization)
	om.Metrics.MemoryUsage.Record(context.Background(), memoryUsage)
	om.Metrics.CPUUsage.Record(context.Background(), cpuUsage)
}

// GetMetricsData returns the current metrics data for Prometheus export
func (om *ObservabilityManager) GetMetricsData() error {
	// In newer OpenTelemetry versions, metrics collection is handled automatically
	return nil
}

// Shutdown gracefully shuts down the observability manager
func (om *ObservabilityManager) Shutdown(ctx context.Context) error {
	return om.traceProvider.Shutdown(ctx)
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// EventProcessingSpan creates a span for event processing
func (om *ObservabilityManager) EventProcessingSpan(ctx context.Context, eventType, eventID string) (context.Context, trace.Span) {
	attrs := []attribute.KeyValue{
		attribute.String("event.type", eventType),
		attribute.String("event.id", eventID),
		attribute.String("component", "event_processing"),
	}

	return om.tracer.Start(ctx, "process_event", trace.WithAttributes(attrs...))
}

// RecordEventPublished records metrics for event publishing
func (om *ObservabilityManager) RecordEventPublished(subject string, duration time.Duration) {
	attrs := []attribute.KeyValue{
		attribute.String("subject", subject),
	}

	om.Metrics.EventPublishDuration.Record(context.Background(), duration.Seconds(), metric.WithAttributes(attrs...))
}

// RecordEventProcessed records metrics for event processing
func (om *ObservabilityManager) RecordEventProcessed(eventType string, duration time.Duration) {
	attrs := []attribute.KeyValue{
		attribute.String("event_type", eventType),
	}

	om.Metrics.EventProcessingDuration.Record(context.Background(), duration.Seconds(), metric.WithAttributes(attrs...))
}

// RecordEventProcessingFailed records metrics for failed event processing
func (om *ObservabilityManager) RecordEventProcessingFailed(eventType, errorType string) {
	attrs := []attribute.KeyValue{
		attribute.String("event_type", eventType),
		attribute.String("error_type", errorType),
	}

	om.Metrics.EventProcessingFailures.Add(context.Background(), 1, metric.WithAttributes(attrs...))
}
