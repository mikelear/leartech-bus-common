package logger

import (
	"context"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type RedisLogger struct {
	logger *zerolog.Logger
}

// NewRedisLogger creates a new RedisLogger instance that wraps the global zerolog logger.
// Global logger is used if no logger is provided (this is the common case).
func NewRedisLogger(logger *zerolog.Logger) *RedisLogger {
	if logger == nil {
		logger = &log.Logger
	}
	return &RedisLogger{logger: logger}
}

func (r *RedisLogger) Printf(ctx context.Context, format string, v ...interface{}) {
	r.logger.Info().Ctx(ctx).Msgf(format, v...)
}
