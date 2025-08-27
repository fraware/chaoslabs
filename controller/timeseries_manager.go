package main

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// TimeSeriesManager manages time-series data operations
type TimeSeriesManager struct {
	client   *mongo.Client
	database *mongo.Database
}

// NewTimeSeriesManager creates a new time-series manager
func NewTimeSeriesManager(mongoURI, databaseName string) (*TimeSeriesManager, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		return nil, err
	}

	database := client.Database(databaseName)

	return &TimeSeriesManager{
		client:   client,
		database: database,
	}, nil
}

// Close closes the connection to MongoDB
func (tsm *TimeSeriesManager) Close() error {
	return tsm.client.Disconnect(context.Background())
}

// Store stores data in the time-series collection
func (tsm *TimeSeriesManager) Store(collection string, data interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	coll := tsm.database.Collection(collection)
	_, err := coll.InsertOne(ctx, data)
	return err
}

// Query queries data from the time-series collection
func (tsm *TimeSeriesManager) Query(collection string, filter interface{}) (*mongo.Cursor, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	coll := tsm.database.Collection(collection)
	return coll.Find(ctx, filter)
}

// InsertLogEvent inserts a log event into the logs collection
func (tsm *TimeSeriesManager) InsertLogEvent(logEvent interface{}) error {
	return tsm.Store("logs", logEvent)
}
