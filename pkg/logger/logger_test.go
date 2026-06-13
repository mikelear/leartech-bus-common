package logger

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitialiseLogger_JSONOutput(t *testing.T) {
	var buf bytes.Buffer

	InitialiseLogger(
		WithOutputStyle(OutputStyleJSON),
		WithLevel(zerolog.InfoLevel),
		WithOutput(&buf),
		WithTimestamp(true),
	)

	log.Info().Msg("test message")

	output := buf.String()
	assert.Contains(t, output, "test message")
	assert.Contains(t, output, `"level":"info"`)

	// Verify it's valid JSON
	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(output), &logEntry)
	require.NoError(t, err)
	assert.Equal(t, "info", logEntry["level"])
	assert.Equal(t, "test message", logEntry["message"])
}

func TestInitialiseLogger_ConsoleOutput(t *testing.T) {
	var buf bytes.Buffer

	InitialiseLogger(
		WithOutputStyle(OutputStyleConsole),
		WithLevel(zerolog.InfoLevel),
		WithOutput(&buf),
		WithTimestamp(true),
	)

	log.Info().Msg("test message")

	output := buf.String()
	assert.Contains(t, output, "test message")
	assert.Contains(t, output, "INF")
}

func TestInitialiseLogger_WithCaller(t *testing.T) {
	var buf bytes.Buffer

	InitialiseLogger(
		WithOutputStyle(OutputStyleJSON),
		WithOutput(&buf),
		WithCaller(true),
		WithTimestamp(false),
	)

	log.Info().Msg("test with caller")

	output := buf.String()
	assert.Contains(t, output, "caller")
	assert.Contains(t, output, "logger_test.go")
}

func TestInitialiseLogger_WithoutTimestamp(t *testing.T) {
	var buf bytes.Buffer

	InitialiseLogger(
		WithOutputStyle(OutputStyleJSON),
		WithOutput(&buf),
		WithTimestamp(false),
	)

	log.Info().Msg("test without timestamp")

	output := buf.String()
	// Verify JSON structure doesn't have time field
	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(output), &logEntry)
	require.NoError(t, err)
	assert.NotContains(t, logEntry, "time")
}

func TestInitialiseLogger_LogLevel(t *testing.T) {
	tests := []struct {
		name         string
		level        zerolog.Level
		logFunc      func()
		shouldAppear bool
	}{
		{
			name:  "debug level logs debug",
			level: zerolog.DebugLevel,
			logFunc: func() {
				log.Debug().Msg("debug message")
			},
			shouldAppear: true,
		},
		{
			name:  "info level ignores debug",
			level: zerolog.InfoLevel,
			logFunc: func() {
				log.Debug().Msg("debug message")
			},
			shouldAppear: false,
		},
		{
			name:  "error level ignores info",
			level: zerolog.ErrorLevel,
			logFunc: func() {
				log.Info().Msg("info message")
			},
			shouldAppear: false,
		},
		{
			name:  "error level logs error",
			level: zerolog.ErrorLevel,
			logFunc: func() {
				log.Error().Msg("error message")
			},
			shouldAppear: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			InitialiseLogger(
				WithOutputStyle(OutputStyleJSON),
				WithLevel(tt.level),
				WithOutput(&buf),
				WithTimestamp(false),
			)

			tt.logFunc()

			output := buf.String()
			if tt.shouldAppear {
				assert.NotEmpty(t, output, "expected log output")
			} else {
				assert.Empty(t, output, "expected no log output")
			}
		})
	}
}

func TestInitialiseLogger_CustomTimeFormat(t *testing.T) {
	var buf bytes.Buffer

	InitialiseLogger(
		WithOutputStyle(OutputStyleJSON),
		WithOutput(&buf),
		WithTimeFieldFormat(time.RFC822),
		WithTimestamp(true),
	)

	log.Info().Msg("test time format")

	output := buf.String()
	assert.Contains(t, output, "time")

	// Verify it's valid JSON with time field
	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(output), &logEntry)
	require.NoError(t, err)
	assert.NotEmpty(t, logEntry["time"])
}

func TestInitCLILogger(t *testing.T) {
	var buf bytes.Buffer

	InitCLILogger(
		WithOutput(&buf),
	)

	log.Info().Msg("cli message")

	output := buf.String()
	assert.Contains(t, output, "cli message")
	// Console output should have INF prefix
	assert.Contains(t, output, "INF")
}

func TestInitServiceLogger(t *testing.T) {
	var buf bytes.Buffer

	InitServiceLogger(
		WithOutput(&buf),
		WithTimestamp(false), // Override default for easier testing
	)

	log.Info().Msg("service message")

	output := buf.String()
	assert.Contains(t, output, "service message")
	assert.Contains(t, output, "caller") // Service logger should include caller

	// Verify it's JSON
	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(output), &logEntry)
	require.NoError(t, err)
	assert.Equal(t, "info", logEntry["level"])
}

func TestUpdateLoggerContext(t *testing.T) {
	var buf bytes.Buffer

	// Create a logger that writes to our buffer
	logger := zerolog.New(&buf)
	ctx := logger.WithContext(t.Context())

	// Add context values
	ctx = UpdateLoggerContext(ctx, "request_id", "12345")
	ctx = UpdateLoggerContext(ctx, "user_id", "user-abc")

	// Log using the context logger
	log.Ctx(ctx).Info().Msg("test context")

	output := buf.String()
	assert.Contains(t, output, "request_id")
	assert.Contains(t, output, "12345")
	assert.Contains(t, output, "user_id")
	assert.Contains(t, output, "user-abc")

	// Verify structure
	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(output), &logEntry)
	require.NoError(t, err)
	assert.Equal(t, "12345", logEntry["request_id"])
	assert.Equal(t, "user-abc", logEntry["user_id"])
}

func TestSetLogLevel(t *testing.T) {
	tests := []struct {
		name      string
		levelStr  string
		logFunc   func()
		shouldLog bool
	}{
		{
			name:     "set to debug",
			levelStr: "debug",
			logFunc: func() {
				log.Debug().Msg("debug message")
			},
			shouldLog: true,
		},
		{
			name:     "set to error filters info",
			levelStr: "error",
			logFunc: func() {
				log.Info().Msg("info message")
			},
			shouldLog: false,
		},
		{
			name:     "set to warn allows warn",
			levelStr: "warn",
			logFunc: func() {
				log.Warn().Msg("warn message")
			},
			shouldLog: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			InitialiseLogger(
				WithOutputStyle(OutputStyleJSON),
				WithOutput(&buf),
				WithTimestamp(false),
			)

			SetLogLevel(tt.levelStr)

			tt.logFunc()

			output := buf.String()
			if tt.shouldLog {
				assert.NotEmpty(t, output, "expected log output")
			} else {
				assert.Empty(t, output, "expected no log output")
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := defaultConfig()

	assert.Equal(t, OutputStyleJSON, cfg.outputStyle)
	assert.Equal(t, zerolog.InfoLevel, cfg.level)
	assert.Equal(t, time.RFC3339, cfg.timeFieldFormat)
	assert.NotNil(t, cfg.output)
	assert.False(t, cfg.addCaller)
	assert.True(t, cfg.addTimestamp)
}

func TestOutputStyle_Values(t *testing.T) {
	// Ensure the constants are defined correctly
	assert.Equal(t, OutputStyleJSON, OutputStyle("json"))
	assert.Equal(t, OutputStyleConsole, OutputStyle("console"))
}

func TestInitServiceLogger_OverrideDefaults(t *testing.T) {
	var buf bytes.Buffer

	// Override service logger defaults with custom level
	InitServiceLogger(
		WithOutput(&buf),
		WithLevel(zerolog.WarnLevel),
		WithTimestamp(false),
	)

	// Info should not log (we set to Warn level)
	log.Info().Msg("info message")
	output := buf.String()
	assert.Empty(t, output)

	// Warn should log
	log.Warn().Msg("warn message")
	output = buf.String()
	assert.Contains(t, output, "warn message")
}

func TestInitCLILogger_OverrideDefaults(t *testing.T) {
	var buf bytes.Buffer

	// Override CLI logger defaults
	InitCLILogger(
		WithOutput(&buf),
		WithLevel(zerolog.DebugLevel),
		WithCaller(true), // CLI usually doesn't have caller
	)

	log.Debug().Msg("debug message")

	output := buf.String()
	assert.Contains(t, output, "debug message")
	assert.Contains(t, output, "DBG")
	// With console output, caller appears differently
	assert.True(t, strings.Contains(output, "logger_test.go") || strings.Contains(output, ">"))
}
