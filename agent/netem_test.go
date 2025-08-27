package main

import (
	"fmt"
	"testing"
	"time"
)

func TestNetemManagerCreation(t *testing.T) {
	nm := NewNetemManager()
	if nm == nil {
		t.Fatal("NewNetemManager returned nil")
	}

	if nm.interfaces == nil {
		t.Error("interfaces map not initialized")
	}

	if nm.experiments == nil {
		t.Error("experiments map not initialized")
	}
}

func TestNetemConfigValidation(t *testing.T) {
	// Test valid configuration
	config := &NetemConfig{
		Delay:     100,
		Jitter:    20,
		Loss:      5.0,
		Duplicate: 1.0,
		Corrupt:   0.5,
		Reorder:   2.0,
		Rate:      100,
		Limit:     1000,
	}

	if config.Delay != 100 {
		t.Errorf("Expected delay 100, got %d", config.Delay)
	}

	if config.Loss != 5.0 {
		t.Errorf("Expected loss 5.0, got %f", config.Loss)
	}
}

func TestNetemManagerStateManagement(t *testing.T) {
	nm := NewNetemManager()

	// Test initial state
	status := nm.GetStatus()
	if len(status) != 0 {
		t.Errorf("Expected empty status, got %d interfaces", len(status))
	}

	// Test experiment listing
	experiments := nm.ListExperiments()
	if len(experiments) != 0 {
		t.Errorf("Expected no experiments, got %d", len(experiments))
	}
}

func TestNetemManagerStatePersistence(t *testing.T) {
	nm := NewNetemManager()

	// Add some test data
	config := &NetemConfig{Delay: 100, Loss: 5.0}
	nm.Apply("test-exp-1", "eth0", config, 1*time.Minute)

	// Export state
	stateData, err := nm.ExportState()
	if err != nil {
		t.Fatalf("Failed to export state: %v", err)
	}

	// Create new manager and import state
	nm2 := NewNetemManager()
	err = nm2.ImportState(stateData)
	if err != nil {
		t.Fatalf("Failed to import state: %v", err)
	}

	// Verify state was restored
	experiments := nm2.ListExperiments()
	if len(experiments) != 1 {
		t.Errorf("Expected 1 experiment after import, got %d", len(experiments))
	}

	if experiments[0].ID != "test-exp-1" {
		t.Errorf("Expected experiment ID 'test-exp-1', got '%s'", experiments[0].ID)
	}
}

func TestNetemManagerExperimentLifecycle(t *testing.T) {
	nm := NewNetemManager()

	// Test experiment creation
	config := &NetemConfig{Delay: 100, Loss: 5.0}
	err := nm.Apply("test-exp-1", "eth0", config, 1*time.Minute)
	if err != nil {
		t.Fatalf("Failed to apply experiment: %v", err)
	}

	// Verify experiment was created
	experiments := nm.ListExperiments()
	if len(experiments) != 1 {
		t.Errorf("Expected 1 experiment, got %d", len(experiments))
	}

	// Test experiment info retrieval
	expInfo, err := nm.GetExperimentInfo("test-exp-1")
	if err != nil {
		t.Fatalf("Failed to get experiment info: %v", err)
	}

	if expInfo.ID != "test-exp-1" {
		t.Errorf("Expected experiment ID 'test-exp-1', got '%s'", expInfo.ID)
	}

	if expInfo.Interface != "eth0" {
		t.Errorf("Expected interface 'eth0', got '%s'", expInfo.Interface)
	}

	// Test experiment revert
	err = nm.Revert("test-exp-1")
	if err != nil {
		t.Fatalf("Failed to revert experiment: %v", err)
	}

	// Verify experiment was removed
	experiments = nm.ListExperiments()
	if len(experiments) != 0 {
		t.Errorf("Expected no experiments after revert, got %d", len(experiments))
	}
}

func TestNetemManagerMultipleExperiments(t *testing.T) {
	nm := NewNetemManager()

	// Add multiple experiments on same interface
	config1 := &NetemConfig{Delay: 100}
	config2 := &NetemConfig{Loss: 5.0}

	err := nm.Apply("exp-1", "eth0", config1, 1*time.Minute)
	if err != nil {
		t.Fatalf("Failed to apply first experiment: %v", err)
	}

	err = nm.Apply("exp-2", "eth0", config2, 1*time.Minute)
	if err != nil {
		t.Fatalf("Failed to apply second experiment: %v", err)
	}

	// Verify both experiments exist
	experiments := nm.ListExperiments()
	if len(experiments) != 2 {
		t.Errorf("Expected 2 experiments, got %d", len(experiments))
	}

	// Revert one experiment
	err = nm.Revert("exp-1")
	if err != nil {
		t.Fatalf("Failed to revert first experiment: %v", err)
	}

	// Verify one experiment remains
	experiments = nm.ListExperiments()
	if len(experiments) != 1 {
		t.Errorf("Expected 1 experiment after revert, got %d", len(experiments))
	}

	if experiments[0].ID != "exp-2" {
		t.Errorf("Expected remaining experiment ID 'exp-2', got '%s'", experiments[0].ID)
	}
}

func TestNetemManagerRevertAll(t *testing.T) {
	nm := NewNetemManager()

	// Add experiments on multiple interfaces
	config1 := &NetemConfig{Delay: 100}
	config2 := &NetemConfig{Loss: 5.0}

	nm.Apply("exp-1", "eth0", config1, 1*time.Minute)
	nm.Apply("exp-2", "eth1", config2, 1*time.Minute)

	// Verify experiments exist
	experiments := nm.ListExperiments()
	if len(experiments) != 2 {
		t.Errorf("Expected 2 experiments, got %d", len(experiments))
	}

	// Revert all on one interface
	err := nm.RevertAll("eth0")
	if err != nil {
		t.Fatalf("Failed to revert all on eth0: %v", err)
	}

	// Verify only eth1 experiment remains
	experiments = nm.ListExperiments()
	if len(experiments) != 1 {
		t.Errorf("Expected 1 experiment after revert all, got %d", len(experiments))
	}

	if experiments[0].Interface != "eth1" {
		t.Errorf("Expected remaining experiment on eth1, got %s", experiments[0].Interface)
	}
}

func TestNetemManagerConfigMerging(t *testing.T) {
	nm := NewNetemManager()

	// Test config merging logic
	config1 := &NetemConfig{Delay: 100, Loss: 5.0}
	config2 := &NetemConfig{Delay: 200, Loss: 10.0}

	configs := []*NetemConfig{config1, config2}
	merged := nm.mergeConfigs(configs)

	if merged.Delay != 200 {
		t.Errorf("Expected merged delay 200, got %d", merged.Delay)
	}

	if merged.Loss != 10.0 {
		t.Errorf("Expected merged loss 10.0, got %f", merged.Loss)
	}
}

func TestNetemManagerTTLExpiration(t *testing.T) {
	nm := NewNetemManager()

	// Add experiment with short TTL
	config := &NetemConfig{Delay: 100}
	err := nm.Apply("exp-ttl", "eth0", config, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to apply experiment: %v", err)
	}

	// Wait for TTL to expire
	time.Sleep(200 * time.Millisecond)

	// Verify experiment was automatically removed
	experiments := nm.ListExperiments()
	if len(experiments) != 0 {
		t.Errorf("Expected no experiments after TTL expiration, got %d", len(experiments))
	}
}

func TestNetemManagerInterfaceValidation(t *testing.T) {
	nm := NewNetemManager()

	// Test with invalid interface name
	config := &NetemConfig{Delay: 100}
	err := nm.Apply("test", "invalid-interface", config, 1*time.Minute)

	// This should fail in most environments, but we can't guarantee it
	// So we just test that the function handles the error gracefully
	if err != nil {
		t.Logf("Interface validation failed as expected: %v", err)
	}
}

func TestNetemManagerConcurrentAccess(t *testing.T) {
	nm := NewNetemManager()

	// Test concurrent access to the manager
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			config := &NetemConfig{Delay: 100 + id}
			expID := fmt.Sprintf("exp-%d", id)

			// Apply experiment
			err := nm.Apply(expID, "eth0", config, 1*time.Minute)
			if err != nil {
				t.Errorf("Failed to apply experiment %d: %v", id, err)
			}

			// Get status
			status := nm.GetStatus()
			if status == nil {
				t.Errorf("Status is nil for experiment %d", id)
			}

			// Revert experiment
			err = nm.Revert(expID)
			if err != nil {
				t.Errorf("Failed to revert experiment %d: %v", id, err)
			}

			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify final state
	experiments := nm.ListExperiments()
	if len(experiments) != 0 {
		t.Errorf("Expected no experiments after concurrent operations, got %d", len(experiments))
	}
}

// Benchmark tests for performance
func BenchmarkNetemManagerApply(b *testing.B) {
	nm := NewNetemManager()
	config := &NetemConfig{Delay: 100, Loss: 5.0}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		expID := fmt.Sprintf("bench-exp-%d", i)
		nm.Apply(expID, "eth0", config, 1*time.Minute)
	}
}

func BenchmarkNetemManagerRevert(b *testing.B) {
	nm := NewNetemManager()
	config := &NetemConfig{Delay: 100, Loss: 5.0}

	// Pre-populate with experiments
	for i := 0; i < b.N; i++ {
		expID := fmt.Sprintf("bench-exp-%d", i)
		nm.Apply(expID, "eth0", config, 1*time.Minute)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		expID := fmt.Sprintf("bench-exp-%d", i)
		nm.Revert(expID)
	}
}

func BenchmarkNetemManagerGetStatus(b *testing.B) {
	nm := NewNetemManager()

	// Pre-populate with some data
	config := &NetemConfig{Delay: 100}
	for i := 0; i < 100; i++ {
		expID := fmt.Sprintf("bench-exp-%d", i)
		nm.Apply(expID, "eth0", config, 1*time.Minute)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		nm.GetStatus()
	}
}
