package redis

import (
	"github.com/redis/go-redis/v9"
)

var (
	// ErrClosed is returned when the Redis client is already closed.
	// Wrapping the redis package error so we don't have to import both redis-go & this redis package everywhere.
	ErrClosed = redis.ErrClosed
)
