package lock

import (
	"context"

	"github.com/mikelear/leartech-bus-common/pkg/cache"
	"github.com/mikelear/leartech-bus-common/pkg/redis"

	"time"
)

// Lock is a distributed locking implementation backed by a cache.
// It supports both Redis-backed distributed locks and in-memory locks depending
// on the cache backend used during construction.
//
// Lock operations are safe to call concurrently. The lock uses cache key existence
// to track lock state - if a key exists in cache, the lock is held.
//
// Lock expiration is managed by the underlying cache's TTL. If a process crashes
// while holding a lock, the lock will automatically expire after the TTL.
type Lock struct {
	cache cache.Cache[struct{}]
}

// NewRedisLock creates a new distributed lock using Redis as the backend.
// This is suitable for coordinating access to shared resources across multiple
// service instances in a distributed system.
//
// Parameters:
//   - redis: Redis client implementing the RedisClient interface
//   - ttl: Lock expiration time. Locks automatically release after this duration.
//     Choose a TTL longer than expected operation time but short enough to recover from crashes.
//     Common values: 30s for quick operations, 1-5m for longer operations.
//   - prefix: Key prefix for namespace isolation (e.g., "resource-locks", "job-locks")
func NewRedisLock(redis redis.RedisClient, ttl time.Duration, prefix string) *Lock {
	return &Lock{
		cache: cache.NewRedisCache[struct{}](redis, ttl, prefix),
	}
}

// NewInMemoryLock creates a new local lock using in-memory cache as the backend.
// This is suitable for single-instance applications, development, and testing.
//
// Note: This lock is local to a single process. It does not provide distributed
// locking across multiple service instances. For production use cases requiring
// distributed coordination, use NewRedisLock instead.
//
// Parameters:
//   - ttl: Lock expiration time. Locks automatically release after this duration.
//   - prefix: Key prefix for namespace isolation (e.g., "local-locks", "test-locks")
func NewInMemoryLock(ttl time.Duration, prefix string) *Lock {
	return &Lock{
		cache: cache.NewInMemoryCache[struct{}](ttl, prefix),
	}
}

// AcquireLock attempts to acquire a lock for the given key.
// Returns true if the lock was successfully acquired, false if already locked.
//
// If the lock is already held (by this process or another), this method returns
// false immediately without blocking. The caller should handle lock acquisition
// failure appropriately (e.g., return error, retry with backoff, or skip operation).
//
// The lock will automatically expire after the configured TTL if not released.
// Always release locks explicitly using ReleaseLock when done.
func (c *Lock) AcquireLock(ctx context.Context, key string) bool {
	if c.isLocked(ctx, key) {
		return false
	}
	c.cache.Set(ctx, key, struct{}{})
	return true
}

// ReleaseLock releases the lock for the given key.
// If the key is not currently locked, this is a no-op.
//
// Always call ReleaseLock when finished with a lock. Use defer to ensure
// the lock is released even if panics occur:
func (c *Lock) ReleaseLock(ctx context.Context, key string) {
	c.cache.Delete(ctx, key)
}

// TryFunc attempts to acquire a lock for the given key,
// executes the provided function fn while holding the lock, and then releases the lock.
// If the lock cannot be acquired, fn is not executed and (false, nil) is returned.
// If the lock is acquired, fn is executed and (true, error) is returned where error
// is any error returned by fn.
func (c *Lock) TryFunc(ctx context.Context, key string, fn func() error) (bool, error) {
	if !c.AcquireLock(ctx, key) {
		return false, nil
	}
	defer c.ReleaseLock(ctx, key)
	return true, fn()
}

// isLocked checks if a lock is currently held for the given key.
func (c *Lock) isLocked(ctx context.Context, key string) bool {
	_, exists := c.cache.Get(ctx, key)
	return exists
}
