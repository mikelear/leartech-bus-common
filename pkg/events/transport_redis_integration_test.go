//go:build integration

package events_test

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mikelear/leartech-bus-common/pkg/events"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// requireRedis returns a live redis client or SKIPs the test if no broker
// is reachable. Point at a local Redis with REDIS_ADDR=localhost:6379.
func requireRedis(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		t.Skipf("no reachable Redis at %s: %v", addr, err)
	}
	return client
}

// TestRedisTransport_Integration_PublishSubscribe verifies an actual
// round-trip through Redis PUBLISH/SUBSCRIBE. This test is gated by the
// `integration` build tag and skipped when no Redis broker is available.
func TestRedisTransport_Integration_PublishSubscribe(t *testing.T) {
	client := requireRedis(t)

	// Use a unique channel per test run so parallel CI executions don't
	// step on each other.
	topic := "leartech-bus-common:events-test:" + time.Now().Format("20060102T150405.000000000")

	tr, err := events.NewRedisTransport(client)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tr.Close() })

	var mu sync.Mutex
	got := make([]string, 0, 3)
	done := make(chan struct{})
	var expected atomic.Int32
	expected.Store(3)

	require.NoError(t, tr.Subscribe(topic, func(d []byte) {
		mu.Lock()
		got = append(got, string(d))
		remaining := len(got)
		mu.Unlock()
		if int32(remaining) >= expected.Load() {
			select {
			case <-done:
			default:
				close(done)
			}
		}
	}))

	// Give Redis a moment to register the subscription -- go-redis
	// pub/sub is asynchronous, and publishing before the subscribe
	// completes drops the messages.
	time.Sleep(100 * time.Millisecond)

	for _, msg := range []string{"one", "two", "three"} {
		require.NoError(t, tr.Publish(topic, []byte(msg)))
	}

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for messages; got %v", got)
	}

	mu.Lock()
	defer mu.Unlock()
	assert.ElementsMatch(t, []string{"one", "two", "three"}, got)
}
