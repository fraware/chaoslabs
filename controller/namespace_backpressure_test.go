package main

import (
	"encoding/json"
	"sync"
	"testing"
	"time"
)

func TestDiffEmitEngine_BasicDiff(t *testing.T) {
	config := &DiffEmitConfig{
		MaxStateHistory: 10,
		DiffThreshold:   0.1,
		DeepCompare:     true,
	}

	engine := NewDiffEmitEngine(config)

	// First emission - should emit full data
	data1 := map[string]interface{}{
		"id":     "exp-1",
		"status": "running",
		"count":  10,
	}

	result1, err := engine.ComputeDiff("test-key", data1)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !result1.HasChanges {
		t.Error("First emission should have changes")
	}

	if result1.ChangePercent != 1.0 {
		t.Errorf("Expected 100%% change for first emission, got %.2f%%", result1.ChangePercent*100)
	}

	// Second emission - same data, should not emit
	result2, err := engine.ComputeDiff("test-key", data1)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result2.HasChanges {
		t.Error("Same data should not trigger changes")
	}

	if result2.ChangePercent != 0.0 {
		t.Errorf("Expected 0%% change for same data, got %.2f%%", result2.ChangePercent*100)
	}

	// Third emission - partial change
	data3 := map[string]interface{}{
		"id":     "exp-1",
		"status": "completed", // Changed
		"count":  10,
	}

	result3, err := engine.ComputeDiff("test-key", data3)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !result3.HasChanges {
		t.Error("Changed data should trigger changes")
	}

	if len(result3.ChangedFields) != 1 || result3.ChangedFields[0] != "status" {
		t.Errorf("Expected 'status' field to be changed, got: %v", result3.ChangedFields)
	}

	// Verify change percentage is reasonable (1 out of 3 fields = ~33%)
	expectedPercent := 1.0 / 3.0
	if result3.ChangePercent < expectedPercent-0.1 || result3.ChangePercent > expectedPercent+0.1 {
		t.Errorf("Expected change percent around %.2f%%, got %.2f%%", 
			expectedPercent*100, result3.ChangePercent*100)
	}
}

func TestDiffEmitEngine_ArrayDiff(t *testing.T) {
	config := &DiffEmitConfig{
		MaxStateHistory: 10,
		DiffThreshold:   0.0, // Emit all changes
		DeepCompare:     true,
	}

	engine := NewDiffEmitEngine(config)

	// Initial array
	data1 := map[string]interface{}{
		"items": []interface{}{"a", "b", "c"},
	}

	result1, err := engine.ComputeDiff("array-test", data1)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !result1.HasChanges {
		t.Error("First emission should have changes")
	}

	// Modified array
	data2 := map[string]interface{}{
		"items": []interface{}{"a", "modified-b", "c", "d"}, // Changed + added
	}

	result2, err := engine.ComputeDiff("array-test", data2)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !result2.HasChanges {
		t.Error("Array modification should trigger changes")
	}

	if len(result2.ChangedFields) != 1 || result2.ChangedFields[0] != "items" {
		t.Errorf("Expected 'items' field to be changed, got: %v", result2.ChangedFields)
	}
}

func TestDiffEmitEngine_IgnoreFields(t *testing.T) {
	config := &DiffEmitConfig{
		MaxStateHistory: 10,
		DiffThreshold:   0.0,
		DeepCompare:     true,
		IgnoreFields:    []string{"timestamp", "updated_*"},
	}

	engine := NewDiffEmitEngine(config)

	// Initial data
	data1 := map[string]interface{}{
		"id":        "exp-1",
		"status":    "running",
		"timestamp": "2023-01-01T00:00:00Z",
		"updated_at": "2023-01-01T00:00:00Z",
	}

	result1, err := engine.ComputeDiff("ignore-test", data1)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Update only ignored fields
	data2 := map[string]interface{}{
		"id":        "exp-1",
		"status":    "running",
		"timestamp": "2023-01-01T01:00:00Z", // Changed but ignored
		"updated_at": "2023-01-01T01:00:00Z", // Changed but ignored
	}

	result2, err := engine.ComputeDiff("ignore-test", data2)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result2.HasChanges {
		t.Error("Changes to ignored fields should not trigger emission")
	}

	// Update non-ignored field
	data3 := map[string]interface{}{
		"id":        "exp-1",
		"status":    "completed", // Changed and not ignored
		"timestamp": "2023-01-01T02:00:00Z",
		"updated_at": "2023-01-01T02:00:00Z",
	}

	result3, err := engine.ComputeDiff("ignore-test", data3)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !result3.HasChanges {
		t.Error("Changes to non-ignored fields should trigger emission")
	}

	if len(result3.ChangedFields) != 1 || result3.ChangedFields[0] != "status" {
		t.Errorf("Expected only 'status' field to be changed, got: %v", result3.ChangedFields)
	}
}

func TestDiffEmitEngine_Threshold(t *testing.T) {
	config := &DiffEmitConfig{
		MaxStateHistory: 10,
		DiffThreshold:   0.5, // 50% threshold
		DeepCompare:     true,
	}

	engine := NewDiffEmitEngine(config)

	// Initial data with 4 fields
	data1 := map[string]interface{}{
		"field1": "value1",
		"field2": "value2",
		"field3": "value3",
		"field4": "value4",
	}

	result1, err := engine.ComputeDiff("threshold-test", data1)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Change 1 out of 4 fields (25% < 50% threshold)
	data2 := map[string]interface{}{
		"field1": "changed-value1",
		"field2": "value2",
		"field3": "value3",
		"field4": "value4",
	}

	result2, err := engine.ComputeDiff("threshold-test", data2)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result2.HasChanges {
		t.Error("Changes below threshold should not be emitted")
	}

	// Change 2 out of 4 fields (50% >= 50% threshold)
	data3 := map[string]interface{}{
		"field1": "changed-value1",
		"field2": "changed-value2",
		"field3": "value3",
		"field4": "value4",
	}

	result3, err := engine.ComputeDiff("threshold-test", data3)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !result3.HasChanges {
		t.Error("Changes at threshold should be emitted")
	}
}

func TestDiffEmitEngine_Concurrency(t *testing.T) {
	config := &DiffEmitConfig{
		MaxStateHistory: 100,
		DiffThreshold:   0.0,
		DeepCompare:     true,
	}

	engine := NewDiffEmitEngine(config)

	// Test concurrent access
	var wg sync.WaitGroup
	numGoroutines := 10
	numOperations := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("key-%d", goroutineID)
				data := map[string]interface{}{
					"goroutine_id": goroutineID,
					"operation":    j,
					"timestamp":    time.Now().Unix(),
				}

				_, err := engine.ComputeDiff(key, data)
				if err != nil {
					t.Errorf("Goroutine %d, operation %d failed: %v", goroutineID, j, err)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify metrics
	metrics := engine.GetMetrics()
	expectedComparisons := int64(numGoroutines * numOperations)
	if metrics.TotalComparisons != expectedComparisons {
		t.Errorf("Expected %d total comparisons, got %d", 
			expectedComparisons, metrics.TotalComparisons)
	}
}

func TestDiffEmitEngine_NestedObjects(t *testing.T) {
	config := &DiffEmitConfig{
		MaxStateHistory: 10,
		DiffThreshold:   0.0,
		DeepCompare:     true,
	}

	engine := NewDiffEmitEngine(config)

	// Initial nested data
	data1 := map[string]interface{}{
		"experiment": map[string]interface{}{
			"id":     "exp-1",
			"config": map[string]interface{}{
				"duration": 300,
				"targets":  []interface{}{"server1", "server2"},
			},
		},
		"status": "running",
	}

	result1, err := engine.ComputeDiff("nested-test", data1)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Modify nested value
	data2 := map[string]interface{}{
		"experiment": map[string]interface{}{
			"id":     "exp-1",
			"config": map[string]interface{}{
				"duration": 600, // Changed
				"targets":  []interface{}{"server1", "server2"},
			},
		},
		"status": "running",
	}

	result2, err := engine.ComputeDiff("nested-test", data2)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !result2.HasChanges {
		t.Error("Nested changes should be detected")
	}

	if len(result2.ChangedFields) != 1 || result2.ChangedFields[0] != "experiment" {
		t.Errorf("Expected 'experiment' field to be changed, got: %v", result2.ChangedFields)
	}

	// Verify diff contains nested information
	diffJSON, _ := json.MarshalIndent(result2.Diff, "", "  ")
	t.Logf("Nested diff: %s", diffJSON)
}

func TestDiffEmitEngine_Performance(t *testing.T) {
	config := &DiffEmitConfig{
		MaxStateHistory: 1000,
		DiffThreshold:   0.0,
		DeepCompare:     true,
	}

	engine := NewDiffEmitEngine(config)

	// Create large data structure
	largeData := make(map[string]interface{})
	for i := 0; i < 1000; i++ {
		largeData[fmt.Sprintf("field_%d", i)] = fmt.Sprintf("value_%d", i)
	}

	// Measure first diff (baseline)
	start := time.Now()
	result1, err := engine.ComputeDiff("perf-test", largeData)
	firstDiffTime := time.Since(start)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Measure second diff (should be faster due to hash optimization)
	start = time.Now()
	result2, err := engine.ComputeDiff("perf-test", largeData)
	secondDiffTime := time.Since(start)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Second diff should be much faster (hash comparison)
	if secondDiffTime > firstDiffTime/10 {
		t.Errorf("Hash optimization not working: first=%v, second=%v", 
			firstDiffTime, secondDiffTime)
	}

	if result2.HasChanges {
		t.Error("Identical data should not show changes")
	}

	// Make small change and measure
	largeData["field_500"] = "modified_value"
	
	start = time.Now()
	result3, err := engine.ComputeDiff("perf-test", largeData)
	thirdDiffTime := time.Since(start)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !result3.HasChanges {
		t.Error("Modified data should show changes")
	}

	// Log performance metrics
	t.Logf("Performance: first=%v, second=%v, third=%v", 
		firstDiffTime, secondDiffTime, thirdDiffTime)
	t.Logf("Compute time from result: %v", result3.ComputeTime)

	// Verify compute time is reasonable (< 10ms for 1000 fields)
	if result3.ComputeTime > 10*time.Millisecond {
		t.Errorf("Diff computation too slow: %v", result3.ComputeTime)
	}
}

func TestDiffEmitEngine_StateCleanup(t *testing.T) {
	config := &DiffEmitConfig{
		MaxStateHistory: 5, // Small limit for testing
		DiffThreshold:   0.0,
		DeepCompare:     true,
	}

	engine := NewDiffEmitEngine(config)

	// Add more states than the limit
	for i := 0; i < 10; i++ {
		data := map[string]interface{}{
			"id":    fmt.Sprintf("item-%d", i),
			"value": i,
		}
		
		_, err := engine.ComputeDiff(fmt.Sprintf("key-%d", i), data)
		if err != nil {
			t.Fatalf("Unexpected error for item %d: %v", i, err)
		}
		
		// Add small delay to ensure different timestamps
		time.Sleep(1 * time.Millisecond)
	}

	// Trigger cleanup manually
	engine.performCleanup()

	// Check that state store size is within limit
	metrics := engine.GetMetrics()
	if metrics.StateStoreSize > config.MaxStateHistory {
		t.Errorf("State store not cleaned up: size=%d, limit=%d", 
			metrics.StateStoreSize, config.MaxStateHistory)
	}

	// Verify that most recent states are preserved
	for i := 5; i < 10; i++ {
		data := map[string]interface{}{
			"id":    fmt.Sprintf("item-%d", i),
			"value": i,
		}
		
		result, err := engine.ComputeDiff(fmt.Sprintf("key-%d", i), data)
		if err != nil {
			t.Fatalf("Unexpected error checking preserved state %d: %v", i, err)
		}
		
		// Should not have changes since data is the same
		if result.HasChanges {
			t.Errorf("Recent state %d should be preserved and show no changes", i)
		}
	}
}