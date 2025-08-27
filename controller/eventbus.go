package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel/trace"
)

// EventBus manages the NATS JetStream event streaming infrastructure
type EventBus struct {
	nc            *nats.Conn
	js            jetstream.JetStream
	streams       map[string]jetstream.Stream
	consumers     map[string]jetstream.Consumer
	mu            sync.RWMutex
	observability *ObservabilityManager
}

// Event represents a system event with metadata
type Event struct {
	ID            string                 `json:"id"`
	Type          string                 `json:"type"`
	Source        string                 `json:"source"`
	Timestamp     time.Time              `json:"timestamp"`
	Data          map[string]interface{} `json:"data"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	CorrelationID string                 `json:"correlation_id,omitempty"`
	TraceID       string                 `json:"trace_id,omitempty"`
	SpanID        string                 `json:"span_id,omitempty"`
}

// EventHandler processes events from streams
type EventHandler func(ctx context.Context, event *Event) error

// StreamConfig defines stream configuration
type StreamConfig struct {
	Name          string
	Subjects      []string
	Retention     jetstream.RetentionPolicy
	MaxAge        time.Duration
	MaxBytes      int64
	MaxMsgs       int64
	MaxMsgSize    int32
	Storage       jetstream.StorageType
	Replicas      int
	DiscardPolicy jetstream.DiscardPolicy
}

// ConsumerConfig defines consumer configuration
type ConsumerConfig struct {
	Name          string
	Stream        string
	FilterSubject string
	DeliverPolicy jetstream.DeliverPolicy
	AckPolicy     jetstream.AckPolicy
	AckWait       time.Duration
	MaxDeliver    int
	MaxAckPending int
	BackOff       []time.Duration
	IdleHeartbeat time.Duration
	FlowControl   bool
	HeadersOnly   bool
}

// NewEventBus creates a new event bus instance
func NewEventBus(natsURL string, observability *ObservabilityManager) (*EventBus, error) {
	nc, err := nats.Connect(natsURL,
		nats.Name("chaoslabs-eventbus"),
		nats.ReconnectWait(time.Second),
		nats.MaxReconnects(-1),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			log.Printf("[EventBus] Disconnected: %v", err)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Printf("[EventBus] Reconnected to %s", nc.ConnectedUrl())
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	eb := &EventBus{
		nc:            nc,
		js:            js,
		streams:       make(map[string]jetstream.Stream),
		consumers:     make(map[string]jetstream.Consumer),
		observability: observability,
	}

	// Initialize default streams
	if err := eb.initializeStreams(); err != nil {
		return nil, fmt.Errorf("failed to initialize streams: %w", err)
	}

	return eb, nil
}

// initializeStreams creates the default stream configuration
func (eb *EventBus) initializeStreams() error {
	streams := []StreamConfig{
		{
			Name:      "EXPERIMENTS",
			Subjects:  []string{"experiments.*"},
			Retention: jetstream.WorkQueuePolicy,
			MaxAge:    24 * time.Hour,
			MaxBytes:  100 * 1024 * 1024, // 100MB
			MaxMsgs:   100000,
			Storage:   jetstream.FileStorage,
			Replicas:  1,
		},
		{
			Name:      "LOGS",
			Subjects:  []string{"logs.*"},
			Retention: jetstream.LimitsPolicy,
			MaxAge:    7 * 24 * time.Hour,
			MaxBytes:  1 * 1024 * 1024 * 1024, // 1GB
			MaxMsgs:   1000000,
			Storage:   jetstream.FileStorage,
			Replicas:  1,
		},
		{
			Name:      "METRICS",
			Subjects:  []string{"metrics.*"},
			Retention: jetstream.LimitsPolicy,
			MaxAge:    1 * 24 * time.Hour,
			MaxBytes:  100 * 1024 * 1024, // 100MB
			MaxMsgs:   100000,
			Storage:   jetstream.FileStorage,
			Replicas:  1,
		},
		{
			Name:      "NOTIFICATIONS",
			Subjects:  []string{"notifications.*"},
			Retention: jetstream.WorkQueuePolicy,
			MaxAge:    1 * time.Hour,
			MaxBytes:  50 * 1024 * 1024, // 50MB
			MaxMsgs:   10000,
			Storage:   jetstream.FileStorage,
			Replicas:  1,
		},
	}

	for _, config := range streams {
		if err := eb.createStream(config); err != nil {
			return fmt.Errorf("failed to create stream %s: %w", config.Name, err)
		}
	}

	return nil
}

// createStream creates a new stream
func (eb *EventBus) createStream(config StreamConfig) error {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	// Check if stream already exists
	if _, exists := eb.streams[config.Name]; exists {
		return nil
	}

	stream, err := eb.js.CreateStream(context.Background(), jetstream.StreamConfig{
		Name:       config.Name,
		Subjects:   config.Subjects,
		Retention:  config.Retention,
		MaxAge:     config.MaxAge,
		MaxBytes:   config.MaxBytes,
		MaxMsgs:    config.MaxMsgs,
		MaxMsgSize: config.MaxMsgSize,
		Storage:    config.Storage,
		Replicas:   config.Replicas,
		// DiscardPolicy field removed in newer NATS versions
	})
	if err != nil {
		return fmt.Errorf("failed to create stream: %w", err)
	}

	eb.streams[config.Name] = stream
	log.Printf("[EventBus] Created stream: %s", config.Name)

	return nil
}

// Publish publishes an event to a stream
func (eb *EventBus) Publish(ctx context.Context, subject string, event *Event) error {
	start := time.Now()
	defer func() {
		eb.observability.RecordEventPublished(subject, time.Since(start))
	}()

	// Generate event ID if not provided
	if event.ID == "" {
		event.ID = generateEventID(event)
	}

	// Set timestamp if not provided
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Extract trace context if available
	if span := trace.SpanFromContext(ctx); span != nil {
		event.TraceID = span.SpanContext().TraceID().String()
		event.SpanID = span.SpanContext().SpanID().String()
	}

	// Marshal event
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Publish with headers for idempotency
	headers := nats.Header{}
	headers.Set("X-Event-ID", event.ID)
	headers.Set("X-Event-Type", event.Type)
	headers.Set("X-Source", event.Source)

	// Publish to JetStream
	ack, err := eb.js.PublishMsg(ctx, &nats.Msg{
		Subject: subject,
		Data:    data,
		Header:  headers,
	})
	if err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	log.Printf("[EventBus] Published event %s to %s (seq: %d)", event.ID, subject, ack.Sequence)
	return nil
}

// Subscribe creates a subscription to a stream
func (eb *EventBus) Subscribe(ctx context.Context, config ConsumerConfig, handler EventHandler) error {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	// Check if consumer already exists
	if _, exists := eb.consumers[config.Name]; exists {
		return fmt.Errorf("consumer %s already exists", config.Name)
	}

	// Create consumer
	consumer, err := eb.js.CreateConsumer(ctx, config.Stream, jetstream.ConsumerConfig{
		Name:          config.Name,
		FilterSubject: config.FilterSubject,
		DeliverPolicy: config.DeliverPolicy,
		AckPolicy:     config.AckPolicy,
		AckWait:       config.AckWait,
		MaxDeliver:    config.MaxDeliver,
		MaxAckPending: config.MaxAckPending,
		BackOff:       config.BackOff,
		IdleHeartbeat: config.IdleHeartbeat,
		FlowControl:   config.FlowControl,
		HeadersOnly:   config.HeadersOnly,
	})
	if err != nil {
		return fmt.Errorf("failed to create consumer: %w", err)
	}

	eb.consumers[config.Name] = consumer

	// Start consuming messages
	go eb.consumeMessages(ctx, consumer, handler)

	log.Printf("[EventBus] Created consumer: %s for stream: %s", config.Name, config.Stream)
	return nil
}

// consumeMessages processes messages from a consumer
func (eb *EventBus) consumeMessages(ctx context.Context, consumer jetstream.Consumer, handler EventHandler) {
	msgs, err := consumer.Consume(func(msg jetstream.Msg) {
		defer msg.Ack()

		// Parse event
		var event Event
		if err := json.Unmarshal(msg.Data(), &event); err != nil {
			log.Printf("[EventBus] Failed to unmarshal event: %v", err)
			return
		}

		// Create context with trace
		ctx, span := eb.observability.EventProcessingSpan(ctx, event.Type, event.ID)
		defer span.End()

		// Process event
		start := time.Now()
		if err := handler(ctx, &event); err != nil {
			log.Printf("[EventBus] Handler failed for event %s: %v", event.ID, err)
			eb.observability.RecordEventProcessingFailed(event.Type, err.Error())
		} else {
			eb.observability.RecordEventProcessed(event.Type, time.Since(start))
		}
	})
	if err != nil {
		log.Printf("[EventBus] Failed to consume messages: %v", err)
		return
	}
	defer msgs.Stop()

	// Wait for context cancellation
	<-ctx.Done()
	log.Printf("[EventBus] Consumer stopped: %s", consumer.CachedInfo().Name)
}

// CreateDeadLetterQueue creates a DLQ stream for failed messages
func (eb *EventBus) CreateDeadLetterQueue(streamName string) error {
	dlqConfig := StreamConfig{
		Name:      fmt.Sprintf("%s_DLQ", streamName),
		Subjects:  []string{fmt.Sprintf("%s.dlq", streamName)},
		Retention: jetstream.LimitsPolicy,
		MaxAge:    7 * 24 * time.Hour, // Keep DLQ messages for a week
		MaxBytes:  100 * 1024 * 1024,  // 100MB
		MaxMsgs:   10000,
		Storage:   jetstream.FileStorage,
		Replicas:  1,
	}

	return eb.createStream(dlqConfig)
}

// ReplayEvents replays events from a specific sequence number
func (eb *EventBus) ReplayEvents(ctx context.Context, streamName string, fromSeq uint64, handler EventHandler) error {
	_, exists := eb.streams[streamName]
	if !exists {
		return fmt.Errorf("stream %s not found", streamName)
	}

	// Create a replay consumer
	consumer, err := eb.js.CreateConsumer(ctx, streamName, jetstream.ConsumerConfig{
		Name:          fmt.Sprintf("replay_%d", time.Now().Unix()),
		DeliverPolicy: jetstream.DeliverByStartSequencePolicy,
		OptStartSeq:   fromSeq,
		AckPolicy:     jetstream.AckNonePolicy, // Don't track acks for replay
	})
	if err != nil {
		return fmt.Errorf("failed to create replay consumer: %w", err)
	}
	// Consumer cleanup handled automatically in newer NATS versions

	// Consume messages
	msgs, err := consumer.Consume(func(msg jetstream.Msg) {
		var event Event
		if err := json.Unmarshal(msg.Data(), &event); err != nil {
			log.Printf("[EventBus] Failed to unmarshal replay event: %v", err)
			return
		}

		if err := handler(ctx, &event); err != nil {
			log.Printf("[EventBus] Replay handler failed for event %s: %v", event.ID, err)
		}
	})
	if err != nil {
		return fmt.Errorf("failed to consume replay messages: %w", err)
	}
	defer msgs.Stop()

	// Wait for context cancellation or completion
	<-ctx.Done()
	return nil
}

// GetStreamInfo returns information about a stream
func (eb *EventBus) GetStreamInfo(streamName string) (*jetstream.StreamInfo, error) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	stream, exists := eb.streams[streamName]
	if !exists {
		return nil, fmt.Errorf("stream %s not found", streamName)
	}

	return stream.Info(context.Background())
}

// GetConsumerInfo returns information about a consumer
func (eb *EventBus) GetConsumerInfo(streamName, consumerName string) (*jetstream.ConsumerInfo, error) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	consumer, exists := eb.consumers[consumerName]
	if !exists {
		return nil, fmt.Errorf("consumer %s not found", consumerName)
	}

	return consumer.Info(context.Background())
}

// Close closes the event bus
func (eb *EventBus) Close() error {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	// Close all consumers
	for name := range eb.consumers {
		// Consumer cleanup handled automatically in newer NATS versions
		log.Printf("[EventBus] Closing consumer %s", name)
	}

	// Close NATS connection
	if eb.nc != nil {
		eb.nc.Close()
	}

	return nil
}

// generateEventID generates a unique event ID
func generateEventID(event *Event) string {
	data := fmt.Sprintf("%s-%s-%s-%v", event.Type, event.Source, event.Timestamp, event.Data)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash[:8])
}

// Context helpers
func ctx() context.Context {
	return context.Background()
}
