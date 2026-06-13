package cache

import (
	"context"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"regexp"
	"sync"
	"time"
)

type cacheItem[T any] struct {
	value    T
	cachedAt time.Time
}

// InMemory is a local in-process cache implementation with TTL-based expiration.
// It stores data in a synchronized map and is safe for concurrent access.
//
// Note: This cache is local to a single process instance. It does not provide
// distributed caching across multiple service instances. For production use cases
// requiring distributed caching, use Redis[T] instead.
//
// InMemory is ideal for:
//   - Development and testing
//   - Single-instance applications
//   - Caching expensive computations within a process
//   - Reducing load on external cache services
type InMemory[T any] struct {
	lock   sync.RWMutex
	data   map[string]cacheItem[T]
	ttl    time.Duration
	prefix string
}

// NewInMemoryCache creates a new in-memory cache for type T.
//
// Parameters:
//   - ttl: Time-to-live for cache entries. Expired entries are removed on access. Use 0 for no expiration.
//   - prefix: Key prefix for namespace isolation (e.g., "sessions", "config")
//
// Note: This is a local, non-distributed cache. For production use cases requiring
// cache sharing across multiple instances, use NewRedisCache instead.
func NewInMemoryCache[T any](ttl time.Duration, prefix string) *InMemory[T] {
	c := &InMemory[T]{
		data:   make(map[string]cacheItem[T]),
		ttl:    ttl,
		prefix: prefix,
	}
	return c
}

func (i *InMemory[T]) Get(_ context.Context, key string) (T, bool) {
	i.lock.Lock()
	defer i.lock.Unlock()
	item, exist := i.data[getFullKey(i.prefix, key)]

	// If the item exists but the TTL has expired, delete it and return not found
	if exist && i.hasTTLExpiredForItem(item) {
		var zero T
		delete(i.data, getFullKey(i.prefix, key))
		return zero, false
	}
	return item.value, exist
}

func (i *InMemory[T]) Set(_ context.Context, key string, value T) {
	i.lock.Lock()
	defer i.lock.Unlock()

	i.data[getFullKey(i.prefix, key)] = cacheItem[T]{
		value:    value,
		cachedAt: time.Now(),
	}
}

func (i *InMemory[T]) Delete(_ context.Context, key string) {
	i.lock.Lock()
	defer i.lock.Unlock()
	delete(i.data, getFullKey(i.prefix, key))
}

func (i *InMemory[T]) DeleteKeysMatching(ctx context.Context, pattern string) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return errors.Wrapf(err, "failed to compile regex pattern \"%s\"", pattern)
	}
	i.lock.Lock()
	defer i.lock.Unlock()
	var deleted int
	for key := range i.data {
		if re.MatchString(key) {
			delete(i.data, key)
			deleted++
		}
	}
	log.Info().Ctx(ctx).Msgf("Deleted %d key(s) matching pattern \"%s\"", deleted, pattern)
	return nil
}

func (i *InMemory[T]) hasTTLExpiredForItem(item cacheItem[T]) bool {
	return i.ttl > 0 && time.Since(item.cachedAt) > i.ttl
}
