// Package lock provides distributed locking capabilities built on the cache abstraction.
//
// The lock package enables coordination of access to shared resources across multiple service
// instances or goroutines. It uses the cache package as its storage backend, supporting both
// Redis-based distributed locks and in-memory locks for single-instance applications.
package lock

import (
	"context"
)

// Locker provides distributed locking operations for coordinating access to shared resources.
//
// Implementations:
//   - Lock: Supports both Redis (distributed) and in-memory (local) backends
//
// All operations are safe to call concurrently. AcquireLock returns false if the lock
// is already held by another process or goroutine. Locks automatically expire after
// their configured TTL.
//
// Lock keys are automatically prefixed with the configured prefix to provide namespace
// isolation between different lock domains or services.
type Locker interface {
	// AcquireLock attempts to acquire a lock for the given key.
	// Returns true if the lock was successfully acquired, false if the lock is already held.
	//
	// The lock will automatically expire after the configured TTL if not released.
	// The key is automatically prefixed with the lock's configured prefix.
	AcquireLock(ctx context.Context, key string) bool

	// ReleaseLock releases the lock for the given key.
	// If the key is not locked, this is a no-op.
	//
	// Always call ReleaseLock when finished with a lock, preferably using defer
	// to ensure release even if panics occur.
	//
	// The key is automatically prefixed with the lock's configured prefix.
	ReleaseLock(ctx context.Context, key string)

	// TryFunc acquires a lock for the given key, executes the provided function,
	// and then releases the lock.
	// Returns (true, error) if the lock was acquired and fn was executed, where error
	// is any error returned by fn.
	// Returns (false, nil) if the lock could not be acquired and fn was not executed.
	TryFunc(ctx context.Context, key string, fn func() error) (bool, error)
}
