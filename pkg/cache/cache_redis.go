package cache

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	rediscommon "github.com/mikelear/leartech-bus-common/pkg/redis"

	"time"
)

// Redis is a distributed cache implementation backed by Redis.
// It uses JSON serialization to store values and supports TTL-based expiration.
//
// All cache operations are safe to call concurrently. Errors during operations
// are logged but do not cause panics; Get returns (zero-value, false) on errors.
//
// Type T must be JSON-serializable (compatible with encoding/json).
type Redis[T any] struct {
	client rediscommon.RedisClient
	ttl    time.Duration
	prefix string
}

// NewRedisCache creates a new Redis-backed cache for type T.
//
// Parameters:
//   - client: Redis client implementing the RedisClient interface
//   - ttl: Time-to-live for cache entries. Use 0 for no expiration.
//   - prefix: Key prefix for namespace isolation (e.g., "user-service", "session")
func NewRedisCache[T any](client rediscommon.RedisClient, ttl time.Duration, prefix string) *Redis[T] {
	return &Redis[T]{
		client: client,
		ttl:    ttl,
		prefix: prefix,
	}
}

func (r *Redis[T]) Get(ctx context.Context, key string) (value T, exist bool) {
	key = getFullKey(r.prefix, key)
	data, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		switch {
		case errors.Is(err, redis.Nil):
			// Key does not exist so return false
			return value, false
		default:
			// Some other error occurred
			log.Ctx(ctx).Error().Err(err).Str("key", key).Msg("failed to get key from redis")
			return value, false
		}
	}
	if err = json.Unmarshal(data, &value); err != nil {
		log.Ctx(ctx).Error().Err(err).Msgf("failed to unmarshal key \"%s\" from redis", key)
		return value, false
	}
	return value, true
}

func (r *Redis[T]) Set(ctx context.Context, key string, value T) {
	key = getFullKey(r.prefix, key)
	data, err := json.Marshal(value)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Str("key", key).Msg("failed to marshal key from redis")
		return
	}
	if err = r.client.Set(ctx, key, data, r.ttl).Err(); err != nil {
		log.Ctx(ctx).Error().Err(err).Str("key", key).Msg("failed to set key in redis")
	}
}

func (r *Redis[T]) Delete(ctx context.Context, key string) {
	key = getFullKey(r.prefix, key)
	if err := r.client.Del(ctx, key).Err(); err != nil {
		log.Ctx(ctx).Error().Err(err).Str("key", key).Msg("failed to delete key from redis")
	}
}

func (r *Redis[T]) DeleteKeysMatching(ctx context.Context, pattern string) error {
	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		return errors.Wrap(err, "failed to get keys matching pattern")
	}
	if len(keys) == 0 {
		// No keys to delete so return
		return nil
	}
	log.Ctx(ctx).Info().Msgf("Deleted %d key(s) matching pattern \"%s\"", len(keys), pattern)
	if err = r.client.Del(ctx, keys...).Err(); err != nil {
		return errors.Wrap(err, "failed to delete keys matching pattern")
	}
	return nil
}
