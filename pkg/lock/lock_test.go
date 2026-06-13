package lock_test

import (
	"github.com/mikelear/leartech-bus-common/pkg/lock"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestLock(t *testing.T) {
	prefix := "test-lock"
	ttl := 100 * time.Millisecond
	key := "resource"
	ctx := t.Context()
	locker := lock.NewInMemoryLock(ttl, prefix)

	// Acquire lock for the first time
	acquired := locker.AcquireLock(ctx, key)
	require.True(t, acquired)

	// Try to acquire the lock again immediately
	acquired = locker.AcquireLock(ctx, key)
	require.False(t, acquired)

	// Release the lock
	locker.ReleaseLock(ctx, key)

	// Try to acquire the lock again after release
	acquired = locker.AcquireLock(ctx, key)
	require.True(t, acquired)

	// Wait for TTL to expire
	time.Sleep(ttl + 100*time.Millisecond)

	// Try to acquire the lock after TTL expiration
	acquired = locker.AcquireLock(ctx, key)
	require.True(t, acquired)

	// Clean up
	locker.ReleaseLock(ctx, key)
}

func TestTryFunc(t *testing.T) {
	prefix := "test-lock"
	ttl := 100 * time.Millisecond
	key := "resource"
	ctx := t.Context()

	locker := lock.NewInMemoryLock(ttl, prefix)

	// Try to execute function under lock
	var hit bool
	fnRan, err := locker.TryFunc(ctx, key, func() error {
		hit = true
		return nil
	})
	require.NoError(t, err)
	require.True(t, hit)
	require.True(t, fnRan)

	// Acquire the lock manually
	acquired := locker.AcquireLock(ctx, key)
	require.True(t, acquired)

	// Try to execute function under lock when already locked
	hit = false
	fnRan, err = locker.TryFunc(ctx, key, func() error {
		hit = true
		return nil
	})
	require.NoError(t, err)
	require.False(t, hit)
	require.False(t, fnRan)
}
