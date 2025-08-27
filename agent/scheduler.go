package main

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// Scheduler manages scheduled operations using a hierarchical timing wheel
type Scheduler struct {
	mu         sync.RWMutex
	wheels     []*TimingWheel
	operations map[string]*ScheduledOperation
	nextOpID   int64
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	stats      *SchedulerStats
}

// TimingWheel represents a level in the hierarchical timing wheel
type TimingWheel struct {
	level       int           // Level in the hierarchy (0 = highest precision)
	slotCount   int           // Number of slots in this wheel
	slotSize    time.Duration // Duration of each slot
	currentSlot int           // Current slot position
	slots       [][]*ScheduledOperation
	next        *TimingWheel // Next level wheel
}

// ScheduledOperation represents a scheduled task
type ScheduledOperation struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Type     string                 `json:"type"` // "once", "repeating", "cron"
	Schedule *ScheduleConfig        `json:"schedule"`
	Handler  OperationHandler       `json:"-"`
	Metadata map[string]interface{} `json:"metadata"`
	Created  time.Time              `json:"created"`
	NextRun  time.Time              `json:"next_run"`
	LastRun  time.Time              `json:"last_run"`
	RunCount int64                  `json:"run_count"`
	MaxRuns  int64                  `json:"max_runs"`
	Jitter   time.Duration          `json:"jitter"`
	Priority int                    `json:"priority"`
	index    int                    // For heap operations
}

// ScheduleConfig holds scheduling configuration
type ScheduleConfig struct {
	// For one-time operations
	RunAt time.Time `json:"run_at,omitempty"`

	// For repeating operations
	Interval    time.Duration `json:"interval,omitempty"`
	RepeatCount int64         `json:"repeat_count,omitempty"`

	// For cron-like operations
	CronExpression string `json:"cron_expression,omitempty"`

	// Jitter configuration
	JitterPercent float64 `json:"jitter_percent,omitempty"` // 0.0 to 1.0
}

// OperationHandler is the function type for scheduled operations
type OperationHandler func(ctx context.Context, op *ScheduledOperation) error

// SchedulerStats holds scheduler performance metrics
type SchedulerStats struct {
	mu                  sync.RWMutex
	TotalOperations     int64         `json:"total_operations"`
	ActiveOperations    int64         `json:"active_operations"`
	CompletedOperations int64         `json:"completed_operations"`
	FailedOperations    int64         `json:"failed_operations"`
	AverageLatency      time.Duration `json:"average_latency"`
	LastTickTime        time.Time     `json:"last_tick_time"`
	TickCount           int64         `json:"tick_count"`
	DriftTotal          time.Duration `json:"drift_total"`
	MaxDrift            time.Duration `json:"max_drift"`
}

// NewScheduler creates a new hierarchical timing wheel scheduler
func NewScheduler() *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())

	s := &Scheduler{
		operations: make(map[string]*ScheduledOperation),
		ctx:        ctx,
		cancel:     cancel,
		stats:      &SchedulerStats{},
	}

	// Create hierarchical timing wheels
	// Level 0: 1ms precision, 1000 slots (1 second)
	// Level 1: 1s precision, 60 slots (1 minute)
	// Level 2: 1m precision, 60 slots (1 hour)
	// Level 3: 1h precision, 24 slots (1 day)
	s.wheels = []*TimingWheel{
		{level: 0, slotCount: 1000, slotSize: time.Millisecond},
		{level: 1, slotCount: 60, slotSize: time.Second},
		{level: 2, slotCount: 60, slotSize: time.Minute},
		{level: 3, slotCount: 24, slotSize: time.Hour},
	}

	// Initialize slots and link wheels
	for i, wheel := range s.wheels {
		wheel.slots = make([][]*ScheduledOperation, wheel.slotCount)
		for j := range wheel.slots {
			wheel.slots[j] = make([]*ScheduledOperation, 0)
		}

		if i < len(s.wheels)-1 {
			wheel.next = s.wheels[i+1]
		}
	}

	// Start the ticker
	s.startTicker()

	return s
}

// startTicker starts the main scheduler loop
func (s *Scheduler) startTicker() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:
				s.tick()
			}
		}
	}()
}

// tick advances the timing wheel and executes due operations
func (s *Scheduler) tick() {
	start := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Advance level 0 wheel
	s.advanceWheel(s.wheels[0])

	// Update stats
	s.stats.mu.Lock()
	s.stats.TickCount++
	s.stats.LastTickTime = time.Now()

	// Calculate drift
	drift := time.Since(start) - time.Millisecond
	s.stats.DriftTotal += drift
	if drift > s.stats.MaxDrift {
		s.stats.MaxDrift = drift
	}
	s.stats.mu.Unlock()
}

// advanceWheel advances a timing wheel and cascades operations
func (s *Scheduler) advanceWheel(wheel *TimingWheel) {
	wheel.currentSlot = (wheel.currentSlot + 1) % wheel.slotCount

	// Execute operations in current slot
	slot := wheel.slots[wheel.currentSlot]
	for _, op := range slot {
		if op.NextRun.Before(time.Now()) {
			s.executeOperation(op)
		}
	}

	// Clear the slot
	wheel.slots[wheel.currentSlot] = wheel.slots[wheel.currentSlot][:0]

	// Cascade to next level if this wheel completed a cycle
	if wheel.currentSlot == 0 && wheel.next != nil {
		s.advanceWheel(wheel.next)
	}
}

// executeOperation executes a scheduled operation
func (s *Scheduler) executeOperation(op *ScheduledOperation) {
	// Apply jitter if configured
	actualRunTime := op.NextRun
	if op.Jitter > 0 {
		jitter := time.Duration(rand.Int63n(int64(op.Jitter)))
		actualRunTime = actualRunTime.Add(jitter)
	}

	// Execute the operation
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
		defer cancel()

		start := time.Now()
		err := op.Handler(ctx, op)
		latency := time.Since(start)

		// Update stats
		s.stats.mu.Lock()
		s.stats.CompletedOperations++
		if err != nil {
			s.stats.FailedOperations++
		}
		// Update average latency using exponential moving average
		if s.stats.AverageLatency == 0 {
			s.stats.AverageLatency = latency
		} else {
			s.stats.AverageLatency = (s.stats.AverageLatency*9 + latency) / 10
		}
		s.stats.mu.Unlock()

		// Update operation state
		s.mu.Lock()
		op.LastRun = time.Now()
		op.RunCount++

		// Schedule next run if repeating
		if op.Type == "repeating" && (op.MaxRuns == 0 || op.RunCount < op.MaxRuns) {
			op.NextRun = op.NextRun.Add(op.Schedule.Interval)
			s.scheduleOperation(op)
		} else if op.Type == "cron" {
			// Parse cron expression and calculate next run
			nextRun, err := s.parseCronExpression(op.Schedule.CronExpression, op.NextRun)
			if err == nil {
				op.NextRun = nextRun
				s.scheduleOperation(op)
			}
		} else {
			// One-time operation completed, remove it
			delete(s.operations, op.ID)
			s.stats.mu.Lock()
			s.stats.ActiveOperations--
			s.stats.mu.Unlock()
		}
		s.mu.Unlock()
	}()
}

// ScheduleOperation schedules a new operation
func (s *Scheduler) ScheduleOperation(name, opType string, schedule *ScheduleConfig, handler OperationHandler, metadata map[string]interface{}) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	opID := fmt.Sprintf("op_%d", s.nextOpID)
	s.nextOpID++

	op := &ScheduledOperation{
		ID:       opID,
		Name:     name,
		Type:     opType,
		Schedule: schedule,
		Handler:  handler,
		Metadata: metadata,
		Created:  time.Now(),
		Priority: 0,
	}

	// Calculate next run time
	switch opType {
	case "once":
		op.NextRun = schedule.RunAt
	case "repeating":
		op.NextRun = time.Now().Add(schedule.Interval)
	case "cron":
		nextRun, err := s.parseCronExpression(schedule.CronExpression, time.Now())
		if err != nil {
			return "", fmt.Errorf("invalid cron expression: %w", err)
		}
		op.NextRun = nextRun
	default:
		return "", fmt.Errorf("unsupported operation type: %s", opType)
	}

	// Apply jitter if configured
	if schedule.JitterPercent > 0 {
		interval := op.NextRun.Sub(time.Now())
		jitter := time.Duration(float64(interval) * schedule.JitterPercent * rand.Float64())
		op.NextRun = op.NextRun.Add(jitter)
		op.Jitter = jitter
	}

	// Store operation
	s.operations[opID] = op
	s.stats.mu.Lock()
	s.stats.TotalOperations++
	s.stats.ActiveOperations++
	s.stats.mu.Unlock()

	// Schedule the operation
	s.scheduleOperation(op)

	return opID, nil
}

// scheduleOperation adds an operation to the appropriate timing wheel
func (s *Scheduler) scheduleOperation(op *ScheduledOperation) {
	// Find the appropriate wheel level
	var targetWheel *TimingWheel
	delay := op.NextRun.Sub(time.Now())

	for _, wheel := range s.wheels {
		if delay < time.Duration(wheel.slotCount)*wheel.slotSize {
			targetWheel = wheel
			break
		}
	}

	if targetWheel == nil {
		// Operation is too far in the future, use the highest level wheel
		targetWheel = s.wheels[len(s.wheels)-1]
	}

	// Calculate slot
	slotIndex := int(delay/targetWheel.slotSize) % targetWheel.slotCount
	targetWheel.slots[slotIndex] = append(targetWheel.slots[slotIndex], op)
}

// CancelOperation cancels a scheduled operation
func (s *Scheduler) CancelOperation(opID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	op, exists := s.operations[opID]
	if !exists {
		return fmt.Errorf("operation %s not found", opID)
	}

	// Remove from all wheels
	for _, wheel := range s.wheels {
		for _, slot := range wheel.slots {
			for i, scheduledOp := range slot {
				if scheduledOp.ID == opID {
					slot = append(slot[:i], slot[i+1:]...)
					break
				}
			}
		}
	}

	delete(s.operations, opID)
	s.stats.mu.Lock()
	s.stats.ActiveOperations--
	s.stats.mu.Unlock()

	return nil
}

// GetOperation retrieves a scheduled operation
func (s *Scheduler) GetOperation(opID string) (*ScheduledOperation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	op, exists := s.operations[opID]
	return op, exists
}

// ListOperations returns all scheduled operations
func (s *Scheduler) ListOperations() []*ScheduledOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ops := make([]*ScheduledOperation, 0, len(s.operations))
	for _, op := range s.operations {
		ops = append(ops, op)
	}
	return ops
}

// GetStats returns scheduler performance statistics
func (s *Scheduler) GetStats() *SchedulerStats {
	s.stats.mu.RLock()
	defer s.stats.mu.RUnlock()

	// Calculate average drift
	var avgDrift time.Duration
	if s.stats.TickCount > 0 {
		avgDrift = s.stats.DriftTotal / time.Duration(s.stats.TickCount)
	}

	return &SchedulerStats{
		TotalOperations:     s.stats.TotalOperations,
		ActiveOperations:    s.stats.ActiveOperations,
		CompletedOperations: s.stats.CompletedOperations,
		FailedOperations:    s.stats.FailedOperations,
		AverageLatency:      s.stats.AverageLatency,
		LastTickTime:        s.stats.LastTickTime,
		TickCount:           s.stats.TickCount,
		DriftTotal:          s.stats.DriftTotal,
		MaxDrift:            s.stats.MaxDrift,
	}
}

// parseCronExpression parses a simple cron expression (minute hour day month weekday)
func (s *Scheduler) parseCronExpression(cronExpr string, from time.Time) (time.Time, error) {
	// Simple cron parser - supports basic patterns like "*/5 * * * *"
	// This is a simplified version; for production use a proper cron library

	// For now, return a basic implementation
	// TODO: Implement proper cron parsing
	return from.Add(time.Minute), nil
}

// Stop stops the scheduler and waits for all operations to complete
func (s *Scheduler) Stop() {
	s.cancel()
	s.wg.Wait()
}

// Shutdown gracefully shuts down the scheduler
func (s *Scheduler) Shutdown(ctx context.Context) error {
	s.cancel()

	// Wait for completion or context cancellation
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
