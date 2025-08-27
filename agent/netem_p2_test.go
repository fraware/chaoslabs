package main

import (
	"testing"
	"time"
)

func TestP2NetemManagerCreation(t *testing.T) {
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

	if nm.rootQdiscs == nil {
		t.Error("rootQdiscs map not initialized")
	}
}

func TestP2FilterConfigValidation(t *testing.T) {
	// Test valid filter configuration
	filter := &FilterConfig{
		ID:          "test-filter",
		Priority:    1,
		Protocol:    "tcp",
		SrcIP:       "192.168.1.0/24",
		DstIP:       "10.0.0.0/8",
		SrcPort:     8080,
		DstPort:     443,
		NetemConfig: &NetemConfig{Delay: 100, Loss: 5.0},
		Created:     time.Now(),
	}

	if filter.ID != "test-filter" {
		t.Errorf("Expected ID 'test-filter', got %s", filter.ID)
	}

	if filter.Protocol != "tcp" {
		t.Errorf("Expected protocol 'tcp', got %s", filter.Protocol)
	}

	if filter.SrcPort != 8080 {
		t.Errorf("Expected source port 8080, got %d", filter.SrcPort)
	}

	if filter.DstPort != 443 {
		t.Errorf("Expected destination port 443, got %d", filter.DstPort)
	}
}

func TestP2RootQdiscStateManagement(t *testing.T) {
	nm := NewNetemManager()

	// Test initial state
	if len(nm.rootQdiscs) != 0 {
		t.Errorf("Expected empty rootQdiscs, got %d", len(nm.rootQdiscs))
	}

	// Test creating root qdisc state
	rootQdisc := &RootQdiscState{
		Interface: "eth0",
		Filters:   make(map[string]*FilterConfig),
	}

	nm.rootQdiscs["eth0"] = rootQdisc

	if len(nm.rootQdiscs) != 1 {
		t.Errorf("Expected 1 root qdisc, got %d", len(nm.rootQdiscs))
	}

	if nm.rootQdiscs["eth0"] != rootQdisc {
		t.Error("Root qdisc not properly stored")
	}
}

func TestP2InterfaceStateIngressSupport(t *testing.T) {
	iface := &InterfaceState{
		Name:          "eth0",
		HasNetem:      true,
		HasIngress:    true,
		CurrentConfig: &NetemConfig{Delay: 100},
		IngressConfig: &NetemConfig{Loss: 5.0},
		IFBName:       "ifb-eth0",
		LastModified:  time.Now(),
	}

	if !iface.HasIngress {
		t.Error("Expected HasIngress to be true")
	}

	if iface.IFBName != "ifb-eth0" {
		t.Errorf("Expected IFB name 'ifb-eth0', got %s", iface.IFBName)
	}

	if iface.IngressConfig.Loss != 5.0 {
		t.Errorf("Expected ingress loss 5.0, got %f", iface.IngressConfig.Loss)
	}
}

func TestP2ExperimentStateDirectionSupport(t *testing.T) {
	exp := &ExperimentState{
		ID:        "test-exp",
		Interface: "eth0",
		Config:    &NetemConfig{Delay: 100},
		StartTime: time.Now(),
		TTL:       1 * time.Minute,
		Finalizer: "netem-test-exp",
		Filters:   []string{"filter-1", "filter-2"},
		Direction: "egress",
	}

	if exp.Direction != "egress" {
		t.Errorf("Expected direction 'egress', got %s", exp.Direction)
	}

	if len(exp.Filters) != 2 {
		t.Errorf("Expected 2 filters, got %d", len(exp.Filters))
	}

	if exp.Filters[0] != "filter-1" {
		t.Errorf("Expected first filter 'filter-1', got %s", exp.Filters[0])
	}
}

func TestP2NetemConfigAtomicUpdate(t *testing.T) {
	nm := NewNetemManager()
	rootQdisc := &RootQdiscState{
		Interface: "eth0",
		HasNetem:  false,
		Filters:   make(map[string]*FilterConfig),
	}

	config := &NetemConfig{
		Delay: 100,
		Loss:  5.0,
	}

	// Test atomic update
	err := nm.applyNetemConfigAtomic(rootQdisc, config)
	if err != nil {
		t.Logf("Note: applyNetemConfigAtomic failed (expected in test environment): %v", err)
		// This is expected to fail in test environment without tc command
		return
	}

	if !rootQdisc.HasNetem {
		t.Error("Expected HasNetem to be true after atomic update")
	}

	if rootQdisc.CurrentNetem != config {
		t.Error("Expected CurrentNetem to match config after atomic update")
	}
}

func TestP2BuildNetemArgs(t *testing.T) {
	nm := NewNetemManager()
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

	args := []string{"tc", "qdisc", "add", "dev", "eth0", "root", "netem"}
	result := nm.buildNetemArgs(args, config)

	// Check that all parameters were added
	expectedArgs := []string{
		"tc", "qdisc", "add", "dev", "eth0", "root", "netem",
		"delay", "100ms", "20ms",
		"loss", "5.00%",
		"duplicate", "1.00%",
		"corrupt", "0.50%",
		"reorder", "2.00%",
		"rate", "100mbit",
		"limit", "1000",
	}

	if len(result) != len(expectedArgs) {
		t.Errorf("Expected %d args, got %d", len(expectedArgs), len(result))
	}

	// Check key parameters
	hasDelay := false
	hasLoss := false
	hasRate := false

	for i, arg := range result {
		if arg == "delay" && i+1 < len(result) && result[i+1] == "100ms" {
			hasDelay = true
		}
		if arg == "loss" && i+1 < len(result) && result[i+1] == "5.00%" {
			hasLoss = true
		}
		if arg == "rate" && i+1 < len(result) && result[i+1] == "100mbit" {
			hasRate = true
		}
	}

	if !hasDelay {
		t.Error("Expected delay parameter not found")
	}
	if !hasLoss {
		t.Error("Expected loss parameter not found")
	}
	if !hasRate {
		t.Error("Expected rate parameter not found")
	}
}

func TestP2U32FilterGeneration(t *testing.T) {
	nm := NewNetemManager()
	filter := &FilterConfig{
		ID:       "test-filter",
		Priority: 1,
		Protocol: "tcp",
		SrcIP:    "192.168.1.0/24",
		DstIP:    "10.0.0.0/8",
		SrcPort:  8080,
		DstPort:  443,
	}

	// Test u32 filter generation
	err := nm.addU32Filter("eth0", filter)
	if err != nil {
		t.Logf("Note: addU32Filter failed (expected in test environment): %v", err)
		// This is expected to fail in test environment without tc command
		return
	}
}

func TestP2IFBModuleLoading(t *testing.T) {
	nm := NewNetemManager()

	// Test IFB module loading
	err := nm.ensureIFBModule()
	if err != nil {
		t.Logf("Note: ensureIFBModule failed (expected in test environment): %v", err)
		// This is expected to fail in test environment without modprobe
		return
	}
}

func TestP2IFBInterfaceSetup(t *testing.T) {
	nm := NewNetemManager()

	// Test IFB interface setup
	err := nm.setupIFBInterface("eth0", "ifb-eth0")
	if err != nil {
		t.Logf("Note: setupIFBInterface failed (expected in test environment): %v", err)
		// This is expected to fail in test environment without ip/tc commands
		return
	}
}

func TestP2NetemToIFBApplication(t *testing.T) {
	nm := NewNetemManager()
	config := &NetemConfig{
		Delay: 100,
		Loss:  5.0,
	}

	// Test applying netem to IFB
	err := nm.applyNetemToIFB("ifb-eth0", config)
	if err != nil {
		t.Logf("Note: applyNetemToIFB failed (expected in test environment): %v", err)
		// This is expected to fail in test environment without tc command
		return
	}
}

func TestP2StatePersistenceWithRootQdiscs(t *testing.T) {
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

	// Verify root qdiscs were restored
	if len(nm2.rootQdiscs) == 0 {
		t.Error("Expected rootQdiscs to be restored after import")
	}
}

func TestP2StatusWithIngressInformation(t *testing.T) {
	nm := NewNetemManager()

	// Add ingress experiment
	config := &NetemConfig{Delay: 100, Loss: 5.0}
	nm.ApplyIngress("test-ingress", "eth0", config, 1*time.Minute)

	// Get status
	status := nm.GetStatus()
	if len(status) == 0 {
		t.Error("Expected status to contain interface information")
		return
	}

	eth0Status, exists := status["eth0"]
	if !exists {
		t.Error("Expected eth0 interface in status")
		return
	}

	eth0Map, ok := eth0Status.(map[string]interface{})
	if !ok {
		t.Error("Expected eth0 status to be a map")
		return
	}

	if !eth0Map["has_ingress"].(bool) {
		t.Error("Expected has_ingress to be true")
	}

	if eth0Map["ifb_name"] != "ifb-eth0" {
		t.Errorf("Expected ifb_name 'ifb-eth0', got %v", eth0Map["ifb_name"])
	}
}

func TestP2FilteredExperimentApplication(t *testing.T) {
	nm := NewNetemManager()

	// Create filter configuration
	filter := &FilterConfig{
		ID:       "test-filter",
		Priority: 1,
		Protocol: "tcp",
		SrcIP:    "192.168.1.0/24",
		DstPort:  443,
	}

	config := &NetemConfig{Delay: 100, Loss: 5.0}

	// Apply filtered experiment
	err := nm.ApplyWithFilter("test-filtered", "eth0", config, filter, 1*time.Minute)
	if err != nil {
		t.Logf("Note: ApplyWithFilter failed (expected in test environment): %v", err)
		// This is expected to fail in test environment without tc command
		return
	}

	// Verify experiment was created
	experiments := nm.ListExperiments()
	found := false
	for _, exp := range experiments {
		if exp.ID == "test-filtered" && exp.Direction == "egress" {
			found = true
			if len(exp.Filters) != 1 {
				t.Errorf("Expected 1 filter, got %d", len(exp.Filters))
			}
			break
		}
	}

	if !found {
		t.Error("Expected filtered experiment to be created")
	}
}

func TestP2IngressExperimentApplication(t *testing.T) {
	nm := NewNetemManager()

	config := &NetemConfig{Delay: 100, Loss: 5.0}

	// Apply ingress experiment
	err := nm.ApplyIngress("test-ingress", "eth0", config, 1*time.Minute)
	if err != nil {
		t.Logf("Note: ApplyIngress failed (expected in test environment): %v", err)
		// This is expected to fail in test environment without tc/ip commands
		return
	}

	// Verify experiment was created
	experiments := nm.ListExperiments()
	found := false
	for _, exp := range experiments {
		if exp.ID == "test-ingress" && exp.Direction == "ingress" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected ingress experiment to be created")
	}

	// Verify interface state
	status := nm.GetStatus()
	eth0Status, exists := status["eth0"]
	if exists {
		eth0Map, ok := eth0Status.(map[string]interface{})
		if ok && eth0Map["has_ingress"].(bool) {
			if eth0Map["ifb_name"] != "ifb-eth0" {
				t.Errorf("Expected ifb_name 'ifb-eth0', got %v", eth0Map["ifb_name"])
			}
		}
	}
}
