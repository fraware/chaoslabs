package main

import (
	"flag"
	"fmt"
	"log"
	"strings"
)

func main() {
	var (
		mongoURI      = flag.String("uri", "mongodb://localhost:27017", "MongoDB connection URI")
		database      = flag.String("db", "chaoslabs", "Database name")
		collections   = flag.String("collections", "logs,experiments,metrics", "Comma-separated list of collections to migrate")
		runBenchmarks = flag.Bool("benchmarks", true, "Run performance benchmarks before/after migration")
		dryRun        = flag.Bool("dry-run", false, "Show what would be migrated without actually doing it")
		help          = flag.Bool("help", false, "Show help")
	)
	flag.Parse()

	if *help {
		showHelp()
		return
	}

	log.Printf("ChaosLabs MongoDB Time-Series Migration Tool")
	log.Printf("==========================================")
	log.Printf("MongoDB URI: %s", *mongoURI)
	log.Printf("Database: %s", *database)
	log.Printf("Collections: %s", *collections)
	log.Printf("Dry run: %v", *dryRun)

	if *dryRun {
		log.Println("DRY RUN MODE - No changes will be made")
	}

	// Parse collections
	collectionList := strings.Split(*collections, ",")
	for i, col := range collectionList {
		collectionList[i] = strings.TrimSpace(col)
	}

	// Create migrator
	migrator, err := NewTimeSeriesMigrator(*mongoURI, *database)
	if err != nil {
		log.Fatalf("Failed to create migrator: %v", err)
	}
	defer migrator.Close()

	// Run benchmarks before migration
	if *runBenchmarks {
		log.Println("\nRunning pre-migration benchmarks...")
		if err := migrator.RunBenchmarks(); err != nil {
			log.Printf("Warning: Benchmark failed: %v", err)
		}
	}

	// Migrate collections
	if !*dryRun {
		log.Println("\nStarting migration...")
		for _, collectionName := range collectionList {
			if collectionName == "" {
				continue
			}

			log.Printf("\nMigrating collection: %s", collectionName)
			if err := migrator.MigrateCollection(collectionName); err != nil {
				log.Printf("Error migrating %s: %v", collectionName, err)
				continue
			}
		}

		// Run benchmarks after migration
		if *runBenchmarks {
			log.Println("\nRunning post-migration benchmarks...")
			if err := migrator.RunBenchmarks(); err != nil {
				log.Printf("Warning: Post-migration benchmark failed: %v", err)
			}
		}

		// Generate report
		log.Println("\nGenerating migration report...")
		if err := migrator.GenerateReport(); err != nil {
			log.Printf("Warning: Failed to generate report: %v", err)
		}
	} else {
		log.Println("\nDRY RUN - Would migrate the following collections:")
		for _, collectionName := range collectionList {
			if collectionName != "" {
				log.Printf("  - %s → %s_ts", collectionName, collectionName)
			}
		}
	}

	log.Println("\nMigration tool completed!")
}

func showHelp() {
	fmt.Println(`ChaosLabs MongoDB Time-Series Migration Tool

This tool migrates regular MongoDB collections to time-series collections with TTL indexes
for improved performance and automatic data retention.

Usage:
  migrate [flags]

Flags:
  -uri string
        MongoDB connection URI (default "mongodb://localhost:27017")
  -db string
        Database name (default "chaoslabs")
  -collections string
        Comma-separated list of collections to migrate (default "logs,experiments,metrics")
  -benchmarks
        Run performance benchmarks before/after migration (default true)
  -dry-run
        Show what would be migrated without actually doing it
  -help
        Show this help message

Examples:
  # Migrate all default collections
  migrate

  # Migrate specific collections with custom MongoDB URI
  migrate -uri "mongodb://user:pass@localhost:27017" -collections "logs,events"

  # Dry run to see what would be migrated
  migrate -dry-run

  # Skip benchmarks for faster migration
  migrate -benchmarks=false

The tool will:
1. Create time-series collections with proper indexes
2. Migrate data from regular collections
3. Run performance benchmarks before/after
4. Generate a detailed migration report
5. Set up TTL indexes for automatic data expiration

Time-series collections provide:
- Better performance for time-range queries
- Automatic data expiration via TTL indexes
- Optimized storage for time-series data
- Improved aggregation performance

Retention policies:
- Hot data: MongoDB time-series (1 day - 1 week)
- Warm data: Object storage (1 week - 1 month)
- Cold data: Immutable sink (1 month - 1 year)
`)
}
