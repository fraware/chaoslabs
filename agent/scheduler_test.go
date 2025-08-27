package main

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestNewScheduler(t *testing.T) {
	scheduler := NewScheduler()
	defer scheduler.Stop()

	if scheduler == nil {
		t.Fatal("NewScheduler returned nil")
	}

	// Check that wheels are properly initialized
	if len(scheduler.wheels) != 4 {
		t.Errorf("Expected 4 wheels, got %d", len(scheduler.wheels))
	}

	// Check wheel configurations
	expectedConfigs := []struct {
		level     int
		slotCount int
		slotSize  time.Duration
	}{
		{0, 1000, time.Millisecond},
		{1, 60, time.Second},
		{2, 60, time.Minute},
		{3, 24, time.Hour},
	}

	for i, expected := range expectedConfigs {
		wheel := scheduler.wheels[i]
		if wheel.level != expected.level {
			t.Errorf("Wheel %d: expected level %d, got %d", i, expected.level, wheel.level)
		}
		if wheel.slotCount != expected.slotCount {
			t.Errorf("Wheel %d: expected slotCount %d, got %d", i, expected.slotCount, wheel.slotCount)
		}
		if wheel.slotSize != expected.slotSize {
			t.Errorf("Wheel %d: expected slotSize %v, got %v", i, expected.slotSize, wheel.slotSize)
		}
	}
}

func TestScheduleOneTimeOperation(t *testing.T) {
	scheduler := NewScheduler()
	defer scheduler.Stop()

	executed := make(chan bool, 1)
	handler := func(ctx context.Context, op *ScheduledOperation) error {
		executed <- true
		return nil
	}

	// Schedule operation to run in 100ms
	runAt := time.Now().Add(100 * time.Millisecond)
	schedule := &ScheduleConfig{RunAt: runAt}

	opID, err := scheduler.ScheduleOperation("test", "once", schedule, handler, nil)
	if err != nil {
		t.Fatalf("Failed to schedule operation: %v", err)
	}

	// Wait for execution
	select {
	case <-executed:
		// Success
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Operation did not execute within expected time")
	}

	// Check that operation was removed
	op, exists := scheduler.GetOperation(opID)
	if exists {
		t.Error("One-time operation should have been removed after execution")
	}
}

func TestScheduleRepeatingOperation(t *testing.T) {
	scheduler := NewScheduler()
	defer scheduler.Stop()

	executionCount := 0
	var mu sync.Mutex
	executed := make(chan bool, 3)

	handler := func(ctx context.Context, op *ScheduledOperation) error {
		mu.Lock()
		executionCount++
		count := executionCount
		mu.Unlock()

		if count <= 3 {
			executed <- true
		}
		return nil
	}

	// Schedule repeating operation every 50ms, max 3 runs
	schedule := &ScheduleConfig{
		Interval:    50 * time.Millisecond,
		RepeatCount: 3,
	}

	opID, err := scheduler.ScheduleOperation("test", "repeating", schedule, handler, nil)
	if err != nil {
		t.Fatalf("Failed to schedule operation: %v", err)
	}

	// Wait for 3 executions
	for i := 0; i < 3; i++ {
		select {
		case <-executed:
			// Success
		case <-time.After(200 * time.Millisecond):
			t.Fatalf("Operation %d did not execute within expected time", i+1)
		}
	}

	// Check that operation was removed after max runs
	time.Sleep(100 * time.Millisecond) // Wait for cleanup
	op, exists := scheduler.GetOperation(opID)
	if exists {
		t.Error("Repeating operation should have been removed after max runs")
	}

	if executionCount != 3 {
		t.Errorf("Expected 3 executions, got %d", executionCount)
	}
}

func TestCancelOperation(t *testing.T) {
	scheduler := NewScheduler()
	defer scheduler.Stop()

	executed := make(chan bool, 1)
	handler := func(ctx context.Context, op *ScheduledOperation) error {
		executed <- true
		return nil
	}

	// Schedule operation to run in 1 second
	runAt := time.Now().Add(1 * time.Second)
	schedule := &ScheduleConfig{RunAt: runAt}

	opID, err := scheduler.ScheduleOperation("test", "once", schedule, handler, nil)
	if err != nil {
		t.Fatalf("Failed to schedule operation: %v", err)
	}

	// Cancel the operation
	err = scheduler.CancelOperation(opID)
	if err != nil {
		t.Fatalf("Failed to cancel operation: %v", err)
	}

	// Wait for the scheduled time to pass
	time.Sleep(1 * time.Second)

	// Check that operation was not executed
	select {
	case <-executed:
		t.Fatal("Cancelled operation should not have executed")
	default:
		// Success - operation was cancelled
	}

	// Check that operation was removed
	op, exists := scheduler.GetOperation(opID)
	if exists {
		t.Error("Cancelled operation should have been removed")
	}
}

func TestSchedulerStats(t *testing.T) {
	scheduler := NewScheduler()
	defer scheduler.Stop()

	// Schedule and execute a few operations
	handler := func(ctx context.Context, op *ScheduledOperation) error {
		return nil
	}

	// Schedule 3 operations
	for i := 0; i < 3; i++ {
		runAt := time.Now().Add(50 * time.Millisecond)
		schedule := &ScheduleConfig{RunAt: runAt}
		_, err := scheduler.ScheduleOperation("test", "once", schedule, handler, nil)
		if err != nil {
			t.Fatalf("Failed to schedule operation: %v", err)
		}
	}

	// Wait for execution
	time.Sleep(100 * time.Millisecond)

	// Check stats
	stats := scheduler.GetStats()
	if stats.TotalOperations != 3 {
		t.Errorf("Expected 3 total operations, got %d", stats.TotalOperations)
	}
	if stats.CompletedOperations != 3 {
		t.Errorf("Expected 3 completed operations, got %d", stats.CompletedOperations)
	}
	if stats.ActiveOperations != 0 {
		t.Errorf("Expected 0 active operations, got %d", stats.ActiveOperations)
	}
}

func TestJitterInjection(t *testing.T) {
	scheduler := NewScheduler()
	defer scheduler.Stop()

	executionTimes := make([]time.Time, 0)
	var mu sync.Mutex
	executed := make(chan bool, 5)

	handler := func(ctx context.Context, op *ScheduledOperation) error {
		mu.Lock()
		executionTimes = append(executionTimes, time.Now())
		mu.Unlock()
		executed <- true
		return nil
	}

	// Schedule repeating operation every 100ms with 20% jitter
	schedule := &ScheduleConfig{
		Interval:      100 * time.Millisecond,
		JitterPercent: 0.2,
	}

	opID, err := scheduler.ScheduleOperation("test", "repeating", schedule, handler, nil)
	if err != nil {
		t.Fatalf("Failed to schedule operation: %v", err)
	}

	// Wait for 5 executions
	for i := 0; i < 5; i++ {
		select {
		case <-executed:
			// Success
		case <-time.After(200 * time.Millisecond):
			t.Fatalf("Operation %d did not execute within expected time", i+1)
		}
	}

	// Cancel to stop further executions
	scheduler.CancelOperation(opID)

	// Check that executions had jitter (not exactly 100ms apart)
	if len(executionTimes) < 3 {
		t.Skip("Need at least 3 executions to test jitter")
	}

	// Calculate intervals between executions
	intervals := make([]time.Duration, 0)
	for i := 1; i < len(executionTimes); i++ {
		interval := executionTimes[i].Sub(executionTimes[i-1])
		intervals = append(intervals, interval)
	}

	// Check that intervals vary (not all exactly 100ms)
	expectedInterval := 100 * time.Millisecond
	tolerance := 5 * time.Millisecond
	allSame := true

	for _, interval := range intervals {
		if abs(interval-expectedInterval) > tolerance {
			allSame = false
			break
		}
	}

	if allSame {
		t.Error("All execution intervals were the same, jitter may not be working")
	}
}

func TestSchedulerConcurrency(t *testing.T) {
	scheduler := NewScheduler()
	defer scheduler.Stop()

	const numOperations = 100
	const numGoroutines = 10

	var wg sync.WaitGroup
	errors := make(chan error, numOperations)

	// Start multiple goroutines scheduling operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < numOperations/numGoroutines; j++ {
				handler := func(ctx context.Context, op *ScheduledOperation) error {
					return nil
				}

				runAt := time.Now().Add(50 * time.Millisecond)
				schedule := &ScheduleConfig{RunAt: runAt}

				_, err := scheduler.ScheduleOperation("test", "once", schedule, handler, nil)
				if err != nil {
					errors <- err
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Error scheduling operation: %v", err)
	}

	// Wait for execution
	time.Sleep(100 * time.Millisecond)

	// Check stats
	stats := scheduler.GetStats()
	if stats.TotalOperations != int64(numOperations) {
		t.Errorf("Expected %d total operations, got %d", numOperations, stats.TotalOperations)
	}
}

func TestSchedulerShutdown(t *testing.T) {
	scheduler := NewScheduler()

	// Schedule a long-running operation
	executed := make(chan bool, 1)
	handler := func(ctx context.Context, op *ScheduledOperation) error {
		// Simulate long-running operation
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
			executed <- true
			return nil
		}
	}

	runAt := time.Now().Add(10 * time.Millisecond)
	schedule := &ScheduleConfig{RunAt: runAt}

	_, err := scheduler.ScheduleOperation("test", "once", schedule, handler, nil)
	if err != nil {
		t.Fatalf("Failed to schedule operation: %v", err)
	}

	// Shutdown with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = scheduler.Shutdown(ctx)
	if err != context.DeadlineExceeded {
		t.Errorf("Expected DeadlineExceeded error, got %v", err)
	}

	// Check that operation was cancelled
	select {
	case <-executed:
		t.Fatal("Operation should not have completed after shutdown")
	default:
		// Success - operation was cancelled
	}
}

// Benchmark tests for performance validation

func BenchmarkSchedulerSchedule(b *testing.B) {
	scheduler := NewScheduler()
	defer scheduler.Stop()

	handler := func(ctx context.Context, op *ScheduledOperation) error {
		return nil
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			runAt := time.Now().Add(100 * time.Millisecond)
			schedule := &ScheduleConfig{RunAt: runAt}
			_, err := scheduler.ScheduleOperation("test", "once", schedule, handler, nil)
			if err != nil {
				b.Fatalf("Failed to schedule operation: %v", err)
			}
		}
	})
}

func BenchmarkSchedulerTick(b *testing.B) {
	scheduler := NewScheduler()
	defer scheduler.Stop()

	// Pre-schedule operations
	handler := func(ctx context.Context, op *ScheduledOperation) error {
		return nil
	}

	for i := 0; i < 1000; i++ {
		runAt := time.Now().Add(time.Duration(i) * time.Millisecond)
		schedule := &ScheduleConfig{RunAt: runAt}
		_, err := scheduler.ScheduleOperation("test", "once", schedule, handler, nil)
		if err != nil {
			b.Fatalf("Failed to schedule operation: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scheduler.tick()
	}
}

func BenchmarkSchedulerConcurrent(b *testing.B) {
	scheduler := NewScheduler()
	defer scheduler.Stop()

	handler := func(ctx context.Context, op *ScheduledOperation) error {
		return nil
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			runAt := time.Now().Add(100 * time.Millisecond)
			schedule := &ScheduleConfig{RunAt: runAt}
			_, err := scheduler.ScheduleOperation("test", "once", schedule, handler, nil)
			if err != nil {
				b.Fatalf("Failed to schedule operation: %v", err)
			}
		}
	})
}

// Helper function
func abs(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
