# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

`mqube-go-common` is a shared Go library for Spring Financial Group providing common utilities and abstractions for caching, distributed locking, and structured logging. The library is designed to be imported into other Go services and applications.

## Development Commands

### Building
```bash
make build          # Build the application
make build-all      # Build all files including tests
```

### Testing
```bash
make test           # Run unit tests with the "unit" build tag
make test-coverage  # Run tests with coverage report
make test-report-html  # Generate HTML coverage report and open it
```

### Code Quality
```bash
make lint           # Run golangci-lint (strict configuration)
make fmt            # Format code with goimports
make all            # Run fmt, build, test, and lint
```

### Mock Generation
```bash
make mocks          # Generate mock implementations using mockery v3.x
```

Mocks are configured in `.mockery.yml` and generated in the same package as interfaces (e.g., `cache_mock.go`, `locker_mock.go`).

### Pre-commit Hooks
```bash
make pre-commit-install  # Install pre-commit hooks
make pre-commits-run     # Run pre-commit hooks manually
make pre-commit-update   # Update pre-commit hooks
```

### Dependencies
```bash
make tidy-deps      # Run go mod tidy and install generate dependencies
```

## Architecture

### Package Structure

The library is organized into three main packages under `pkg/`:

1. **`pkg/cache`** - Generic caching abstractions with Redis and in-memory implementations
2. **`pkg/lock`** - Distributed locking using cache backends
3. **`pkg/logger`** - Structured logging wrapper around zerolog

### Cache Package (`pkg/cache`)

Provides a generic `Cache[T any]` interface for type-safe caching:

- **Interface**: `Cache[T any]` with methods: `Get`, `Set`, `Delete`, `DeleteKeysMatching`
- **Implementations**:
  - `Redis[T]` - Redis-backed cache with JSON serialization
  - `InMemory[T]` - Local in-memory cache with TTL expiration
- **Redis Client**: `RedisClient` interface wraps `go-redis/v9` client
  - `NewRedisClient(cfg RedisConfig)` - Creates Redis client with failover support
  - Supports both standard and Sentinel configurations
- **Key Structure**: All cache keys use prefix pattern: `{prefix}:{key}`

Example:
```go
cache := cache.NewRedisCache[MyStruct](redisClient, 10*time.Minute, "my-prefix")
value, exists := cache.Get(ctx, "key")
cache.Set(ctx, "key", value)
```

### Lock Package (`pkg/lock`)

Distributed locking built on top of the cache abstraction:

- **Interface**: `Locker` with methods: `AcquireLock`, `ReleaseLock`
- **Implementation**: `Lock` struct uses `Cache[struct{}]` as backend
- **Constructors**:
  - `NewRedisLock(redis, ttl, prefix)` - Distributed locks across services
  - `NewInMemoryLock(ttl, prefix)` - Local locks for single-instance use

The lock implementation checks for key existence before acquiring, sets empty struct on acquire, and deletes on release.

### Logger Package (`pkg/logger`)

Configurable wrapper around `github.com/rs/zerolog`:

- **Configuration**: Functional options pattern with `Option` functions
- **Output Styles**:
  - `OutputStyleJSON` - Structured JSON logging (for services)
  - `OutputStyleConsole` - Human-readable colored output (for CLIs)
- **Initialization Functions**:
  - `InitialiseLogger(opts...)` - Custom configuration
  - `InitServiceLogger(opts...)` - Pre-configured for services (JSON, with caller info)
  - `InitCLILogger(opts...)` - Pre-configured for CLIs (console output)
- **Context Helpers**: `UpdateLoggerContext(ctx, key, value)` adds fields to logger in context
- **Redis Integration**: `NewRedisLogger()` adapts zerolog for go-redis client logging

## Testing Guidelines

- Use build tag `//go:build unit` for unit tests
- Tests are run with `KUBECONFIG=/cluster/connections/not/allowed` to prevent accidental cluster access
- Mock interfaces using mockery for testable designs
- Test files disable certain linters (see `.golangci.yml` exclusions for `_test.go`)

## Linting Standards

This project uses a **very strict** golangci-lint configuration (based on https://gist.github.com/maratori/47a4d00457a92aa426dbd48a18776322):

- 50+ enabled linters including security (gosec), complexity (cyclop, gocyclo), and style checks
- Reports output to both stdout and SonarQube XML format (`build/reports/golangci-report.xml`)
- Key restrictions enforced by `depguard`:
  - Must use `github.com/rs/zerolog` for logging (not logrus or mqa-logging)
  - Must use `math/rand/v2` instead of `math/rand` in non-test files
  - Banned deprecated packages: `github.com/golang/protobuf`, `github.com/satori/go.uuid`, etc.

## Package Usage

This library is imported by other projects:

```go
import "github.com/mikelear/leartech-bus-common/pkg/cache"
import "github.com/mikelear/leartech-bus-common/pkg/lock"
import "github.com/mikelear/leartech-bus-common/pkg/logger"
```

For local development across projects, add a `replace` directive in the consuming project's `go.mod`:
```
replace github.com/mikelear/leartech-bus-common => ../mqube-go-common
```

## Dependencies

Key external dependencies:
- `github.com/redis/go-redis/v9` - Redis client
- `github.com/rs/zerolog` - Structured logging
- `github.com/pkg/errors` - Error wrapping
- `github.com/stretchr/testify` - Testing utilities

Go version: 1.24.5