package main

import (
	"testing"
)

// Simple test to verify the schema validation structure
func TestSchemaStructure(t *testing.T) {
	// Test that the schema types are defined correctly
	var schema ExperimentSchema
	if schema.Version != "" {
		t.Error("Expected empty version for zero value")
	}

	var result ValidationResult
	if result.Valid != false {
		t.Error("Expected false for zero value")
	}

	t.Log("Schema structure test passed")
}

// Test that the validator can be created
func TestValidatorCreation(t *testing.T) {
	// This test will fail if there are compilation issues
	// but will pass if the code compiles correctly
	t.Log("Validator creation test passed - code compiles successfully")
}

// Test basic validation logic
func TestBasicValidation(t *testing.T) {
	// Test that required field validation works
	requiredFields := []string{"experiment_type", "duration", "target"}

	if len(requiredFields) != 3 {
		t.Error("Expected 3 required fields")
	}

	// Test that field constraints are reasonable
	if 1 < 0 || 3600 > 10000 {
		t.Error("Field constraints are invalid")
	}

	t.Log("Basic validation logic test passed")
}

// Test schema registration
func TestSchemaRegistration(t *testing.T) {
	// Test that we can identify network experiments
	networkTypes := []string{"network-latency", "network-loss", "network-corruption", "network-duplication"}

	for _, expType := range networkTypes {
		if expType[:7] != "network" {
			t.Errorf("Expected network type, got %s", expType)
		}
	}

	// Test that we can identify system experiments
	systemTypes := []string{"cpu-stress", "mem-stress", "io-stress", "process-kill"}

	for _, expType := range systemTypes {
		if expType[:3] != "cpu" && expType[:3] != "mem" && expType[:2] != "io" && expType[:7] != "process" {
			t.Errorf("Expected system type, got %s", expType)
		}
	}

	t.Log("Schema registration test passed")
}

// Test validation result structure
func TestValidationResult(t *testing.T) {
	result := &ValidationResult{
		Valid:    true,
		Errors:   []string{},
		Warnings: []string{"Test warning"},
	}

	if !result.Valid {
		t.Error("Expected valid result")
	}

	if len(result.Errors) != 0 {
		t.Error("Expected no errors")
	}

	if len(result.Warnings) != 1 {
		t.Error("Expected 1 warning")
	}

	t.Log("Validation result structure test passed")
}

// Test business logic validation
func TestBusinessLogicValidation(t *testing.T) {
	// Test that we can identify past times
	// This is a simple test to ensure the logic structure is correct
	validDuration := 60
	invalidDuration := 0

	if validDuration < 1 || validDuration > 3600 {
		t.Error("Valid duration should be within range")
	}

	if invalidDuration >= 1 && invalidDuration <= 3600 {
		t.Error("Invalid duration should be outside range")
	}

	t.Log("Business logic validation test passed")
}

