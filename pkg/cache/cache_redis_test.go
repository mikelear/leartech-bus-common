package cache_test

import (
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/mikelear/leartech-bus-common/pkg/cache"
	rediscommon "github.com/mikelear/leartech-bus-common/pkg/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedisCache(t *testing.T) {
	mockRedis := rediscommon.NewMockRedisClient(t)

	ctx := t.Context()
	ttl := 100 * time.Millisecond
	objToCache := "ObjectToCache"
	cacheKey := "Key"
	prefix := "SomePrefix"
	expectedFullKey := prefix + ":" + cacheKey

	c := cache.NewRedisCache[string](mockRedis, ttl, "SomePrefix")

	// Set an object in the cache
	mockRedis.On("Set", ctx, expectedFullKey, []byte(`"`+objToCache+`"`), ttl).Return(redis.NewStatusResult("OK", nil))
	c.Set(ctx, cacheKey, objToCache)

	// Check that we can get the object out the cache
	mockRedis.On("Get", ctx, expectedFullKey).Return(redis.NewStringResult(`"`+objToCache+`"`, nil))
	out, exists := c.Get(ctx, cacheKey)
	assert.Equal(t, objToCache, out)
	assert.True(t, exists)
}

func TestRedisCache_DeleteKeysMatching(t *testing.T) {
	mockRedis := rediscommon.NewMockRedisClient(t)

	ctx := t.Context()
	ttl := 1 * time.Minute
	objToCache := "ObjectToCache"
	cacheKey := "Key"
	prefix := "SomePrefix"
	expectedFullKey := prefix + ":" + cacheKey

	c := cache.NewRedisCache[string](mockRedis, ttl, prefix)

	// Set an object in the cache
	mockRedis.On("Set", ctx, expectedFullKey, []byte(`"`+objToCache+`"`), ttl).Return(redis.NewStatusResult("OK", nil))
	c.Set(ctx, cacheKey, objToCache)
	// Check that we can get the object out the cache
	mockRedis.On("Get", ctx, expectedFullKey).Return(redis.NewStringResult(`"`+objToCache+`"`, nil))
	out, exists := c.Get(ctx, cacheKey)
	assert.Equal(t, objToCache, out)
	assert.True(t, exists)

	// Delete the object using a pattern
	mockRedis.On("Keys", ctx, "*").Return(redis.NewStringSliceResult([]string{expectedFullKey}, nil))
	mockRedis.On("Del", ctx, []string{expectedFullKey}).Return(redis.NewIntResult(1, nil))
	err := c.DeleteKeysMatching(ctx, "*")
	require.NoError(t, err)
}

func TestRedis_Delete(t *testing.T) {
	mockRedis := rediscommon.NewMockRedisClient(t)

	ctx := t.Context()
	ttl := 1 * time.Minute
	objToCache := "ObjectToCache"
	cacheKey := "Key"
	prefix := "SomePrefix"
	expectedFullKey := prefix + ":" + cacheKey
	c := cache.NewRedisCache[string](mockRedis, ttl, prefix)

	// Set an object in the cache
	mockRedis.On("Set", ctx, expectedFullKey, []byte(`"`+objToCache+`"`), ttl).Return(redis.NewStatusResult("OK", nil)).Once()
	c.Set(ctx, cacheKey, objToCache)

	// Check that we can get the object out the cache
	mockRedis.On("Get", ctx, expectedFullKey).Return(redis.NewStringResult(`"`+objToCache+`"`, nil)).Once()
	out, exists := c.Get(ctx, cacheKey)
	assert.Equal(t, objToCache, out)
	assert.True(t, exists)

	// Delete the object
	mockRedis.On("Del", ctx, []string{expectedFullKey}).Return(redis.NewIntResult(1, nil)).Once()
	c.Delete(ctx, cacheKey)
	// Check that the object has been deleted
	mockRedis.On("Get", ctx, expectedFullKey).Return(redis.NewStringResult("", redis.Nil)).Once()
	out, exists = c.Get(ctx, cacheKey)
	assert.Empty(t, out)
	assert.False(t, exists)
}
