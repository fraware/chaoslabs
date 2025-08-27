package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// WriterService consumes events from the event bus and persists them to MongoDB
type WriterService struct {
	eventBus      *EventBus
	storage       *TimeSeriesManager
	processedIDs  map[string]bool
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	observability *ObservabilityManager
}

// NewWriterService creates a new writer service
func NewWriterService(eventBus *EventBus, storage *TimeSeriesManager, observability *ObservabilityManager) *WriterService {
	ctx, cancel := context.WithCancel(context.Background())

	return &WriterService{
		eventBus:      eventBus,
		storage:       storage,
		processedIDs:  make(map[string]bool),
		ctx:           ctx,
		cancel:        cancel,
		observability: observability,
	}
}

// Start starts the writer service
func (ws *WriterService) Start() error {
	// Subscribe to experiment events
	if err := ws.eventBus.Subscribe(ws.ctx, ConsumerConfig{
		Name:          "experiment-writer",
		Stream:        "EXPERIMENTS",
		FilterSubject: "experiments.*",
		DeliverPolicy: jetstream.DeliverNewPolicy,
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       30 * time.Second,
		MaxDeliver:    3,
		MaxAckPending: 100,
		BackOff:       []time.Duration{1 * time.Second, 5 * time.Second, 15 * time.Second},
	}, ws.handleExperimentEvent); err != nil {
		return fmt.Errorf("failed to subscribe to experiment events: %w", err)
	}

	// Subscribe to log events
	if err := ws.eventBus.Subscribe(ws.ctx, ConsumerConfig{
		Name:          "log-writer",
		Stream:        "LOGS",
		FilterSubject: "logs.*",
		DeliverPolicy: jetstream.DeliverNewPolicy,
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       30 * time.Second,
		MaxDeliver:    3,
		MaxAckPending: 1000,
		BackOff:       []time.Duration{1 * time.Second, 5 * time.Second, 15 * time.Second},
	}, ws.handleLogEvent); err != nil {
		return fmt.Errorf("failed to subscribe to log events: %w", err)
	}

	// Subscribe to metric events
	if err := ws.eventBus.Subscribe(ws.ctx, ConsumerConfig{
		Name:          "metric-writer",
		Stream:        "METRICS",
		FilterSubject: "metrics.*",
		DeliverPolicy: jetstream.DeliverNewPolicy,
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       30 * time.Second,
		MaxDeliver:    3,
		MaxAckPending: 1000,
		BackOff:       []time.Duration{1 * time.Second, 5 * time.Second, 15 * time.Second},
	}, ws.handleMetricEvent); err != nil {
		return fmt.Errorf("failed to subscribe to metric events: %w", err)
	}

	log.Printf("[WriterService] Started successfully")
	return nil
}

// Stop stops the writer service
func (ws *WriterService) Stop() {
	ws.cancel()
	ws.wg.Wait()
	log.Printf("[WriterService] Stopped")
}

// handleExperimentEvent processes experiment events
func (ws *WriterService) handleExperimentEvent(ctx context.Context, event *Event) error {
	start := time.Now()
	defer func() {
		ws.observability.RecordEventProcessed("experiment", time.Since(start))
	}()

	// Check idempotency
	if ws.isProcessed(event.ID) {
		log.Printf("[WriterService] Skipping duplicate experiment event: %s", event.ID)
		return nil
	}

	// Parse experiment data
	var experimentData map[string]interface{}
	if err := json.Unmarshal([]byte(fmt.Sprintf("%v", event.Data)), &experimentData); err != nil {
		return fmt.Errorf("failed to unmarshal experiment data: %w", err)
	}

	// Create log event for the experiment
	logEvent := &LogEvent{
		ID:           primitive.NewObjectID(),
		Timestamp:    event.Timestamp,
		AgentID:      event.Source,
		Level:        "info",
		Message:      fmt.Sprintf("Experiment %s executed", event.Type),
		ExperimentID: event.ID,
		Component:    "experiment_runner",
		Metadata: map[string]interface{}{
			"event_type": event.Type,
			"data":       experimentData,
		},
		ReviewStatus: "pending",
		TTL:          timePtr(time.Now().Add(30 * 24 * time.Hour)), // 30 days
	}

	// Persist to MongoDB
	if err := ws.storage.InsertLogEvent(logEvent); err != nil {
		ws.observability.RecordEventProcessingFailed("experiment", err.Error())
		return fmt.Errorf("failed to insert experiment log: %w", err)
	}

	// Mark as processed
	ws.markProcessed(event.ID)
	log.Printf("[WriterService] Processed experiment event: %s", event.ID)
	return nil
}

// handleLogEvent processes log events
func (ws *WriterService) handleLogEvent(ctx context.Context, event *Event) error {
	start := time.Now()
	defer func() {
		ws.observability.RecordEventProcessed("log", time.Since(start))
	}()

	// Check idempotency
	if ws.isProcessed(event.ID) {
		log.Printf("[WriterService] Skipping duplicate log event: %s", event.ID)
		return nil
	}

	// Parse log data
	var logData map[string]interface{}
	if err := json.Unmarshal([]byte(fmt.Sprintf("%v", event.Data)), &logData); err != nil {
		return fmt.Errorf("failed to unmarshal log data: %w", err)
	}

	// Create log event
	logEvent := &LogEvent{
		ID:        primitive.NewObjectID(),
		Timestamp: event.Timestamp,
		AgentID:   event.Source,
		Level:     getStringValue(logData, "level", "info"),
		Message:   getStringValue(logData, "message", "Log event"),
		Component: getStringValue(logData, "component", "unknown"),
		Metadata:  logData,
		TTL:       timePtr(time.Now().Add(7 * 24 * time.Hour)), // 7 days
	}

	// Add experiment ID if present
	if expID, ok := logData["experiment_id"].(string); ok {
		logEvent.ExperimentID = expID
	}

	// Persist to MongoDB
	if err := ws.storage.InsertLogEvent(logEvent); err != nil {
		ws.observability.RecordEventProcessingFailed("log", err.Error())
		return fmt.Errorf("failed to insert log event: %w", err)
	}

	// Mark as processed
	ws.markProcessed(event.ID)
	log.Printf("[WriterService] Processed log event: %s", event.ID)
	return nil
}

// handleMetricEvent processes metric events
func (ws *WriterService) handleMetricEvent(ctx context.Context, event *Event) error {
	start := time.Now()
	defer func() {
		ws.observability.RecordEventProcessed("metric", time.Since(start))
	}()

	// Check idempotency
	if ws.isProcessed(event.ID) {
		log.Printf("[WriterService] Skipping duplicate metric event: %s", event.ID)
		return nil
	}

	// Parse metric data
	var metricData map[string]interface{}
	if err := json.Unmarshal([]byte(fmt.Sprintf("%v", event.Data)), &metricData); err != nil {
		return fmt.Errorf("failed to unmarshal metric data: %w", err)
	}

	// Create log event for metrics
	logEvent := &LogEvent{
		ID:        primitive.NewObjectID(),
		Timestamp: event.Timestamp,
		AgentID:   event.Source,
		Level:     "info",
		Message:   fmt.Sprintf("Metric %s recorded", event.Type),
		Component: "metrics_collector",
		Metadata: map[string]interface{}{
			"event_type": event.Type,
			"metrics":    metricData,
		},
		TTL: timePtr(time.Now().Add(24 * time.Hour)), // 1 day
	}

	// Persist to MongoDB
	if err := ws.storage.InsertLogEvent(logEvent); err != nil {
		ws.observability.RecordEventProcessingFailed("metric", err.Error())
		return fmt.Errorf("failed to insert metric event: %w", err)
	}

	// Mark as processed
	ws.markProcessed(event.ID)
	log.Printf("[WriterService] Processed metric event: %s", event.ID)
	return nil
}

// isProcessed checks if an event ID has already been processed
func (ws *WriterService) isProcessed(eventID string) bool {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	return ws.processedIDs[eventID]
}

// markProcessed marks an event ID as processed
func (ws *WriterService) markProcessed(eventID string) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	ws.processedIDs[eventID] = true
}

// CleanupProcessedIDs removes old processed IDs to prevent memory leaks
func (ws *WriterService) CleanupProcessedIDs() {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	// Keep only the last 10000 processed IDs
	if len(ws.processedIDs) > 10000 {
		// Simple cleanup - just clear the map
		// In production, you might want to implement a more sophisticated cleanup
		ws.processedIDs = make(map[string]bool)
		log.Printf("[WriterService] Cleaned up processed IDs")
	}
}

// GetStats returns writer service statistics
func (ws *WriterService) GetStats() map[string]interface{} {
	ws.mu.RLock()
	defer ws.mu.RUnlock()

	return map[string]interface{}{
		"processed_events": len(ws.processedIDs),
		"status":           "running",
	}
}
