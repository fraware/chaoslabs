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

// LogEvent represents a log entry in the time-series collection
type LogEvent struct {
	ID           primitive.ObjectID     `bson:"_id" json:"id"`
	Timestamp    time.Time              `bson:"timestamp" json:"timestamp"`
	AgentID      string                 `bson:"agent_id" json:"agent_id"`
	Level        string                 `bson:"level" json:"level"`
	Message      string                 `bson:"message" json:"message"`
	ExperimentID string                 `bson:"experiment_id,omitempty" json:"experiment_id,omitempty"`
	Component    string                 `bson:"component" json:"component"`
	Metadata     map[string]interface{} `bson:"metadata,omitempty" json:"metadata,omitempty"`
	ReviewStatus string                 `bson:"review_status,omitempty" json:"review_status,omitempty"`
	TTL          *time.Time             `bson:"ttl,omitempty" json:"ttl,omitempty"`
}

// timePtr returns a pointer to a time value
func timePtr(t time.Time) *time.Time {
	return &t
}

// ReplayService handles event replay for gap healing and audit purposes
type ReplayService struct {
	eventBus      *EventBus
	storage       *TimeSeriesManager
	replayJobs    map[string]*ReplayJob
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	observability *ObservabilityManager
}

// ReplayJob represents a replay operation
type ReplayJob struct {
	ID           string
	StreamName   string
	FromSequence uint64
	ToSequence   uint64
	Status       string // "pending", "running", "completed", "failed"
	Progress     float64
	StartTime    time.Time
	EndTime      time.Time
	Error        string
	Stats        *ReplayStats
}

// ReplayStats holds replay operation statistics
type ReplayStats struct {
	EventsProcessed int64
	EventsSkipped   int64
	EventsFailed    int64
	BytesProcessed  int64
	Duration        time.Duration
}

// ReplayConfig defines replay configuration
type ReplayConfig struct {
	StreamName   string
	FromSequence uint64
	ToSequence   uint64
	BatchSize    int
	Concurrency  int
	DryRun       bool
	Filters      map[string]interface{}
}

// NewReplayService creates a new replay service
func NewReplayService(eventBus *EventBus, storage *TimeSeriesManager, observability *ObservabilityManager) *ReplayService {
	ctx, cancel := context.WithCancel(context.Background())

	return &ReplayService{
		eventBus:      eventBus,
		storage:       storage,
		replayJobs:    make(map[string]*ReplayJob),
		ctx:           ctx,
		cancel:        cancel,
		observability: observability,
	}
}

// StartReplay starts a new replay operation
func (rs *ReplayService) StartReplay(config ReplayConfig) (string, error) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	jobID := fmt.Sprintf("replay_%s_%d_%d_%d", config.StreamName, config.FromSequence, config.ToSequence, time.Now().Unix())

	job := &ReplayJob{
		ID:           jobID,
		StreamName:   config.StreamName,
		FromSequence: config.FromSequence,
		ToSequence:   config.ToSequence,
		Status:       "pending",
		Progress:     0.0,
		StartTime:    time.Now(),
		Stats:        &ReplayStats{},
	}

	rs.replayJobs[jobID] = job

	// Start replay in background
	go rs.executeReplay(job, config)

	log.Printf("[ReplayService] Started replay job: %s", jobID)
	return jobID, nil
}

// executeReplay executes a replay operation
func (rs *ReplayService) executeReplay(job *ReplayJob, config ReplayConfig) {
	defer func() {
		job.EndTime = time.Now()
		job.Stats.Duration = job.EndTime.Sub(job.StartTime)
	}()

	job.Status = "running"
	start := time.Now()

	// Get stream info
	_, err := rs.eventBus.GetStreamInfo(config.StreamName)
	if err != nil {
		job.Status = "failed"
		job.Error = fmt.Sprintf("failed to get stream info: %v", err)
		log.Printf("[ReplayService] Replay job %s failed: %v", job.ID, err)
		return
	}

	// Calculate total events to process
	totalEvents := config.ToSequence - config.FromSequence + 1
	if totalEvents <= 0 {
		job.Status = "failed"
		job.Error = "invalid sequence range"
		return
	}

	// Create replay consumer
	consumer, err := rs.eventBus.js.CreateConsumer(rs.ctx, config.StreamName, jetstream.ConsumerConfig{
		Name:          fmt.Sprintf("replay_%s_%d", job.ID, time.Now().Unix()),
		DeliverPolicy: jetstream.DeliverByStartSequencePolicy,
		OptStartSeq:   config.FromSequence,
		AckPolicy:     jetstream.AckNonePolicy, // Don't track acks for replay
		MaxDeliver:    1,
	})
	if err != nil {
		job.Status = "failed"
		job.Error = fmt.Sprintf("failed to create replay consumer: %v", err)
		return
	}
	// Consumer cleanup handled automatically in newer NATS versions

	// Process messages in batches
	batchSize := config.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}

	processed := int64(0)
	skipped := int64(0)
	failed := int64(0)
	bytesProcessed := int64(0)

	// Start consuming messages
	msgs, err := consumer.Consume(func(msg jetstream.Msg) {
		// Check if we've reached the end sequence
		metadata, err := msg.Metadata()
		if err != nil {
			log.Printf("[ReplayService] Failed to get message metadata: %v", err)
			msg.Nak()
			return
		}
		if metadata.Sequence.Stream > config.ToSequence {
			msg.Ack()
			return
		}

		// Process message
		if err := rs.processReplayMessage(job, msg, config); err != nil {
			failed++
			log.Printf("[ReplayService] Failed to process replay message: %v", err)
		} else {
			processed++
			bytesProcessed += int64(len(msg.Data()))
		}

		// Update progress
		job.Progress = float64(processed+skipped) / float64(totalEvents) * 100.0
		job.Stats.EventsProcessed = processed
		job.Stats.EventsSkipped = skipped
		job.Stats.EventsFailed = failed
		job.Stats.BytesProcessed = bytesProcessed

		msg.Ack()
	})
	if err != nil {
		job.Status = "failed"
		job.Error = fmt.Sprintf("failed to consume messages: %v", err)
		return
	}
	defer msgs.Stop()

	// Wait for completion or context cancellation
	select {
	case <-rs.ctx.Done():
		job.Status = "cancelled"
		job.Error = "replay cancelled"
	case <-time.After(rs.calculateReplayTimeout(totalEvents, batchSize)):
		job.Status = "timeout"
		job.Error = "replay timeout"
	}

	// Final stats update
	job.Stats.EventsProcessed = processed
	job.Stats.EventsSkipped = skipped
	job.Stats.EventsFailed = failed
	job.Stats.BytesProcessed = bytesProcessed
	job.Stats.Duration = time.Since(start)

	if job.Status == "running" {
		job.Status = "completed"
	}

	log.Printf("[ReplayService] Replay job %s completed: processed=%d, skipped=%d, failed=%d",
		job.ID, processed, skipped, failed)
}

// processReplayMessage processes a single replay message
func (rs *ReplayService) processReplayMessage(job *ReplayJob, msg jetstream.Msg, config ReplayConfig) error {
	// Parse event
	var event Event
	if err := json.Unmarshal(msg.Data(), &event); err != nil {
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}

	// Apply filters if specified
	if !rs.applyFilters(&event, config.Filters) {
		return nil // Skip this event
	}

	// Check if this is a dry run
	if config.DryRun {
		return nil
	}

	// Process based on event type
	switch event.Type {
	case "experiment":
		return rs.replayExperimentEvent(&event)
	case "log":
		return rs.replayLogEvent(&event)
	case "metric":
		return rs.replayMetricEvent(&event)
	case "notification":
		return rs.replayNotificationEvent(&event)
	default:
		log.Printf("[ReplayService] Unknown event type: %s", event.Type)
		return nil
	}
}

// applyFilters applies filters to determine if an event should be processed
func (rs *ReplayService) applyFilters(event *Event, filters map[string]interface{}) bool {
	if filters == nil {
		return true
	}

	for key, value := range filters {
		switch key {
		case "event_type":
			if expectedType, ok := value.(string); ok && event.Type != expectedType {
				return false
			}
		case "source":
			if expectedSource, ok := value.(string); ok && event.Source != expectedSource {
				return false
			}
		case "after_timestamp":
			if afterTime, ok := value.(time.Time); ok && event.Timestamp.Before(afterTime) {
				return false
			}
		case "before_timestamp":
			if beforeTime, ok := value.(time.Time); ok && event.Timestamp.After(beforeTime) {
				return false
			}
		}
	}

	return true
}

// replayExperimentEvent replays an experiment event
func (rs *ReplayService) replayExperimentEvent(event *Event) error {
	// Create log event for the experiment
	logEvent := &LogEvent{
		ID:           primitive.NewObjectID(),
		Timestamp:    event.Timestamp,
		AgentID:      event.Source,
		Level:        "info",
		Message:      fmt.Sprintf("Replayed experiment %s", event.Type),
		ExperimentID: event.ID,
		Component:    "experiment_replay",
		Metadata: map[string]interface{}{
			"event_type":  event.Type,
			"data":        event.Data,
			"replayed":    true,
			"replay_time": time.Now(),
		},
		ReviewStatus: "pending",
		TTL:          timePtr(time.Now().Add(30 * 24 * time.Hour)), // 30 days
	}

	return rs.storage.InsertLogEvent(logEvent)
}

// replayLogEvent replays a log event
func (rs *ReplayService) replayLogEvent(event *Event) error {
	// Create log event
	logEvent := &LogEvent{
		ID:        primitive.NewObjectID(),
		Timestamp: event.Timestamp,
		AgentID:   event.Source,
		Level:     getStringValue(event.Data, "level", "info"),
		Message:   getStringValue(event.Data, "message", "Replayed log event"),
		Component: getStringValue(event.Data, "component", "unknown"),
		Metadata:  event.Data,
		TTL:       timePtr(time.Now().Add(7 * 24 * time.Hour)), // 7 days
	}

	// Add replay metadata
	if logEvent.Metadata == nil {
		logEvent.Metadata = make(map[string]interface{})
	}
	logEvent.Metadata["replayed"] = true
	logEvent.Metadata["replay_time"] = time.Now()

	return rs.storage.InsertLogEvent(logEvent)
}

// replayMetricEvent replays a metric event
func (rs *ReplayService) replayMetricEvent(event *Event) error {
	// Create log event for metrics
	logEvent := &LogEvent{
		ID:        primitive.NewObjectID(),
		Timestamp: event.Timestamp,
		AgentID:   event.Source,
		Level:     "info",
		Message:   fmt.Sprintf("Replayed metric %s", event.Type),
		Component: "metrics_replay",
		Metadata: map[string]interface{}{
			"event_type":  event.Type,
			"metrics":     event.Data,
			"replayed":    true,
			"replay_time": time.Now(),
		},
		TTL: timePtr(time.Now().Add(24 * time.Hour)), // 1 day
	}

	return rs.storage.InsertLogEvent(logEvent)
}

// replayNotificationEvent replays a notification event
func (rs *ReplayService) replayNotificationEvent(event *Event) error {
	// For notifications, we might want to re-publish to the event bus
	// or just log the replay
	logEvent := &LogEvent{
		ID:        primitive.NewObjectID(),
		Timestamp: event.Timestamp,
		AgentID:   event.Source,
		Level:     "info",
		Message:   fmt.Sprintf("Replayed notification %s", event.Type),
		Component: "notification_replay",
		Metadata: map[string]interface{}{
			"event_type":  event.Type,
			"data":        event.Data,
			"replayed":    true,
			"replay_time": time.Now(),
		},
		TTL: timePtr(time.Now().Add(1 * time.Hour)), // 1 hour
	}

	return rs.storage.InsertLogEvent(logEvent)
}

// GetReplayJob returns a replay job by ID
func (rs *ReplayService) GetReplayJob(jobID string) (*ReplayJob, bool) {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	job, exists := rs.replayJobs[jobID]
	return job, exists
}

// ListReplayJobs returns all replay jobs
func (rs *ReplayService) ListReplayJobs() []*ReplayJob {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	jobs := make([]*ReplayJob, 0, len(rs.replayJobs))
	for _, job := range rs.replayJobs {
		jobs = append(jobs, job)
	}

	return jobs
}

// CancelReplay cancels a replay job
func (rs *ReplayService) CancelReplay(jobID string) error {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	job, exists := rs.replayJobs[jobID]
	if !exists {
		return fmt.Errorf("replay job %s not found", jobID)
	}

	if job.Status == "completed" || job.Status == "failed" {
		return fmt.Errorf("cannot cancel completed/failed job")
	}

	job.Status = "cancelled"
	job.Error = "cancelled by user"
	job.EndTime = time.Now()

	log.Printf("[ReplayService] Cancelled replay job: %s", jobID)
	return nil
}

// CleanupReplayJobs removes old completed/failed replay jobs
func (rs *ReplayService) CleanupReplayJobs(maxAge time.Duration) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	now := time.Now()
	var jobsToRemove []string

	for jobID, job := range rs.replayJobs {
		if job.Status == "completed" || job.Status == "failed" || job.Status == "cancelled" {
			if now.Sub(job.EndTime) > maxAge {
				jobsToRemove = append(jobsToRemove, jobID)
			}
		}
	}

	for _, jobID := range jobsToRemove {
		delete(rs.replayJobs, jobID)
	}

	if len(jobsToRemove) > 0 {
		log.Printf("[ReplayService] Cleaned up %d old replay jobs", len(jobsToRemove))
	}
}

// calculateReplayTimeout calculates a reasonable timeout for a replay operation
func (rs *ReplayService) calculateReplayTimeout(totalEvents uint64, batchSize int) time.Duration {
	// Estimate 1ms per event + 1 second buffer
	estimatedDuration := time.Duration(totalEvents) * time.Millisecond
	timeout := estimatedDuration + time.Second

	// Cap at 1 hour
	if timeout > time.Hour {
		timeout = time.Hour
	}

	return timeout
}

// GetStats returns replay service statistics
func (rs *ReplayService) GetStats() map[string]interface{} {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	statusCounts := make(map[string]int)
	totalJobs := len(rs.replayJobs)

	for _, job := range rs.replayJobs {
		statusCounts[job.Status]++
	}

	return map[string]interface{}{
		"total_jobs":    totalJobs,
		"status_counts": statusCounts,
		"status":        "running",
	}
}

// Stop stops the replay service
func (rs *ReplayService) Stop() {
	rs.cancel()
	rs.wg.Wait()
	log.Printf("[ReplayService] Stopped")
}

// Helper function for string extraction
func getStringValue(data map[string]interface{}, key, defaultValue string) string {
	if value, ok := data[key].(string); ok {
		return value
	}
	return defaultValue
}
