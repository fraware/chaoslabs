package main

import (
	"testing"
	"time"
)

// TestTimeSeriesManagerCreation tests the creation of a time-series manager
func TestTimeSeriesManagerCreation(t *testing.T) {
	// This test would require a MongoDB instance
	// In CI/CD, we'd use a test container or mock
	t.Skip("Skipping test that requires MongoDB connection")

	// Test would look like:
	// tsm, err := NewTimeSeriesManager("mongodb://localhost:27017", "test")
	// if err != nil {
	//     t.Fatalf("Failed to create time-series manager: %v", err)
	// }
	// defer tsm.Close()
	//
	// if tsm.client == nil {
	//     t.Error("MongoDB client not initialized")
	// }
	//
	// if tsm.database == nil {
	//     t.Error("Database not initialized")
	// }
}

// TestLogEventStructure tests the LogEvent structure
func TestLogEventStructure(t *testing.T) {
	event := &LogEvent{
		Timestamp:    time.Now(),
		AgentID:      "test-agent",
		Level:        "info",
		Message:      "Test log message",
		ExperimentID: "test-exp",
		Component:    "test-component",
		Metadata: map[string]interface{}{
			"test_key": "test_value",
		},
		ReviewStatus: "pending",
		TTL:          time.Now().Add(24 * time.Hour),
	}

	if event.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}

	if event.AgentID != "test-agent" {
		t.Errorf("Expected AgentID 'test-agent', got '%s'", event.AgentID)
	}

	if event.Level != "info" {
		t.Errorf("Expected Level 'info', got '%s'", event.Level)
	}

	if event.Message != "Test log message" {
		t.Errorf("Expected Message 'Test log message', got '%s'", event.Message)
	}

	if event.ExperimentID != "test-exp" {
		t.Errorf("Expected ExperimentID 'test-exp', got '%s'", event.ExperimentID)
	}

	if event.Component != "test-component" {
		t.Errorf("Expected Component 'test-component', got '%s'", event.Component)
	}

	if event.ReviewStatus != "pending" {
		t.Errorf("Expected ReviewStatus 'pending', got '%s'", event.ReviewStatus)
	}

	if event.TTL.IsZero() {
		t.Error("TTL should not be zero")
	}

	// Test metadata
	if event.Metadata["test_key"] != "test_value" {
		t.Errorf("Expected metadata value 'test_value', got '%v'", event.Metadata["test_key"])
	}
}

// TestLogQueryFilter tests the LogQueryFilter structure
func TestLogQueryFilter(t *testing.T) {
	now := time.Now()
	filter := LogQueryFilter{
		StartTime:    now.Add(-24 * time.Hour),
		EndTime:      now,
		AgentID:      "test-agent",
		Level:        "error",
		ExperimentID: "test-exp",
		Component:    "test-component",
		ReviewStatus: "pending",
		Metadata:     map[string]interface{}{"key": "value"},
	}

	if filter.StartTime.IsZero() {
		t.Error("StartTime should not be zero")
	}

	if filter.EndTime.IsZero() {
		t.Error("EndTime should not be zero")
	}

	if filter.AgentID != "test-agent" {
		t.Errorf("Expected AgentID 'test-agent', got '%s'", filter.AgentID)
	}

	if filter.Level != "error" {
		t.Errorf("Expected Level 'error', got '%s'", filter.Level)
	}

	if filter.ExperimentID != "test-exp" {
		t.Errorf("Expected ExperimentID 'test-exp', got '%s'", filter.ExperimentID)
	}

	if filter.Component != "test-component" {
		t.Errorf("Expected Component 'test-component', got '%s'", filter.Component)
	}

	if filter.ReviewStatus != "pending" {
		t.Errorf("Expected ReviewStatus 'pending', got '%s'", filter.ReviewStatus)
	}

	// Test metadata
	if filter.Metadata["key"] != "value" {
		t.Errorf("Expected metadata value 'value', got '%v'", filter.Metadata["key"])
	}
}

// TestRetentionPolicy tests the RetentionPolicy structure
func TestRetentionPolicy(t *testing.T) {
	policy := RetentionPolicy{
		CollectionName: "test_collection",
		HotRetention:   24 * time.Hour,
		WarmRetention:  7 * 24 * time.Hour,
		ColdRetention:  30 * 24 * time.Hour,
		TTLIndex:       7 * 24 * time.Hour,
	}

	if policy.CollectionName != "test_collection" {
		t.Errorf("Expected CollectionName 'test_collection', got '%s'", policy.CollectionName)
	}

	if policy.HotRetention != 24*time.Hour {
		t.Errorf("Expected HotRetention 24h, got %v", policy.HotRetention)
	}

	if policy.WarmRetention != 7*24*time.Hour {
		t.Errorf("Expected WarmRetention 7d, got %v", policy.WarmRetention)
	}

	if policy.ColdRetention != 30*24*time.Hour {
		t.Errorf("Expected ColdRetention 30d, got %v", policy.ColdRetention)
	}

	if policy.TTLIndex != 7*24*time.Hour {
		t.Errorf("Expected TTLIndex 7d, got %v", policy.TTLIndex)
	}
}

// TestCollectionStats tests the CollectionStats structure
func TestCollectionStats(t *testing.T) {
	stats := CollectionStats{
		Count:       1000,
		Size:        1024 * 1024, // 1MB
		StorageSize: 2048 * 1024, // 2MB
		IndexSize:   512 * 1024,  // 512KB
		AvgObjSize:  1024.0,      // 1KB
		QueryTime:   50 * time.Millisecond,
	}

	if stats.Count != 1000 {
		t.Errorf("Expected Count 1000, got %d", stats.Count)
	}

	if stats.Size != 1024*1024 {
		t.Errorf("Expected Size 1MB, got %d", stats.Size)
	}

	if stats.StorageSize != 2048*1024 {
		t.Errorf("Expected StorageSize 2MB, got %d", stats.StorageSize)
	}

	if stats.IndexSize != 512*1024 {
		t.Errorf("Expected IndexSize 512KB, got %d", stats.IndexSize)
	}

	if stats.AvgObjSize != 1024.0 {
		t.Errorf("Expected AvgObjSize 1KB, got %f", stats.AvgObjSize)
	}

	if stats.QueryTime != 50*time.Millisecond {
		t.Errorf("Expected QueryTime 50ms, got %v", stats.QueryTime)
	}
}

// TestMigrationResult tests the MigrationResult structure
func TestMigrationResult(t *testing.T) {
	now := time.Now()
	result := MigrationResult{
		CollectionName:     "test_collection",
		RecordsMigrated:    5000,
		MigrationDuration:  30 * time.Second,
		PerformanceGain:    25.5,
		DiskUsageReduction: 40.0,
		BeforeStats: CollectionStats{
			Count:       5000,
			StorageSize: 100 * 1024 * 1024, // 100MB
			QueryTime:   200 * time.Millisecond,
		},
		AfterStats: CollectionStats{
			Count:       5000,
			StorageSize: 60 * 1024 * 1024, // 60MB
			QueryTime:   150 * time.Millisecond,
		},
	}

	if result.CollectionName != "test_collection" {
		t.Errorf("Expected CollectionName 'test_collection', got '%s'", result.CollectionName)
	}

	if result.RecordsMigrated != 5000 {
		t.Errorf("Expected RecordsMigrated 5000, got %d", result.RecordsMigrated)
	}

	if result.MigrationDuration != 30*time.Second {
		t.Errorf("Expected MigrationDuration 30s, got %v", result.MigrationDuration)
	}

	if result.PerformanceGain != 25.5 {
		t.Errorf("Expected PerformanceGain 25.5, got %f", result.PerformanceGain)
	}

	if result.DiskUsageReduction != 40.0 {
		t.Errorf("Expected DiskUsageReduction 40.0, got %f", result.DiskUsageReduction)
	}

	// Test before stats
	if result.BeforeStats.Count != 5000 {
		t.Errorf("Expected BeforeStats.Count 5000, got %d", result.BeforeStats.Count)
	}

	if result.BeforeStats.StorageSize != 100*1024*1024 {
		t.Errorf("Expected BeforeStats.StorageSize 100MB, got %d", result.BeforeStats.StorageSize)
	}

	if result.BeforeStats.QueryTime != 200*time.Millisecond {
		t.Errorf("Expected BeforeStats.QueryTime 200ms, got %v", result.BeforeStats.QueryTime)
	}

	// Test after stats
	if result.AfterStats.Count != 5000 {
		t.Errorf("Expected AfterStats.Count 5000, got %d", result.AfterStats.Count)
	}

	if result.AfterStats.StorageSize != 60*1024*1024 {
		t.Errorf("Expected AfterStats.StorageSize 60MB, got %d", result.AfterStats.StorageSize)
	}

	if result.AfterStats.QueryTime != 150*time.Millisecond {
		t.Errorf("Expected AfterStats.QueryTime 150ms, got %v", result.AfterStats.QueryTime)
	}
}

// TestQueryBenchmark tests the QueryBenchmark structure
func TestQueryBenchmark(t *testing.T) {
	benchmark := QueryBenchmark{
		CollectionName:   "test_collection",
		TimeRangeQuery:   25 * time.Millisecond,
		AggregationQuery: 50 * time.Millisecond,
	}

	if benchmark.CollectionName != "test_collection" {
		t.Errorf("Expected CollectionName 'test_collection', got '%s'", benchmark.CollectionName)
	}

	if benchmark.TimeRangeQuery != 25*time.Millisecond {
		t.Errorf("Expected TimeRangeQuery 25ms, got %v", benchmark.TimeRangeQuery)
	}

	if benchmark.AggregationQuery != 50*time.Millisecond {
		t.Errorf("Expected AggregationQuery 50ms, got %v", benchmark.AggregationQuery)
	}
}

// BenchmarkTimeSeriesOperations benchmarks time-series operations
func BenchmarkTimeSeriesOperations(b *testing.B) {
	// This would benchmark actual MongoDB operations
	// In CI/CD, we'd use a test container
	b.Skip("Skipping benchmark that requires MongoDB connection")

	// Test would look like:
	// tsm, err := NewTimeSeriesManager("mongodb://localhost:27017", "test")
	// if err != nil {
	//     b.Fatalf("Failed to create time-series manager: %v", err)
	// }
	// defer tsm.Close()
	//
	// b.ResetTimer()
	// for i := 0; i < b.N; i++ {
	//     event := &LogEvent{
	//         Timestamp: time.Now(),
	//         AgentID:   fmt.Sprintf("agent-%d", i),
	//         Level:     "info",
	//         Message:   "Benchmark test message",
	//     }
	//
	//     if err := tsm.InsertLogEvent(event); err != nil {
	//         b.Fatalf("Failed to insert event: %v", err)
	//     }
	// }
}

// TestTimeSeriesManagerMethods tests the time-series manager methods
func TestTimeSeriesManagerMethods(t *testing.T) {
	// This test would require a MongoDB instance
	// In CI/CD, we'd use a test container or mock
	t.Skip("Skipping test that requires MongoDB connection")

	// Test would look like:
	// tsm, err := NewTimeSeriesManager("mongodb://localhost:27017", "test")
	// if err != nil {
	//     t.Fatalf("Failed to create time-series manager: %v", err)
	// }
	// defer tsm.Close()
	//
	// // Test InsertLogEvent
	// event := &LogEvent{
	//     Timestamp: time.Now(),
	//     AgentID:   "test-agent",
	//     Level:     "info",
	//     Message:   "Test message",
	// }
	//
	// if err := tsm.InsertLogEvent(event); err != nil {
	//     t.Fatalf("Failed to insert log event: %v", err)
	// }
	//
	// // Test QueryLogs
	// filter := LogQueryFilter{
	//     StartTime: time.Now().Add(-1 * time.Hour),
	//     EndTime:   time.Now(),
	//     AgentID:   "test-agent",
	// }
	//
	// events, total, err := tsm.QueryLogs(filter, 1, 10)
	// if err != nil {
	//     t.Fatalf("Failed to query logs: %v", err)
	// }
	//
	// if total < 1 {
	//     t.Errorf("Expected at least 1 log event, got %d", total)
	// }
	//
	// if len(events) < 1 {
	//     t.Errorf("Expected at least 1 log event, got %d", len(events))
	// }
	//
	// // Test GetCollectionStats
	// stats, err := tsm.GetCollectionStats()
	// if err != nil {
	//     t.Fatalf("Failed to get collection stats: %v", err)
	// }
	//
	// if len(stats) == 0 {
	//     t.Error("Expected collection stats, got empty map")
	// }
}
