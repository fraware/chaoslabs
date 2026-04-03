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

// ObservabilityManager manages OpenTelemetry instrumentation for the agent
type ObservabilityManager struct {
	tracer        trace.Tracer
	meter         metric.Meter
	exporter      *prometheus.Exporter
	traceProvider *sdktrace.TracerProvider
	Metrics       *Metrics
}

// Metrics holds all the metrics for the agent
type Metrics struct {
	// Experiment execution metrics
	ExperimentsReceived metric.Int64Counter
	ExperimentsExecuted metric.Int64Counter
	ExperimentsFailed   metric.Int64Counter
	ExperimentsActive   metric.Int64UpDownCounter
	ExperimentDuration  metric.Float64Histogram

	// Network fault metrics
	NetworkFaultsApplied  metric.Int64Counter
	NetworkFaultsReverted metric.Int64Counter
	NetworkFaultDuration  metric.Float64Histogram
	NetworkFaultFailures  metric.Int64Counter

	// System stress metrics
	SystemStressApplied  metric.Int64Counter
	SystemStressReverted metric.Int64Counter
	SystemStressDuration metric.Float64Histogram
	SystemStressFailures metric.Int64Counter

	// Resource usage metrics
	CPUUsage              metric.Float64Gauge
	MemoryUsage           metric.Float64Gauge
	NetworkInterfaceCount metric.Int64Gauge
	ActiveQdiscs          metric.Int64Gauge

	// Agent communication metrics
	ControllerRequests metric.Int64Counter
	ControllerLatency  metric.Float64Histogram
	ControllerFailures metric.Int64Counter
}

// NewObservabilityManager creates a new observability manager for the agent
func NewObservabilityManager() (*ObservabilityManager, error) {
	ctx := context.Background()
	tp, err := newOTLPTracerProvider(ctx, "chaoslabs-agent", "1.0.0")
	if err != nil {
		return nil, fmt.Errorf("otlp tracer: %w", err)
	}

	otel.SetTracerProvider(tp)

	// Create meter
	meter := otel.GetMeterProvider().Meter("chaoslabs-agent")

	// Create Prometheus exporter
	exporter, err := prometheus.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create Prometheus exporter: %w", err)
	}

	// Note: In newer OpenTelemetry versions, the metrics exporter is handled differently

	om := &ObservabilityManager{
		tracer:        tp.Tracer("chaoslabs-agent"),
		meter:         meter,
		exporter:      exporter,
		traceProvider: tp,
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

	// Initialize the Metrics struct
	om.Metrics = &Metrics{}

	// Experiment execution metrics
	om.Metrics.ExperimentsReceived, err = om.meter.Int64Counter(
		"experiments_received_total",
		metric.WithDescription("Total number of experiments received from controller"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create experiments_received_total metric: %w", err)
	}

	om.Metrics.ExperimentsExecuted, err = om.meter.Int64Counter(
		"experiments_executed_total",
		metric.WithDescription("Total number of experiments executed successfully"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create experiments_executed_total metric: %w", err)
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

	// Network fault metrics
	om.Metrics.NetworkFaultsApplied, err = om.meter.Int64Counter(
		"network_faults_applied_total",
		metric.WithDescription("Total number of network faults applied"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create network_faults_applied_total metric: %w", err)
	}

	om.Metrics.NetworkFaultsReverted, err = om.meter.Int64Counter(
		"network_faults_reverted_total",
		metric.WithDescription("Total number of network faults reverted"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create network_faults_reverted_total metric: %w", err)
	}

	om.Metrics.NetworkFaultDuration, err = om.meter.Float64Histogram(
		"network_fault_duration_seconds",
		metric.WithDescription("Duration of network faults in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.1, 0.5, 1.0, 5.0, 10.0, 30.0, 60.0),
	)
	if err != nil {
		return fmt.Errorf("failed to create network_fault_duration_seconds metric: %w", err)
	}

	om.Metrics.NetworkFaultFailures, err = om.meter.Int64Counter(
		"network_fault_failures_total",
		metric.WithDescription("Total number of network fault failures"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create network_fault_failures_total metric: %w", err)
	}

	// System stress metrics
	om.Metrics.SystemStressApplied, err = om.meter.Int64Counter(
		"system_stress_applied_total",
		metric.WithDescription("Total number of system stress tests applied"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create system_stress_applied_total metric: %w", err)
	}

	om.Metrics.SystemStressReverted, err = om.meter.Int64Counter(
		"system_stress_reverted_total",
		metric.WithDescription("Total number of system stress tests reverted"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create system_stress_reverted_total metric: %w", err)
	}

	om.Metrics.SystemStressDuration, err = om.meter.Float64Histogram(
		"system_stress_duration_seconds",
		metric.WithDescription("Duration of system stress tests in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.1, 0.5, 1.0, 5.0, 10.0, 30.0, 60.0),
	)
	if err != nil {
		return fmt.Errorf("failed to create system_stress_duration_seconds metric: %w", err)
	}

	om.Metrics.SystemStressFailures, err = om.meter.Int64Counter(
		"system_stress_failures_total",
		metric.WithDescription("Total number of system stress test failures"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create system_stress_failures_total metric: %w", err)
	}

	// Resource usage metrics
	om.Metrics.CPUUsage, err = om.meter.Float64Gauge(
		"cpu_usage_percentage",
		metric.WithDescription("Current CPU usage percentage"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create cpu_usage_percentage metric: %w", err)
	}

	om.Metrics.MemoryUsage, err = om.meter.Float64Gauge(
		"memory_usage_bytes",
		metric.WithDescription("Current memory usage in bytes"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return fmt.Errorf("failed to create memory_usage_bytes metric: %w", err)
	}

	om.Metrics.NetworkInterfaceCount, err = om.meter.Int64Gauge(
		"network_interface_count",
		metric.WithDescription("Number of network interfaces managed"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create network_interface_count metric: %w", err)
	}

	om.Metrics.ActiveQdiscs, err = om.meter.Int64Gauge(
		"active_qdiscs",
		metric.WithDescription("Number of active qdiscs"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create active_qdiscs metric: %w", err)
	}

	// Agent communication metrics
	om.Metrics.ControllerRequests, err = om.meter.Int64Counter(
		"controller_requests_total",
		metric.WithDescription("Total number of requests from controller"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create controller_requests_total metric: %w", err)
	}

	om.Metrics.ControllerLatency, err = om.meter.Float64Histogram(
		"controller_latency_seconds",
		metric.WithDescription("Latency of controller requests in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.01, 0.05, 0.1, 0.5, 1.0, 5.0),
	)
	if err != nil {
		return fmt.Errorf("failed to create controller_latency_seconds metric: %w", err)
	}

	om.Metrics.ControllerFailures, err = om.meter.Int64Counter(
		"controller_failures_total",
		metric.WithDescription("Total number of controller communication failures"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create controller_failures_total metric: %w", err)
	}

	return nil
}

// ExperimentSpan creates a span for experiment execution
func (om *ObservabilityManager) ExperimentSpan(ctx context.Context, experimentID, experimentType string) (context.Context, trace.Span) {
	attrs := []attribute.KeyValue{
		attribute.String("experiment.id", experimentID),
		attribute.String("experiment.type", experimentType),
		attribute.String("component", "experiment_execution"),
	}

	return om.tracer.Start(ctx, "execute_experiment", trace.WithAttributes(attrs...))
}

// NetworkFaultSpan creates a span for network fault operations
func (om *ObservabilityManager) NetworkFaultSpan(ctx context.Context, operation, interfaceName string) (context.Context, trace.Span) {
	attrs := []attribute.KeyValue{
		attribute.String("operation", operation),
		attribute.String("interface", interfaceName),
		attribute.String("component", "network_fault"),
	}

	return om.tracer.Start(ctx, "network_fault", trace.WithAttributes(attrs...))
}

// SystemStressSpan creates a span for system stress operations
func (om *ObservabilityManager) SystemStressSpan(ctx context.Context, operation, stressType string) (context.Context, trace.Span) {
	attrs := []attribute.KeyValue{
		attribute.String("operation", operation),
		attribute.String("stress_type", stressType),
		attribute.String("component", "system_stress"),
	}

	return om.tracer.Start(ctx, "system_stress", trace.WithAttributes(attrs...))
}

// RecordExperimentReceived records metrics for a received experiment
func (om *ObservabilityManager) RecordExperimentReceived(experimentType string) {
	attrs := []attribute.KeyValue{
		attribute.String("experiment_type", experimentType),
	}

	om.Metrics.ExperimentsReceived.Add(context.Background(), 1, metric.WithAttributes(attrs...))
	om.Metrics.ExperimentsActive.Add(context.Background(), 1, metric.WithAttributes(attrs...))
}

// RecordExperimentExecuted records metrics for an executed experiment
func (om *ObservabilityManager) RecordExperimentExecuted(experimentType string, duration time.Duration) {
	attrs := []attribute.KeyValue{
		attribute.String("experiment_type", experimentType),
	}

	om.Metrics.ExperimentsExecuted.Add(context.Background(), 1, metric.WithAttributes(attrs...))
	om.Metrics.ExperimentsActive.Add(context.Background(), -1, metric.WithAttributes(attrs...))
	om.Metrics.ExperimentDuration.Record(context.Background(), duration.Seconds(), metric.WithAttributes(attrs...))
}

// RecordExperimentFailed records metrics for a failed experiment
func (om *ObservabilityManager) RecordExperimentFailed(experimentType, errorType string) {
	attrs := []attribute.KeyValue{
		attribute.String("experiment_type", experimentType),
		attribute.String("error_type", errorType),
	}

	om.Metrics.ExperimentsFailed.Add(context.Background(), 1, metric.WithAttributes(attrs...))
	om.Metrics.ExperimentsActive.Add(context.Background(), -1, metric.WithAttributes(attrs...))
}

// RecordNetworkFaultApplied records metrics for an applied network fault
func (om *ObservabilityManager) RecordNetworkFaultApplied(faultType, interfaceName string) {
	attrs := []attribute.KeyValue{
		attribute.String("fault_type", faultType),
		attribute.String("interface", interfaceName),
	}

	om.Metrics.NetworkFaultsApplied.Add(context.Background(), 1, metric.WithAttributes(attrs...))
}

// RecordNetworkFaultReverted records metrics for a reverted network fault
func (om *ObservabilityManager) RecordNetworkFaultReverted(faultType, interfaceName string, duration time.Duration) {
	attrs := []attribute.KeyValue{
		attribute.String("fault_type", faultType),
		attribute.String("interface", interfaceName),
	}

	om.Metrics.NetworkFaultsReverted.Add(context.Background(), 1, metric.WithAttributes(attrs...))
	om.Metrics.NetworkFaultDuration.Record(context.Background(), duration.Seconds(), metric.WithAttributes(attrs...))
}

// RecordNetworkFaultFailure records metrics for a failed network fault
func (om *ObservabilityManager) RecordNetworkFaultFailure(faultType, interfaceName, errorType string) {
	attrs := []attribute.KeyValue{
		attribute.String("fault_type", faultType),
		attribute.String("interface", interfaceName),
		attribute.String("error_type", errorType),
	}

	om.Metrics.NetworkFaultFailures.Add(context.Background(), 1, metric.WithAttributes(attrs...))
}

// RecordSystemStressApplied records metrics for an applied system stress test
func (om *ObservabilityManager) RecordSystemStressApplied(stressType string) {
	attrs := []attribute.KeyValue{
		attribute.String("stress_type", stressType),
	}

	om.Metrics.SystemStressApplied.Add(context.Background(), 1, metric.WithAttributes(attrs...))
}

// RecordSystemStressReverted records metrics for a reverted system stress test
func (om *ObservabilityManager) RecordSystemStressReverted(stressType string, duration time.Duration) {
	attrs := []attribute.KeyValue{
		attribute.String("stress_type", stressType),
	}

	om.Metrics.SystemStressReverted.Add(context.Background(), 1, metric.WithAttributes(attrs...))
	om.Metrics.SystemStressDuration.Record(context.Background(), duration.Seconds(), metric.WithAttributes(attrs...))
}

// RecordSystemStressFailure records metrics for a failed system stress test
func (om *ObservabilityManager) RecordSystemStressFailure(stressType, errorType string) {
	attrs := []attribute.KeyValue{
		attribute.String("stress_type", stressType),
		attribute.String("error_type", errorType),
	}

	om.Metrics.SystemStressFailures.Add(context.Background(), 1, metric.WithAttributes(attrs...))
}

// RecordControllerRequest records metrics for controller communication
func (om *ObservabilityManager) RecordControllerRequest(operation string, duration time.Duration, success bool) {
	attrs := []attribute.KeyValue{
		attribute.String("operation", operation),
		attribute.Bool("success", success),
	}

	om.Metrics.ControllerRequests.Add(context.Background(), 1, metric.WithAttributes(attrs...))
	om.Metrics.ControllerLatency.Record(context.Background(), duration.Seconds(), metric.WithAttributes(attrs...))

	if !success {
		om.Metrics.ControllerFailures.Add(context.Background(), 1, metric.WithAttributes(attrs...))
	}
}

// UpdateResourceMetrics updates resource usage metrics
func (om *ObservabilityManager) UpdateResourceMetrics(cpuUsage, memoryUsage float64, networkInterfaceCount, activeQdiscs int64) {
	om.Metrics.CPUUsage.Record(context.Background(), cpuUsage)
	om.Metrics.MemoryUsage.Record(context.Background(), memoryUsage)
	om.Metrics.NetworkInterfaceCount.Record(context.Background(), networkInterfaceCount)
	om.Metrics.ActiveQdiscs.Record(context.Background(), activeQdiscs)
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
