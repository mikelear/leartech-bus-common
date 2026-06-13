package logger

import (
	"bytes"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRedisLogger_Printf(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	redisLogger := NewRedisLogger(&logger)

	ctx := t.Context()
	redisLogger.Printf(ctx, "hello %s", "world")

	output := buf.String()
	assert.JSONEq(t, "{\"level\":\"info\",\"message\":\"hello world\"}\n", output)
}
