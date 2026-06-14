package cache_test

import (
	"context"
	"fmt"
	"time"

	"github.com/mikelear/leartech-bus-common/pkg/cache"
	"github.com/mikelear/leartech-bus-common/pkg/redis"
	"github.com/rs/zerolog/log"
)

// User represents a simple user model for demonstration purposes.
type User struct {
	ID        string
	Name      string
	Email     string
	CreatedAt time.Time
}

// ExampleNewRedisCache demonstrates creating and using a Redis-backed cache.
//
//nolint:testableexamples // Cannot start a Redis server in the test environment
func ExampleNewRedisCache() {
	ctx := context.Background()

	// Create Redis client with standard configuration
	client, err := redis.NewRedisClient(redis.RedisConfig{
		URL:      "localhost",
		Port:     6379,
		DB:       0,
		PoolSize: 10,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create redis client")
		return
	}
	defer func() {
		if err := client.Close(); err != nil {
			log.Error().Err(err).Msg("failed to close redis client")
		}
	}()

	// Create a cache for User objects with 10-minute TTL
	userCache := cache.NewRedisCache[User](client, 10*time.Minute, "users")

	// Store a user in the cache
	user := User{
		ID:        "123",
		Name:      "Alice Smith",
		Email:     "alice@example.com",
		CreatedAt: time.Now(),
	}
	userCache.Set(ctx, "123", user)

	// Retrieve the user from cache
	cachedUser, exists := userCache.Get(ctx, "123")
	if !exists {
		log.Warn().Msg("user not found in cache")
		return
	}

	fmt.Printf("Found user: %s (%s)\n", cachedUser.Name, cachedUser.Email)

	// Delete the user from cache
	userCache.Delete(ctx, "123")

	// Verify deletion
	_, exists = userCache.Get(ctx, "123")
	if !exists {
		fmt.Println("User successfully deleted from cache")
	}
}

// ExampleNewRedisCache_sentinel demonstrates Redis Sentinel configuration
// for high availability deployments.
//
//nolint:testableexamples // Cannot start a Redis server in the test environment
func ExampleNewRedisCache_sentinel() {
	ctx := context.Background()

	// Create Redis client with Sentinel configuration
	client, err := redis.NewRedisClient(redis.RedisConfig{
		Sentinels:  "sentinel1.example.com,sentinel2.example.com,sentinel3.example.com",
		MasterName: "mymaster",
		DB:         0,
		PoolSize:   20,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create redis client with sentinels")
		return
	}
	defer func() {
		if err := client.Close(); err != nil {
			log.Error().Err(err).Msg("failed to close redis client")
		}
	}()

	// Create a cache for User objects with 10-minute TTL
	userCache := cache.NewRedisCache[User](client, 10*time.Minute, "users")

	// Store a user in the cache
	user := User{
		ID:        "123",
		Name:      "Alice Smith",
		Email:     "alice@example.com",
		CreatedAt: time.Now(),
	}
	userCache.Set(ctx, "123", user)

	// Retrieve the user from cache
	cachedUser, exists := userCache.Get(ctx, "123")
	if !exists {
		log.Warn().Msg("user not found in cache")
		return
	}

	fmt.Printf("Found user: %s (%s)\n", cachedUser.Name, cachedUser.Email)

	// Delete the user from cache
	userCache.Delete(ctx, "123")

	// Verify deletion
	_, exists = userCache.Get(ctx, "123")
	if !exists {
		fmt.Println("User successfully deleted from cache")
	}
}

// ExampleNewInMemoryCache demonstrates creating and using an in-memory cache.
// This is ideal for development, testing, or single-instance applications.
func ExampleNewInMemoryCache() {
	ctx := context.Background()

	// Create a cache for User objects with 10-minute TTL
	userCache := cache.NewInMemoryCache[User](10*time.Minute, "users")

	// Store a user in the cache
	user := User{
		ID:        "123",
		Name:      "Alice Smith",
		Email:     "alice@example.com",
		CreatedAt: time.Now(),
	}
	userCache.Set(ctx, "123", user)

	// Retrieve the user from cache
	cachedUser, exists := userCache.Get(ctx, "123")
	if !exists {
		log.Warn().Msg("user not found in cache")
		return
	}

	fmt.Printf("Found user: %s (%s)\n", cachedUser.Name, cachedUser.Email)

	// Delete the user from cache
	userCache.Delete(ctx, "123")

	// Verify deletion
	_, exists = userCache.Get(ctx, "123")
	if !exists {
		fmt.Println("User successfully deleted from cache")
	}

	// Output:
	// Found user: Alice Smith (alice@example.com)
	// User successfully deleted from cache
}
