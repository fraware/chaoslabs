package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"
)

// ExperimentSchema defines the structure for experiment validation
type ExperimentSchema struct {
	Version     string                 `json:"version"`
	Type        string                 `json:"type"`
	Required    []string               `json:"required"`
	Properties  map[string]interface{} `json:"properties"`
	Definitions map[string]interface{} `json:"definitions,omitempty"`
}

// ExperimentValidator handles validation of experiment requests
type ExperimentValidator struct {
	schemas map[string]*ExperimentSchema
}

// NewExperimentValidator creates a new validator with built-in schemas
func NewExperimentValidator() *ExperimentValidator {
	validator := &ExperimentValidator{
		schemas: make(map[string]*ExperimentSchema),
	}

	// Register built-in schemas
	validator.registerBuiltinSchemas()

	return validator
}

// registerBuiltinSchemas registers the built-in experiment schemas
func (ev *ExperimentValidator) registerBuiltinSchemas() {
	// Network experiment schema
	ev.schemas["network"] = &ExperimentSchema{
		Version:  "1.0.0",
		Type:     "object",
		Required: []string{"experiment_type", "duration", "target"},
		Properties: map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Unique name for the experiment",
				"minLength":   1,
				"maxLength":   100,
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "Description of the experiment",
				"maxLength":   500,
			},
			"experiment_type": map[string]interface{}{
				"type":        "string",
				"description": "Type of network experiment",
				"enum":        []string{"network-latency", "network-loss", "network-corruption", "network-duplication"},
			},
			"target": map[string]interface{}{
				"type":        "string",
				"description": "Target network interface or host",
				"minLength":   1,
			},
			"duration": map[string]interface{}{
				"type":        "integer",
				"description": "Duration in seconds",
				"minimum":     1,
				"maximum":     3600,
			},
			"delay_ms": map[string]interface{}{
				"type":        "integer",
				"description": "Network latency in milliseconds",
				"minimum":     1,
				"maximum":     10000,
			},
			"loss_percent": map[string]interface{}{
				"type":        "integer",
				"description": "Packet loss percentage",
				"minimum":     0,
				"maximum":     100,
			},
			"start_time": map[string]interface{}{
				"type":        "string",
				"description": "Scheduled start time (RFC3339)",
				"format":      "date-time",
			},
			"parallel": map[string]interface{}{
				"type":        "boolean",
				"description": "Run in parallel across multiple agents",
			},
			"agent_count": map[string]interface{}{
				"type":        "integer",
				"description": "Number of agents to target",
				"minimum":     1,
				"maximum":     100,
			},
		},
	}

	// System stress experiment schema
	ev.schemas["system"] = &ExperimentSchema{
		Version:  "1.0.0",
		Type:     "object",
		Required: []string{"experiment_type", "duration", "target"},
		Properties: map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Unique name for the experiment",
				"minLength":   1,
				"maxLength":   100,
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "Description of the experiment",
				"maxLength":   500,
			},
			"experiment_type": map[string]interface{}{
				"type":        "string",
				"description": "Type of system stress experiment",
				"enum":        []string{"cpu-stress", "mem-stress", "io-stress", "process-kill"},
			},
			"target": map[string]interface{}{
				"type":        "string",
				"description": "Target system or process",
				"minLength":   1,
			},
			"duration": map[string]interface{}{
				"type":        "integer",
				"description": "Duration in seconds",
				"minimum":     1,
				"maximum":     3600,
			},
			"cpu_workers": map[string]interface{}{
				"type":        "integer",
				"description": "Number of CPU stress workers",
				"minimum":     1,
				"maximum":     32,
			},
			"mem_size_mb": map[string]interface{}{
				"type":        "integer",
				"description": "Memory stress size in MB",
				"minimum":     1,
				"maximum":     8192,
			},
			"kill_process": map[string]interface{}{
				"type":        "string",
				"description": "Process name or pattern to kill",
				"minLength":   1,
			},
			"start_time": map[string]interface{}{
				"type":        "string",
				"description": "Scheduled start time (RFC3339)",
				"format":      "date-time",
			},
			"parallel": map[string]interface{}{
				"type":        "boolean",
				"description": "Run in parallel across multiple agents",
			},
			"agent_count": map[string]interface{}{
				"type":        "integer",
				"description": "Number of agents to target",
				"minimum":     1,
				"maximum":     100,
			},
		},
	}
}

// ValidateExperiment validates an experiment request against the appropriate schema
func (ev *ExperimentValidator) ValidateExperiment(expReq *ExperimentRequest) (*ValidationResult, error) {
	result := &ValidationResult{
		Valid:    true,
		Errors:   make([]string, 0),
		Warnings: make([]string, 0),
	}

	// Determine schema based on experiment type
	schema := ev.getSchemaForExperiment(expReq.ExperimentType)
	if schema == nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("Unknown experiment type: %s", expReq.ExperimentType))
		return result, nil
	}

	// Validate required fields
	if err := ev.validateRequiredFields(expReq, schema, result); err != nil {
		return nil, err
	}

	// Validate field types and constraints
	if err := ev.validateFieldConstraints(expReq, schema, result); err != nil {
		return nil, err
	}

	// Validate business logic
	ev.validateBusinessLogic(expReq, result)

	return result, nil
}

// getSchemaForExperiment returns the appropriate schema for the experiment type
func (ev *ExperimentValidator) getSchemaForExperiment(experimentType string) *ExperimentSchema {
	if strings.HasPrefix(experimentType, "network-") {
		return ev.schemas["network"]
	} else if strings.HasPrefix(experimentType, "cpu-") ||
		strings.HasPrefix(experimentType, "mem-") ||
		strings.HasPrefix(experimentType, "io-") ||
		strings.HasPrefix(experimentType, "process-") {
		return ev.schemas["system"]
	}
	return nil
}

// validateRequiredFields checks that all required fields are present
func (ev *ExperimentValidator) validateRequiredFields(expReq *ExperimentRequest, schema *ExperimentSchema, result *ValidationResult) error {
	for _, required := range schema.Required {
		value := ev.getFieldValue(expReq, required)
		if value == nil || (reflect.TypeOf(value).Kind() == reflect.String && value.(string) == "") {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("Required field '%s' is missing or empty", required))
		}
	}
	return nil
}

// validateFieldConstraints validates field types and value constraints
func (ev *ExperimentValidator) validateFieldConstraints(expReq *ExperimentRequest, schema *ExperimentSchema, result *ValidationResult) error {
	// Validate string fields
	if expReq.Name != "" && len(expReq.Name) > 100 {
		result.Valid = false
		result.Errors = append(result.Errors, "Name field exceeds maximum length of 100 characters")
	}

	if expReq.Description != "" && len(expReq.Description) > 500 {
		result.Valid = false
		result.Errors = append(result.Errors, "Description field exceeds maximum length of 500 characters")
	}

	// Validate numeric fields
	if expReq.Duration < 1 || expReq.Duration > 3600 {
		result.Valid = false
		result.Errors = append(result.Errors, "Duration must be between 1 and 3600 seconds")
	}

	if expReq.DelayMs > 0 && (expReq.DelayMs < 1 || expReq.DelayMs > 10000) {
		result.Valid = false
		result.Errors = append(result.Errors, "Delay must be between 1 and 10000 milliseconds")
	}

	if expReq.LossPercent > 0 && (expReq.LossPercent < 0 || expReq.LossPercent > 100) {
		result.Valid = false
		result.Errors = append(result.Errors, "Loss percentage must be between 0 and 100")
	}

	if expReq.CPUWorkers > 0 && (expReq.CPUWorkers < 1 || expReq.CPUWorkers > 32) {
		result.Valid = false
		result.Errors = append(result.Errors, "CPU workers must be between 1 and 32")
	}

	if expReq.MemSizeMB > 0 && (expReq.MemSizeMB < 1 || expReq.MemSizeMB > 8192) {
		result.Valid = false
		result.Errors = append(result.Errors, "Memory size must be between 1 and 8192 MB")
	}

	if expReq.AgentCount > 0 && (expReq.AgentCount < 1 || expReq.AgentCount > 100) {
		result.Valid = false
		result.Errors = append(result.Errors, "Agent count must be between 1 and 100")
	}

	return nil
}

// validateBusinessLogic validates business logic rules
func (ev *ExperimentValidator) validateBusinessLogic(expReq *ExperimentRequest, result *ValidationResult) {
	// Check if start time is in the past
	if !expReq.StartTime.IsZero() && expReq.StartTime.Before(time.Now()) {
		result.Warnings = append(result.Warnings, "Start time is in the past, experiment will start immediately")
	}

	// Check if parallel execution is requested without agent count
	if expReq.Parallel && expReq.AgentCount <= 0 {
		result.Warnings = append(result.Warnings, "Parallel execution requested but no agent count specified, using default of 1")
	}

	// Check if experiment type matches target
	if strings.HasPrefix(expReq.ExperimentType, "network-") && expReq.Target == "" {
		result.Warnings = append(result.Warnings, "Network experiment should specify target interface")
	}

	// Check if system stress experiment has appropriate parameters
	if expReq.ExperimentType == "cpu-stress" && expReq.CPUWorkers <= 0 {
		result.Warnings = append(result.Warnings, "CPU stress experiment should specify number of workers")
	}

	if expReq.ExperimentType == "mem-stress" && expReq.MemSizeMB <= 0 {
		result.Warnings = append(result.Warnings, "Memory stress experiment should specify memory size")
	}
}

// getFieldValue gets the value of a field from the experiment request using reflection
func (ev *ExperimentValidator) getFieldValue(expReq *ExperimentRequest, fieldName string) interface{} {
	v := reflect.ValueOf(expReq)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	field := v.FieldByName(strings.Title(fieldName))
	if !field.IsValid() {
		return nil
	}

	return field.Interface()
}

// ValidationResult contains the results of experiment validation
type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// GetSchema returns the JSON schema for a specific experiment type
func (ev *ExperimentValidator) GetSchema(experimentType string) (*ExperimentSchema, error) {
	schema := ev.getSchemaForExperiment(experimentType)
	if schema == nil {
		return nil, fmt.Errorf("no schema found for experiment type: %s", experimentType)
	}
	return schema, nil
}

// GetSchemaJSON returns the JSON schema as a JSON string
func (ev *ExperimentValidator) GetSchemaJSON(experimentType string) (string, error) {
	schema, err := ev.GetSchema(experimentType)
	if err != nil {
		return "", err
	}

	jsonData, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal schema to JSON: %w", err)
	}

	return string(jsonData), nil
}

// ListSupportedTypes returns a list of supported experiment types
func (ev *ExperimentValidator) ListSupportedTypes() []string {
	types := make([]string, 0)
	for _, schema := range ev.schemas {
		if props, ok := schema.Properties["experiment_type"].(map[string]interface{}); ok {
			if enum, ok := props["enum"].([]string); ok {
				types = append(types, enum...)
			}
		}
	}
	return types
}
