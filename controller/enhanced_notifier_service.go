package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

// AdapterStatus tracks the status of Socket.IO adapters
type AdapterStatus struct {
	Connected bool      `json:"connected"`
	LastPing  time.Time `json:"lastPing"`
	ErrorRate float64   `json:"errorRate"`
}

// P15: Enhanced NotifierService with namespaces and advanced backpressure
type EnhancedNotifierService struct {
	eventBus      *EventBus
	namespaces    map[string]*Namespace
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	observability *ObservabilityManager
	backpressure  *EnhancedBackpressureManager
	redisClient   *redis.Client
	adapterStatus *AdapterStatus
	healthChecker *HealthChecker
	diffEngine    *DiffEngine
	messageRouter *MessageRouter
}

// Namespace groups related rooms and provides isolation
type Namespace struct {
	Name           string                     `json:"name"`
	Rooms          map[string]*EnhancedRoom   `json:"rooms"`
	Clients        map[string]*EnhancedClient `json:"clients"`
	mu             sync.RWMutex
	Config         *NamespaceConfig      `json:"config"`
	Stats          *NamespaceStats       `json:"stats"`
	MessageFilters []MessageFilter       `json:"message_filters"`
	RateLimiter    *NamespaceRateLimiter `json:"-"`
	LoadBalancer   *LoadBalancer         `json:"-"`
}

// NamespaceConfig defines configuration for a namespace
type NamespaceConfig struct {
	MaxRooms         int           `json:"max_rooms"`
	MaxClientsTotal  int           `json:"max_clients_total"`
	DefaultQPS       int           `json:"default_qps"`
	MaxQPS           int           `json:"max_qps"`
	DropStrategy     DropStrategy  `json:"drop_strategy"`
	MergeStrategy    MergeStrategy `json:"merge_strategy"`
	EnableDiffEmit   bool          `json:"enable_diff_emit"`
	CompressionLevel int           `json:"compression_level"`
	Middlewares      []string      `json:"middlewares"`
}

// EnhancedRoom with advanced backpressure and diff-emit capabilities
type EnhancedRoom struct {
	Name            string                     `json:"name"`
	Namespace       string                     `json:"namespace"`
	Clients         map[string]*EnhancedClient `json:"clients"`
	mu              sync.RWMutex
	Config          *RoomConfig                 `json:"config"`
	Stats           *EnhancedRoomStats          `json:"stats"`
	MessageQueue    *PriorityMessageQueue       `json:"-"`
	DiffEmitter     *DiffEmitter                `json:"-"`
	FilterManager   *FilterManager              `json:"-"`
	BackpressureCtl *RoomBackpressureController `json:"-"`
	CreatedAt       time.Time                   `json:"created_at"`
	LastActivity    time.Time                   `json:"last_activity"`
	IsHighFanout    bool                        `json:"is_high_fanout"`
}

// RoomConfig defines room-specific configuration
type RoomConfig struct {
	MaxClients      int           `json:"max_clients"`
	QPSLimit        int           `json:"qps_limit"`
	BurstLimit      int           `json:"burst_limit"`
	DropStrategy    DropStrategy  `json:"drop_strategy"`
	MergeStrategy   MergeStrategy `json:"merge_strategy"`
	EnableDiffEmit  bool          `json:"enable_diff_emit"`
	MessageTTL      time.Duration `json:"message_ttl"`
	PriorityEnabled bool          `json:"priority_enabled"`
	CompressionType string        `json:"compression_type"`
}

// EnhancedClient with filter and compression support
type EnhancedClient struct {
	ID               string                `json:"id"`
	Conn             *websocket.Conn       `json:"-"`
	Namespace        string                `json:"namespace"`
	Rooms            map[string]bool       `json:"rooms"`
	Send             chan *PriorityMessage `json:"-"`
	mu               sync.Mutex
	lastPing         time.Time              `json:"last_ping"`
	Capabilities     map[string]interface{} `json:"capabilities"`
	Filters          []*ClientFilter        `json:"filters"`
	CompressionLevel int                    `json:"compression_level"`
	Priority         ClientPriority         `json:"priority"`
	RateLimiter      *ClientRateLimiter     `json:"-"`
	Stats            *ClientStats           `json:"stats"`
	NodeID           string                 `json:"node_id"`
	UserAgent        string                 `json:"user_agent"`
	IPAddress        string                 `json:"ip_address"`
}

// Strategy types for handling backpressure
type DropStrategy string
type MergeStrategy string
type ClientPriority int

const (
	// Drop strategies
	DropOldest DropStrategy = "drop_oldest"
	DropNewest DropStrategy = "drop_newest"
	DropLowest DropStrategy = "drop_lowest_priority"
	DropRandom DropStrategy = "drop_random"
	DropNone   DropStrategy = "drop_none"

	// Merge strategies
	MergeByType           MergeStrategy = "merge_by_type"
	MergeByKey            MergeStrategy = "merge_by_key"
	MergeNone             MergeStrategy = "merge_none"
	MergeAggregateMetrics MergeStrategy = "merge_aggregate_metrics"

	// Client priorities
	PriorityLow      ClientPriority = 1
	PriorityNormal   ClientPriority = 2
	PriorityHigh     ClientPriority = 3
	PriorityCritical ClientPriority = 4
)

// PriorityMessage represents a message with priority and metadata
type PriorityMessage struct {
	Type       string                 `json:"type"`
	Data       interface{}            `json:"data"`
	Priority   int                    `json:"priority"`
	Timestamp  time.Time              `json:"timestamp"`
	Room       string                 `json:"room"`
	MessageID  string                 `json:"message_id"`
	Metadata   map[string]interface{} `json:"metadata"`
	ExpiresAt  *time.Time             `json:"expires_at,omitempty"`
	Compressed bool                   `json:"compressed"`
	IsDiff     bool                   `json:"is_diff"`
}

// DiffEmitter tracks state changes and emits only differences
type DiffEmitter struct {
	mu           sync.RWMutex
	lastStates   map[string]interface{}
	filters      []*DiffFilter
	enabled      bool
	maxStateSize int
}

// DiffFilter defines what changes to track
type DiffFilter struct {
	Path    string      `json:"path"`
	Type    string      `json:"type"` // "property", "array", "object"
	Options DiffOptions `json:"options"`
}

// DiffOptions configures diff behavior
type DiffOptions struct {
	IgnoreOrder  bool     `json:"ignore_order"`
	IgnoreFields []string `json:"ignore_fields"`
	Threshold    float64  `json:"threshold"`
	DeepCompare  bool     `json:"deep_compare"`
}

// PriorityMessageQueue implements a priority queue for messages
type PriorityMessageQueue struct {
	messages []*PriorityMessage
	mu       sync.RWMutex
	maxSize  int
	strategy DropStrategy
}

// ClientFilter defines filtering criteria for messages
type ClientFilter struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Conditions map[string]interface{} `json:"conditions"`
	Action     string                 `json:"action"` // "include", "exclude", "transform"
	Priority   int                    `json:"priority"`
	Enabled    bool                   `json:"enabled"`
}

// FilterManager manages client filters
type FilterManager struct {
	filters map[string][]*ClientFilter
	mu      sync.RWMutex
}

// RoomBackpressureController manages room-level backpressure
type RoomBackpressureController struct {
	mu          sync.RWMutex
	currentLoad float64
	lastCheck   time.Time
	qpsWindow   []time.Time
	dropCount   int64
	mergeCount  int64
	enabled     bool
	thresholds  BackpressureThresholds
}

// BackpressureThresholds define when to activate different strategies
type BackpressureThresholds struct {
	WarningLoad    float64 `json:"warning_load"`
	CriticalLoad   float64 `json:"critical_load"`
	EmergencyLoad  float64 `json:"emergency_load"`
	DropThreshold  float64 `json:"drop_threshold"`
	MergeThreshold float64 `json:"merge_threshold"`
}

// Enhanced statistics structures
type NamespaceStats struct {
	TotalClients    int64         `json:"total_clients"`
	TotalRooms      int64         `json:"total_rooms"`
	MessagesSent    int64         `json:"messages_sent"`
	MessagesDropped int64         `json:"messages_dropped"`
	MessagesMerged  int64         `json:"messages_merged"`
	AverageLatency  time.Duration `json:"average_latency"`
	PeakConcurrency int           `json:"peak_concurrency"`
	LastReset       time.Time     `json:"last_reset"`
}

type EnhancedRoomStats struct {
	MessagesSent       int64         `json:"messages_sent"`
	MessagesDropped    int64         `json:"messages_dropped"`
	MessagesMerged     int64         `json:"messages_merged"`
	DiffMessagesSent   int64         `json:"diff_messages_sent"`
	AvgLatency         time.Duration `json:"avg_latency"`
	PeakClients        int           `json:"peak_clients"`
	CurrentLoad        float64       `json:"current_load"`
	BackpressureEvents int64         `json:"backpressure_events"`
	LastReset          time.Time     `json:"last_reset"`
	CompressionRatio   float64       `json:"compression_ratio"`
}

type ClientStats struct {
	MessagesReceived   int64         `json:"messages_received"`
	MessagesFiltered   int64         `json:"messages_filtered"`
	DiffMessagesRecv   int64         `json:"diff_messages_received"`
	AverageLatency     time.Duration `json:"average_latency"`
	ConnectionUptime   time.Duration `json:"connection_uptime"`
	CompressionSavings int64         `json:"compression_savings"`
	FilterHitRate      float64       `json:"filter_hit_rate"`
}

// MessageRouter handles intelligent message routing
type MessageRouter struct {
	routes map[string]*RouteConfig
	mu     sync.RWMutex
}

type RouteConfig struct {
	Pattern   string            `json:"pattern"`
	Namespace string            `json:"namespace"`
	Room      string            `json:"room"`
	Filters   []*MessageFilter  `json:"filters"`
	Transform *MessageTransform `json:"transform"`
	RateLimit int               `json:"rate_limit"`
	Priority  int               `json:"priority"`
}

type MessageFilter struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"` // "eq", "ne", "contains", "regex"
	Value    interface{} `json:"value"`
}

type MessageTransform struct {
	Type   string                 `json:"type"` // "modify", "aggregate", "compress"
	Config map[string]interface{} `json:"config"`
}

// LoadBalancer distributes clients across nodes
type LoadBalancer struct {
	strategy string
	nodes    []string
	weights  map[string]int
	mu       sync.RWMutex
}

// NewEnhancedNotifierService creates a new enhanced notifier service
func NewEnhancedNotifierService(eventBus *EventBus, observability *ObservabilityManager, redisURL string) *EnhancedNotifierService {
	ctx, cancel := context.WithCancel(context.Background())

	// Initialize Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisURL,
		Password: "",
		DB:       0,
		PoolSize: 20,
	})

	ns := &EnhancedNotifierService{
		eventBus:      eventBus,
		namespaces:    make(map[string]*Namespace),
		ctx:           ctx,
		cancel:        cancel,
		observability: observability,
		backpressure:  NewEnhancedBackpressureManager(),
		redisClient:   redisClient,
		diffEngine:    NewDiffEngine(),
		messageRouter: NewMessageRouter(),
	}

	// Create default namespaces
	ns.createDefaultNamespaces()

	// Start background services
	go ns.backpressureMonitor()
	go ns.cleanupRoutine()
	go ns.statsCollector()
	go ns.diffStateCleanup()

	return ns
}

// createDefaultNamespaces creates standard namespaces
func (ns *EnhancedNotifierService) createDefaultNamespaces() {
	defaultNamespaces := []string{
		"experiments", // Experiment updates
		"metrics",     // Real-time metrics
		"logs",        // Log streaming
		"alerts",      // Alert notifications
		"admin",       // Administrative messages
	}

	for _, name := range defaultNamespaces {
		config := &NamespaceConfig{
			MaxRooms:         1000,
			MaxClientsTotal:  10000,
			DefaultQPS:       100,
			MaxQPS:           1000,
			DropStrategy:     DropOldest,
			MergeStrategy:    MergeByType,
			EnableDiffEmit:   true,
			CompressionLevel: 1,
		}

		ns.namespaces[name] = &Namespace{
			Name:         name,
			Rooms:        make(map[string]*EnhancedRoom),
			Clients:      make(map[string]*EnhancedClient),
			Config:       config,
			Stats:        &NamespaceStats{LastReset: time.Now()},
			RateLimiter:  NewNamespaceRateLimiter(config.MaxQPS),
			LoadBalancer: NewLoadBalancer("round_robin"),
		}
	}
}

// JoinNamespaceRoom adds a client to a namespaced room
func (ns *EnhancedNotifierService) JoinNamespaceRoom(clientID, namespace, roomName string, filters []*ClientFilter) error {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	// Get or create namespace
	nsObj, exists := ns.namespaces[namespace]
	if !exists {
		return fmt.Errorf("namespace %s not found", namespace)
	}

	nsObj.mu.Lock()
	defer nsObj.mu.Unlock()

	// Check namespace limits
	if len(nsObj.Clients) >= nsObj.Config.MaxClientsTotal {
		return fmt.Errorf("namespace %s at client capacity", namespace)
	}

	// Get client
	client, exists := nsObj.Clients[clientID]
	if !exists {
		return fmt.Errorf("client %s not found in namespace %s", clientID, namespace)
	}

	// Get or create room
	room, exists := nsObj.Rooms[roomName]
	if !exists {
		if len(nsObj.Rooms) >= nsObj.Config.MaxRooms {
			return fmt.Errorf("namespace %s at room capacity", namespace)
		}

		room = ns.createEnhancedRoom(roomName, namespace, nsObj.Config)
		nsObj.Rooms[roomName] = room
	}

	// Check room capacity and backpressure
	room.mu.Lock()
	defer room.mu.Unlock()

	if len(room.Clients) >= room.Config.MaxClients {
		return fmt.Errorf("room %s at capacity", roomName)
	}

	if room.BackpressureCtl.enabled && room.BackpressureCtl.currentLoad > room.BackpressureCtl.thresholds.CriticalLoad {
		return fmt.Errorf("room %s under high load, rejecting new clients", roomName)
	}

	// Add client to room
	room.Clients[clientID] = client
	client.Rooms[roomName] = true

	// Apply filters
	if len(filters) > 0 {
		client.Filters = append(client.Filters, filters...)
		room.FilterManager.AddClientFilters(clientID, filters)
	}

	// Update statistics
	room.Stats.PeakClients = max(room.Stats.PeakClients, len(room.Clients))
	room.LastActivity = time.Now()

	log.Printf("[EnhancedNotifier] Client %s joined %s/%s with %d filters",
		clientID, namespace, roomName, len(filters))

	return nil
}

// BroadcastToNamespaceRoom sends a message with advanced features
func (ns *EnhancedNotifierService) BroadcastToNamespaceRoom(namespace, roomName, messageType string, data interface{}, options *BroadcastOptions) error {
	ns.mu.RLock()
	nsObj, exists := ns.namespaces[namespace]
	ns.mu.RUnlock()

	if !exists {
		return fmt.Errorf("namespace %s not found", namespace)
	}

	nsObj.mu.RLock()
	room, exists := nsObj.Rooms[roomName]
	nsObj.mu.RUnlock()

	if !exists {
		return fmt.Errorf("room %s not found in namespace %s", roomName, namespace)
	}

	// Check namespace rate limit
	if !nsObj.RateLimiter.Allow() {
		nsObj.Stats.MessagesDropped++
		return fmt.Errorf("namespace %s rate limit exceeded", namespace)
	}

	// Check room backpressure
	room.mu.RLock()
	if room.BackpressureCtl.enabled {
		currentLoad := room.BackpressureCtl.getCurrentLoad()
		if currentLoad > room.BackpressureCtl.thresholds.EmergencyLoad {
			room.mu.RUnlock()
			room.Stats.MessagesDropped++
			return fmt.Errorf("room %s under emergency load", roomName)
		}
	}
	clients := make([]*EnhancedClient, 0, len(room.Clients))
	for _, client := range room.Clients {
		clients = append(clients, client)
	}
	room.mu.RUnlock()

	// Create priority message
	msg := &PriorityMessage{
		Type:      messageType,
		Data:      data,
		Priority:  options.Priority,
		Timestamp: time.Now(),
		Room:      roomName,
		MessageID: generateMessageID(),
		Metadata:  options.Metadata,
	}

	if options.TTL > 0 {
		expiresAt := time.Now().Add(options.TTL)
		msg.ExpiresAt = &expiresAt
	}

	// Apply diff emit if enabled
	if room.Config.EnableDiffEmit && room.DiffEmitter.enabled {
		diffMsg, isDiff := room.DiffEmitter.EmitDiff(messageType, data)
		if isDiff {
			msg.Data = diffMsg
			msg.IsDiff = true
			room.Stats.DiffMessagesSent++
		}
	}

	// Handle backpressure strategies
	if room.BackpressureCtl.enabled {
		currentLoad := room.BackpressureCtl.getCurrentLoad()

		if currentLoad > room.BackpressureCtl.thresholds.MergeThreshold && room.Config.MergeStrategy != MergeNone {
			if merged := room.tryMergeMessage(msg); merged {
				room.Stats.MessagesMerged++
				return nil
			}
		}

		if currentLoad > room.BackpressureCtl.thresholds.DropThreshold && room.Config.DropStrategy != DropNone {
			if room.MessageQueue.IsFull() {
				room.handleDrop(msg)
				room.Stats.MessagesDropped++
				return nil
			}
		}
	}

	// Send to clients
	sent := 0
	for _, client := range clients {
		if ns.shouldSendToClient(client, msg) {
			// Apply compression if supported
			finalMsg := msg
			if client.CompressionLevel > 0 && options.AllowCompression {
				finalMsg = ns.compressMessage(msg, client.CompressionLevel)
			}

			select {
			case client.Send <- finalMsg:
				sent++
				client.Stats.MessagesReceived++
			default:
				// Client buffer full
				if client.Priority >= PriorityHigh {
					// For high priority clients, try to make room
					select {
					case <-client.Send: // Drop one message
						client.Send <- finalMsg
						sent++
					default:
						// Still full, log warning
						log.Printf("[EnhancedNotifier] High priority client %s buffer full", client.ID)
					}
				}
			}
		} else {
			client.Stats.MessagesFiltered++
		}
	}

	// Update statistics
	room.Stats.MessagesSent++
	nsObj.Stats.MessagesSent++
	room.LastActivity = time.Now()

	// Publish to Redis for horizontal scaling
	if ns.redisClient != nil {
		ns.publishToRedis(namespace, roomName, msg)
	}

	log.Printf("[EnhancedNotifier] Broadcast to %s/%s: sent to %d clients", namespace, roomName, sent)
	return nil
}

// BroadcastOptions configures broadcast behavior
type BroadcastOptions struct {
	Priority         int                    `json:"priority"`
	TTL              time.Duration          `json:"ttl"`
	Metadata         map[string]interface{} `json:"metadata"`
	AllowCompression bool                   `json:"allow_compression"`
	RequireDelivery  bool                   `json:"require_delivery"`
	FilterClients    []*ClientFilter        `json:"filter_clients"`
}

// createEnhancedRoom creates a new enhanced room with default configuration
func (ns *EnhancedNotifierService) createEnhancedRoom(name, namespace string, nsConfig *NamespaceConfig) *EnhancedRoom {
	room := &EnhancedRoom{
		Name:         name,
		Namespace:    namespace,
		Clients:      make(map[string]*EnhancedClient),
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		Config: &RoomConfig{
			MaxClients:      1000,
			QPSLimit:        nsConfig.DefaultQPS,
			BurstLimit:      nsConfig.DefaultQPS * 2,
			DropStrategy:    nsConfig.DropStrategy,
			MergeStrategy:   nsConfig.MergeStrategy,
			EnableDiffEmit:  nsConfig.EnableDiffEmit,
			MessageTTL:      5 * time.Minute,
			PriorityEnabled: true,
			CompressionType: "gzip",
		},
		Stats: &EnhancedRoomStats{
			LastReset: time.Now(),
		},
		MessageQueue:    NewPriorityMessageQueue(1000, nsConfig.DropStrategy),
		DiffEmitter:     NewDiffEmitter(nsConfig.EnableDiffEmit),
		FilterManager:   NewFilterManager(),
		BackpressureCtl: NewRoomBackpressureController(),
	}

	// Determine if this is a high fanout room based on name patterns
	highFanoutPatterns := []string{"metrics", "logs", "broadcast", "global"}
	for _, pattern := range highFanoutPatterns {
		if strings.Contains(strings.ToLower(name), pattern) {
			room.IsHighFanout = true
			room.Config.MaxClients = 10000
			room.Config.EnableDiffEmit = true
			break
		}
	}

	return room
}

// shouldSendToClient determines if a message should be sent to a specific client
func (ns *EnhancedNotifierService) shouldSendToClient(client *EnhancedClient, msg *PriorityMessage) bool {
	// Check message expiry
	if msg.ExpiresAt != nil && time.Now().After(*msg.ExpiresAt) {
		return false
	}

	// Apply client filters
	for _, filter := range client.Filters {
		if !filter.Enabled {
			continue
		}

		if !ns.applyFilter(filter, msg) {
			return false
		}
	}

	// Check client rate limit
	if client.RateLimiter != nil && !client.RateLimiter.Allow() {
		return false
	}

	return true
}

// applyFilter applies a client filter to a message
func (ns *EnhancedNotifierService) applyFilter(filter *ClientFilter, msg *PriorityMessage) bool {
	switch filter.Type {
	case "message_type":
		if expectedType, ok := filter.Conditions["type"].(string); ok {
			return msg.Type == expectedType
		}
	case "priority":
		if minPriority, ok := filter.Conditions["min_priority"].(float64); ok {
			return float64(msg.Priority) >= minPriority
		}
	case "room":
		if expectedRoom, ok := filter.Conditions["room"].(string); ok {
			return msg.Room == expectedRoom
		}
	case "custom":
		// Custom filter logic based on conditions
		return ns.evaluateCustomFilter(filter.Conditions, msg)
	}

	return true
}

// evaluateCustomFilter evaluates custom filter conditions
func (ns *EnhancedNotifierService) evaluateCustomFilter(conditions map[string]interface{}, msg *PriorityMessage) bool {
	// Implement custom filter logic
	// This is a simplified example
	for key, expectedValue := range conditions {
		if actualValue, exists := msg.Metadata[key]; exists {
			if actualValue != expectedValue {
				return false
			}
		}
	}
	return true
}

// GetNamespaceStats returns statistics for a specific namespace
func (ns *EnhancedNotifierService) GetNamespaceStats(namespace string) (*NamespaceStats, error) {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	nsObj, exists := ns.namespaces[namespace]
	if !exists {
		return nil, fmt.Errorf("namespace %s not found", namespace)
	}

	nsObj.mu.RLock()
	defer nsObj.mu.RUnlock()

	// Update current statistics
	stats := *nsObj.Stats
	stats.TotalClients = int64(len(nsObj.Clients))
	stats.TotalRooms = int64(len(nsObj.Rooms))

	return &stats, nil
}

// GetRoomStats returns statistics for a specific room
func (ns *EnhancedNotifierService) GetRoomStats(namespace, roomName string) (*EnhancedRoomStats, error) {
	ns.mu.RLock()
	nsObj, exists := ns.namespaces[namespace]
	ns.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("namespace %s not found", namespace)
	}

	nsObj.mu.RLock()
	room, exists := nsObj.Rooms[roomName]
	nsObj.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("room %s not found in namespace %s", roomName, namespace)
	}

	room.mu.RLock()
	defer room.mu.RUnlock()

	stats := *room.Stats
	stats.CurrentLoad = room.BackpressureCtl.getCurrentLoad()

	return &stats, nil
}

// Background monitoring and cleanup routines

func (ns *EnhancedNotifierService) backpressureMonitor() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ns.updateBackpressureMetrics()
		case <-ns.ctx.Done():
			return
		}
	}
}

func (ns *EnhancedNotifierService) updateBackpressureMetrics() {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	for _, nsObj := range ns.namespaces {
		nsObj.mu.RLock()
		for _, room := range nsObj.Rooms {
			room.BackpressureCtl.updateLoad()
		}
		nsObj.mu.RUnlock()
	}
}

func (ns *EnhancedNotifierService) cleanupRoutine() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ns.cleanupExpiredMessages()
			ns.cleanupStaleClients()
			ns.cleanupEmptyRooms()
		case <-ns.ctx.Done():
			return
		}
	}
}

func (ns *EnhancedNotifierService) statsCollector() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ns.collectAndReportStats()
		case <-ns.ctx.Done():
			return
		}
	}
}

func (ns *EnhancedNotifierService) diffStateCleanup() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ns.cleanupDiffStates()
		case <-ns.ctx.Done():
			return
		}
	}
}

// Helper functions for new components

func NewEnhancedBackpressureManager() *EnhancedBackpressureManager {
	// Implementation placeholder
	return &EnhancedBackpressureManager{}
}

func NewDiffEngine() *DiffEngine {
	// Implementation placeholder
	return &DiffEngine{}
}

func NewMessageRouter() *MessageRouter {
	return &MessageRouter{
		routes: make(map[string]*RouteConfig),
	}
}

func NewNamespaceRateLimiter(maxQPS int) *NamespaceRateLimiter {
	// Implementation placeholder
	return &NamespaceRateLimiter{}
}

func NewLoadBalancer(strategy string) *LoadBalancer {
	return &LoadBalancer{
		strategy: strategy,
		nodes:    []string{},
		weights:  make(map[string]int),
	}
}

func NewPriorityMessageQueue(maxSize int, strategy DropStrategy) *PriorityMessageQueue {
	return &PriorityMessageQueue{
		messages: make([]*PriorityMessage, 0, maxSize),
		maxSize:  maxSize,
		strategy: strategy,
	}
}

func NewDiffEmitter(enabled bool) *DiffEmitter {
	return &DiffEmitter{
		lastStates:   make(map[string]interface{}),
		filters:      []*DiffFilter{},
		enabled:      enabled,
		maxStateSize: 1000,
	}
}

func NewFilterManager() *FilterManager {
	return &FilterManager{
		filters: make(map[string][]*ClientFilter),
	}
}

func NewRoomBackpressureController() *RoomBackpressureController {
	return &RoomBackpressureController{
		qpsWindow: make([]time.Time, 0, 100),
		enabled:   true,
		thresholds: BackpressureThresholds{
			WarningLoad:    0.7,
			CriticalLoad:   0.85,
			EmergencyLoad:  0.95,
			DropThreshold:  0.8,
			MergeThreshold: 0.75,
		},
	}
}

// Placeholder type definitions for compilation
type EnhancedBackpressureManager struct{}
type DiffEngine struct{}
type NamespaceRateLimiter struct{}
type ClientRateLimiter struct{}

// Stub implementations
func (rbpc *RoomBackpressureController) getCurrentLoad() float64     { return rbpc.currentLoad }
func (rbpc *RoomBackpressureController) updateLoad()                 { rbpc.currentLoad = 0.5 }
func (nrl *NamespaceRateLimiter) Allow() bool                        { return true }
func (crl *ClientRateLimiter) Allow() bool                           { return true }
func (room *EnhancedRoom) tryMergeMessage(msg *PriorityMessage) bool { return false }
func (room *EnhancedRoom) handleDrop(msg *PriorityMessage)           {}
func (pmq *PriorityMessageQueue) IsFull() bool                       { return len(pmq.messages) >= pmq.maxSize }
func (de *DiffEmitter) EmitDiff(msgType string, data interface{}) (interface{}, bool) {
	return data, false
}
func (fm *FilterManager) AddClientFilters(clientID string, filters []*ClientFilter) {}
func (ns *EnhancedNotifierService) compressMessage(msg *PriorityMessage, level int) *PriorityMessage {
	return msg
}
func (ns *EnhancedNotifierService) publishToRedis(namespace, room string, msg *PriorityMessage) {}
func (ns *EnhancedNotifierService) cleanupExpiredMessages()                                     {}
func (ns *EnhancedNotifierService) cleanupStaleClients()                                        {}
func (ns *EnhancedNotifierService) cleanupEmptyRooms()                                          {}
func (ns *EnhancedNotifierService) collectAndReportStats()                                      {}
func (ns *EnhancedNotifierService) cleanupDiffStates()                                          {}

func generateMessageID() string {
	return fmt.Sprintf("msg_%d", time.Now().UnixNano())
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
