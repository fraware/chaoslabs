package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"
)

// DiffEmitEngine implements efficient diff-based message emission
type DiffEmitEngine struct {
	mu              sync.RWMutex
	stateStore      map[string]*StateSnapshot
	config          *DiffEmitConfig
	metrics         *DiffEmitMetrics
	compressionAlgo CompressionAlgorithm
}

// StateSnapshot represents a point-in-time state for diff calculation
type StateSnapshot struct {
	Data       interface{}            `json:"data"`
	Hash       string                 `json:"hash"`
	Timestamp  time.Time              `json:"timestamp"`
	Version    int64                  `json:"version"`
	Metadata   map[string]interface{} `json:"metadata"`
	Size       int                    `json:"size"`
	ComputedAt time.Time              `json:"computed_at"`
}

// DiffEmitConfig configures diff emission behavior
type DiffEmitConfig struct {
	MaxStateHistory   int           `json:"max_state_history"`
	DiffThreshold     float64       `json:"diff_threshold"`    // 0.0-1.0, minimum change to emit
	CompressionLevel  int           `json:"compression_level"` // 1-9
	BatchSize         int           `json:"batch_size"`
	FlushInterval     time.Duration `json:"flush_interval"`
	IncludeMetadata   bool          `json:"include_metadata"`
	DeepCompare       bool          `json:"deep_compare"`
	IgnoreFields      []string      `json:"ignore_fields"`
	CompressThreshold int           `json:"compress_threshold"` // Minimum size to compress
}

// DiffEmitMetrics tracks diff emission performance
type DiffEmitMetrics struct {
	mu                   sync.RWMutex
	TotalComparisons     int64   `json:"total_comparisons"`
	DiffEmissionsSkipped int64   `json:"diff_emissions_skipped"`
	DiffEmissionsSent    int64   `json:"diff_emissions_sent"`
	AvgComputeTime       float64 `json:"avg_compute_time_ms"`
	CompressionRatio     float64 `json:"compression_ratio"`
	StateStoreSize       int     `json:"state_store_size"`
	MemoryUsage          int64   `json:"memory_usage_bytes"`
	CacheHitRate         float64 `json:"cache_hit_rate"`
}

// DiffResult represents the result of a diff operation
type DiffResult struct {
	HasChanges      bool          `json:"has_changes"`
	ChangePercent   float64       `json:"change_percent"`
	ChangedFields   []string      `json:"changed_fields"`
	AddedFields     []string      `json:"added_fields"`
	RemovedFields   []string      `json:"removed_fields"`
	Diff            interface{}   `json:"diff"`
	PreviousVersion int64         `json:"previous_version"`
	NewVersion      int64         `json:"new_version"`
	ComputeTime     time.Duration `json:"compute_time"`
	Compressed      bool          `json:"compressed"`
	OriginalSize    int           `json:"original_size"`
	CompressedSize  int           `json:"compressed_size"`
}

// CompressionAlgorithm defines compression behavior
type CompressionAlgorithm string

const (
	CompressionNone   CompressionAlgorithm = "none"
	CompressionGzip   CompressionAlgorithm = "gzip"
	CompressionLZ4    CompressionAlgorithm = "lz4"
	CompressionBrotli CompressionAlgorithm = "brotli"
	CompressionDelta  CompressionAlgorithm = "delta" // Delta compression for arrays
)

// NewDiffEmitEngine creates a new diff emit engine
func NewDiffEmitEngine(config *DiffEmitConfig) *DiffEmitEngine {
	if config == nil {
		config = &DiffEmitConfig{
			MaxStateHistory:   100,
			DiffThreshold:     0.01, // 1% change threshold
			CompressionLevel:  6,
			BatchSize:         50,
			FlushInterval:     5 * time.Second,
			IncludeMetadata:   true,
			DeepCompare:       true,
			CompressThreshold: 1024, // 1KB
		}
	}

	engine := &DiffEmitEngine{
		stateStore:      make(map[string]*StateSnapshot),
		config:          config,
		metrics:         &DiffEmitMetrics{},
		compressionAlgo: CompressionGzip,
	}

	// Start background cleanup
	go engine.cleanupStates()

	return engine
}

// ComputeDiff computes the difference between current and previous state
func (de *DiffEmitEngine) ComputeDiff(key string, currentData interface{}) (*DiffResult, error) {
	start := time.Now()

	de.mu.Lock()
	defer de.mu.Unlock()

	// Update metrics
	de.metrics.TotalComparisons++

	// Get previous state
	previousState, exists := de.stateStore[key]

	// Create current state snapshot
	currentHash, err := de.computeHash(currentData)
	if err != nil {
		return nil, fmt.Errorf("failed to compute hash: %w", err)
	}

	currentSize := de.estimateSize(currentData)
	currentSnapshot := &StateSnapshot{
		Data:       currentData,
		Hash:       currentHash,
		Timestamp:  time.Now(),
		Version:    1,
		Size:       currentSize,
		ComputedAt: time.Now(),
	}

	if exists {
		currentSnapshot.Version = previousState.Version + 1
	}

	// Quick hash comparison
	if exists && previousState.Hash == currentHash {
		de.metrics.DiffEmissionsSkipped++
		return &DiffResult{
			HasChanges:      false,
			ChangePercent:   0.0,
			PreviousVersion: previousState.Version,
			NewVersion:      currentSnapshot.Version,
			ComputeTime:     time.Since(start),
		}, nil
	}

	// Compute detailed diff if hashes differ
	var diff interface{}
	var changePercent float64
	var changedFields, addedFields, removedFields []string

	if exists && de.config.DeepCompare {
		diffResult := de.computeDetailedDiff(previousState.Data, currentData)
		diff = diffResult.Diff
		changePercent = diffResult.ChangePercent
		changedFields = diffResult.ChangedFields
		addedFields = diffResult.AddedFields
		removedFields = diffResult.RemovedFields
	} else {
		// For new keys or when deep compare is disabled, send full data
		diff = currentData
		changePercent = 1.0
	}

	// Check if change meets threshold
	hasChanges := changePercent >= de.config.DiffThreshold

	result := &DiffResult{
		HasChanges:      hasChanges,
		ChangePercent:   changePercent,
		ChangedFields:   changedFields,
		AddedFields:     addedFields,
		RemovedFields:   removedFields,
		Diff:            diff,
		PreviousVersion: 0,
		NewVersion:      currentSnapshot.Version,
		ComputeTime:     time.Since(start),
		OriginalSize:    currentSize,
	}

	if exists {
		result.PreviousVersion = previousState.Version
	}

	// Apply compression if needed
	if hasChanges && currentSize >= de.config.CompressThreshold {
		compressedDiff, compressed := de.compressDiff(diff)
		if compressed {
			result.Diff = compressedDiff
			result.Compressed = true
			result.CompressedSize = de.estimateSize(compressedDiff)

			// Update compression metrics
			if result.CompressedSize > 0 {
				ratio := float64(result.CompressedSize) / float64(result.OriginalSize)
				de.updateCompressionMetrics(ratio)
			}
		}
	}

	// Store current state for future comparisons
	de.stateStore[key] = currentSnapshot

	// Update metrics
	if hasChanges {
		de.metrics.DiffEmissionsSent++
	} else {
		de.metrics.DiffEmissionsSkipped++
	}

	computeTimeMs := float64(time.Since(start).Nanoseconds()) / 1e6
	de.updateAvgComputeTime(computeTimeMs)

	return result, nil
}

// computeDetailedDiff performs detailed comparison between two objects
func (de *DiffEmitEngine) computeDetailedDiff(previous, current interface{}) *DiffResult {
	result := &DiffResult{
		ChangedFields: []string{},
		AddedFields:   []string{},
		RemovedFields: []string{},
	}

	// Convert to comparable format
	prevMap := de.toMap(previous)
	currMap := de.toMap(current)

	if prevMap == nil || currMap == nil {
		// If not maps, do simple comparison
		if !reflect.DeepEqual(previous, current) {
			result.Diff = current
			result.ChangePercent = 1.0
			result.ChangedFields = append(result.ChangedFields, "root")
		}
		return result
	}

	// Create diff map
	diffMap := make(map[string]interface{})
	allFields := make(map[string]bool)

	// Collect all field names
	for field := range prevMap {
		allFields[field] = true
	}
	for field := range currMap {
		allFields[field] = true
	}

	changedCount := 0
	totalFields := len(allFields)

	// Compare each field
	for field := range allFields {
		if de.shouldIgnoreField(field) {
			continue
		}

		prevVal, prevExists := prevMap[field]
		currVal, currExists := currMap[field]

		if !prevExists && currExists {
			// Field added
			result.AddedFields = append(result.AddedFields, field)
			diffMap[field] = map[string]interface{}{
				"action": "added",
				"value":  currVal,
			}
			changedCount++
		} else if prevExists && !currExists {
			// Field removed
			result.RemovedFields = append(result.RemovedFields, field)
			diffMap[field] = map[string]interface{}{
				"action": "removed",
				"value":  prevVal,
			}
			changedCount++
		} else if prevExists && currExists {
			// Field exists in both, check if changed
			if !reflect.DeepEqual(prevVal, currVal) {
				result.ChangedFields = append(result.ChangedFields, field)

				// For complex nested changes, provide detailed diff
				if de.isComplexType(currVal) {
					nestedDiff := de.computeNestedDiff(prevVal, currVal)
					diffMap[field] = map[string]interface{}{
						"action":   "modified",
						"previous": prevVal,
						"current":  currVal,
						"diff":     nestedDiff,
					}
				} else {
					diffMap[field] = map[string]interface{}{
						"action":   "modified",
						"previous": prevVal,
						"current":  currVal,
					}
				}
				changedCount++
			}
		}
	}

	// Calculate change percentage
	if totalFields > 0 {
		result.ChangePercent = float64(changedCount) / float64(totalFields)
	}

	result.Diff = diffMap
	return result
}

// computeNestedDiff handles nested object/array comparisons
func (de *DiffEmitEngine) computeNestedDiff(previous, current interface{}) interface{} {
	// Handle arrays
	if prevArray, ok := previous.([]interface{}); ok {
		if currArray, ok := current.([]interface{}); ok {
			return de.computeArrayDiff(prevArray, currArray)
		}
	}

	// Handle maps/objects
	if prevMap, ok := previous.(map[string]interface{}); ok {
		if currMap, ok := current.(map[string]interface{}); ok {
			return de.computeMapDiff(prevMap, currMap)
		}
	}

	// For primitive types or mixed types, return simple diff
	return map[string]interface{}{
		"previous": previous,
		"current":  current,
	}
}

// computeArrayDiff computes differences between arrays
func (de *DiffEmitEngine) computeArrayDiff(previous, current []interface{}) interface{} {
	diff := map[string]interface{}{
		"type":    "array",
		"changes": []interface{}{},
	}

	maxLen := len(previous)
	if len(current) > maxLen {
		maxLen = len(current)
	}

	changes := []interface{}{}

	for i := 0; i < maxLen; i++ {
		if i >= len(previous) {
			// Item added
			changes = append(changes, map[string]interface{}{
				"index":  i,
				"action": "added",
				"value":  current[i],
			})
		} else if i >= len(current) {
			// Item removed
			changes = append(changes, map[string]interface{}{
				"index":  i,
				"action": "removed",
				"value":  previous[i],
			})
		} else if !reflect.DeepEqual(previous[i], current[i]) {
			// Item modified
			changes = append(changes, map[string]interface{}{
				"index":    i,
				"action":   "modified",
				"previous": previous[i],
				"current":  current[i],
			})
		}
	}

	diff["changes"] = changes
	diff["length_change"] = len(current) - len(previous)

	return diff
}

// computeMapDiff computes differences between maps
func (de *DiffEmitEngine) computeMapDiff(previous, current map[string]interface{}) interface{} {
	diff := map[string]interface{}{
		"type":    "object",
		"changes": map[string]interface{}{},
	}

	changes := make(map[string]interface{})
	allKeys := make(map[string]bool)

	// Collect all keys
	for key := range previous {
		allKeys[key] = true
	}
	for key := range current {
		allKeys[key] = true
	}

	// Compare each key
	for key := range allKeys {
		prevVal, prevExists := previous[key]
		currVal, currExists := current[key]

		if !prevExists && currExists {
			changes[key] = map[string]interface{}{
				"action": "added",
				"value":  currVal,
			}
		} else if prevExists && !currExists {
			changes[key] = map[string]interface{}{
				"action": "removed",
				"value":  prevVal,
			}
		} else if !reflect.DeepEqual(prevVal, currVal) {
			changes[key] = map[string]interface{}{
				"action":   "modified",
				"previous": prevVal,
				"current":  currVal,
			}
		}
	}

	diff["changes"] = changes
	return diff
}

// Helper methods

func (de *DiffEmitEngine) computeHash(data interface{}) (string, error) {
	// Convert to JSON for consistent hashing
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	// Sort keys for consistent hashing
	var normalized interface{}
	if err := json.Unmarshal(jsonData, &normalized); err != nil {
		return "", err
	}

	normalizedData := de.normalizeForHashing(normalized)
	normalizedJSON, err := json.Marshal(normalizedData)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(normalizedJSON)
	return hex.EncodeToString(hash[:]), nil
}

func (de *DiffEmitEngine) normalizeForHashing(data interface{}) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		normalized := make(map[string]interface{})
		keys := make([]string, 0, len(v))

		// Sort keys for consistent ordering
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		for _, key := range keys {
			if !de.shouldIgnoreField(key) {
				normalized[key] = de.normalizeForHashing(v[key])
			}
		}
		return normalized

	case []interface{}:
		normalized := make([]interface{}, len(v))
		for i, item := range v {
			normalized[i] = de.normalizeForHashing(item)
		}
		return normalized

	default:
		return v
	}
}

func (de *DiffEmitEngine) shouldIgnoreField(field string) bool {
	for _, ignored := range de.config.IgnoreFields {
		if field == ignored {
			return true
		}
		// Support wildcard matching
		if strings.HasSuffix(ignored, "*") {
			prefix := strings.TrimSuffix(ignored, "*")
			if strings.HasPrefix(field, prefix) {
				return true
			}
		}
	}
	return false
}

func (de *DiffEmitEngine) toMap(data interface{}) map[string]interface{} {
	if m, ok := data.(map[string]interface{}); ok {
		return m
	}

	// Try to convert via JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil
	}

	var m map[string]interface{}
	if err := json.Unmarshal(jsonData, &m); err != nil {
		return nil
	}

	return m
}

func (de *DiffEmitEngine) isComplexType(data interface{}) bool {
	switch data.(type) {
	case map[string]interface{}, []interface{}, map[interface{}]interface{}:
		return true
	default:
		return false
	}
}

func (de *DiffEmitEngine) estimateSize(data interface{}) int {
	// Simple size estimation based on JSON serialization
	jsonData, err := json.Marshal(data)
	if err != nil {
		return 0
	}
	return len(jsonData)
}

func (de *DiffEmitEngine) compressDiff(diff interface{}) (interface{}, bool) {
	// Implementation depends on compression algorithm
	switch de.compressionAlgo {
	case CompressionDelta:
		return de.deltaCompress(diff)
	case CompressionGzip:
		return de.gzipCompress(diff)
	default:
		return diff, false
	}
}

func (de *DiffEmitEngine) deltaCompress(diff interface{}) (interface{}, bool) {
	// Delta compression for array-like data
	// This is a simplified implementation
	return diff, false
}

func (de *DiffEmitEngine) gzipCompress(diff interface{}) (interface{}, bool) {
	// GZIP compression implementation
	// This would use actual gzip compression in production
	return diff, false
}

func (de *DiffEmitEngine) updateCompressionMetrics(ratio float64) {
	de.metrics.mu.Lock()
	defer de.metrics.mu.Unlock()

	// Update rolling average
	de.metrics.CompressionRatio = (de.metrics.CompressionRatio + ratio) / 2
}

func (de *DiffEmitEngine) updateAvgComputeTime(timeMs float64) {
	de.metrics.mu.Lock()
	defer de.metrics.mu.Unlock()

	// Update rolling average
	de.metrics.AvgComputeTime = (de.metrics.AvgComputeTime + timeMs) / 2
}

// Background cleanup of old states
func (de *DiffEmitEngine) cleanupStates() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		de.performCleanup()
	}
}

func (de *DiffEmitEngine) performCleanup() {
	de.mu.Lock()
	defer de.mu.Unlock()

	if len(de.stateStore) <= de.config.MaxStateHistory {
		return
	}

	// Sort by timestamp and keep only recent states
	type stateEntry struct {
		key       string
		timestamp time.Time
	}

	var entries []stateEntry
	for key, state := range de.stateStore {
		entries = append(entries, stateEntry{
			key:       key,
			timestamp: state.Timestamp,
		})
	}

	// Sort by timestamp (newest first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].timestamp.After(entries[j].timestamp)
	})

	// Keep only the most recent entries
	toDelete := len(entries) - de.config.MaxStateHistory
	for i := de.config.MaxStateHistory; i < len(entries) && toDelete > 0; i++ {
		delete(de.stateStore, entries[i].key)
		toDelete--
	}

	// Update metrics
	de.metrics.mu.Lock()
	de.metrics.StateStoreSize = len(de.stateStore)
	de.metrics.mu.Unlock()
}

// GetMetrics returns current diff emit metrics
func (de *DiffEmitEngine) GetMetrics() *DiffEmitMetrics {
	de.metrics.mu.RLock()
	defer de.metrics.mu.RUnlock()

	// Return a copy
	metrics := *de.metrics
	return &metrics
}

// Reset clears all stored states and resets metrics
func (de *DiffEmitEngine) Reset() {
	de.mu.Lock()
	defer de.mu.Unlock()

	de.stateStore = make(map[string]*StateSnapshot)

	de.metrics.mu.Lock()
	de.metrics = &DiffEmitMetrics{}
	de.metrics.mu.Unlock()
}
