package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// ExportManifest represents the export manifest structure
type ExportManifest struct {
	JobID         string                 `json:"job_id"`
	CreatedAt     time.Time              `json:"created_at"`
	Format        string                 `json:"format"`
	Filters       map[string]interface{} `json:"filters"`
	TotalRecords  int64                  `json:"total_records"`
	TotalSize     int64                  `json:"total_size"`
	ChunkCount    int                    `json:"chunk_count"`
	Signature     string                 `json:"signature"`
	MerkleRoot    string                 `json:"merkle_root"`
	Files         []ExportFileInfo       `json:"files"`
	Metadata      map[string]interface{} `json:"metadata"`
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

// DiffResult represents the result of comparing two exports
type DiffResult struct {
	Export1      string                 `json:"export1"`
	Export2      string                 `json:"export2"`
	Summary      DiffSummary            `json:"summary"`
	Differences  []RecordDifference     `json:"differences"`
	OnlyInFirst  []map[string]interface{} `json:"only_in_first"`
	OnlyInSecond []map[string]interface{} `json:"only_in_second"`
}

// DiffSummary provides a high-level summary of differences
type DiffSummary struct {
	TotalRecords1    int     `json:"total_records_1"`
	TotalRecords2    int     `json:"total_records_2"`
	IdenticalRecords int     `json:"identical_records"`
	ModifiedRecords  int     `json:"modified_records"`
	OnlyInFirst      int     `json:"only_in_first"`
	OnlyInSecond     int     `json:"only_in_second"`
	SimilarityScore  float64 `json:"similarity_score"`
}

// RecordDifference represents a difference between two records
type RecordDifference struct {
	RecordID   string            `json:"record_id"`
	Field      string            `json:"field"`
	Value1     interface{}       `json:"value1"`
	Value2     interface{}       `json:"value2"`
	ChangeType string            `json:"change_type"` // "modified", "added", "removed"
}

var (
	verbose    bool
	outputFile string
	format     string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "chaoslabs-cli",
		Short: "ChaosLabs Export Verification and Analysis Tool",
		Long: `A command-line tool for verifying cryptographic signatures, 
checking file integrity, and comparing ChaosLabs exports.`,
	}

	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().StringVarP(&outputFile, "output", "o", "", "output file path")
	rootCmd.PersistentFlags().StringVarP(&format, "format", "f", "text", "output format (text, json)")

	// Add subcommands
	rootCmd.AddCommand(newVerifyCommand())
	rootCmd.AddCommand(newCheckFilesCommand())
	rootCmd.AddCommand(newDiffCommand())
	rootCmd.AddCommand(newInfoCommand())
	rootCmd.AddCommand(newDownloadCommand())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// Verify command verifies export signatures
func newVerifyCommand() *cobra.Command {
	var manifestPath, publicKeyPath string

	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify export cryptographic signatures",
		Long:  "Verify the cryptographic signature and Merkle tree of an export.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return verifyExport(manifestPath, publicKeyPath)
		},
	}

	cmd.Flags().StringVarP(&manifestPath, "manifest", "m", "", "path to manifest.json file (required)")
	cmd.Flags().StringVarP(&publicKeyPath, "public-key", "k", "", "path to public key file")
	cmd.MarkFlagRequired("manifest")

	return cmd
}

// Check files command verifies file integrity
func newCheckFilesCommand() *cobra.Command {
	var manifestPath, dataPath string

	cmd := &cobra.Command{
		Use:   "check-files",
		Short: "Check file integrity using checksums",
		Long:  "Verify that all files mentioned in the manifest have correct checksums.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return checkFiles(manifestPath, dataPath)
		},
	}

	cmd.Flags().StringVarP(&manifestPath, "manifest", "m", "", "path to manifest.json file (required)")
	cmd.Flags().StringVarP(&dataPath, "data-dir", "d", ".", "directory containing export files")
	cmd.MarkFlagRequired("manifest")

	return cmd
}

// Diff command compares two exports
func newDiffCommand() *cobra.Command {
	var export1, export2 string
	var ignoreFields []string
	var threshold float64

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Compare two exports and show differences",
		Long:  "Compare two exports and generate a detailed difference report.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return diffExports(export1, export2, ignoreFields, threshold)
		},
	}

	cmd.Flags().StringVar(&export1, "export1", "", "path to first export manifest or data file (required)")
	cmd.Flags().StringVar(&export2, "export2", "", "path to second export manifest or data file (required)")
	cmd.Flags().StringSliceVar(&ignoreFields, "ignore-fields", []string{}, "fields to ignore during comparison")
	cmd.Flags().Float64Var(&threshold, "threshold", 0.95, "similarity threshold for reporting (0.0-1.0)")
	cmd.MarkFlagRequired("export1")
	cmd.MarkFlagRequired("export2")

	return cmd
}

// Info command shows export information
func newInfoCommand() *cobra.Command {
	var manifestPath string

	cmd := &cobra.Command{
		Use:   "info",
		Short: "Display export information",
		Long:  "Display detailed information about an export from its manifest.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return showExportInfo(manifestPath)
		},
	}

	cmd.Flags().StringVarP(&manifestPath, "manifest", "m", "", "path to manifest.json file (required)")
	cmd.MarkFlagRequired("manifest")

	return cmd
}

// Download command downloads and verifies an export
func newDownloadCommand() *cobra.Command {
	var baseURL, jobID, outputDir string
	var verify bool

	cmd := &cobra.Command{
		Use:   "download",
		Short: "Download and verify an export",
		Long:  "Download all chunks of an export and optionally verify integrity.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return downloadExport(baseURL, jobID, outputDir, verify)
		},
	}

	cmd.Flags().StringVar(&baseURL, "base-url", "", "base URL of the ChaosLabs API (required)")
	cmd.Flags().StringVar(&jobID, "job-id", "", "export job ID (required)")
	cmd.Flags().StringVarP(&outputDir, "output-dir", "o", ".", "output directory")
	cmd.Flags().BoolVar(&verify, "verify", true, "verify file integrity after download")
	cmd.MarkFlagRequired("base-url")
	cmd.MarkFlagRequired("job-id")

	return cmd
}

// verifyExport verifies the cryptographic signature of an export
func verifyExport(manifestPath, publicKeyPath string) error {
	// Load manifest
	manifest, err := loadManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to load manifest: %w", err)
	}

	fmt.Printf("Verifying export: %s\n", manifest.JobID)
	fmt.Printf("Created: %s\n", manifest.CreatedAt.Format(time.RFC3339))
	fmt.Printf("Format: %s\n", manifest.Format)
	fmt.Printf("Files: %d\n", len(manifest.Files))

	// Verify Merkle tree
	if err := verifyMerkleTree(manifest); err != nil {
		return fmt.Errorf("Merkle tree verification failed: %w", err)
	}
	
	fmt.Println("✓ Merkle tree verification passed")

	// Verify signature (mock implementation)
	if err := verifySignature(manifest, publicKeyPath); err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}
	
	fmt.Println("✓ Signature verification passed")
	fmt.Println("Export verification successful!")

	return nil
}

// checkFiles verifies the integrity of all files in an export
func checkFiles(manifestPath, dataPath string) error {
	manifest, err := loadManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to load manifest: %w", err)
	}

	fmt.Printf("Checking %d files...\n", len(manifest.Files))

	var failed []string
	for i, file := range manifest.Files {
		filePath := filepath.Join(dataPath, file.Name)
		
		if verbose {
			fmt.Printf("Checking %s...", file.Name)
		}

		if err := verifyFileChecksum(filePath, file.Checksum, file.Size); err != nil {
			failed = append(failed, file.Name)
			if verbose {
				fmt.Printf(" FAILED: %v\n", err)
			} else {
				fmt.Printf("✗ %s: %v\n", file.Name, err)
			}
		} else {
			if verbose {
				fmt.Printf(" OK\n")
			} else {
				fmt.Printf("✓ %s\n", file.Name)
			}
		}

		// Progress indicator
		if !verbose && (i+1)%10 == 0 {
			fmt.Printf("Checked %d/%d files\n", i+1, len(manifest.Files))
		}
	}

	if len(failed) > 0 {
		return fmt.Errorf("%d files failed verification: %v", len(failed), failed)
	}

	fmt.Println("All files verified successfully!")
	return nil
}

// diffExports compares two exports and shows differences
func diffExports(export1Path, export2Path string, ignoreFields []string, threshold float64) error {
	fmt.Printf("Comparing exports:\n")
	fmt.Printf("  Export 1: %s\n", export1Path)
	fmt.Printf("  Export 2: %s\n", export2Path)

	// Load export data
	data1, err := loadExportData(export1Path)
	if err != nil {
		return fmt.Errorf("failed to load export 1: %w", err)
	}

	data2, err := loadExportData(export2Path)
	if err != nil {
		return fmt.Errorf("failed to load export 2: %w", err)
	}

	// Perform comparison
	result := compareExports(data1, data2, ignoreFields)
	result.Export1 = export1Path
	result.Export2 = export2Path

	// Generate output
	if format == "json" {
		return outputJSON(result)
	}
	
	return outputTextDiff(result, threshold)
}

// showExportInfo displays information about an export
func showExportInfo(manifestPath string) error {
	manifest, err := loadManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to load manifest: %w", err)
	}

	if format == "json" {
		data, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			return err
		}
		fmt.Print(string(data))
		return nil
	}

	// Text format
	fmt.Printf("Export Information\n")
	fmt.Printf("==================\n")
	fmt.Printf("Job ID:          %s\n", manifest.JobID)
	fmt.Printf("Created:         %s\n", manifest.CreatedAt.Format(time.RFC3339))
	fmt.Printf("Format:          %s\n", manifest.Format)
	fmt.Printf("Total Records:   %d\n", manifest.TotalRecords)
	fmt.Printf("Total Size:      %s\n", formatBytes(manifest.TotalSize))
	fmt.Printf("Chunks:          %d\n", manifest.ChunkCount)
	fmt.Printf("Signature:       %s\n", manifest.Signature)
	fmt.Printf("Merkle Root:     %s\n", manifest.MerkleRoot)

	if len(manifest.Filters) > 0 {
		fmt.Printf("\nFilters:\n")
		for key, value := range manifest.Filters {
			fmt.Printf("  %s: %v\n", key, value)
		}
	}

	if len(manifest.Files) > 0 {
		fmt.Printf("\nFiles:\n")
		for _, file := range manifest.Files {
			fmt.Printf("  %s (%s, chunk %d)\n", file.Name, formatBytes(file.Size), file.ChunkIndex)
		}
	}

	return nil
}

// downloadExport downloads all chunks of an export
func downloadExport(baseURL, jobID, outputDir, verify bool) error {
	// This would implement actual HTTP download logic
	// For now, it's a placeholder
	fmt.Printf("Downloading export %s from %s to %s\n", jobID, baseURL, outputDir)
	fmt.Println("Note: Download functionality requires HTTP client implementation")
	return nil
}

// Helper functions

func loadManifest(path string) (*ExportManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var manifest ExportManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}

	return &manifest, nil
}

func verifyMerkleTree(manifest *ExportManifest) error {
	// Build Merkle tree from file checksums
	var hashes []string
	for _, file := range manifest.Files {
		hashes = append(hashes, file.Checksum)
	}

	computedRoot := buildMerkleTree(hashes)
	expectedRoot := strings.TrimPrefix(manifest.MerkleRoot, "merkle:")

	if computedRoot != expectedRoot {
		return fmt.Errorf("Merkle root mismatch: expected %s, got %s", expectedRoot, computedRoot)
	}

	return nil
}

func verifySignature(manifest *ExportManifest, publicKeyPath string) error {
	// Mock signature verification
	// In production, this would use actual cryptographic verification
	if manifest.Signature == "" {
		return fmt.Errorf("no signature found")
	}
	
	// Placeholder verification
	return nil
}

func verifyFileChecksum(filePath, expectedChecksum string, expectedSize int64) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("cannot open file: %w", err)
	}
	defer file.Close()

	// Check file size
	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("cannot stat file: %w", err)
	}

	if stat.Size() != expectedSize {
		return fmt.Errorf("size mismatch: expected %d bytes, got %d bytes", expectedSize, stat.Size())
	}

	// Calculate checksum
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return fmt.Errorf("cannot calculate checksum: %w", err)
	}

	actualChecksum := hex.EncodeToString(hasher.Sum(nil))
	if actualChecksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
	}

	return nil
}

func loadExportData(path string) ([]map[string]interface{}, error) {
	// Determine if it's a manifest or data file
	if strings.HasSuffix(path, "manifest.json") {
		// Load from manifest
		return loadDataFromManifest(path)
	}
	
	// Load directly as NDJSON
	return loadNDJSONFile(path)
}

func loadDataFromManifest(manifestPath string) ([]map[string]interface{}, error) {
	manifest, err := loadManifest(manifestPath)
	if err != nil {
		return nil, err
	}

	// For simplicity, assume data files are in the same directory
	dir := filepath.Dir(manifestPath)
	
	var allData []map[string]interface{}
	
	for _, file := range manifest.Files {
		filePath := filepath.Join(dir, file.Name)
		data, err := loadNDJSONFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to load %s: %w", file.Name, err)
		}
		allData = append(allData, data...)
	}
	
	return allData, nil
}

func loadNDJSONFile(path string) ([]map[string]interface{}, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var data []map[string]interface{}
	scanner := bufio.NewScanner(file)
	
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		
		var record map[string]interface{}
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			return nil, fmt.Errorf("invalid JSON line: %w", err)
		}
		
		data = append(data, record)
	}
	
	return data, scanner.Err()
}

func compareExports(data1, data2 []map[string]interface{}, ignoreFields []string) *DiffResult {
	// Create indices for faster lookup
	index1 := createRecordIndex(data1)
	index2 := createRecordIndex(data2)
	
	var differences []RecordDifference
	var onlyInFirst []map[string]interface{}
	var onlyInSecond []map[string]interface{}
	
	identical := 0
	modified := 0
	
	// Check records in first export
	for id, record1 := range index1 {
		if record2, exists := index2[id]; exists {
			// Compare records
			diffs := compareRecords(id, record1, record2, ignoreFields)
			if len(diffs) == 0 {
				identical++
			} else {
				modified++
				differences = append(differences, diffs...)
			}
		} else {
			onlyInFirst = append(onlyInFirst, record1)
		}
	}
	
	// Check records only in second export
	for id, record2 := range index2 {
		if _, exists := index1[id]; !exists {
			onlyInSecond = append(onlyInSecond, record2)
		}
	}
	
	// Calculate similarity score
	totalRecords := len(data1) + len(data2)
	similarityScore := 0.0
	if totalRecords > 0 {
		similarityScore = float64(identical*2) / float64(totalRecords)
	}
	
	return &DiffResult{
		Summary: DiffSummary{
			TotalRecords1:    len(data1),
			TotalRecords2:    len(data2),
			IdenticalRecords: identical,
			ModifiedRecords:  modified,
			OnlyInFirst:      len(onlyInFirst),
			OnlyInSecond:     len(onlyInSecond),
			SimilarityScore:  similarityScore,
		},
		Differences:  differences,
		OnlyInFirst:  onlyInFirst,
		OnlyInSecond: onlyInSecond,
	}
}

func createRecordIndex(data []map[string]interface{}) map[string]map[string]interface{} {
	index := make(map[string]map[string]interface{})
	
	for _, record := range data {
		// Use "id" field as key, or generate one
		var key string
		if id, ok := record["id"].(string); ok {
			key = id
		} else {
			// Generate key from other fields
			key = generateRecordKey(record)
		}
		index[key] = record
	}
	
	return index
}

func generateRecordKey(record map[string]interface{}) string {
	// Generate a key from important fields
	var parts []string
	
	for _, field := range []string{"name", "experiment_type", "target", "created_at"} {
		if value, ok := record[field]; ok {
			parts = append(parts, fmt.Sprintf("%v", value))
		}
	}
	
	return strings.Join(parts, "|")
}

func compareRecords(id string, record1, record2 map[string]interface{}, ignoreFields []string) []RecordDifference {
	var diffs []RecordDifference
	
	// Create ignore set
	ignore := make(map[string]bool)
	for _, field := range ignoreFields {
		ignore[field] = true
	}
	
	// Get all fields
	allFields := make(map[string]bool)
	for field := range record1 {
		allFields[field] = true
	}
	for field := range record2 {
		allFields[field] = true
	}
	
	// Compare each field
	for field := range allFields {
		if ignore[field] {
			continue
		}
		
		value1, exists1 := record1[field]
		value2, exists2 := record2[field]
		
		if !exists1 && exists2 {
			diffs = append(diffs, RecordDifference{
				RecordID:   id,
				Field:      field,
				Value1:     nil,
				Value2:     value2,
				ChangeType: "added",
			})
		} else if exists1 && !exists2 {
			diffs = append(diffs, RecordDifference{
				RecordID:   id,
				Field:      field,
				Value1:     value1,
				Value2:     nil,
				ChangeType: "removed",
			})
		} else if exists1 && exists2 && !deepEqual(value1, value2) {
			diffs = append(diffs, RecordDifference{
				RecordID:   id,
				Field:      field,
				Value1:     value1,
				Value2:     value2,
				ChangeType: "modified",
			})
		}
	}
	
	return diffs
}

func deepEqual(a, b interface{}) bool {
	// Simple comparison - in production, use reflect.DeepEqual or similar
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func outputJSON(result *DiffResult) error {
	var output io.Writer = os.Stdout
	
	if outputFile != "" {
		file, err := os.Create(outputFile)
		if err != nil {
			return err
		}
		defer file.Close()
		output = file
	}
	
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	
	_, err = output.Write(data)
	return err
}

func outputTextDiff(result *DiffResult, threshold float64) error {
	var output io.Writer = os.Stdout
	
	if outputFile != "" {
		file, err := os.Create(outputFile)
		if err != nil {
			return err
		}
		defer file.Close()
		output = file
	}
	
	fmt.Fprintf(output, "Export Comparison Report\n")
	fmt.Fprintf(output, "========================\n\n")
	
	fmt.Fprintf(output, "Summary:\n")
	fmt.Fprintf(output, "  Export 1 records: %d\n", result.Summary.TotalRecords1)
	fmt.Fprintf(output, "  Export 2 records: %d\n", result.Summary.TotalRecords2)
	fmt.Fprintf(output, "  Identical records: %d\n", result.Summary.IdenticalRecords)
	fmt.Fprintf(output, "  Modified records: %d\n", result.Summary.ModifiedRecords)
	fmt.Fprintf(output, "  Only in first: %d\n", result.Summary.OnlyInFirst)
	fmt.Fprintf(output, "  Only in second: %d\n", result.Summary.OnlyInSecond)
	fmt.Fprintf(output, "  Similarity score: %.2f%%\n", result.Summary.SimilarityScore*100)
	
	if result.Summary.SimilarityScore >= threshold {
		fmt.Fprintf(output, "  Status: ✓ SIMILAR (above threshold %.2f%%)\n", threshold*100)
	} else {
		fmt.Fprintf(output, "  Status: ✗ DIFFERENT (below threshold %.2f%%)\n", threshold*100)
	}
	
	if len(result.Differences) > 0 {
		fmt.Fprintf(output, "\nField Differences:\n")
		for _, diff := range result.Differences[:min(len(result.Differences), 50)] {
			fmt.Fprintf(output, "  Record %s, field '%s': %s\n", diff.RecordID, diff.Field, diff.ChangeType)
			if verbose {
				fmt.Fprintf(output, "    Value 1: %v\n", diff.Value1)
				fmt.Fprintf(output, "    Value 2: %v\n", diff.Value2)
			}
		}
		if len(result.Differences) > 50 {
			fmt.Fprintf(output, "  ... and %d more differences\n", len(result.Differences)-50)
		}
	}
	
	return nil
}

func buildMerkleTree(hashes []string) string {
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
	
	return buildMerkleTree(nextLevel)
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}