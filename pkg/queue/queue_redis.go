package queue

import (
	"context"
	"encoding/json"

	rediscommon "github.com/mikelear/leartech-bus-common/pkg/redis"
)

// RedisQueue is a distributed cache implementation backed by RedisQueue.
// It uses JSON serialization to store values.
//
// Type T must be JSON-serializable (compatible with encoding/json).
type RedisQueue[T any] struct {
	client rediscommon.RedisClient
}

// NewRedisQueue creates a new RedisQueue-backed queue for type T.
//
// Parameters:
//   - client: RedisQueue client implementing the RedisClient interface
//   - prefix: Key prefix for namespace isolation (e.g., "user-service", "session")
func NewRedisQueue[T any](client rediscommon.RedisClient) *RedisQueue[T] {
	return &RedisQueue[T]{
		client: client,
	}
}

func (r *RedisQueue[T]) Push(ctx context.Context, queue string, value T) error {
	eventData, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return r.client.LPush(ctx, queue, string(eventData)).Err()
}

func (r *RedisQueue[T]) Pop(ctx context.Context, queue string) (T, error) {
	var value T

	eventData, err := r.client.RPop(ctx, queue).Bytes()
	if err != nil {
		return value, err
	}

	if err = json.Unmarshal(eventData, &value); err != nil {
		return value, err
	}

	return value, nil
}
