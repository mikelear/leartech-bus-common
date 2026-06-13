package lock_test

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/mikelear/leartech-bus-common/pkg/lock"
	"github.com/mikelear/leartech-bus-common/pkg/redis"
)

// ExampleNewRedisLock demonstrates creating and using a distributed lock with Redis.
//
//nolint:testableexamples // Cannot start a Redis server in the test environment
func ExampleNewRedisLock() {
	ctx := context.Background()

	// Create Redis client with standard configuration
	// See pkg/cache/example_test.go for more details
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

	// Create a distributed lock with 30-second TTL
	// The lock will automatically expire after 30 seconds if not released
	locker := lock.NewRedisLock(client, 30*time.Second, "resource-locks")

	// Try to acquire a lock for a specific resource
	resourceID := "user:123:profile"
	acquired := locker.AcquireLock(ctx, resourceID)
	if !acquired {
		fmt.Println("Failed to acquire lock - resource is locked by another process")
		return
	}

	fmt.Println("Lock acquired successfully")

	// Perform critical section work here
	// processUserProfile(resourceID)

	// Always release the lock when done
	locker.ReleaseLock(ctx, resourceID)
	fmt.Println("Lock released")
}

// ExampleNewInMemoryLock demonstrates creating and using an in-memory lock.
// This is ideal for single-instance applications, development, or testing.
func ExampleNewInMemoryLock() {
	ctx := context.Background()

	// Create an in-memory lock with 10-second TTL
	locker := lock.NewInMemoryLock(10*time.Second, "local-locks")

	// Try to acquire a lock for a local resource
	resourceID := "file:export-123.csv"
	acquired := locker.AcquireLock(ctx, resourceID)
	if !acquired {
		fmt.Println("Failed to acquire lock - resource is already being processed")
		return
	}

	fmt.Println("Lock acquired - processing file")

	// Perform file processing
	// processFile(resourceID)

	// Release the lock when done
	locker.ReleaseLock(ctx, resourceID)
	fmt.Println("File processed and lock released")

	// Output:
	// Lock acquired - processing file
	// File processed and lock released
}

// ExampleLock_AcquireLock demonstrates various lock acquisition scenarios.
func ExampleLock_AcquireLock() {
	ctx := context.Background()
	locker := lock.NewInMemoryLock(5*time.Second, "example-locks")

	resourceID := "shared-resource"

	// First acquisition - should succeed
	if locker.AcquireLock(ctx, resourceID) {
		fmt.Println("First lock acquisition: success")
	}

	// Second acquisition attempt - should fail (already locked)
	if !locker.AcquireLock(ctx, resourceID) {
		fmt.Println("Second lock acquisition: failed (already locked)")
	}

	// Release the lock
	locker.ReleaseLock(ctx, resourceID)

	// Third acquisition attempt - should succeed (lock was released)
	if locker.AcquireLock(ctx, resourceID) {
		fmt.Println("Third lock acquisition: success (after release)")
	}

	// Clean up
	locker.ReleaseLock(ctx, resourceID)

	// Output:
	// First lock acquisition: success
	// Second lock acquisition: failed (already locked)
	// Third lock acquisition: success (after release)
}

// ExampleLock_AcquireLock_defer demonstrates how to use defer to ensure locks are released.
func ExampleLock_AcquireLock_defer() {
	ctx := context.Background()
	locker := lock.NewInMemoryLock(5*time.Second, "defer-locks")

	resourceID := "resourceID"

	// Acquire the lock
	isLocked := locker.AcquireLock(ctx, resourceID)
	if !isLocked {
		fmt.Println("Failed to acquire lock")
		return
	}
	defer locker.ReleaseLock(ctx, resourceID)

	// Do critical section work
	fmt.Println("Lock acquired, executing critical section")

	// Output:
	// Lock acquired, executing critical section
}

// ExampleLock_TryFunc demonstrates using TryFunc to execute a function under a lock.
func ExampleLock_TryFunc() {
	ctx := context.Background()
	locker := lock.NewInMemoryLock(5*time.Second, "tryfunc-locks")

	resourceID := "task-456"

	fnRan, err := locker.TryFunc(ctx, resourceID, func() error {
		fmt.Println("Lock acquired in TryFunc, executing task")
		// Do task work here
		return nil
	})
	if err != nil {
		fmt.Printf("Error executing task: %v\n", err)
		return
	}
	if fnRan {
		fmt.Println("Task completed successfully")
	}

	// Output:
	// Lock acquired in TryFunc, executing task
	// Task completed successfully
}
