// Package testutil provides utilities for integration testing with real MongoDB, Redis and Meilisearch
package testutil

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"matter-core/internal/config"
	"matter-core/internal/repository"

	"github.com/meilisearch/meilisearch-go"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// TestEnv holds test environment resources
type TestEnv struct {
	MongoRepo *repository.MongoRepo
	RedisRepo *repository.RedisRepo
	MeiliRepo *repository.MeiliRepo
	Config    *config.Config
	ctx       context.Context
	cancel    context.CancelFunc
	dbName    string
	indexName string
}

// getEnvOrDefault returns environment variable value or default
func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// NewTestEnv creates a new test environment with real MongoDB, Redis and Meilisearch connections
// It creates a unique database/index for each test to ensure isolation
func NewTestEnv(t *testing.T) *TestEnv {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	// Generate unique names for test isolation
	timestamp := time.Now().UnixNano()
	dbName := fmt.Sprintf("test_matter_%d", timestamp)
	indexName := fmt.Sprintf("test_entries_%d", timestamp)

	// Get connection strings from environment or use defaults
	mongoURI := getEnvOrDefault("TEST_MONGO_URI", "mongodb://localhost:27017")
	redisAddr := getEnvOrDefault("TEST_REDIS_ADDR", "localhost:6379")
	redisPassword := getEnvOrDefault("TEST_REDIS_PASSWORD", "")
	meiliHost := getEnvOrDefault("TEST_MEILI_HOST", "http://localhost:7700")
	meiliKey := getEnvOrDefault("TEST_MEILI_KEY", "")

	// Create MongoDB repository
	mongoRepo, err := repository.NewMongoRepo(mongoURI, dbName)
	if err != nil {
		cancel()
		t.Skipf("Skipping test: MongoDB not available at %s: %v", mongoURI, err)
	}

	// Create Redis repository
	redisRepo, err := repository.NewRedisRepo(redisAddr, redisPassword, 15) // Use DB 15 for tests
	if err != nil {
		mongoRepo.Close(ctx)
		cancel()
		t.Skipf("Skipping test: Redis not available at %s: %v", redisAddr, err)
	}

	// Create Meilisearch repository (optional - some tests may not need it)
	var meiliRepo *repository.MeiliRepo
	meiliRepo, err = repository.NewMeiliRepoWithIndex(meiliHost, meiliKey, indexName)
	if err != nil {
		t.Logf("Warning: Meilisearch not available at %s: %v (search tests will be skipped)", meiliHost, err)
	}

	cfg := &config.Config{
		MongoURI:        mongoURI,
		MongoDB:         dbName,
		RedisAddr:       redisAddr,
		RedisDB:         15,
		MeilisearchHost: meiliHost,
		MeilisearchKey:  meiliKey,
		AdminEmail:      "admin@test.com",
		FrontendURL:     "http://localhost:3000",
		BackendURL:      "http://localhost:8080",
	}

	return &TestEnv{
		MongoRepo: mongoRepo,
		RedisRepo: redisRepo,
		MeiliRepo: meiliRepo,
		Config:    cfg,
		ctx:       ctx,
		cancel:    cancel,
		dbName:    dbName,
		indexName: indexName,
	}
}

// Context returns the test context
func (e *TestEnv) Context() context.Context {
	return e.ctx
}

// Cleanup cleans up test resources - drops the test database, clears Redis, and deletes Meilisearch index
func (e *TestEnv) Cleanup(t *testing.T) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Drop the test database
	if e.MongoRepo != nil {
		client, err := mongo.Connect(ctx, options.Client().ApplyURI(e.Config.MongoURI))
		if err == nil {
			_ = client.Database(e.dbName).Drop(ctx)
			_ = client.Disconnect(ctx)
		}
		_ = e.MongoRepo.Close(ctx)
	}

	// Clear Redis test data
	if e.RedisRepo != nil {
		_ = e.RedisRepo.Close()
	}

	// Delete Meilisearch test index
	if e.MeiliRepo != nil {
		meiliClient := meilisearch.New(e.Config.MeilisearchHost, meilisearch.WithAPIKey(e.Config.MeilisearchKey))
		_, _ = meiliClient.DeleteIndex(e.indexName)
	}

	e.cancel()
}

// FlushRedis clears all data in the Redis test database
func (e *TestEnv) FlushRedis(t *testing.T) {
	t.Helper()

	redisClient := redis.NewClient(&redis.Options{
		Addr:     e.Config.RedisAddr,
		Password: e.Config.RedisPassword,
		DB:       e.Config.RedisDB,
	})
	defer redisClient.Close()

	if err := redisClient.FlushDB(e.ctx).Err(); err != nil {
		t.Logf("Warning: failed to flush Redis: %v", err)
	}
}

// RequireMongo skips the test if MongoDB is not available
func RequireMongo(t *testing.T) {
	t.Helper()
	mongoURI := getEnvOrDefault("TEST_MONGO_URI", "mongodb://localhost:27017")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		t.Skipf("Skipping test: MongoDB not available: %v", err)
	}
	defer client.Disconnect(ctx)

	if err := client.Ping(ctx, nil); err != nil {
		t.Skipf("Skipping test: MongoDB not responding: %v", err)
	}
}

// RequireRedis skips the test if Redis is not available
func RequireRedis(t *testing.T) {
	t.Helper()
	redisAddr := getEnvOrDefault("TEST_REDIS_ADDR", "localhost:6379")
	redisPassword := getEnvOrDefault("TEST_REDIS_PASSWORD", "")

	client := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       15,
	})
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Skipping test: Redis not available: %v", err)
	}
}

// RequireMeili skips the test if Meilisearch is not available
func RequireMeili(t *testing.T) {
	t.Helper()
	meiliHost := getEnvOrDefault("TEST_MEILI_HOST", "http://localhost:7700")
	meiliKey := getEnvOrDefault("TEST_MEILI_KEY", "")

	client := meilisearch.New(meiliHost, meilisearch.WithAPIKey(meiliKey))
	if !client.IsHealthy() {
		t.Skipf("Skipping test: Meilisearch not available at %s", meiliHost)
	}
}

// HasMeili returns true if MeiliRepo is available
func (e *TestEnv) HasMeili() bool {
	return e.MeiliRepo != nil
}

// MongoReactionTarget is re-exported for tests
type MongoReactionTarget = repository.MongoReactionTarget

// ReactionTarget is re-exported for tests
type ReactionTarget = repository.ReactionTarget

// IsValidSchemaKey is re-exported for tests
var IsValidSchemaKey = repository.IsValidSchemaKey
