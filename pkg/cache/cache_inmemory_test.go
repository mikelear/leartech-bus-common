package cache_test

import (
	"github.com/mikelear/leartech-bus-common/pkg/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestNewInMemoryCache(t *testing.T) {
	ctx := t.Context()
	ttl := 100 * time.Millisecond
	objToCache := "ObjectToCache"
	cacheKey := "Key"
	c := cache.NewInMemoryCache[string](ttl, "SomePrefix")

	// Set an object in the cache
	c.Set(ctx, cacheKey, objToCache)

	// Check that we can get the object out the cache
	out, exists := c.Get(ctx, cacheKey)
	assert.Equal(t, objToCache, out)
	assert.True(t, exists)

	// Check that the object gets garbage collected
	time.Sleep(ttl)
	out, exists = c.Get(ctx, cacheKey)
	assert.Empty(t, out)
	assert.False(t, exists)
}

func TestInMemoryCache_DeleteKeysMatching(t *testing.T) {
	ctx := t.Context()
	ttl := 1 * time.Minute
	objToCache := "ObjectToCache"
	cacheKey := "Key"
	prefix := "SomePrefix"
	c := cache.NewInMemoryCache[string](ttl, prefix)

	// Set an object in the cache
	c.Set(ctx, cacheKey, objToCache)
	// Check that we can get the object out the cache
	out, exists := c.Get(ctx, cacheKey)
	assert.Equal(t, objToCache, out)
	assert.True(t, exists)

	// Delete the object using a pattern
	err := c.DeleteKeysMatching(ctx, "SomePrefix*")
	require.NoError(t, err)

	// Check that the object has been deleted
	out, exists = c.Get(ctx, cacheKey)
	assert.Empty(t, out)
	assert.False(t, exists)
}

func TestInMemory_Delete(t *testing.T) {
	ctx := t.Context()
	ttl := 1 * time.Minute
	objToCache := "ObjectToCache"
	cacheKey := "Key"
	prefix := "SomePrefix"
	c := cache.NewInMemoryCache[string](ttl, prefix)

	// Set an object in the cache
	c.Set(ctx, cacheKey, objToCache)
	// Check that we can get the object out the cache
	out, exists := c.Get(ctx, cacheKey)
	assert.Equal(t, objToCache, out)
	assert.True(t, exists)

	// Delete the object
	c.Delete(ctx, cacheKey)

	// Check that the object has been deleted
	out, exists = c.Get(ctx, cacheKey)
	assert.Empty(t, out)
	assert.False(t, exists)
}
