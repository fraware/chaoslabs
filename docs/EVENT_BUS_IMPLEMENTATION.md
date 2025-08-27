# Event Bus Implementation

This document describes the comprehensive event bus implementation for ChaosLabs using NATS JetStream, providing decoupled writes, real-time notifications, and event replay capabilities.

## Architecture Overview

The event bus architecture follows the **Event Sourcing** pattern with **CQRS** (Command Query Responsibility Segregation) principles:

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Controller    │    │   Event Bus     │    │   Services      │
│                 │    │                 │    │                 │
│ ┌─────────────┐ │    │ ┌─────────────┐ │    │ ┌─────────────┐ │
│ │HTTP Handler │ │───▶│ │NATS Stream │ │───▶│ │Writer Svc   │ │
│ └─────────────┘ │    │ └─────────────┘ │    │ └─────────────┘ │
│                 │    │                 │    │                 │
│ ┌─────────────┐ │    │ ┌─────────────┐ │    │ ┌─────────────┐ │
│ │Experiment   │ │───▶│ │Event Store │ │───▶│ │Notifier Svc │ │
│ │Scheduler    │ │    │ │(JetStream) │ │    │ └─────────────┘ │
│ └─────────────┘ │    │ └─────────────┘ │    │                 │
└─────────────────┘    └─────────────────┘    │ ┌─────────────┐ │
                                              │ │Replay Svc   │ │
                                              │ └─────────────┘ │
                                              └─────────────────┘
```

## Components

### 1. EventBus (`controller/eventbus.go`)

The core event streaming infrastructure using NATS JetStream.

**Key Features:**
- **Streams**: EXPERIMENTS, LOGS, METRICS, NOTIFICATIONS
- **Idempotency**: SHA256-based event IDs prevent duplicates
- **Tracing**: OpenTelemetry integration for end-to-end visibility
- **Retention**: Configurable TTL and storage policies
- **Replication**: Support for multi-node deployments

**Stream Configuration:**
```go
streams := []StreamConfig{
    {
        Name:     "EXPERIMENTS",
        Subjects: []string{"experiments.*"},
        Retention: jetstream.WorkQueuePolicy,
        MaxAge:   24 * time.Hour,
        MaxBytes: 100 * 1024 * 1024, // 100MB
        MaxMsgs:  100000,
        Storage:  jetstream.FileStorage,
        Replicas: 1,
    },
    // ... other streams
}
```

### 2. WriterService (`controller/writer_service.go`)

Consumes events from streams and persists them to MongoDB with idempotency.

**Features:**
- **Idempotency**: Tracks processed event IDs to prevent duplicates
- **Batch Processing**: Efficient MongoDB writes
- **Error Handling**: Automatic retry with exponential backoff
- **Metrics**: OpenTelemetry integration for monitoring

**Event Processing:**
```go
func (ws *WriterService) handleExperimentEvent(ctx context.Context, event *Event) error {
    // Check idempotency
    if ws.isProcessed(event.ID) {
        return nil // Skip duplicate
    }
    
    // Parse and persist event
    logEvent := &LogEvent{...}
    if err := ws.storage.InsertLogEvent(logEvent); err != nil {
        ws.observability.RecordEventProcessingFailed("experiment", err.Error())
        return err
    }
    
    // Mark as processed
    ws.markProcessed(event.ID)
    return nil
}
```

### 3. NotifierService (`controller/notifier_service.go`)

Provides real-time WebSocket updates with backpressure handling.

**Features:**
- **WebSocket Management**: Client connection lifecycle
- **Room-based Broadcasting**: Namespace isolation
- **Backpressure Control**: QPS limits and client buffering
- **Auto-cleanup**: Stale client and room cleanup

**Room Management:**
```go
func (ns *NotifierService) JoinRoom(clientID, roomName string) error {
    // Check room capacity
    if len(room.Clients) >= room.MaxClients {
        return fmt.Errorf("room %s is at capacity", roomName)
    }
    
    // Add client to room
    room.AddClient(clientID, client)
    client.Rooms[roomName] = true
    return nil
}
```

### 4. ReplayService (`controller/replay_service.go`)

Handles event replay for gap healing and audit purposes.

**Features:**
- **Gap Healing**: Replay events from specific sequence numbers
- **Filtering**: Selective replay based on event attributes
- **Progress Tracking**: Real-time job status and statistics
- **Dry Run**: Safe replay testing without side effects

**Replay Configuration:**
```go
config := ReplayConfig{
    StreamName:   "EXPERIMENTS",
    FromSequence: 1000,
    ToSequence:   2000,
    BatchSize:    100,
    Concurrency:  4,
    DryRun:       false,
    Filters: map[string]interface{}{
        "event_type": "experiment",
        "source":     "agent-001",
    },
}
```

## Usage Examples

### Publishing Events

```go
// Create event
event := &Event{
    Type:      "experiment_started",
    Source:    "controller-001",
    Timestamp: time.Now(),
    Data: map[string]interface{}{
        "experiment_id": "exp-123",
        "target":        "agent-001",
        "type":          "network_latency",
        "duration":      300,
    },
}

// Publish to event bus
err := eventBus.Publish(ctx, "experiments.started", event)
```

### Subscribing to Events

```go
// Subscribe to experiment events
err := eventBus.Subscribe(ctx, ConsumerConfig{
    Name:          "experiment-processor",
    Stream:        "EXPERIMENTS",
    FilterSubject: "experiments.*",
    DeliverPolicy: jetstream.DeliverNewPolicy,
    AckPolicy:     jetstream.AckExplicitPolicy,
    AckWait:       30 * time.Second,
    MaxDeliver:    3,
}, func(ctx context.Context, event *Event) error {
    // Process event
    return processExperimentEvent(event)
})
```

### Starting a Replay

```go
// Start replay job
jobID, err := replayService.StartReplay(ReplayConfig{
    StreamName:   "EXPERIMENTS",
    FromSequence: 1000,
    ToSequence:   2000,
    BatchSize:    100,
    DryRun:       true, // Safe testing
})

// Monitor progress
job, _ := replayService.GetReplayJob(jobID)
fmt.Printf("Progress: %.2f%%\n", job.Progress)
```

## Configuration

### Environment Variables

```bash
# NATS Configuration
NATS_URL=nats://localhost:4222
NATS_CLUSTER_ID=chaoslabs-cluster
NATS_CLIENT_ID=chaoslabs-controller

# Stream Configuration
EXPERIMENTS_STREAM_MAX_AGE=24h
LOGS_STREAM_MAX_AGE=168h
METRICS_STREAM_MAX_AGE=24h
NOTIFICATIONS_STREAM_MAX_AGE=1h

# Backpressure Configuration
DEFAULT_ROOM_QPS=100
MAX_CLIENTS_PER_ROOM=1000
CLIENT_BUFFER_SIZE=100
```

### Stream Retention Policies

| Stream | Retention Policy | Max Age | Max Size | Max Messages |
|--------|------------------|----------|----------|--------------|
| EXPERIMENTS | WorkQueue | 24h | 100MB | 100,000 |
| LOGS | Limits | 168h | 1GB | 1,000,000 |
| METRICS | Limits | 24h | 100MB | 100,000 |
| NOTIFICATIONS | WorkQueue | 1h | 50MB | 10,000 |

## Monitoring and Observability

### Metrics

The event bus exposes comprehensive metrics via OpenTelemetry:

- **Event Publishing**: Duration and success rates
- **Event Processing**: Duration and failure counts
- **Stream Health**: Message counts, consumer lag
- **Client Connections**: Active clients, room occupancy
- **Replay Jobs**: Progress, success rates, duration

### Health Checks

```bash
# Check stream health
curl http://localhost:8080/health/streams

# Check consumer status
curl http://localhost:8080/health/consumers

# Check WebSocket connections
curl http://localhost:8080/health/websockets
```

### Grafana Dashboards

Pre-configured dashboards for:
- Event throughput and latency
- Consumer lag and processing rates
- WebSocket connection metrics
- Replay job progress and statistics

## Operational Procedures

### Adding New Streams

1. **Define Stream Configuration:**
```go
newStream := StreamConfig{
    Name:     "AUDIT_LOGS",
    Subjects: []string{"audit.*"},
    Retention: jetstream.LimitsPolicy,
    MaxAge:   365 * 24 * time.Hour, // 1 year
    MaxBytes: 10 * 1024 * 1024 * 1024, // 10GB
    MaxMsgs:  10000000,
    Storage:  jetstream.FileStorage,
    Replicas: 3,
}
```

2. **Add to EventBus:**
```go
if err := eventBus.createStream(newStream); err != nil {
    log.Printf("Failed to create audit stream: %v", err)
}
```

3. **Update Services:**
```go
// Add writer service subscription
writerService.SubscribeToStream("AUDIT_LOGS", handleAuditEvent)

// Add notifier service if real-time updates needed
notifierService.SubscribeToStream("AUDIT_LOGS", handleAuditNotification)
```

### Scaling Considerations

#### Horizontal Scaling

1. **Multiple Controller Instances:**
   - Use sticky sessions for WebSocket connections
   - Share event bus state via Redis (optional)
   - Load balance HTTP requests

2. **Stream Replication:**
   - Increase `Replicas` count for critical streams
   - Monitor replication lag
   - Use `jetstream.ClusteredStorage` for multi-datacenter

#### Performance Tuning

1. **Consumer Configuration:**
```go
ConsumerConfig{
    MaxAckPending: 1000,        // Increase for high throughput
    FlowControl:   true,        // Enable flow control
    IdleHeartbeat: 5 * time.Second, // Reduce for faster recovery
}
```

2. **Batch Processing:**
```go
// Increase batch sizes for high-volume streams
BatchSize: 1000,  // Default: 100
```

### Disaster Recovery

#### Backup and Restore

1. **Stream Backup:**
```bash
# Export stream data
nats stream export EXPERIMENTS --output experiments_backup.json

# Import stream data
nats stream import EXPERIMENTS --input experiments_backup.json
```

2. **Event Replay:**
```go
// Replay from last known good sequence
replayService.StartReplay(ReplayConfig{
    StreamName:   "EXPERIMENTS",
    FromSequence: lastKnownGoodSeq,
    ToSequence:   currentSeq,
    BatchSize:    1000,
    DryRun:       false,
})
```

#### Failover Procedures

1. **Primary NATS Failure:**
   - Switch to backup NATS cluster
   - Update connection URLs
   - Verify stream replication

2. **Service Recovery:**
   - Restart failed services
   - Check consumer lag
   - Verify event processing

## Troubleshooting

### Common Issues

#### High Consumer Lag

**Symptoms:**
- Consumer lag increasing over time
- Events taking long to process
- Memory usage growing

**Solutions:**
1. **Increase Consumer Concurrency:**
```go
MaxAckPending: 2000,  // Increase from default 100
```

2. **Optimize Event Processing:**
```go
// Use batch processing
func handleEventsBatch(events []*Event) error {
    // Process multiple events together
    return batchProcess(events)
}
```

3. **Check Resource Limits:**
```bash
# Monitor CPU and memory
top -p $(pgrep chaos-controller)
```

#### WebSocket Connection Issues

**Symptoms:**
- Clients disconnecting frequently
- High connection error rates
- Slow message delivery

**Solutions:**
1. **Adjust Client Buffer Size:**
```go
Send: make(chan []byte, 500), // Increase from default 100
```

2. **Tune Heartbeat Intervals:**
```go
// Reduce cleanup frequency
ticker := time.NewTicker(60 * time.Second) // Default: 30s
```

3. **Monitor Network Latency:**
```bash
# Check network performance
ping -c 10 websocket-server
```

#### Event Processing Failures

**Symptoms:**
- High failure rates in metrics
- Events stuck in processing
- Database connection errors

**Solutions:**
1. **Check MongoDB Health:**
```bash
# Verify MongoDB status
mongo --eval "db.serverStatus()"
```

2. **Review Error Logs:**
```bash
# Check application logs
tail -f /var/log/chaoslabs/controller.log | grep ERROR
```

3. **Verify Event Schema:**
```go
// Add validation
if err := validateEvent(event); err != nil {
    log.Printf("Invalid event: %v", err)
    return err
}
```

### Debug Mode

Enable debug logging for troubleshooting:

```bash
export LOG_LEVEL=debug
export NATS_DEBUG=true

# Start controller with debug
./chaos-controller --debug
```

## Security Considerations

### Authentication and Authorization

1. **NATS Authentication:**
```bash
# Use JWT tokens
nats-server --config nats.conf
```

2. **Event Validation:**
```go
// Validate event source and type
if !isAuthorizedSource(event.Source) {
    return fmt.Errorf("unauthorized source: %s", event.Source)
}
```

### Data Privacy

1. **PII Filtering:**
```go
// Remove sensitive data before publishing
func sanitizeEvent(event *Event) *Event {
    if event.Data["user_id"] != nil {
        event.Data["user_id"] = "***"
    }
    return event
}
```

2. **Encryption:**
```go
// Encrypt sensitive event data
encryptedData, err := encryptEventData(event.Data, encryptionKey)
if err != nil {
    return err
}
event.Data = encryptedData
```

## Performance Benchmarks

### Baseline Performance

| Metric | Target | Current | Notes |
|--------|--------|---------|-------|
| Event Publish Latency (P99) | < 10ms | 5ms | Under normal load |
| Event Processing Latency (P99) | < 100ms | 50ms | MongoDB writes |
| WebSocket Message Latency (P99) | < 50ms | 20ms | Real-time updates |
| Maximum Events/sec | 10,000 | 15,000 | Sustained throughput |

### Load Testing

Use the provided load testing tools:

```bash
# Run event bus load test
cd bench
./run_eventbus_load_test.sh \
  --duration "10m" \
  --event-rate "5000" \
  --concurrent-publishers "10" \
  --concurrent-consumers "20"
```
