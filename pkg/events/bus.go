// Package events provides a domain event-bus with a locked, cross-language
// wire contract (the Envelope) and a pluggable Transport.
//
// # Wire contract
//
// The [Envelope] struct is JSON-serialised with a fixed key order:
//
//	{"id","type","source","ts","tenant_id","correlation_id","payload"}
//
// This matches the Python `leartech_bus` library byte-for-byte, so a Go
// consumer can decode a Python-produced envelope unchanged (and vice-versa).
// See envelope_test.go's TestFromJSON_PythonShape for the guard.
//
// # Usage
//
//	transport := events.NewInMemoryTransport()
//	bus := events.NewBus(transport, "orchestrator")
//	defer func() { _ = bus.Close() }()
//
//	err := bus.Subscribe("initiatives", func(ctx events.Context, env events.Envelope) {
//	    // handle env
//	})
//
//	_, err = bus.Publish(events.Context{TenantID: "tenant-42"},
//	    "initiative.fired",
//	    map[string]any{"initiative_id": "init-1"},
//	    "initiatives")
//
// # Correlation chains
//
// When a handler publishes a follow-up event, it should propagate the
// correlation ID from the inbound envelope:
//
//	func handler(ctx events.Context, env events.Envelope) {
//	    _, _ = bus.Publish(events.Context{
//	        TenantID:      env.TenantID,
//	        CorrelationID: env.CorrelationID,
//	    }, "initiative.completed", nil, "initiatives")
//	}
//
// # Tenancy filtering
//
// Handlers can be scoped to a single tenant via [WithTenantFilter]:
//
//	_ = bus.Subscribe("initiatives", handler, events.WithTenantFilter("tenant-42"))
//
// Envelopes whose tenant_id doesn't match are silently dropped by the bus
// before reaching the handler.
package events

import (
	"github.com/pkg/errors"
)

// Context carries the tenant + correlation identity of an event operation.
// It is not a Go context.Context — it exists as a first-class concept
// because tenant and correlation are part of the wire contract itself.
type Context struct {
	// TenantID scopes the event to a tenant. Empty is treated as [SystemTenant].
	TenantID string

	// CorrelationID chains an event to its causal predecessor. Empty starts
	// a new chain (a fresh UUID is generated at publish time).
	CorrelationID string
}

// Handler processes a decoded envelope. Handlers should be idempotent —
// transports may deliver an envelope more than once, and the bus does not
// deduplicate.
type Handler func(ctx Context, env Envelope)

// Bus is the domain event-bus surface. Publish sends events; Subscribe
// registers handlers for a topic; Close releases transport resources.
type Bus interface {
	// Publish stamps id/ts/tenant/correlation on the envelope, serialises
	// it, and hands it to the transport. Returns the finalised envelope
	// (with defaults applied) so callers can inspect ID/TS/CorrelationID.
	Publish(ctx Context, typ string, payload map[string]any, topic string) (Envelope, error)

	// Subscribe registers a handler for a topic. Options can narrow which
	// envelopes reach the handler (e.g. [WithTenantFilter]).
	Subscribe(topic string, h Handler, opts ...SubOption) error

	// Close releases underlying transport resources.
	Close() error
}

// SubOption configures a subscription. It is exported as an opaque
// functional option so extensions (e.g. per-type filters, batching) can be
// added without breaking Subscribe's signature.
type SubOption func(*subOptions)

type subOptions struct {
	tenantFilter    string
	hasTenantFilter bool
}

// WithTenantFilter drops envelopes whose tenant_id doesn't match the given
// tenant before invoking the handler. Multi-tenant subscribers omit this
// option; single-tenant subscribers set it to isolate their view.
func WithTenantFilter(tenant string) SubOption {
	return func(o *subOptions) {
		o.tenantFilter = tenant
		o.hasTenantFilter = true
	}
}

// NewBus constructs a Bus backed by the given transport. `source` names the
// service publishing events (stamped into every envelope).
func NewBus(transport Transport, source string) Bus {
	return &bus{transport: transport, source: source}
}

type bus struct {
	transport Transport
	source    string
}

func (b *bus) Publish(ctx Context, typ string, payload map[string]any, topic string) (Envelope, error) {
	env := Envelope{
		Type:          typ,
		Source:        b.source,
		TenantID:      ctx.TenantID,
		CorrelationID: ctx.CorrelationID,
		Payload:       payload,
	}
	env.EnsureDefaults()

	data, err := env.ToJSON()
	if err != nil {
		return Envelope{}, errors.Wrap(err, "bus: serialise envelope")
	}
	if err = b.transport.Publish(topic, data); err != nil {
		return Envelope{}, errors.Wrap(err, "bus: transport publish")
	}
	return env, nil
}

func (b *bus) Subscribe(topic string, h Handler, opts ...SubOption) error {
	cfg := subOptions{}
	for _, opt := range opts {
		opt(&cfg)
	}
	cb := func(data []byte) {
		env, err := FromJSON(data)
		if err != nil {
			// Malformed inbound envelope: drop rather than crash the
			// subscriber goroutine. Transport-level logging is expected
			// to surface persistent decode failures.
			return
		}
		if cfg.hasTenantFilter && env.TenantID != cfg.tenantFilter {
			return
		}
		h(Context{
			TenantID:      env.TenantID,
			CorrelationID: env.CorrelationID,
		}, env)
	}
	if err := b.transport.Subscribe(topic, cb); err != nil {
		return errors.Wrap(err, "bus: transport subscribe")
	}
	return nil
}

func (b *bus) Close() error {
	if err := b.transport.Close(); err != nil {
		return errors.Wrap(err, "bus: close transport")
	}
	return nil
}
