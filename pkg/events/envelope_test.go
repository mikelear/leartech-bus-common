//go:build unit

package events_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mikelear/leartech-bus-common/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEnvelope_RoundTrip verifies that a fully-populated envelope survives
// a ToJSON -> FromJSON cycle unchanged.
func TestEnvelope_RoundTrip(t *testing.T) {
	original := events.Envelope{
		ID:            "abc123",
		Type:          "initiative.fired",
		Source:        "orchestrator",
		TS:            "2026-07-08T12:00:00Z",
		TenantID:      "tenant-42",
		CorrelationID: "corr-xyz",
		Payload: map[string]any{
			"initiative_id": "init-1",
			"attempts":      float64(3), // JSON numbers round-trip as float64
		},
	}

	data, err := original.ToJSON()
	require.NoError(t, err)

	decoded, err := events.FromJSON(data)
	require.NoError(t, err)

	assert.Equal(t, original, decoded)
}

// TestEnvelope_ToJSON_KeyOrder pins the cross-language wire ordering:
// id, type, source, ts, tenant_id, correlation_id, payload.
//
// Go's encoding/json emits struct fields in declaration order, so the
// only way this test regresses is if the Envelope struct field order
// changes -- which would silently break Python interop.
func TestEnvelope_ToJSON_KeyOrder(t *testing.T) {
	e := events.Envelope{
		ID:            "id-value",
		Type:          "type-value",
		Source:        "source-value",
		TS:            "ts-value",
		TenantID:      "tenant-value",
		CorrelationID: "corr-value",
		Payload:       map[string]any{"k": "v"},
	}

	data, err := e.ToJSON()
	require.NoError(t, err)

	expectedOrder := []string{
		`"id"`, `"type"`, `"source"`, `"ts"`,
		`"tenant_id"`, `"correlation_id"`, `"payload"`,
	}
	s := string(data)
	prev := -1
	for _, key := range expectedOrder {
		idx := strings.Index(s, key)
		require.NotEqual(t, -1, idx, "key %s missing from JSON: %s", key, s)
		require.Greater(t, idx, prev, "key %s appeared before previous key; JSON=%s", key, s)
		prev = idx
	}
}

// TestEnvelope_ToJSON_NoHTMLEscape verifies that HTML-sensitive characters
// pass through verbatim. Go's default encoder would escape `<`, `>`, `&`
// -- which would break byte-parity with Python's json.dumps (which does
// not escape them by default).
func TestEnvelope_ToJSON_NoHTMLEscape(t *testing.T) {
	e := events.Envelope{
		ID:            "id",
		Type:          "html.escape.test",
		Source:        "svc",
		TS:            "2026-07-08T12:00:00Z",
		TenantID:      "system",
		CorrelationID: "corr",
		Payload:       map[string]any{"note": "<script>&"},
	}
	data, err := e.ToJSON()
	require.NoError(t, err)
	// Raw HTML-sensitive chars must pass through verbatim.
	assert.Contains(t, string(data), "<script>&")
	// And their Unicode-escaped forms must NOT appear (which is the
	// default encoder behaviour with SetEscapeHTML enabled).
	assert.NotContains(t, string(data), "\\u003c")
	assert.NotContains(t, string(data), "\\u003e")
	assert.NotContains(t, string(data), "\\u0026")
}

// TestEnvelope_EnsureDefaults_AllEmpty verifies that all four auto-fillable
// fields (ID, TS, CorrelationID, TenantID) are populated when empty.
func TestEnvelope_EnsureDefaults_AllEmpty(t *testing.T) {
	e := events.Envelope{Type: "e.test"}
	before := time.Now().UTC().Add(-time.Second)
	e.EnsureDefaults()
	after := time.Now().UTC().Add(time.Second)

	assert.NotEmpty(t, e.ID)
	assert.Len(t, e.ID, 32, "ID should be uuid hex (32 chars, no dashes)")
	assert.NotContains(t, e.ID, "-", "ID should be hex, no dashes")

	assert.NotEmpty(t, e.TS)
	parsed, err := time.Parse(time.RFC3339, e.TS)
	require.NoError(t, err, "TS should be RFC3339-parseable")
	assert.True(t, !parsed.Before(before) && !parsed.After(after),
		"TS %s should fall within [%s, %s]", e.TS, before, after)

	assert.NotEmpty(t, e.CorrelationID)
	assert.Len(t, e.CorrelationID, 32)

	assert.Equal(t, events.SystemTenant, e.TenantID)
}

// TestEnvelope_EnsureDefaults_PreservesSetValues verifies that already-populated
// fields are not overwritten.
func TestEnvelope_EnsureDefaults_PreservesSetValues(t *testing.T) {
	e := events.Envelope{
		ID:            "custom-id",
		TS:            "2020-01-01T00:00:00Z",
		TenantID:      "acme",
		CorrelationID: "custom-corr",
	}
	e.EnsureDefaults()

	assert.Equal(t, "custom-id", e.ID)
	assert.Equal(t, "2020-01-01T00:00:00Z", e.TS)
	assert.Equal(t, "acme", e.TenantID)
	assert.Equal(t, "custom-corr", e.CorrelationID)
}

// TestFromJSON_PythonShape is the cross-language byte-compat guard: a
// hand-written JSON string modelled on the Python leartech_bus lib's
// output must decode with every field populated. Any regression here
// breaks the wire contract.
func TestFromJSON_PythonShape(t *testing.T) {
	// This literal mimics `json.dumps(envelope.__dict__, separators=(',',':'))`
	// from the Python producer -- compact, snake_case, expected key order.
	pythonJSON := []byte(
		`{"id":"7a3d0c0e5c9c4c6a9f8b1e2d3c4b5a67",` +
			`"type":"initiative.fired",` +
			`"source":"orchestrator",` +
			`"ts":"2026-07-08T12:34:56Z",` +
			`"tenant_id":"tenant-42",` +
			`"correlation_id":"1122334455667788990011223344",` +
			`"payload":{"initiative_id":"init-1","attempts":3}}`,
	)

	env, err := events.FromJSON(pythonJSON)
	require.NoError(t, err)

	assert.Equal(t, "7a3d0c0e5c9c4c6a9f8b1e2d3c4b5a67", env.ID)
	assert.Equal(t, "initiative.fired", env.Type)
	assert.Equal(t, "orchestrator", env.Source)
	assert.Equal(t, "2026-07-08T12:34:56Z", env.TS)
	assert.Equal(t, "tenant-42", env.TenantID)
	assert.Equal(t, "1122334455667788990011223344", env.CorrelationID)
	require.NotNil(t, env.Payload)
	assert.Equal(t, "init-1", env.Payload["initiative_id"])
	// Numbers in JSON decode to float64 through map[string]any.
	assert.InEpsilon(t, 3.0, env.Payload["attempts"], 1e-9)
}

// TestFromJSON_MalformedReturnsError verifies FromJSON wraps parse errors
// rather than returning a partially-populated envelope.
func TestFromJSON_MalformedReturnsError(t *testing.T) {
	env, err := events.FromJSON([]byte(`{"id":`))
	require.Error(t, err)
	assert.Equal(t, events.Envelope{}, env)
}

// TestFromJSON_UnknownKeysIgnored verifies forward compatibility: unknown
// keys in an inbound envelope don't cause a parse failure.
func TestFromJSON_UnknownKeysIgnored(t *testing.T) {
	data := []byte(`{"id":"x","type":"t","source":"s","ts":"2026-07-08T00:00:00Z",` +
		`"tenant_id":"system","correlation_id":"c","payload":{"k":"v"},"future_field":42}`)
	env, err := events.FromJSON(data)
	require.NoError(t, err)
	assert.Equal(t, "x", env.ID)
	assert.Equal(t, "v", env.Payload["k"])
}

// TestEnvelope_ToJSON_ByteExactShape asserts that the compact JSON output
// matches an expected byte sequence for a fully-specified envelope.
// This locks the wire shape (no leading/trailing whitespace, no trailing
// newline, no HTML escaping).
func TestEnvelope_ToJSON_ByteExactShape(t *testing.T) {
	e := events.Envelope{
		ID:            "abc",
		Type:          "e.t",
		Source:        "s",
		TS:            "2026-01-01T00:00:00Z",
		TenantID:      "system",
		CorrelationID: "corr",
		Payload:       map[string]any{"n": float64(1)},
	}
	data, err := e.ToJSON()
	require.NoError(t, err)
	want := `{"id":"abc","type":"e.t","source":"s","ts":"2026-01-01T00:00:00Z","tenant_id":"system","correlation_id":"corr","payload":{"n":1}}`
	assert.Equal(t, want, string(data))
	// Ensure the encoder didn't leave a trailing newline.
	assert.False(t, strings.HasSuffix(string(data), "\n"))
	// Sanity: json.Unmarshal accepts our output.
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
}
