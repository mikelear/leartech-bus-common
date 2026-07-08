package events

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
)

// SystemTenant is the tenant identifier assigned to events that are not
// scoped to a specific tenant (platform/system-level events).
const SystemTenant = "system"

// Envelope is the wire representation of a domain event on the bus.
//
// It is a **cross-language contract**: the JSON serialisation must be
// byte-identical to the Python `leartech_bus` library so a Go consumer can
// decode an envelope published by a Python producer (and vice-versa).
//
// The struct fields are declared in the exact key order the contract
// mandates (id, type, source, ts, tenant_id, correlation_id, payload).
// Go's encoding/json emits struct fields in declaration order, so consumers
// receive the keys in this order too.
type Envelope struct {
	// ID uniquely identifies this envelope. Filled with a new UUID (hex,
	// no dashes) when empty at publish time.
	ID string `json:"id"`

	// Type is the dotted, past-tense event name (e.g. "initiative.fired").
	Type string `json:"type"`

	// Source names the service that emitted the event.
	Source string `json:"source"`

	// TS is the RFC3339 UTC timestamp when the event was produced.
	// Filled with time.Now().UTC() when empty at publish time.
	TS string `json:"ts"`

	// TenantID is the tenancy scope for this event. Defaults to
	// SystemTenant ("system") when unset.
	TenantID string `json:"tenant_id"`

	// CorrelationID chains a series of causally related events together.
	// Filled with a new UUID (hex) when empty at publish time — this
	// starts a new causal chain. Downstream handlers reuse this value
	// when they publish follow-up events.
	CorrelationID string `json:"correlation_id"`

	// Payload is the domain-specific body of the event. Opaque to the
	// bus; the producer and consumer must agree on its shape by event type.
	Payload map[string]any `json:"payload"`
}

// EnsureDefaults populates any empty auto-fillable fields on the envelope:
// ID, TS, CorrelationID (fresh values), and TenantID (SystemTenant).
//
// This mutates the receiver. Publish invokes it before serialisation so the
// envelope returned to the caller reflects the finalised values.
func (e *Envelope) EnsureDefaults() {
	if e.ID == "" {
		e.ID = newUUIDHex()
	}
	if e.TS == "" {
		e.TS = time.Now().UTC().Format(time.RFC3339)
	}
	if e.CorrelationID == "" {
		e.CorrelationID = newUUIDHex()
	}
	if e.TenantID == "" {
		e.TenantID = SystemTenant
	}
}

// ToJSON serialises the envelope to compact JSON.
//
// HTML escaping is disabled so that special characters inside string values
// (e.g. `<`, `>`, `&`) are emitted verbatim rather than as `<` escapes.
// This matches Python's default `json.dumps` behaviour and preserves
// byte-compatibility across languages.
//
// The caller is expected to have populated required fields (Type, Source);
// auto-fillable fields are left as-is unless EnsureDefaults is called first
// (Bus.Publish calls it automatically).
func (e *Envelope) ToJSON() ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(e); err != nil {
		return nil, errors.Wrap(err, "envelope: marshal to json")
	}
	// json.Encoder appends a trailing newline; strip it to match the
	// Python `json.dumps` output.
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// FromJSON parses a JSON envelope. Unknown keys are ignored (forward
// compatibility). Missing keys leave zero values in place — callers that
// care about strict shape should validate after decoding.
func FromJSON(data []byte) (Envelope, error) {
	var e Envelope
	if err := json.Unmarshal(data, &e); err != nil {
		return Envelope{}, errors.Wrap(err, "envelope: unmarshal from json")
	}
	return e, nil
}

// newUUIDHex returns a 32-character lowercase hex UUID (no dashes),
// matching the Python contract's `uuid.uuid4().hex` shape.
func newUUIDHex() string {
	id := uuid.New()
	return hex.EncodeToString(id[:])
}
