package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewExperimentValidator(t *testing.T) {
	validator := NewExperimentValidator()
	if validator == nil {
		t.Fatal("Expected validator to be created")
	}

	if len(validator.schemas) != 2 {
		t.Errorf("Expected 2 schemas, got %d", len(validator.schemas))
	}

	// Check if network schema exists
	if validator.schemas["network"] == nil {
		t.Error("Network schema not found")
	}

	// Check if system schema exists
	if validator.schemas["system"] == nil {
		t.Error("System schema not found")
	}
}

func TestValidateNetworkExperiment(t *testing.T) {
	validator := NewExperimentValidator()

	// Valid network experiment
	validExp := &ExperimentRequest{
		Name:           "test-network",
		Description:    "Test network latency",
		ExperimentType: "network-latency",
		Target:         "eth0",
		Duration:       60,
		DelayMs:        100,
	}

	result, err := validator.ValidateExperiment(validExp)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !result.Valid {
		t.Errorf("Expected valid experiment, got errors: %v", result.Errors)
	}

	// Invalid network experiment - missing required fields
	invalidExp := &ExperimentRequest{
		Name:           "test-network",
		ExperimentType: "network-latency",
		// Missing Target and Duration
	}

	result, err = validator.ValidateExperiment(invalidExp)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Valid {
		t.Error("Expected invalid experiment")
	}

	if len(result.Errors) == 0 {
		t.Error("Expected validation errors")
	}
}

func TestValidateSystemExperiment(t *testing.T) {
	validator := NewExperimentValidator()

	// Valid CPU stress experiment
	validExp := &ExperimentRequest{
		Name:           "test-cpu",
		Description:    "Test CPU stress",
		ExperimentType: "cpu-stress",
		Target:         "localhost",
		Duration:       120,
		CPUWorkers:     4,
	}

	result, err := validator.ValidateExperiment(validExp)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !result.Valid {
		t.Errorf("Expected valid experiment, got errors: %v", result.Errors)
	}

	// Valid memory stress experiment
	validMemExp := &ExperimentRequest{
		Name:           "test-mem",
		Description:    "Test memory stress",
		ExperimentType: "mem-stress",
		Target:         "localhost",
		Duration:       60,
		MemSizeMB:      512,
	}

	result, err = validator.ValidateExperiment(validMemExp)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !result.Valid {
		t.Errorf("Expected valid experiment, got errors: %v", result.Errors)
	}
}

func TestValidateFieldConstraints(t *testing.T) {
	validator := NewExperimentValidator()

	// Test duration constraints
	exp := &ExperimentRequest{
		Name:           "test-constraints",
		ExperimentType: "network-latency",
		Target:         "eth0",
		Duration:       0, // Invalid: too low
	}

	result, err := validator.ValidateExperiment(exp)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Valid {
		t.Error("Expected invalid experiment due to duration constraint")
	}

	// Test delay constraints
	exp.Duration = 60
	exp.DelayMs = 15000 // Invalid: too high

	result, err = validator.ValidateExperiment(exp)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Valid {
		t.Error("Expected invalid experiment due to delay constraint")
	}

	// Test CPU workers constraints
	exp.DelayMs = 100
	exp.ExperimentType = "cpu-stress"
	exp.CPUWorkers = 50 // Invalid: too high

	result, err = validator.ValidateExperiment(exp)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Valid {
		t.Error("Expected invalid experiment due to CPU workers constraint")
	}
}

func TestValidateBusinessLogic(t *testing.T) {
	validator := NewExperimentValidator()

	// Test start time in the past
	pastTime := time.Now().Add(-1 * time.Hour)
	exp := &ExperimentRequest{
		Name:           "test-past-time",
		ExperimentType: "network-latency",
		Target:         "eth0",
		Duration:       60,
		StartTime:      pastTime,
	}

	result, err := validator.ValidateExperiment(exp)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !result.Valid {
		t.Errorf("Expected valid experiment, got errors: %v", result.Errors)
	}

	if len(result.Warnings) == 0 {
		t.Error("Expected warning about past start time")
	}

	// Test parallel execution without agent count
	exp.StartTime = time.Time{} // Reset
	exp.Parallel = true
	exp.AgentCount = 0

	result, err = validator.ValidateExperiment(exp)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !result.Valid {
		t.Errorf("Expected valid experiment, got errors: %v", result.Errors)
	}

	if len(result.Warnings) == 0 {
		t.Error("Expected warning about parallel execution without agent count")
	}
}

func TestGetSchema(t *testing.T) {
	validator := NewExperimentValidator()

	// Test network schema
	schema, err := validator.GetSchema("network-latency")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if schema == nil {
		t.Fatal("Expected schema to be returned")
	}

	if schema.Type != "object" {
		t.Errorf("Expected object type, got %s", schema.Type)
	}

	// Test system schema
	schema, err = validator.GetSchema("cpu-stress")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if schema == nil {
		t.Fatal("Expected schema to be returned")
	}

	// Test unknown schema
	_, err = validator.GetSchema("unknown-type")
	if err == nil {
		t.Error("Expected error for unknown schema type")
	}
}

func TestGetSchemaJSON(t *testing.T) {
	validator := NewExperimentValidator()

	jsonStr, err := validator.GetSchemaJSON("network-latency")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if jsonStr == "" {
		t.Error("Expected JSON string to be returned")
	}

	// Verify it's valid JSON by parsing it back
	var schema ExperimentSchema
	if err := json.Unmarshal([]byte(jsonStr), &schema); err != nil {
		t.Errorf("Generated JSON is invalid: %v", err)
	}
}

func TestListSupportedTypes(t *testing.T) {
	validator := NewExperimentValidator()

	types := validator.ListSupportedTypes()
	if len(types) == 0 {
		t.Error("Expected supported types to be returned")
	}

	// Check for expected network types
	expectedNetworkTypes := []string{"network-latency", "network-loss", "network-corruption", "network-duplication"}
	for _, expectedType := range expectedNetworkTypes {
		found := false
		for _, actualType := range types {
			if actualType == expectedType {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected network type %s not found", expectedType)
		}
	}

	// Check for expected system types
	expectedSystemTypes := []string{"cpu-stress", "mem-stress", "io-stress", "process-kill"}
	for _, expectedType := range expectedSystemTypes {
		found := false
		for _, actualType := range types {
			if actualType == expectedType {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected system type %s not found", expectedType)
		}
	}
}

func TestValidateExperimentWithDefaults(t *testing.T) {
	validator := NewExperimentValidator()

	// Test experiment with minimal required fields
	minimalExp := &ExperimentRequest{
		ExperimentType: "network-latency",
		Target:         "eth0",
		Duration:       30,
	}

	result, err := validator.ValidateExperiment(minimalExp)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !result.Valid {
		t.Errorf("Expected valid experiment, got errors: %v", result.Errors)
	}

	// Test experiment with all optional fields
	fullExp := &ExperimentRequest{
		Name:           "comprehensive-test",
		Description:    "A comprehensive test with all fields",
		ExperimentType: "cpu-stress",
		Target:         "localhost",
		Duration:       120,
		CPUWorkers:     8,
		StartTime:      time.Now().Add(1 * time.Hour),
		Parallel:       true,
		AgentCount:     3,
	}

	result, err = validator.ValidateExperiment(fullExp)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !result.Valid {
		t.Errorf("Expected valid experiment, got errors: %v", result.Errors)
	}
}

func BenchmarkValidateExperiment(b *testing.B) {
	validator := NewExperimentValidator()
	exp := &ExperimentRequest{
		Name:           "benchmark-test",
		Description:    "Benchmark test",
		ExperimentType: "network-latency",
		Target:         "eth0",
		Duration:       60,
		DelayMs:        100,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := validator.ValidateExperiment(exp)
		if err != nil {
			b.Fatalf("Unexpected error: %v", err)
		}
	}
}

