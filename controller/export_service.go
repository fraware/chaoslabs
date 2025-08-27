package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// ExportService handles data export and eDiscovery operations
type ExportService struct {
	storage       ExportStorage
	crypto        CryptoManager
	observability *ObservabilityManager
	jobs          map[string]*ExportJob
	mu            sync.RWMutex
}

// ExportJob represents an export operation
type ExportJob struct {
	ID          string                 `json:"id"`
	UserID      string                 `json:"user_id"`
	Status      ExportStatus           `json:"status"`
	Format      ExportFormat           `json:"format"`
	Filters     ExportFilters          `json:"filters"`
	CreatedAt   time.Time              `json:"created_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Progress    float64                `json:"progress"`
	TotalSize   int64                  `json:"total_size"`
	ChunkCount  int                    `json:"chunk_count"`
	Signature   string                 `json:"signature"`
	MerkleRoot  string                 `json:"merkle_root"`
	ManifestURL string                 `json:"manifest_url"`
	Error       string                 `json:"error,omitempty"`
	Metadata    map[string]interface{} `json:"metadata"`
	ExpiresAt   time.Time              `json:"expires_at"`
}

// ExportStatus represents the status of an export job
type ExportStatus string

const (
	ExportStatusPending    ExportStatus = "pending"
	ExportStatusProcessing ExportStatus = "processing"
	ExportStatusCompleted  ExportStatus = "completed"
	ExportStatusFailed     ExportStatus = "failed"
	ExportStatusExpired    ExportStatus = "expired"
)

// ExportFormat represents supported export formats
type ExportFormat string

const (
	ExportFormatNDJSON  ExportFormat = "ndjson"
	ExportFormatParquet ExportFormat = "parquet"
	ExportFormatCSV     ExportFormat = "csv"
	ExportFormatZIP     ExportFormat = "zip"
)

// ExportFilters defines filtering criteria for exports
type ExportFilters struct {
	StartDate      *time.Time `json:"start_date,omitempty"`
	EndDate        *time.Time `json:"end_date,omitempty"`
	ExperimentType string     `json:"experiment_type,omitempty"`
	Status         string     `json:"status,omitempty"`
	Target         string     `json:"target,omitempty"`
	UserID         string     `json:"user_id,omitempty"`
	Tags           []string   `json:"tags,omitempty"`
}

// ExportManifest contains export metadata and verification info
type ExportManifest struct {
	JobID                    string                 `json:"job_id"`
	CreatedAt                time.Time              `json:"created_at"`
	Format                   ExportFormat           `json:"format"`
	Filters                  ExportFilters          `json:"filters"`
	TotalRecords             int64                  `json:"total_records"`
	TotalSize                int64                  `json:"total_size"`
	ChunkCount               int                    `json:"chunk_count"`
	Signature                string                 `json:"signature"`
	MerkleRoot               string                 `json:"merkle_root"`
	Files                    []ExportFileInfo       `json:"files"`
	Metadata                 map[string]interface{} `json:"metadata"`
	VerificationInstructions string                 `json:"verification_instructions"`
}

// ExportFileInfo contains information about individual export files
type ExportFileInfo struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Size       int64  `json:"size"`
	Checksum   string `json:"checksum"`
	ChunkIndex int    `json:"chunk_index"`
	StartByte  int64  `json:"start_byte"`
	EndByte    int64  `json:"end_byte"`
}

// ExportStorage interface for different storage backends
type ExportStorage interface {
	Store(key string, data []byte) error
	Retrieve(key string) ([]byte, error)
	GetURL(key string) (string, error)
	Delete(key string) error
	List(prefix string) ([]string, error)
}

// CryptoManager handles cryptographic operations
type CryptoManager struct {
	privateKey []byte
	publicKey  []byte
}

// Prometheus metrics for export service
var (
	exportJobsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "export_jobs_total",
			Help: "Total number of export jobs",
		},
		[]string{"format", "status", "user_id"},
	)

	exportJobDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "export_job_duration_seconds",
			Help:    "Export job duration in seconds",
			Buckets: []float64{1, 5, 10, 30, 60, 300, 600, 1800, 3600},
		},
		[]string{"format", "status"},
	)

	exportDataVolume = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "export_data_volume_bytes",
			Help:    "Export data volume in bytes",
			Buckets: prometheus.ExponentialBuckets(1024, 2, 20), // 1KB to 1GB
		},
		[]string{"format"},
	)
)

func init() {
	prometheus.MustRegister(exportJobsTotal)
	prometheus.MustRegister(exportJobDuration)
	prometheus.MustRegister(exportDataVolume)
}

// NewExportService creates a new export service
func NewExportService(storage ExportStorage, observability *ObservabilityManager) *ExportService {
	return &ExportService{
		storage:       storage,
		crypto:        NewCryptoManager(),
		observability: observability,
		jobs:          make(map[string]*ExportJob),
	}
}

// NewCryptoManager creates a new crypto manager
func NewCryptoManager() CryptoManager {
	// In production, load actual keys from secure storage
	return CryptoManager{
		privateKey: []byte("mock-private-key"),
		publicKey:  []byte("mock-public-key"),
	}
}

// CreateExportJob creates a new export job
func (es *ExportService) CreateExportJob(ctx context.Context, userID string, format ExportFormat, filters ExportFilters) (*ExportJob, error) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("export.format", string(format)),
		attribute.String("export.user_id", userID),
	)

	jobID := generateJobID()

	job := &ExportJob{
		ID:        jobID,
		UserID:    userID,
		Status:    ExportStatusPending,
		Format:    format,
		Filters:   filters,
		CreatedAt: time.Now(),
		Progress:  0.0,
		Metadata:  make(map[string]interface{}),
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour), // 7 days expiry
	}

	es.mu.Lock()
	es.jobs[jobID] = job
	es.mu.Unlock()

	// Start background processing
	go es.processExportJob(ctx, job)

	exportJobsTotal.WithLabelValues(string(format), string(ExportStatusPending), userID).Inc()

	log.Printf("[ExportService] Created export job %s for user %s", jobID, userID)
	return job, nil
}

// GetExportJob retrieves an export job by ID
func (es *ExportService) GetExportJob(jobID string) (*ExportJob, error) {
	es.mu.RLock()
	defer es.mu.RUnlock()

	job, exists := es.jobs[jobID]
	if !exists {
		return nil, fmt.Errorf("export job %s not found", jobID)
	}

	return job, nil
}

// ListExportJobs lists export jobs for a user
func (es *ExportService) ListExportJobs(userID string) ([]*ExportJob, error) {
	es.mu.RLock()
	defer es.mu.RUnlock()

	var jobs []*ExportJob
	for _, job := range es.jobs {
		if job.UserID == userID {
			jobs = append(jobs, job)
		}
	}

	// Sort by creation time (newest first)
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].CreatedAt.After(jobs[j].CreatedAt)
	})

	return jobs, nil
}

// processExportJob processes an export job in the background
func (es *ExportService) processExportJob(ctx context.Context, job *ExportJob) {
	start := time.Now()

	defer func() {
		duration := time.Since(start).Seconds()
		exportJobDuration.WithLabelValues(string(job.Format), string(job.Status)).Observe(duration)
		exportJobsTotal.WithLabelValues(string(job.Format), string(job.Status), job.UserID).Inc()
	}()

	// Update job status
	es.updateJobStatus(job.ID, ExportStatusProcessing, 0.0, "")

	// Fetch data based on filters
	data, err := es.fetchFilteredData(ctx, job.Filters)
	if err != nil {
		es.updateJobStatus(job.ID, ExportStatusFailed, 0.0, err.Error())
		return
	}

	es.updateJobStatus(job.ID, ExportStatusProcessing, 0.2, "Data fetched, formatting...")

	// Format data according to requested format
	formattedData, err := es.formatData(data, job.Format)
	if err != nil {
		es.updateJobStatus(job.ID, ExportStatusFailed, 0.2, err.Error())
		return
	}

	es.updateJobStatus(job.ID, ExportStatusProcessing, 0.6, "Data formatted, creating chunks...")

	// Create chunks and store
	chunks, err := es.createChunks(formattedData, job.ID)
	if err != nil {
		es.updateJobStatus(job.ID, ExportStatusFailed, 0.6, err.Error())
		return
	}

	es.updateJobStatus(job.ID, ExportStatusProcessing, 0.8, "Creating signatures and manifest...")

	// Generate cryptographic signatures and Merkle tree
	signature, merkleRoot, err := es.generateCryptoProofs(chunks)
	if err != nil {
		es.updateJobStatus(job.ID, ExportStatusFailed, 0.8, err.Error())
		return
	}

	// Create and store manifest
	manifest, err := es.createManifest(job, chunks, signature, merkleRoot)
	if err != nil {
		es.updateJobStatus(job.ID, ExportStatusFailed, 0.9, err.Error())
		return
	}

	manifestURL, err := es.storeManifest(job.ID, manifest)
	if err != nil {
		es.updateJobStatus(job.ID, ExportStatusFailed, 0.95, err.Error())
		return
	}

	// Update job with final details
	es.mu.Lock()
	job.Status = ExportStatusCompleted
	job.Progress = 1.0
	job.TotalSize = calculateTotalSize(chunks)
	job.ChunkCount = len(chunks)
	job.Signature = signature
	job.MerkleRoot = merkleRoot
	job.ManifestURL = manifestURL
	now := time.Now()
	job.CompletedAt = &now
	es.mu.Unlock()

	exportDataVolume.WithLabelValues(string(job.Format)).Observe(float64(job.TotalSize))

	log.Printf("[ExportService] Completed export job %s in %v", job.ID, time.Since(start))
}

// updateJobStatus updates the status and progress of an export job
func (es *ExportService) updateJobStatus(jobID string, status ExportStatus, progress float64, errorMsg string) {
	es.mu.Lock()
	defer es.mu.Unlock()

	if job, exists := es.jobs[jobID]; exists {
		job.Status = status
		job.Progress = progress
		if errorMsg != "" {
			job.Error = errorMsg
		}
	}
}

// fetchFilteredData fetches data based on the provided filters
func (es *ExportService) fetchFilteredData(ctx context.Context, filters ExportFilters) ([]map[string]interface{}, error) {
	// Mock implementation - in production, this would query your actual data store
	var data []map[string]interface{}

	// Generate sample data for demonstration
	for i := 0; i < 10000; i++ {
		record := map[string]interface{}{
			"id":              fmt.Sprintf("exp-%d", i),
			"name":            fmt.Sprintf("Experiment %d", i),
			"experiment_type": []string{"network_latency", "cpu_stress", "memory_stress"}[i%3],
			"status":          []string{"completed", "failed", "running"}[i%3],
			"target":          fmt.Sprintf("server-%d", i%10),
			"duration":        300 + (i % 1800),
			"created_at":      time.Now().Add(-time.Duration(i) * time.Hour).Format(time.RFC3339),
			"metadata":        map[string]interface{}{"version": "1.0", "tags": []string{"test"}},
		}

		// Apply filters
		if es.matchesFilters(record, filters) {
			data = append(data, record)
		}
	}

	return data, nil
}

// matchesFilters checks if a record matches the provided filters
func (es *ExportService) matchesFilters(record map[string]interface{}, filters ExportFilters) bool {
	if filters.ExperimentType != "" {
		if expType, ok := record["experiment_type"].(string); !ok || expType != filters.ExperimentType {
			return false
		}
	}

	if filters.Status != "" {
		if status, ok := record["status"].(string); !ok || status != filters.Status {
			return false
		}
	}

	if filters.Target != "" {
		if target, ok := record["target"].(string); !ok || target != filters.Target {
			return false
		}
	}

	// Date filtering would be implemented here
	// Tag filtering would be implemented here

	return true
}

// formatData formats data according to the requested format
func (es *ExportService) formatData(data []map[string]interface{}, format ExportFormat) ([]byte, error) {
	switch format {
	case ExportFormatNDJSON:
		return es.formatAsNDJSON(data)
	case ExportFormatParquet:
		return es.formatAsParquet(data)
	case ExportFormatCSV:
		return es.formatAsCSV(data)
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

// formatAsNDJSON formats data as NDJSON (newline-delimited JSON)
func (es *ExportService) formatAsNDJSON(data []map[string]interface{}) ([]byte, error) {
	var buffer bytes.Buffer

	for _, record := range data {
		jsonData, err := json.Marshal(record)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal record: %w", err)
		}

		buffer.Write(jsonData)
		buffer.WriteByte('\n')
	}

	return buffer.Bytes(), nil
}

// formatAsParquet formats data as Parquet (mock implementation)
func (es *ExportService) formatAsParquet(data []map[string]interface{}) ([]byte, error) {
	// In production, use a proper Parquet library like github.com/xitongsys/parquet-go
	// This is a mock implementation
	header := "-- Parquet Format Export --\n"
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, err
	}

	return append([]byte(header), jsonData...), nil
}

// formatAsCSV formats data as CSV
func (es *ExportService) formatAsCSV(data []map[string]interface{}) ([]byte, error) {
	if len(data) == 0 {
		return []byte{}, nil
	}

	var buffer bytes.Buffer

	// Extract headers from first record
	var headers []string
	for key := range data[0] {
		headers = append(headers, key)
	}
	sort.Strings(headers) // Ensure consistent order

	// Write CSV header
	buffer.WriteString(strings.Join(headers, ","))
	buffer.WriteByte('\n')

	// Write data rows
	for _, record := range data {
		var values []string
		for _, header := range headers {
			value := fmt.Sprintf("%v", record[header])
			// Escape commas and quotes
			if strings.Contains(value, ",") || strings.Contains(value, "\"") {
				value = fmt.Sprintf("\"%s\"", strings.ReplaceAll(value, "\"", "\"\""))
			}
			values = append(values, value)
		}
		buffer.WriteString(strings.Join(values, ","))
		buffer.WriteByte('\n')
	}

	return buffer.Bytes(), nil
}

// createChunks splits data into chunks for download resumption
func (es *ExportService) createChunks(data []byte, jobID string) ([]ExportFileInfo, error) {
	const chunkSize = 10 * 1024 * 1024 // 10MB chunks

	var chunks []ExportFileInfo
	_ = int64(len(data)) // totalSize for potential future use

	for i := 0; i < len(data); i += chunkSize {
		end := i + chunkSize
		if end > len(data) {
			end = len(data)
		}

		chunk := data[i:end]
		chunkIndex := len(chunks)
		filename := fmt.Sprintf("%s_chunk_%03d.dat", jobID, chunkIndex)

		// Calculate checksum
		hasher := sha256.New()
		hasher.Write(chunk)
		checksum := hex.EncodeToString(hasher.Sum(nil))

		// Store chunk
		err := es.storage.Store(fmt.Sprintf("exports/%s/%s", jobID, filename), chunk)
		if err != nil {
			return nil, fmt.Errorf("failed to store chunk %d: %w", chunkIndex, err)
		}

		chunks = append(chunks, ExportFileInfo{
			Name:       filename,
			Path:       fmt.Sprintf("exports/%s/%s", jobID, filename),
			Size:       int64(len(chunk)),
			Checksum:   checksum,
			ChunkIndex: chunkIndex,
			StartByte:  int64(i),
			EndByte:    int64(end - 1),
		})
	}

	return chunks, nil
}

// generateCryptoProofs generates cryptographic signatures and Merkle tree
func (es *ExportService) generateCryptoProofs(chunks []ExportFileInfo) (string, string, error) {
	// Create hash list for Merkle tree
	var hashes []string
	for _, chunk := range chunks {
		hashes = append(hashes, chunk.Checksum)
	}

	// Build Merkle tree
	merkleRoot := es.buildMerkleTree(hashes)

	// Generate signature (mock implementation)
	signature := fmt.Sprintf("sha256:%s", es.signData(merkleRoot))

	return signature, fmt.Sprintf("merkle:%s", merkleRoot), nil
}

// buildMerkleTree builds a Merkle tree from hashes
func (es *ExportService) buildMerkleTree(hashes []string) string {
	if len(hashes) == 0 {
		return ""
	}

	if len(hashes) == 1 {
		return hashes[0]
	}

	var nextLevel []string

	for i := 0; i < len(hashes); i += 2 {
		var combined string
		if i+1 < len(hashes) {
			combined = hashes[i] + hashes[i+1]
		} else {
			combined = hashes[i] + hashes[i] // Duplicate if odd number
		}

		hasher := sha256.New()
		hasher.Write([]byte(combined))
		nextLevel = append(nextLevel, hex.EncodeToString(hasher.Sum(nil)))
	}

	return es.buildMerkleTree(nextLevel)
}

// signData signs data with the private key (mock implementation)
func (es *ExportService) signData(data string) string {
	hasher := sha256.New()
	hasher.Write([]byte(data + string(es.crypto.privateKey)))
	return hex.EncodeToString(hasher.Sum(nil))
}

// createManifest creates the export manifest
func (es *ExportService) createManifest(job *ExportJob, chunks []ExportFileInfo, signature, merkleRoot string) (*ExportManifest, error) {
	manifest := &ExportManifest{
		JobID:        job.ID,
		CreatedAt:    job.CreatedAt,
		Format:       job.Format,
		Filters:      job.Filters,
		TotalRecords: int64(len(chunks)),
		TotalSize:    calculateTotalSize(chunks),
		ChunkCount:   len(chunks),
		Signature:    signature,
		MerkleRoot:   merkleRoot,
		Files:        chunks,
		Metadata:     job.Metadata,
		VerificationInstructions: `
To verify this export:
1. Download the CLI tool: curl -L https://github.com/your-org/chaoslabs-cli/releases/latest/download/chaoslabs-cli
2. Verify signature: chaoslabs-cli verify --manifest manifest.json --public-key public.pem
3. Check file integrity: chaoslabs-cli check-files --manifest manifest.json
4. Compare with another export: chaoslabs-cli diff export1.json export2.json
`,
	}

	return manifest, nil
}

// storeManifest stores the manifest and returns its URL
func (es *ExportService) storeManifest(jobID string, manifest *ExportManifest) (string, error) {
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal manifest: %w", err)
	}

	manifestKey := fmt.Sprintf("exports/%s/manifest.json", jobID)
	err = es.storage.Store(manifestKey, manifestData)
	if err != nil {
		return "", fmt.Errorf("failed to store manifest: %w", err)
	}

	return es.storage.GetURL(manifestKey)
}

// calculateTotalSize calculates the total size of all chunks
func calculateTotalSize(chunks []ExportFileInfo) int64 {
	var total int64
	for _, chunk := range chunks {
		total += chunk.Size
	}
	return total
}

// generateJobID generates a unique job ID
func generateJobID() string {
	return fmt.Sprintf("export_%d_%s", time.Now().Unix(), generateRandomString(8))
}

// HTTP Handlers

// StartExportHandler handles POST /api/exports
func (es *ExportService) StartExportHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Format  string        `json:"format"`
		Filters ExportFilters `json:"filters"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Extract user ID from auth context (mock)
	userID := extractUserID(r)

	format := ExportFormat(req.Format)
	if format == "" {
		format = ExportFormatNDJSON
	}

	job, err := es.CreateExportJob(r.Context(), userID, format, req.Filters)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

// GetExportHandler handles GET /api/exports/{jobId}
func (es *ExportService) GetExportHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	jobID := extractJobID(r.URL.Path)
	if jobID == "" {
		http.Error(w, "Missing job ID", http.StatusBadRequest)
		return
	}

	job, err := es.GetExportJob(jobID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Check if user has access to this job
	userID := extractUserID(r)
	if job.UserID != userID {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

// ListExportsHandler handles GET /api/exports
func (es *ExportService) ListExportsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := extractUserID(r)
	jobs, err := es.ListExportJobs(userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"exports": jobs,
		"total":   len(jobs),
	})
}

// DownloadChunkHandler handles GET /api/exports/{jobId}/chunks/{chunkIndex}
func (es *ExportService) DownloadChunkHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	jobID := extractJobID(r.URL.Path)
	chunkIndexStr := extractChunkIndex(r.URL.Path)

	chunkIndex, err := strconv.Atoi(chunkIndexStr)
	if err != nil {
		http.Error(w, "Invalid chunk index", http.StatusBadRequest)
		return
	}

	job, err := es.GetExportJob(jobID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Check access
	userID := extractUserID(r)
	if job.UserID != userID {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	if job.Status != ExportStatusCompleted {
		http.Error(w, "Export not ready", http.StatusConflict)
		return
	}

	// Support range requests for resumable downloads
	rangeHeader := r.Header.Get("Range")

	filename := fmt.Sprintf("%s_chunk_%03d.dat", jobID, chunkIndex)
	filePath := fmt.Sprintf("exports/%s/%s", jobID, filename)

	data, err := es.storage.Retrieve(filePath)
	if err != nil {
		http.Error(w, "Chunk not found", http.StatusNotFound)
		return
	}

	// Set appropriate headers
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Length", strconv.FormatInt(int64(len(data)), 10))

	// Handle range requests
	if rangeHeader != "" {
		es.handleRangeRequest(w, r, data, rangeHeader)
		return
	}

	w.Write(data)
}

// handleRangeRequest handles partial content requests
func (es *ExportService) handleRangeRequest(w http.ResponseWriter, r *http.Request, data []byte, rangeHeader string) {
	// Parse range header (simplified implementation)
	// Format: "bytes=start-end"
	ranges := strings.TrimPrefix(rangeHeader, "bytes=")
	parts := strings.Split(ranges, "-")

	if len(parts) != 2 {
		http.Error(w, "Invalid range header", http.StatusRequestedRangeNotSatisfiable)
		return
	}

	start, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || start < 0 {
		start = 0
	}

	end := int64(len(data) - 1)
	if parts[1] != "" {
		if e, err := strconv.ParseInt(parts[1], 10, 64); err == nil && e < int64(len(data)) {
			end = e
		}
	}

	if start > end || start >= int64(len(data)) {
		http.Error(w, "Invalid range", http.StatusRequestedRangeNotSatisfiable)
		return
	}

	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
	w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
	w.WriteHeader(http.StatusPartialContent)

	w.Write(data[start : end+1])
}

// Helper functions
func extractUserID(r *http.Request) string {
	// In production, extract from JWT token or session
	return r.Header.Get("X-User-ID")
}

func extractJobID(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if part == "exports" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

func extractChunkIndex(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if part == "chunks" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[i%len(charset)]
	}
	return string(b)
}
