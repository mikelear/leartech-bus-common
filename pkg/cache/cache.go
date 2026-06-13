// Package cache provides generic, type-safe caching abstractions with multiple backend implementations.
//
// The cache package offers a unified interface for caching data with support for both distributed
// (Redis) and local (in-memory) storage backends. All implementations use a prefix-based key
// structure to namespace cache entries and support TTL-based expiration.
//
// # Key Features
//
//   - Type-safe generic interface Cache[T any]
//   - Redis-backed distributed caching with JSON serialization
//   - Local in-memory caching with TTL expiration
//   - Automatic key prefixing for namespace isolation
//   - Pattern-based batch deletion
//   - Redis Sentinel support for high availability
//
// # Key Structure
//
// All cache keys follow the pattern: {prefix}:{key}
// For example, with prefix "user-service" and key "profile:123", the full key becomes "user-service:profile:123".
// This allows multiple services to share the same Redis instance without key collisions.
package cache

import (
	"context"
	"fmt"
)

// Cache is a generic interface for type-safe caching operations.
// It supports storing and retrieving values of any type T, with automatic
// key prefixing and TTL-based expiration.
//
// Implementations:
//   - Redis[T]: Distributed cache using Redis with JSON serialization
//   - InMemory[T]: Local in-process cache with TTL expiration
//
// All operations are safe to call concurrently. Errors during cache operations
// are logged but do not panic; Get returns (zero-value, false) on errors.
type Cache[T any] interface {
	// Get retrieves the value stored in the cache for the given key.
	// It returns the value and a boolean indicating whether the key exists.
	// If the key does not exist or an error occurs, exist will be false and
	// value will be the zero value for type T.
	//
	// The key is automatically prefixed with the cache's configured prefix.
	Get(ctx context.Context, key string) (value T, exist bool)

	// Set stores a value in the cache for the given key.
	// The value is stored with the cache's configured TTL.
	// Errors during the operation are logged but not returned.
	//
	// The key is automatically prefixed with the cache's configured prefix.
	Set(ctx context.Context, key string, value T)

	// Delete removes a key from the cache.
	// If the key does not exist, this is a no-op.
	// Errors during the operation are logged but not returned.
	//
	// The key is automatically prefixed with the cache's configured prefix.
	Delete(ctx context.Context, key string)

	// DeleteKeysMatching deletes all keys matching the given pattern.
	// For Redis: pattern uses Redis glob-style patterns (e.g., "user:*", "session:?bc")
	// For InMemory: pattern is a regular expression
	//
	// Returns an error if the pattern is invalid or if deletion fails.
	// Logs the number of keys deleted.
	DeleteKeysMatching(ctx context.Context, pattern string) error
}

func getFullKey(prefix, key string) string {
	return fmt.Sprintf("%s:%s", prefix, key)
}
