package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// NetemManager manages network fault injection using tc netem with improved performance
type NetemManager struct {
	mu           sync.RWMutex
	interfaces   map[string]*InterfaceState
	experiments  map[string]*ExperimentState
	cleanupTimer *time.Timer
	// P2: Track root qdisc state for atomic updates
	rootQdiscs map[string]*RootQdiscState
}

// InterfaceState tracks the state of a network interface
type InterfaceState struct {
	Name          string
	HasNetem      bool
	OriginalQdisc string
	CurrentConfig *NetemConfig
	LastModified  time.Time
	// P2: Add ingress impairment support
	HasIngress    bool
	IngressConfig *NetemConfig
	IFBName       string // Associated IFB interface name
}

// NetemConfig represents a netem configuration
type NetemConfig struct {
	Delay     int     `json:"delay_ms"`          // Delay in milliseconds
	Jitter    int     `json:"jitter_ms"`         // Jitter in milliseconds
	Loss      float64 `json:"loss_percent"`      // Packet loss percentage
	Duplicate float64 `json:"duplicate_percent"` // Duplicate packet percentage
	Corrupt   float64 `json:"corrupt_percent"`   // Corrupt packet percentage
	Reorder   float64 `json:"reorder_percent"`   // Reorder packet percentage
	Rate      int     `json:"rate_mbps"`         // Rate limiting in Mbps
	Limit     int     `json:"limit_packets"`     // Queue limit in packets
}

// RootQdiscState tracks the root qdisc configuration for atomic updates
type RootQdiscState struct {
	Interface     string
	HasNetem      bool
	OriginalQdisc string
	CurrentNetem  *NetemConfig
	Filters       map[string]*FilterConfig
	LastModified  time.Time
}

// FilterConfig represents a selective filter for targeted impairment
type FilterConfig struct {
	ID          string       `json:"id"`
	Priority    int          `json:"priority"`
	Protocol    string       `json:"protocol"` // tcp, udp, icmp, etc.
	SrcIP       string       `json:"src_ip"`   // Source IP range
	DstIP       string       `json:"dst_ip"`   // Destination IP range
	SrcPort     int          `json:"src_port"` // Source port
	DstPort     int          `json:"dst_port"` // Destination port
	NetemConfig *NetemConfig `json:"netem_config"`
	Created     time.Time    `json:"created"`
}

// ExperimentState tracks an active experiment
type ExperimentState struct {
	ID        string
	Interface string
	Config    *NetemConfig
	StartTime time.Time
	TTL       time.Duration
	Finalizer string
	// P2: Add filter support
	Filters   []string `json:"filters"`   // List of filter IDs
	Direction string   `json:"direction"` // ingress, egress, or both
}

// NewNetemManager creates a new netem manager
func NewNetemManager() *NetemManager {
	nm := &NetemManager{
		interfaces:  make(map[string]*InterfaceState),
		experiments: make(map[string]*ExperimentState),
		rootQdiscs:  make(map[string]*RootQdiscState),
	}

	// Start cleanup goroutine
	go nm.cleanupRoutine()

	return nm
}

// Apply applies a network fault configuration to an interface
func (nm *NetemManager) Apply(expID, interfaceName string, config *NetemConfig, ttl time.Duration) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	// Validate interface
	if err := nm.validateInterface(interfaceName); err != nil {
		return fmt.Errorf("interface validation failed: %w", err)
	}

	// Get or create interface state
	iface, exists := nm.interfaces[interfaceName]
	if !exists {
		iface = &InterfaceState{
			Name:          interfaceName,
			HasNetem:      false,
			OriginalQdisc: "",
			LastModified:  time.Now(),
		}
		nm.interfaces[interfaceName] = iface
	}

	// Store original qdisc if this is the first modification
	if !iface.HasNetem {
		if err := nm.backupOriginalQdisc(iface); err != nil {
			return fmt.Errorf("failed to backup original qdisc: %w", err)
		}
	}

	// Apply netem configuration
	if err := nm.applyNetemConfig(iface, config); err != nil {
		return fmt.Errorf("failed to apply netem config: %w", err)
	}

	// Update interface state
	iface.HasNetem = true
	iface.CurrentConfig = config
	iface.LastModified = time.Now()

	// Record experiment
	experiment := &ExperimentState{
		ID:        expID,
		Interface: interfaceName,
		Config:    config,
		StartTime: time.Now(),
		TTL:       ttl,
		Finalizer: fmt.Sprintf("netem-%s", expID),
	}
	nm.experiments[expID] = experiment

	// Set cleanup timer
	if ttl > 0 {
		time.AfterFunc(ttl, func() {
			nm.Revert(expID)
		})
	}

	log.Printf("[Netem] Applied fault to %s: %+v", interfaceName, config)
	return nil
}

// ApplyWithFilter applies a network fault with selective filtering
func (nm *NetemManager) ApplyWithFilter(expID, interfaceName string, config *NetemConfig, filter *FilterConfig, ttl time.Duration) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	// Validate interface
	if err := nm.validateInterface(interfaceName); err != nil {
		return fmt.Errorf("interface validation failed: %w", err)
	}

	// Get or create root qdisc state
	rootQdisc, exists := nm.rootQdiscs[interfaceName]
	if !exists {
		rootQdisc = &RootQdiscState{
			Interface: interfaceName,
			Filters:   make(map[string]*FilterConfig),
		}
		nm.rootQdiscs[interfaceName] = rootQdisc
	}

	// Store original qdisc if this is the first modification
	if !rootQdisc.HasNetem {
		if err := nm.backupOriginalQdiscForRoot(rootQdisc); err != nil {
			return fmt.Errorf("failed to backup original qdisc: %w", err)
		}
	}

	// Add filter
	filter.ID = fmt.Sprintf("filter-%s-%s", expID, interfaceName)
	filter.Created = time.Now()
	rootQdisc.Filters[filter.ID] = filter

	// Apply netem with filter
	if err := nm.applyNetemWithFilter(rootQdisc, config, filter); err != nil {
		return fmt.Errorf("failed to apply netem with filter: %w", err)
	}

	// Update root qdisc state
	rootQdisc.HasNetem = true
	rootQdisc.CurrentNetem = config
	rootQdisc.LastModified = time.Now()

	// Get or create interface state
	iface, exists := nm.interfaces[interfaceName]
	if !exists {
		iface = &InterfaceState{
			Name:          interfaceName,
			HasNetem:      true,
			CurrentConfig: config,
			LastModified:  time.Now(),
		}
		nm.interfaces[interfaceName] = iface
	} else {
		iface.HasNetem = true
		iface.CurrentConfig = config
		iface.LastModified = time.Now()
	}

	// Record experiment
	experiment := &ExperimentState{
		ID:        expID,
		Interface: interfaceName,
		Config:    config,
		StartTime: time.Now(),
		TTL:       ttl,
		Finalizer: fmt.Sprintf("netem-%s", expID),
		Filters:   []string{filter.ID},
		Direction: "egress",
	}
	nm.experiments[expID] = experiment

	// Set cleanup timer
	if ttl > 0 {
		time.AfterFunc(ttl, func() {
			nm.Revert(expID)
		})
	}

	log.Printf("[Netem] Applied filtered fault to %s: %+v with filter %s", interfaceName, config, filter.ID)
	return nil
}

// ApplyIngress applies ingress impairment using IFB (Intermediate Functional Block)
func (nm *NetemManager) ApplyIngress(expID, interfaceName string, config *NetemConfig, ttl time.Duration) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	// Validate interface
	if err := nm.validateInterface(interfaceName); err != nil {
		return fmt.Errorf("interface validation failed: %w", err)
	}

	// Check if IFB module is loaded
	if err := nm.ensureIFBModule(); err != nil {
		return fmt.Errorf("IFB module not available: %w", err)
	}

	// Setup IFB interface for ingress
	ifbName := fmt.Sprintf("ifb-%s", interfaceName)
	if err := nm.setupIFBInterface(interfaceName, ifbName); err != nil {
		return fmt.Errorf("failed to setup IFB interface: %w", err)
	}

	// Apply netem to IFB interface
	if err := nm.applyNetemToIFB(ifbName, config); err != nil {
		return fmt.Errorf("failed to apply netem to IFB: %w", err)
	}

	// Get or create interface state
	iface, exists := nm.interfaces[interfaceName]
	if !exists {
		iface = &InterfaceState{
			Name:          interfaceName,
			HasIngress:    true,
			IngressConfig: config,
			LastModified:  time.Now(),
			IFBName:       ifbName,
		}
		nm.interfaces[interfaceName] = iface
	} else {
		iface.HasIngress = true
		iface.IngressConfig = config
		iface.LastModified = time.Now()
		iface.IFBName = ifbName
	}

	// Record experiment
	experiment := &ExperimentState{
		ID:        expID,
		Interface: interfaceName,
		Config:    config,
		StartTime: time.Now(),
		TTL:       ttl,
		Finalizer: fmt.Sprintf("ingress-%s", expID),
		Direction: "ingress",
	}
	nm.experiments[expID] = experiment

	// Set cleanup timer
	if ttl > 0 {
		time.AfterFunc(ttl, func() {
			nm.Revert(expID)
		})
	}

	log.Printf("[Netem] Applied ingress fault to %s via %s: %+v", interfaceName, ifbName, config)
	return nil
}

// Revert reverts a specific experiment
func (nm *NetemManager) Revert(expID string) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	experiment, exists := nm.experiments[expID]
	if !exists {
		return fmt.Errorf("experiment %s not found", expID)
	}

	iface, exists := nm.interfaces[experiment.Interface]
	if !exists {
		return fmt.Errorf("interface %s not found", experiment.Interface)
	}

	// Remove the experiment
	delete(nm.experiments, expID)

	// Check if this was the last experiment on this interface
	hasOtherExperiments := false
	for _, exp := range nm.experiments {
		if exp.Interface == experiment.Interface {
			hasOtherExperiments = true
			break
		}
	}

	if !hasOtherExperiments {
		// Restore original qdisc
		if err := nm.restoreOriginalQdisc(iface); err != nil {
			return fmt.Errorf("failed to restore original qdisc: %w", err)
		}
		iface.HasNetem = false
		iface.CurrentConfig = nil
	} else {
		// Recalculate netem config for remaining experiments
		if err := nm.recalculateNetemConfig(iface); err != nil {
			return fmt.Errorf("failed to recalculate netem config: %w", err)
		}
	}

	log.Printf("[Netem] Reverted experiment %s on %s", expID, experiment.Interface)
	return nil
}

// RevertAll reverts all experiments on an interface
func (nm *NetemManager) RevertAll(interfaceName string) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	iface, exists := nm.interfaces[interfaceName]
	if !exists {
		return fmt.Errorf("interface %s not found", interfaceName)
	}

	// Remove all experiments on this interface
	var experimentsToRemove []string
	for expID, exp := range nm.experiments {
		if exp.Interface == interfaceName {
			experimentsToRemove = append(experimentsToRemove, expID)
		}
	}

	for _, expID := range experimentsToRemove {
		delete(nm.experiments, expID)
	}

	// Restore original qdisc
	if err := nm.restoreOriginalQdisc(iface); err != nil {
		return fmt.Errorf("failed to restore original qdisc: %w", err)
	}

	iface.HasNetem = false
	iface.CurrentConfig = nil

	log.Printf("[Netem] Reverted all experiments on %s", interfaceName)
	return nil
}

// GetStatus returns the current status of all interfaces
func (nm *NetemManager) GetStatus() map[string]interface{} {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	status := make(map[string]interface{})

	for name, iface := range nm.interfaces {
		ifaceStatus := map[string]interface{}{
			"has_netem":        iface.HasNetem,
			"has_ingress":      iface.HasIngress,
			"last_modified":    iface.LastModified,
			"experiment_count": 0,
		}

		if iface.HasNetem {
			ifaceStatus["current_config"] = iface.CurrentConfig
		}

		if iface.HasIngress {
			ifaceStatus["ingress_config"] = iface.IngressConfig
			ifaceStatus["ifb_name"] = iface.IFBName
		}

		// Count experiments on this interface
		for _, exp := range nm.experiments {
			if exp.Interface == name {
				ifaceStatus["experiment_count"] = ifaceStatus["experiment_count"].(int) + 1
			}
		}

		status[name] = ifaceStatus
	}

	return status
}

// validateInterface checks if an interface exists and supports netem
func (nm *NetemManager) validateInterface(interfaceName string) error {
	// Check if interface exists
	cmd := exec.Command("ip", "link", "show", interfaceName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("interface %s does not exist", interfaceName)
	}

	// Check if tc is available
	if _, err := exec.LookPath("tc"); err != nil {
		return fmt.Errorf("tc command not available")
	}

	// Check if netem module is loaded
	cmd = exec.Command("modprobe", "sch_netem")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("netem module not available")
	}

	return nil
}

// backupOriginalQdisc saves the original qdisc configuration
func (nm *NetemManager) backupOriginalQdisc(iface *InterfaceState) error {
	cmd := exec.Command("tc", "qdisc", "show", "dev", iface.Name)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get qdisc info: %w", err)
	}

	// Parse the output to find the root qdisc
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "qdisc") && !strings.Contains(line, "netem") {
			iface.OriginalQdisc = strings.TrimSpace(line)
			break
		}
	}

	return nil
}

// applyNetemConfig applies netem configuration using tc qdisc change
func (nm *NetemManager) applyNetemConfig(iface *InterfaceState, config *NetemConfig) error {
	var args []string

	if iface.HasNetem {
		// Modify existing netem
		args = []string{"tc", "qdisc", "change", "dev", iface.Name, "root", "netem"}
	} else {
		// Add new netem
		args = []string{"tc", "qdisc", "add", "dev", iface.Name, "root", "netem"}
	}

	// Build netem parameters
	if config.Delay > 0 {
		args = append(args, "delay", fmt.Sprintf("%dms", config.Delay))
		if config.Jitter > 0 {
			args = append(args, fmt.Sprintf("%dms", config.Jitter))
		}
	}

	if config.Loss > 0 {
		args = append(args, "loss", fmt.Sprintf("%.2f%%", config.Loss))
	}

	if config.Duplicate > 0 {
		args = append(args, "duplicate", fmt.Sprintf("%.2f%%", config.Duplicate))
	}

	if config.Corrupt > 0 {
		args = append(args, "corrupt", fmt.Sprintf("%.2f%%", config.Corrupt))
	}

	if config.Reorder > 0 {
		args = append(args, "reorder", fmt.Sprintf("%.2f%%", config.Reorder))
	}

	if config.Rate > 0 {
		args = append(args, "rate", fmt.Sprintf("%dmbit", config.Rate))
	}

	if config.Limit > 0 {
		args = append(args, "limit", strconv.Itoa(config.Limit))
	}

	// Execute tc command
	cmd := exec.Command(args[0], args[1:]...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tc command failed: %w", err)
	}

	return nil
}

// restoreOriginalQdisc restores the original qdisc configuration
func (nm *NetemManager) restoreOriginalQdisc(iface *InterfaceState) error {
	if iface.OriginalQdisc == "" {
		// No original qdisc to restore, just remove netem
		cmd := exec.Command("tc", "qdisc", "del", "dev", iface.Name, "root")
		return cmd.Run()
	}

	// Parse and restore original qdisc
	// This is a simplified implementation - in production you'd want more sophisticated parsing
	cmd := exec.Command("tc", "qdisc", "del", "dev", iface.Name, "root")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to remove netem: %w", err)
	}

	// Restore original qdisc if it wasn't the default
	if !strings.Contains(iface.OriginalQdisc, "fq_codel") && !strings.Contains(iface.OriginalQdisc, "pfifo_fast") {
		// Parse original qdisc and restore it
		// This would require parsing the qdisc string and building the appropriate tc command
		log.Printf("[Netem] Note: Original qdisc restoration not fully implemented for: %s", iface.OriginalQdisc)
	}

	return nil
}

// recalculateNetemConfig recalculates netem config for remaining experiments
func (nm *NetemManager) recalculateNetemConfig(iface *InterfaceState) error {
	// Find all remaining experiments on this interface
	var configs []*NetemConfig
	for _, exp := range nm.experiments {
		if exp.Interface == iface.Name {
			configs = append(configs, exp.Config)
		}
	}

	if len(configs) == 0 {
		return nm.restoreOriginalQdisc(iface)
	}

	// Merge configurations (simplified - in production you'd want more sophisticated merging)
	mergedConfig := nm.mergeConfigs(configs)
	return nm.applyNetemConfig(iface, mergedConfig)
}

// mergeConfigs merges multiple netem configurations
func (nm *NetemManager) mergeConfigs(configs []*NetemConfig) *NetemConfig {
	if len(configs) == 0 {
		return &NetemConfig{}
	}

	if len(configs) == 1 {
		return configs[0]
	}

	// Simple merging strategy - take the maximum values
	merged := &NetemConfig{}
	for _, config := range configs {
		if config.Delay > merged.Delay {
			merged.Delay = config.Delay
		}
		if config.Jitter > merged.Jitter {
			merged.Jitter = config.Jitter
		}
		if config.Loss > merged.Loss {
			merged.Loss = config.Loss
		}
		if config.Duplicate > merged.Duplicate {
			merged.Duplicate = config.Duplicate
		}
		if config.Corrupt > merged.Corrupt {
			merged.Corrupt = config.Corrupt
		}
		if config.Reorder > merged.Reorder {
			merged.Reorder = config.Reorder
		}
		if config.Rate > merged.Rate {
			merged.Rate = config.Rate
		}
		if config.Limit > merged.Limit {
			merged.Limit = config.Limit
		}
	}

	return merged
}

// cleanupRoutine periodically cleans up expired experiments
func (nm *NetemManager) cleanupRoutine() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		nm.mu.Lock()
		now := time.Now()
		var expiredExperiments []string

		for expID, exp := range nm.experiments {
			if exp.TTL > 0 && now.Sub(exp.StartTime) > exp.TTL {
				expiredExperiments = append(expiredExperiments, expID)
			}
		}

		nm.mu.Unlock()

		// Revert expired experiments
		for _, expID := range expiredExperiments {
			nm.Revert(expID)
		}
	}
}

// GetExperimentInfo returns information about a specific experiment
func (nm *NetemManager) GetExperimentInfo(expID string) (*ExperimentState, error) {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	exp, exists := nm.experiments[expID]
	if !exists {
		return nil, fmt.Errorf("experiment %s not found", expID)
	}

	return exp, nil
}

// ListExperiments returns all active experiments
func (nm *NetemManager) ListExperiments() []*ExperimentState {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	experiments := make([]*ExperimentState, 0, len(nm.experiments))
	for _, exp := range nm.experiments {
		experiments = append(experiments, exp)
	}

	return experiments
}

// ExportState exports the current state for persistence
func (nm *NetemManager) ExportState() ([]byte, error) {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	state := struct {
		Interfaces  map[string]*InterfaceState  `json:"interfaces"`
		Experiments map[string]*ExperimentState `json:"experiments"`
		RootQdiscs  map[string]*RootQdiscState  `json:"root_qdiscs"`
		Timestamp   time.Time                   `json:"timestamp"`
	}{
		Interfaces:  nm.interfaces,
		Experiments: nm.experiments,
		RootQdiscs:  nm.rootQdiscs,
		Timestamp:   time.Now(),
	}

	return json.MarshalIndent(state, "", "  ")
}

// ImportState imports state from persistence
func (nm *NetemManager) ImportState(data []byte) error {
	var state struct {
		Interfaces  map[string]*InterfaceState  `json:"interfaces"`
		Experiments map[string]*ExperimentState `json:"experiments"`
		RootQdiscs  map[string]*RootQdiscState  `json:"root_qdiscs"`
		Timestamp   time.Time                   `json:"timestamp"`
	}

	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to unmarshal state: %w", err)
	}

	nm.mu.Lock()
	defer nm.mu.Unlock()

	nm.interfaces = state.Interfaces
	nm.experiments = state.Experiments
	nm.rootQdiscs = state.RootQdiscs

	// Restart cleanup routine
	if nm.cleanupTimer != nil {
		nm.cleanupTimer.Stop()
	}
	go nm.cleanupRoutine()

	return nil
}

// P2: Helper functions for improved netem functionality

// backupOriginalQdiscForRoot saves the original qdisc configuration for root qdisc state
func (nm *NetemManager) backupOriginalQdiscForRoot(rootQdisc *RootQdiscState) error {
	cmd := exec.Command("tc", "qdisc", "show", "dev", rootQdisc.Interface)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get qdisc info: %w", err)
	}

	// Parse the output to find the root qdisc
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "qdisc") && !strings.Contains(line, "netem") {
			rootQdisc.OriginalQdisc = strings.TrimSpace(line)
			break
		}
	}

	return nil
}

// applyNetemWithFilter applies netem with selective filtering
func (nm *NetemManager) applyNetemWithFilter(rootQdisc *RootQdiscState, config *NetemConfig, filter *FilterConfig) error {
	// First ensure we have a netem qdisc
	if !rootQdisc.HasNetem {
		if err := nm.applyNetemConfigAtomic(rootQdisc, config); err != nil {
			return err
		}
	}

	// Add filter using u32 classifier
	if err := nm.addU32Filter(rootQdisc.Interface, filter); err != nil {
		return fmt.Errorf("failed to add u32 filter: %w", err)
	}

	return nil
}

// addU32Filter adds a u32 filter for selective impairment
func (nm *NetemManager) addU32Filter(interfaceName string, filter *FilterConfig) error {
	// Build u32 filter command
	args := []string{"tc", "filter", "add", "dev", interfaceName, "protocol", "ip"}

	// Add priority
	args = append(args, "prio", strconv.Itoa(filter.Priority))

	// Add u32 classifier
	args = append(args, "u32")

	// Add match conditions
	if filter.Protocol != "" {
		args = append(args, "match", "ip", "protocol", filter.Protocol, "0xff")
	}

	if filter.SrcIP != "" {
		args = append(args, "match", "ip", "src", filter.SrcIP)
	}

	if filter.DstIP != "" {
		args = append(args, "match", "ip", "dst", filter.DstIP)
	}

	if filter.SrcPort > 0 {
		args = append(args, "match", "ip", "sport", strconv.Itoa(filter.SrcPort), "0xffff")
	}

	if filter.DstPort > 0 {
		args = append(args, "match", "ip", "dport", strconv.Itoa(filter.DstPort), "0xffff")
	}

	// Add action to divert to netem
	args = append(args, "flowid", "1:1")

	// Execute tc command
	cmd := exec.Command(args[0], args[1:]...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("u32 filter command failed: %w", err)
	}

	return nil
}

// applyNetemConfigAtomic applies netem configuration using tc qdisc change for atomic updates
func (nm *NetemManager) applyNetemConfigAtomic(rootQdisc *RootQdiscState, config *NetemConfig) error {
	var args []string

	if rootQdisc.HasNetem {
		// P2: Use tc qdisc change for atomic updates
		args = []string{"tc", "qdisc", "change", "dev", rootQdisc.Interface, "root", "netem"}
	} else {
		// Add new netem
		args = []string{"tc", "qdisc", "add", "dev", rootQdisc.Interface, "root", "netem"}
	}

	// Build netem parameters
	args = nm.buildNetemArgs(args, config)

	// Execute tc command
	cmd := exec.Command(args[0], args[1:]...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tc command failed: %w", err)
	}

	return nil
}

// buildNetemArgs builds netem command arguments
func (nm *NetemManager) buildNetemArgs(args []string, config *NetemConfig) []string {
	if config.Delay > 0 {
		args = append(args, "delay", fmt.Sprintf("%dms", config.Delay))
		if config.Jitter > 0 {
			args = append(args, fmt.Sprintf("%dms", config.Jitter))
		}
	}

	if config.Loss > 0 {
		args = append(args, "loss", fmt.Sprintf("%.2f%%", config.Loss))
	}

	if config.Duplicate > 0 {
		args = append(args, "duplicate", fmt.Sprintf("%.2f%%", config.Duplicate))
	}

	if config.Corrupt > 0 {
		args = append(args, "corrupt", fmt.Sprintf("%.2f%%", config.Corrupt))
	}

	if config.Reorder > 0 {
		args = append(args, "reorder", fmt.Sprintf("%.2f%%", config.Reorder))
	}

	if config.Rate > 0 {
		args = append(args, "rate", fmt.Sprintf("%dmbit", config.Rate))
	}

	if config.Limit > 0 {
		args = append(args, "limit", strconv.Itoa(config.Limit))
	}

	return args
}

// ensureIFBModule ensures the IFB module is loaded
func (nm *NetemManager) ensureIFBModule() error {
	cmd := exec.Command("modprobe", "ifb")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to load IFB module: %w", err)
	}
	return nil
}

// setupIFBInterface sets up IFB interface for ingress impairment
func (nm *NetemManager) setupIFBInterface(originalInterface, ifbName string) error {
	// Create IFB interface
	cmd := exec.Command("ip", "link", "add", ifbName, "type", "ifb")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add IFB interface: %w", err)
	}

	// Bring IFB interface up
	cmd = exec.Command("ip", "link", "set", ifbName, "up")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to bring up IFB interface: %w", err)
	}

	// Add qdisc to IFB interface
	cmd = exec.Command("tc", "qdisc", "add", "dev", ifbName, "root", "handle", "1:", "htb", "default", "1")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add qdisc to IFB: %w", err)
	}

	// Add class for netem
	cmd = exec.Command("tc", "class", "add", "dev", ifbName, "parent", "1:", "classid", "1:1", "htb", "rate", "1000mbit")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add class to IFB: %w", err)
	}

	// Add netem to IFB class
	cmd = exec.Command("tc", "qdisc", "add", "dev", ifbName, "parent", "1:1", "handle", "10:", "netem")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add netem to IFB: %w", err)
	}

	// Add ingress qdisc to original interface
	cmd = exec.Command("tc", "qdisc", "add", "dev", originalInterface, "handle", "ffff:", "ingress")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add ingress qdisc: %w", err)
	}

	// Add filter to redirect ingress traffic to IFB
	cmd = exec.Command("tc", "filter", "add", "dev", originalInterface, "parent", "ffff:", "protocol", "ip", "u32", "match", "u32", "0", "0", "flowid", "1:1", "action", "mirred", "egress", "redirect", "dev", ifbName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add ingress filter: %w", err)
	}

	return nil
}

// applyNetemToIFB applies netem configuration to IFB interface
func (nm *NetemManager) applyNetemToIFB(ifbName string, config *NetemConfig) error {
	args := []string{"tc", "qdisc", "change", "dev", ifbName, "parent", "1:1", "handle", "10:", "netem"}
	args = nm.buildNetemArgs(args, config)

	cmd := exec.Command(args[0], args[1:]...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to apply netem to IFB: %w", err)
	}

	return nil
}
