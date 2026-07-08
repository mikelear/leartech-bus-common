//go:build unit

package events_test

import (
	"fmt"

	"github.com/mikelear/leartech-bus-common/pkg/events"
)

// ExampleNewBus demonstrates the end-to-end round-trip of the bus with the
// in-memory transport: subscribe a handler, publish an event, and observe
// the tenant / correlation stamps.
func ExampleNewBus() {
	transport := events.NewInMemoryTransport()
	bus := events.NewBus(transport, "orchestrator")
	defer func() { _ = bus.Close() }()

	_ = bus.Subscribe("initiatives", func(_ events.Context, env events.Envelope) {
		fmt.Printf("received %s from %s (tenant=%s)\n", env.Type, env.Source, env.TenantID)
	})

	_, _ = bus.Publish(
		events.Context{TenantID: "acme"},
		"initiative.fired",
		map[string]any{"initiative_id": "init-1"},
		"initiatives",
	)

	// Output:
	// received initiative.fired from orchestrator (tenant=acme)
}

// ExampleWithTenantFilter shows a subscription that only sees envelopes
// scoped to a specific tenant. Publishes for other tenants are silently
// dropped before reaching the handler.
func ExampleWithTenantFilter() {
	transport := events.NewInMemoryTransport()
	bus := events.NewBus(transport, "svc")
	defer func() { _ = bus.Close() }()

	_ = bus.Subscribe("t", func(_ events.Context, env events.Envelope) {
		fmt.Printf("saw tenant=%s\n", env.TenantID)
	}, events.WithTenantFilter("keep"))

	for _, tenant := range []string{"keep", "drop", "keep"} {
		_, _ = bus.Publish(events.Context{TenantID: tenant}, "e", nil, "t")
	}

	// Output:
	// saw tenant=keep
	// saw tenant=keep
}
