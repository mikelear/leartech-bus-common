package events

import (
	"sync"

	"github.com/pkg/errors"
)

// InMemoryTransport is a synchronous, in-process fan-out transport suitable
// for unit tests and single-instance applications. Publish invokes every
// registered subscriber for the topic serially, on the caller's goroutine,
// before returning.
//
// Because delivery is synchronous, a slow subscriber blocks the publisher
// — this is deliberate for test determinism. Production callers should use
// [RedisTransport] (or another asynchronous transport) instead.
type InMemoryTransport struct {
	mu       sync.RWMutex
	closed   bool
	handlers map[string][]func([]byte)
}

// NewInMemoryTransport constructs an in-memory transport with no subscribers.
func NewInMemoryTransport() *InMemoryTransport {
	return &InMemoryTransport{
		handlers: make(map[string][]func([]byte)),
	}
}

// Publish delivers data to every registered subscriber for topic. Returns
// an error only if the transport has been closed.
func (t *InMemoryTransport) Publish(topic string, data []byte) error {
	t.mu.RLock()
	if t.closed {
		t.mu.RUnlock()
		return errors.New("inmemory transport: closed")
	}
	// Snapshot the handler slice under the read lock, then release it
	// before invoking handlers so a subscriber that publishes back into
	// the transport cannot deadlock.
	handlers := make([]func([]byte), len(t.handlers[topic]))
	copy(handlers, t.handlers[topic])
	t.mu.RUnlock()

	for _, h := range handlers {
		h(data)
	}
	return nil
}

// Subscribe appends cb to the handler list for topic. Multiple subscribers
// per topic are supported; they receive envelopes in registration order.
func (t *InMemoryTransport) Subscribe(topic string, cb func([]byte)) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return errors.New("inmemory transport: closed")
	}
	t.handlers[topic] = append(t.handlers[topic], cb)
	return nil
}

// Close marks the transport as closed. Subsequent Publish / Subscribe
// calls return an error. Idempotent.
func (t *InMemoryTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closed = true
	t.handlers = nil
	return nil
}
