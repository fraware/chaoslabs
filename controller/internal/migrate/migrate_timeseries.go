package migrate

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MigrationResult holds migration performance metrics
type MigrationResult struct {
	CollectionName     string          `json:"collectionName"`
	RecordsMigrated    int64           `json:"recordsMigrated"`
	MigrationDuration  time.Duration   `json:"migrationDuration"`
	BeforeStats        CollectionStats `json:"beforeStats"`
	AfterStats         CollectionStats `json:"afterStats"`
	PerformanceGain    float64         `json:"performanceGain"`
	DiskUsageReduction float64         `json:"diskUsageReduction"`
}

// CollectionStats holds collection performance metrics
type CollectionStats struct {
	Count       int64         `json:"count"`
	Size        int64         `json:"size"`
	StorageSize int64         `json:"storageSize"`
	IndexSize   int64         `json:"indexSize"`
	AvgObjSize  float64       `json:"avgObjSize"`
	QueryTime   time.Duration `json:"queryTime"`
}

// TimeSeriesMigrator handles migration from regular to time-series collections
type TimeSeriesMigrator struct {
	client   *mongo.Client
	database *mongo.Database
	results  []MigrationResult
}

// NewTimeSeriesMigrator creates a new migrator
func NewTimeSeriesMigrator(mongoURI, databaseName string) (*TimeSeriesMigrator, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	database := client.Database(databaseName)
	return &TimeSeriesMigrator{
		client:   client,
		database: database,
		results:  make([]MigrationResult, 0),
	}, nil
}

// MigrateCollection migrates a regular collection to time-series format
func (m *TimeSeriesMigrator) MigrateCollection(collectionName string) error {
	log.Printf("Starting migration of collection: %s", collectionName)

	beforeStats, err := m.getCollectionStats(collectionName)
	if err != nil {
		return fmt.Errorf("failed to get before stats: %s", err)
	}

	timeSeriesName := collectionName + "_ts"
	if err := m.createTimeSeriesCollection(timeSeriesName); err != nil {
		return fmt.Errorf("failed to create time-series collection: %w", err)
	}

	startTime := time.Now()
	recordsMigrated, err := m.migrateData(collectionName, timeSeriesName)
	if err != nil {
		return fmt.Errorf("failed to migrate data: %w", err)
	}
	migrationDuration := time.Since(startTime)

	afterStats, err := m.getCollectionStats(timeSeriesName)
	if err != nil {
		return fmt.Errorf("failed to get after stats: %w", err)
	}

	performanceGain := m.calculatePerformanceGain(beforeStats, afterStats)
	diskUsageReduction := m.calculateDiskUsageReduction(beforeStats, afterStats)

	result := MigrationResult{
		CollectionName:     collectionName,
		RecordsMigrated:    recordsMigrated,
		MigrationDuration:  migrationDuration,
		BeforeStats:        beforeStats,
		AfterStats:         afterStats,
		PerformanceGain:    performanceGain,
		DiskUsageReduction: diskUsageReduction,
	}
	m.results = append(m.results, result)

	log.Printf("Migration completed for %s: %d records in %v", collectionName, recordsMigrated, migrationDuration)
	return nil
}

func (m *TimeSeriesMigrator) createTimeSeriesCollection(name string) error {
	opts := options.CreateCollection().SetTimeSeriesOptions(
		options.TimeSeries().SetTimeField("timestamp").SetMetaField("agentId"),
	)

	return m.database.CreateCollection(context.Background(), name, opts)
}

func (m *TimeSeriesMigrator) migrateData(sourceName, targetName string) (int64, error) {
	sourceCollection := m.database.Collection(sourceName)
	targetCollection := m.database.Collection(targetName)

	cursor, err := sourceCollection.Find(context.Background(), bson.M{})
	if err != nil {
		return 0, fmt.Errorf("failed to query source collection: %w", err)
	}
	defer cursor.Close(context.Background())

	var documents []bson.M
	if err := cursor.All(context.Background(), &documents); err != nil {
		return 0, fmt.Errorf("failed to decode documents: %w", err)
	}

	var transformedDocs []interface{}
	for _, doc := range documents {
		transformed := m.transformDocument(doc)
		transformedDocs = append(transformedDocs, transformed)
	}

	if len(transformedDocs) > 0 {
		_, err = targetCollection.InsertMany(context.Background(), transformedDocs)
		if err != nil {
			return 0, fmt.Errorf("failed to insert into target collection: %w", err)
		}
	}

	return int64(len(transformedDocs)), nil
}

func (m *TimeSeriesMigrator) transformDocument(doc bson.M) bson.M {
	transformed := bson.M{}

	if timestamp, exists := doc["timestamp"]; exists {
		transformed["timestamp"] = timestamp
	} else if createdAt, exists := doc["createdAt"]; exists {
		transformed["timestamp"] = createdAt
	} else if updatedAt, exists := doc["updatedAt"]; exists {
		transformed["timestamp"] = updatedAt
	} else {
		transformed["timestamp"] = time.Now()
	}

	if agentId, exists := doc["agentId"]; exists {
		transformed["agentId"] = agentId
	} else if agent, exists := doc["agent"]; exists {
		transformed["agentId"] = agent
	} else {
		transformed["agentId"] = "unknown"
	}

	for key, value := range doc {
		if key != "_id" {
			transformed[key] = value
		}
	}

	transformed["expireAt"] = time.Now().Add(7 * 24 * time.Hour)

	return transformed
}

func (m *TimeSeriesMigrator) getCollectionStats(collectionName string) (CollectionStats, error) {
	collection := m.database.Collection(collectionName)

	statsCmd := bson.D{{Key: "collStats", Value: collectionName}}
	var result bson.M
	if err := m.database.RunCommand(context.Background(), statsCmd).Decode(&result); err != nil {
		return CollectionStats{}, fmt.Errorf("failed to get collection stats: %w", err)
	}

	stats := CollectionStats{
		Count:       result["count"].(int64),
		Size:        result["size"].(int64),
		StorageSize: result["storageSize"].(int64),
		IndexSize:   result["totalIndexSize"].(int64),
	}

	if avgObjSize, exists := result["avgObjSize"]; exists {
		if avgObjSizeInt, ok := avgObjSize.(int64); ok {
			stats.AvgObjSize = float64(avgObjSizeInt)
		}
	}

	queryTime, err := m.measureQueryPerformance(collection)
	if err != nil {
		log.Printf("Warning: failed to measure query performance: %v", err)
		queryTime = 0
	}
	stats.QueryTime = queryTime

	return stats, nil
}

func (m *TimeSeriesMigrator) measureQueryPerformance(collection *mongo.Collection) (time.Duration, error) {
	filter := bson.M{
		"timestamp": bson.M{
			"$gte": time.Now().Add(-24 * time.Hour),
		},
	}

	start := time.Now()
	cursor, err := collection.Find(context.Background(), filter, options.Find().SetLimit(100))
	if err != nil {
		return 0, err
	}
	defer cursor.Close(context.Background())

	var results []bson.M
	if err := cursor.All(context.Background(), &results); err != nil {
		return 0, err
	}

	return time.Since(start), nil
}

func (m *TimeSeriesMigrator) calculatePerformanceGain(before, after CollectionStats) float64 {
	if before.QueryTime == 0 {
		return 0
	}
	return float64(before.QueryTime-after.QueryTime) / float64(before.QueryTime) * 100
}

func (m *TimeSeriesMigrator) calculateDiskUsageReduction(before, after CollectionStats) float64 {
	if before.StorageSize == 0 {
		return 0
	}
	return float64(before.StorageSize-after.StorageSize) / float64(before.StorageSize) * 100
}

// RunBenchmarks runs performance benchmarks before and after migration
func (m *TimeSeriesMigrator) RunBenchmarks() error {
	log.Println("Running performance benchmarks...")

	beforeResults, err := m.runQueryBenchmarks()
	if err != nil {
		return fmt.Errorf("failed to run before benchmarks: %w", err)
	}

	afterResults, err := m.runTimeSeriesBenchmarks()
	if err != nil {
		return fmt.Errorf("failed to run after benchmarks: %w", err)
	}

	m.compareBenchmarkResults(beforeResults, afterResults)

	return nil
}

func (m *TimeSeriesMigrator) runQueryBenchmarks() (map[string]QueryBenchmark, error) {
	results := make(map[string]QueryBenchmark)

	collections := []string{"logs", "experiments", "metrics"}
	for _, name := range collections {
		collection := m.database.Collection(name)

		timeRangeDuration, err := m.benchmarkTimeRangeQuery(collection)
		if err != nil {
			log.Printf("Warning: failed to benchmark time-range query for %s: %v", name, err)
			continue
		}

		aggregationDuration, err := m.benchmarkAggregationQuery(collection)
		if err != nil {
			log.Printf("Warning: failed to benchmark aggregation query for %s: %v", name, err)
			continue
		}

		results[name] = QueryBenchmark{
			CollectionName:     name,
			TimeRangeQuery:     timeRangeDuration,
			AggregationQuery: aggregationDuration,
		}
	}

	return results, nil
}

func (m *TimeSeriesMigrator) runTimeSeriesBenchmarks() (map[string]QueryBenchmark, error) {
	results := make(map[string]QueryBenchmark)

	collections := []string{"logs_ts", "experiments_ts", "metrics_ts"}
	for _, name := range collections {
		collection := m.database.Collection(name)

		timeRangeDuration, err := m.benchmarkTimeRangeQuery(collection)
		if err != nil {
			log.Printf("Warning: failed to benchmark time-range query for %s: %v", name, err)
			continue
		}

		aggregationDuration, err := m.benchmarkAggregationQuery(collection)
		if err != nil {
			log.Printf("Warning: failed to benchmark aggregation query for %s: %v", name, err)
			continue
		}

		results[name] = QueryBenchmark{
			CollectionName:     name,
			TimeRangeQuery:     timeRangeDuration,
			AggregationQuery: aggregationDuration,
		}
	}

	return results, nil
}

// QueryBenchmark holds query performance metrics
type QueryBenchmark struct {
	CollectionName   string        `json:"collectionName"`
	TimeRangeQuery   time.Duration `json:"timeRangeQuery"`
	AggregationQuery time.Duration `json:"aggregationQuery"`
}

func (m *TimeSeriesMigrator) benchmarkTimeRangeQuery(collection *mongo.Collection) (time.Duration, error) {
	filter := bson.M{
		"timestamp": bson.M{
			"$gte": time.Now().Add(-24 * time.Hour),
			"$lte": time.Now(),
		},
	}

	start := time.Now()
	cursor, err := collection.Find(context.Background(), filter)
	if err != nil {
		return 0, err
	}
	defer cursor.Close(context.Background())

	var results []bson.M
	if err := cursor.All(context.Background(), &results); err != nil {
		return 0, err
	}

	return time.Since(start), nil
}

func (m *TimeSeriesMigrator) benchmarkAggregationQuery(collection *mongo.Collection) (time.Duration, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"timestamp": bson.M{
				"$gte": time.Now().Add(-24 * time.Hour),
			},
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":   "$agentId",
			"count": bson.M{"$sum": 1},
		}}},
	}

	start := time.Now()
	cursor, err := collection.Aggregate(context.Background(), pipeline)
	if err != nil {
		return 0, err
	}
	defer cursor.Close(context.Background())

	var results []bson.M
	if err := cursor.All(context.Background(), &results); err != nil {
		return 0, err
	}

	return time.Since(start), nil
}

func (m *TimeSeriesMigrator) compareBenchmarkResults(before, after map[string]QueryBenchmark) {
	log.Println("\n=== Benchmark Comparison Results ===")

	for collectionName, beforeBench := range before {
		if afterBench, exists := after[collectionName+"_ts"]; exists {
			log.Printf("\nCollection: %s", collectionName)

			if beforeBench.TimeRangeQuery > 0 {
				improvement := float64(beforeBench.TimeRangeQuery-afterBench.TimeRangeQuery) / float64(beforeBench.TimeRangeQuery) * 100
				log.Printf("  Time-range query: %v → %v (%.1f%% improvement)",
					beforeBench.TimeRangeQuery, afterBench.TimeRangeQuery, improvement)
			}

			if beforeBench.AggregationQuery > 0 {
				improvement := float64(beforeBench.AggregationQuery-afterBench.AggregationQuery) / float64(beforeBench.AggregationQuery) * 100
				log.Printf("  Aggregation query: %v → %v (%.1f%% improvement)",
					beforeBench.AggregationQuery, afterBench.AggregationQuery, improvement)
			}
		}
	}
}

// GenerateReport generates a comprehensive migration report
func (m *TimeSeriesMigrator) GenerateReport() error {
	report := struct {
		Timestamp        time.Time              `json:"timestamp"`
		MigrationResults []MigrationResult      `json:"migrationResults"`
		Summary          map[string]interface{} `json:"summary"`
	}{
		Timestamp:        time.Now(),
		MigrationResults: m.results,
		Summary:          m.calculateSummary(),
	}

	filename := fmt.Sprintf("migration_report_%s.json", time.Now().Format("20060102_150405"))
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}

	if err := os.WriteFile(filename, data, 0600); err != nil {
		return fmt.Errorf("failed to write report: %w", err)
	}

	log.Printf("Migration report saved to: %s", filename)
	return nil
}

func (m *TimeSeriesMigrator) calculateSummary() map[string]interface{} {
	totalRecords := int64(0)
	totalDuration := time.Duration(0)
	avgPerformanceGain := 0.0
	avgDiskReduction := 0.0

	for _, result := range m.results {
		totalRecords += result.RecordsMigrated
		totalDuration += result.MigrationDuration
		avgPerformanceGain += result.PerformanceGain
		avgDiskReduction += result.DiskUsageReduction
	}

	count := len(m.results)
	if count > 0 {
		avgPerformanceGain /= float64(count)
		avgDiskReduction /= float64(count)
	}

	return map[string]interface{}{
		"totalCollections":     count,
		"totalRecordsMigrated": totalRecords,
		"totalMigrationTime":   totalDuration.String(),
		"avgPerformanceGain":   avgPerformanceGain,
		"avgDiskReduction":     avgDiskReduction,
	}
}

// Close closes the MongoDB connection
func (m *TimeSeriesMigrator) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return m.client.Disconnect(ctx)
}
