//go:build unit

package events_test

import (
	"sync"
	"testing"

	"github.com/mikelear/leartech-bus-common/pkg/events"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBus_PublishSubscribe(t *testing.T) {
	tr := events.NewInMemoryTransport()
	bus := events.NewBus(tr, "orchestrator")
	t.Cleanup(func() { _ = bus.Close() })

	var mu sync.Mutex
	var received []events.Envelope
	require.NoError(t, bus.Subscribe("initiatives", func(_ events.Context, env events.Envelope) {
		mu.Lock()
		defer mu.Unlock()
		received = append(received, env)
	}))

	published, err := bus.Publish(
		events.Context{TenantID: "acme"},
		"initiative.fired",
		map[string]any{"id": "init-1"},
		"initiatives",
	)
	require.NoError(t, err)
	assert.Equal(t, "initiative.fired", published.Type)
	assert.Equal(t, "orchestrator", published.Source)
	assert.Equal(t, "acme", published.TenantID)
	assert.NotEmpty(t, published.ID, "publish should stamp an ID")
	assert.NotEmpty(t, published.TS, "publish should stamp a timestamp")
	assert.NotEmpty(t, published.CorrelationID, "publish should stamp a correlation id")

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, received, 1)
	assert.Equal(t, published, received[0], "subscriber should observe the same envelope")
}

func TestBus_Publish_DefaultsSystemTenant(t *testing.T) {
	tr := events.NewInMemoryTransport()
	bus := events.NewBus(tr, "svc")
	t.Cleanup(func() { _ = bus.Close() })

	env, err := bus.Publish(events.Context{}, "t.event", nil, "topic")
	require.NoError(t, err)
	assert.Equal(t, events.SystemTenant, env.TenantID)
}

// TestBus_CorrelationChain verifies the intended usage: a handler receiving
// event A publishes event B and propagates the correlation_id so both share
// a causal chain.
func TestBus_CorrelationChain(t *testing.T) {
	tr := events.NewInMemoryTransport()
	bus := events.NewBus(tr, "svc")
	t.Cleanup(func() { _ = bus.Close() })

	var (
		mu       sync.Mutex
		observed []events.Envelope
	)
	require.NoError(t, bus.Subscribe("topic", func(_ events.Context, env events.Envelope) {
		mu.Lock()
		observed = append(observed, env)
		mu.Unlock()
		if env.Type == "step.one" {
			_, err := bus.Publish(
				events.Context{
					TenantID:      env.TenantID,
					CorrelationID: env.CorrelationID,
				},
				"step.two",
				nil,
				"topic",
			)
			require.NoError(t, err)
		}
	}))

	first, err := bus.Publish(events.Context{TenantID: "t"}, "step.one", nil, "topic")
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, observed, 2, "both events should have been delivered")

	assert.Equal(t, "step.one", observed[0].Type)
	assert.Equal(t, "step.two", observed[1].Type)
	assert.Equal(t, first.CorrelationID, observed[0].CorrelationID)
	assert.Equal(t, first.CorrelationID, observed[1].CorrelationID,
		"chained event should share the causal id")
	// Distinct message identity even though the chain matches.
	assert.NotEqual(t, observed[0].ID, observed[1].ID)
}

func TestBus_WithTenantFilter_Drops(t *testing.T) {
	tr := events.NewInMemoryTransport()
	bus := events.NewBus(tr, "svc")
	t.Cleanup(func() { _ = bus.Close() })

	var seen []events.Envelope
	require.NoError(t, bus.Subscribe("topic", func(_ events.Context, env events.Envelope) {
		seen = append(seen, env)
	}, events.WithTenantFilter("keep")))

	_, err := bus.Publish(events.Context{TenantID: "keep"}, "e", nil, "topic")
	require.NoError(t, err)
	_, err = bus.Publish(events.Context{TenantID: "drop"}, "e", nil, "topic")
	require.NoError(t, err)
	_, err = bus.Publish(events.Context{TenantID: "keep"}, "e", nil, "topic")
	require.NoError(t, err)

	require.Len(t, seen, 2, "only matching-tenant envelopes should reach the handler")
	for _, env := range seen {
		assert.Equal(t, "keep", env.TenantID)
	}
}

func TestBus_WithoutTenantFilter_PassesAll(t *testing.T) {
	tr := events.NewInMemoryTransport()
	bus := events.NewBus(tr, "svc")
	t.Cleanup(func() { _ = bus.Close() })

	var seen []events.Envelope
	require.NoError(t, bus.Subscribe("topic", func(_ events.Context, env events.Envelope) {
		seen = append(seen, env)
	}))

	for _, tenant := range []string{"a", "b", "c"} {
		_, err := bus.Publish(events.Context{TenantID: tenant}, "e", nil, "topic")
		require.NoError(t, err)
	}
	assert.Len(t, seen, 3)
}

func TestBus_Close_ClosesTransport(t *testing.T) {
	tr := events.NewInMemoryTransport()
	bus := events.NewBus(tr, "svc")
	require.NoError(t, bus.Close())

	// Underlying transport is closed, so subsequent publish must fail.
	_, err := bus.Publish(events.Context{}, "e", nil, "topic")
	assert.Error(t, err)
}

// TestBus_Subscribe_MalformedEnvelopeIsDropped verifies that a garbage
// payload delivered by the transport does not panic the subscriber; the
// handler simply is not invoked.
func TestBus_Subscribe_MalformedEnvelopeIsDropped(t *testing.T) {
	tr := events.NewInMemoryTransport()
	bus := events.NewBus(tr, "svc")
	t.Cleanup(func() { _ = bus.Close() })

	handlerCalled := false
	require.NoError(t, bus.Subscribe("topic", func(_ events.Context, _ events.Envelope) {
		handlerCalled = true
	}))

	// Bypass the bus and inject a raw malformed payload straight through
	// the transport, exactly as an ill-behaved peer might.
	require.NoError(t, tr.Publish("topic", []byte("not-json")))
	assert.False(t, handlerCalled)
}

// erroringTransport lets us assert that Publish surfaces transport errors.
type erroringTransport struct{}

func (erroringTransport) Publish(string, []byte) error         { return errors.New("boom") }
func (erroringTransport) Subscribe(string, func([]byte)) error { return errors.New("sub-boom") }
func (erroringTransport) Close() error                         { return nil }

func TestBus_Publish_WrapsTransportError(t *testing.T) {
	bus := events.NewBus(erroringTransport{}, "svc")
	_, err := bus.Publish(events.Context{}, "e", nil, "topic")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}

func TestBus_Subscribe_WrapsTransportError(t *testing.T) {
	bus := events.NewBus(erroringTransport{}, "svc")
	err := bus.Subscribe("topic", func(events.Context, events.Envelope) {})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sub-boom")
}
