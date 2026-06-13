package redis

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"github.com/mikelear/leartech-bus-common/pkg/logger"
)

// RedisClient is an interface wrapping the Redis client operations needed for caching.
// It abstracts the go-redis client to allow for easier testing and mocking.
//
// The *redis.Client from github.com/redis/go-redis/v9 implements this interface.
type RedisClient interface {
	Get(ctx context.Context, key string) *redis.StringCmd
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	Ping(ctx context.Context) *redis.StatusCmd
	Keys(ctx context.Context, pattern string) *redis.StringSliceCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	LPush(ctx context.Context, key string, values ...interface{}) *redis.IntCmd
	RPop(ctx context.Context, key string) *redis.StringCmd
	Close() error
}

// RedisConfig contains configuration for connecting to Redis.
// It supports both standard and Sentinel (high availability) configurations.
type RedisConfig struct {
	// URL is the Redis host address (default: "localhost")
	URL string `yaml:"url"`

	// Port is the Redis port (default: 6379)
	Port int `yaml:"port"`

	// DB is the Redis database number (default: 0)
	DB int `yaml:"db"`

	// Sentinels is a comma-separated list of sentinel hosts for high availability.
	// If provided, Redis Sentinel mode will be used instead of direct connection.
	// Example: "sentinel1.example.com,sentinel2.example.com"
	Sentinels string `env:"REDIS_SENTINELS"`

	// MasterName is the name of the master instance when using Redis Sentinel.
	// Required when Sentinels is provided.
	MasterName string `yaml:"masterName"`

	// PoolSize is the maximum number of socket connections (default: 10 per CPU)
	PoolSize int `yaml:"poolSize"`
}

// NewRedisClient creates and initializes a Redis client with the given configuration.
// It automatically configures either a standard client or a Sentinel failover client
// based on the presence of the Sentinels configuration.
//
// The client is tested with a PING command before being returned. If the ping fails,
// an error is returned.
func NewRedisClient(cfg RedisConfig) (*redis.Client, error) {
	redis.SetLogger(logger.NewRedisLogger(nil))

	var client *redis.Client
	if cfg.Sentinels != "" {
		log.Info().Msg("Redis sentinels configured, using fail-over client")
		client = configureFailOverClient(cfg)
	} else {
		log.Info().Msg("Redis sentinels not configured, using standard client")
		client = configureStandardClient(cfg)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ping, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, errors.Wrap(err, "redis client ping error")
	}
	if ping != "PONG" {
		return nil, errors.New("redis failed to respond correctly to ping")
	}
	return client, nil
}

func configureFailOverClient(cfg RedisConfig) *redis.Client {
	sentinels := strings.Split(cfg.Sentinels, ",")
	for i := range sentinels {
		sentinels[i] += ":26379"
	}
	return redis.NewFailoverClient(&redis.FailoverOptions{
		MasterName:    cfg.MasterName,
		SentinelAddrs: sentinels,
		DB:            cfg.DB,
		PoolSize:      cfg.PoolSize,
	})
}

func configureStandardClient(cfg RedisConfig) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.URL, strconv.Itoa(cfg.Port)),
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})
}
