package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	// Prometheus metrics for the controller.
	experimentCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "controller_experiment_total",
			Help: "Total number of experiments started by the controller",
		},
	)
	experimentDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "controller_experiment_duration_seconds",
			Help:    "Histogram of experiment durations",
			Buckets: prometheus.LinearBuckets(5, 5, 10),
		},
	)
)

func init() {
	prometheus.MustRegister(experimentCounter)
	prometheus.MustRegister(experimentDuration)
}

// ExperimentRequest represents the payload for starting an experiment.
type ExperimentRequest struct {
	Name           string `json:"name" validate:"required,min=1,max=100"`
	Description    string `json:"description" validate:"max=500"`
	ExperimentType string `json:"experiment_type" validate:"required,experiment_type"`
	Target         string `json:"target" validate:"required,min=1,max=100"`
	Duration       int    `json:"duration" validate:"required,positive_duration,min=1,max=3600"` // seconds
	DelayMs        int    `json:"delay_ms" validate:"min=0,max=10000"`                           // network latency
	LossPercent    int    `json:"loss_percent" validate:"min=0,max=100"`                         // packet loss
	CPUWorkers     int    `json:"cpu_workers" validate:"min=0,max=32"`
	MemSizeMB      int    `json:"mem_size_mb" validate:"min=0,max=16384"`
	KillProcess    string `json:"kill_process" validate:"max=100"`
	// Scheduling
	StartTime  time.Time `json:"start_time"`                           // optional, for scheduling
	Parallel   bool      `json:"parallel"`                             // run multiple agents in parallel?
	AgentCount int       `json:"agent_count" validate:"min=1,max=100"` // how many agents to target in parallel?
}

// We’ll store experiments in memory for demonstration purposes.
var experimentList = make([]ExperimentRequest, 0)
var listMutex sync.Mutex

// registerHandlers sets up the HTTP endpoints.
func registerHandlers(mux *http.ServeMux, healthChecker *HealthChecker) {
	mux.HandleFunc("/start", startExperimentHandler)
	mux.HandleFunc("/stop", stopExperimentHandler)
	mux.HandleFunc("/experiments", experimentsHandler)
	mux.HandleFunc("/healthz", healthChecker.HealthzHandler)
	mux.HandleFunc("/readyz", healthChecker.ReadyzHandler)
	mux.HandleFunc("/metrics-info", healthChecker.MetricsHandler)
}

// startExperimentHandler handles the start experiment request.
func startExperimentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Unable to read request", http.StatusBadRequest)
		return
	}

	var expReq ExperimentRequest
	if err := json.Unmarshal(body, &expReq); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "invalid_json",
			"message": "Request body must be valid JSON",
			"details": err.Error(),
		})
		return
	}

	// Validate request using middleware validator
	if validator, ok := r.Context().Value("validator").(*ValidationMiddleware); ok {
		if validationErr := validator.Validate(expReq); validationErr != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(validationErr)
			return
		}
	}

	log.Printf("[Controller] Received experiment request: %+v", expReq)

	// Save to experiment list (demo only).
	listMutex.Lock()
	experimentList = append(experimentList, expReq)
	listMutex.Unlock()

	// If a start_time is specified in the future, schedule the experiment.
	if !expReq.StartTime.IsZero() && expReq.StartTime.After(time.Now()) {
		delay := time.Until(expReq.StartTime)
		log.Printf("[Controller] Scheduling experiment '%s' to start in %v", expReq.Name, delay)
		go func(req ExperimentRequest) {
			time.Sleep(delay)
			dispatchExperiment(req)
		}(expReq)
	} else {
		// Otherwise, start immediately
		go dispatchExperiment(expReq)
	}

	response := map[string]string{
		"status":  "scheduled",
		"message": "Experiment scheduled (or started immediately).",
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// getAgentEndpoints returns a slice of agent endpoints by reading the environment variable.
func getAgentEndpoints() []string {
	endpointsStr := os.Getenv("AGENT_ENDPOINTS")
	if endpointsStr != "" {
		// Split by comma and trim any extra whitespace.
		endpoints := strings.Split(endpointsStr, ",")
		for i, ep := range endpoints {
			endpoints[i] = strings.TrimSpace(ep)
		}
		return endpoints
	}
	// Default to localhost if no environment variable is set.
	return []string{"http://localhost:9090/inject"}
}

// dispatchExperiment sends the injection request to one or more agents.
func dispatchExperiment(expReq ExperimentRequest) {
	log.Printf("[Controller] Dispatching experiment '%s' to agent(s)...", expReq.Name)

	// Get agent endpoints (could be across clusters)
	agentEndpoints := getAgentEndpoints()

	// Prepare JSON for the agent.
	jsonData, err := json.Marshal(expReq)
	if err != nil {
		log.Printf("[Controller] Error marshalling experiment request: %v", err)
		return
	}

	// Dispatch to agents in parallel if requested.
	if expReq.Parallel && expReq.AgentCount > 1 {
		var wg sync.WaitGroup
		for i := 0; i < expReq.AgentCount; i++ {
			// Cycle through the list if AgentCount > len(agentEndpoints).
			agentURL := agentEndpoints[i%len(agentEndpoints)]
			wg.Add(1)
			go func(url string) {
				defer wg.Done()
				makeAgentRequest(url, jsonData)
			}(agentURL)
		}
		wg.Wait()
	} else {
		// Single or immediate dispatch.
		agentURL := agentEndpoints[0]
		makeAgentRequest(agentURL, jsonData)
	}
}

func makeAgentRequest(agentURL string, data []byte) {
	resp, err := http.Post(agentURL, "application/json", bytes.NewBuffer(data))
	if err != nil {
		log.Printf("[Controller] Error sending injection request to Agent at %s: %v", agentURL, err)
		return
	}
	defer resp.Body.Close()
	log.Printf("[Controller] Agent at %s responded with: %s", agentURL, resp.Status)
}

// stopExperimentHandler handles the stop experiment request (stubbed).
func stopExperimentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	// TODO: Add logic to stop or cancel a running experiment.
	log.Println("[Controller] Stop experiment endpoint called (not implemented).")
	response := map[string]string{
		"status":  "stopped",
		"message": "Stop experiment is not implemented yet.",
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// experimentsHandler returns the list of scheduled or completed experiments.
func experimentsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	listMutex.Lock()
	defer listMutex.Unlock()
	json.NewEncoder(w).Encode(experimentList)
}
