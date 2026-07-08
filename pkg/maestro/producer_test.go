//go:build unit

package maestro_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mikelear/leartech-bus-common/pkg/maestro"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureTransport is a minimal http.RoundTripper that records the last
// request and returns a canned response. Used to introspect the wire body
// the producer emits without spinning up a full httptest server.
type captureTransport struct {
	captured *http.Request
	body     []byte
	status   int
	respBody string
}

func (c *captureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	c.captured = req
	if req.Body != nil {
		body, _ := io.ReadAll(req.Body)
		c.body = body
		_ = req.Body.Close()
	}
	status := c.status
	if status == 0 {
		status = http.StatusOK
	}
	respBody := c.respBody
	if respBody == "" {
		respBody = `{"received":true,"success":true}`
	}
	rec := httptest.NewRecorder()
	rec.Header().Set("Content-Type", "application/json")
	rec.WriteHeader(status)
	_, _ = rec.WriteString(respBody)
	return rec.Result(), nil
}

func TestProducer_Announce_WireBodyShape(t *testing.T) {
	tr := &captureTransport{}
	hc := &http.Client{Transport: tr}

	prod, err := maestro.NewProducer("https://maestro.example.com", "orchestrator",
		maestro.WithProducerHTTPClient(hc))
	require.NoError(t, err)

	resp, err := prod.Announce(context.Background(), maestro.Event{
		Name: "initiative.fired",
		Annotations: map[string]string{
			"initiative_id": "init-42",
			"tenant_id":     "acme",
		},
		ActionedBy: "user-7",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotNil(t, resp.Success)
	assert.True(t, *resp.Success)

	// The producer must POST to /api/event_maestro/announce_event with
	// exactly the four canonical keys (name, producedBy, annotations,
	// actionedBy). Anything else is a wire-contract regression.
	require.NotNil(t, tr.captured, "expected a captured request")
	assert.Equal(t, http.MethodPost, tr.captured.Method)
	assert.Equal(t, "/api/event_maestro/announce_event", tr.captured.URL.Path)
	assert.Equal(t, "application/json", tr.captured.Header.Get("Content-Type"))

	var wire map[string]any
	require.NoError(t, json.Unmarshal(tr.body, &wire),
		"produced body must be valid JSON: %s", string(tr.body))
	assert.Equal(t, "initiative.fired", wire["name"])
	assert.Equal(t, "orchestrator", wire["producedBy"])
	assert.Equal(t, "user-7", wire["actionedBy"])

	// Annotations must be a JSON object with the exact keys we passed —
	// no extra "id" or nesting.
	annos, ok := wire["annotations"].(map[string]any)
	require.True(t, ok, "annotations must decode to a JSON object")
	assert.Equal(t, "init-42", annos["initiative_id"])
	assert.Equal(t, "acme", annos["tenant_id"])
}

func TestProducer_Announce_OmitsEmptyActionedBy(t *testing.T) {
	tr := &captureTransport{}
	hc := &http.Client{Transport: tr}

	prod, err := maestro.NewProducer("https://maestro.example.com", "svc",
		maestro.WithProducerHTTPClient(hc))
	require.NoError(t, err)

	_, err = prod.Announce(context.Background(), maestro.Event{
		Name: "case.updated",
	})
	require.NoError(t, err)

	var wire map[string]any
	require.NoError(t, json.Unmarshal(tr.body, &wire))
	_, hasActionedBy := wire["actionedBy"]
	assert.False(t, hasActionedBy,
		"empty ActionedBy must not appear in the wire body (got %s)", string(tr.body))
	_, hasAnnos := wire["annotations"]
	assert.False(t, hasAnnos,
		"empty Annotations must not appear in the wire body (got %s)", string(tr.body))
}

func TestProducer_Announce_ErrorOnBrokerRejection(t *testing.T) {
	tr := &captureTransport{
		respBody: `{"success":false,"errorDescription":"unknown event"}`,
	}
	hc := &http.Client{Transport: tr}

	prod, err := maestro.NewProducer("https://maestro.example.com", "svc",
		maestro.WithProducerHTTPClient(hc))
	require.NoError(t, err)

	_, err = prod.Announce(context.Background(), maestro.Event{Name: "bogus"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown event")
}

func TestProducer_Announce_ErrorOnNon200(t *testing.T) {
	tr := &captureTransport{
		status:   http.StatusInternalServerError,
		respBody: `{"success":false,"errorDescription":"boom"}`,
	}
	hc := &http.Client{Transport: tr}

	prod, err := maestro.NewProducer("https://maestro.example.com", "svc",
		maestro.WithProducerHTTPClient(hc))
	require.NoError(t, err)

	_, err = prod.Announce(context.Background(), maestro.Event{Name: "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status 500")
}

func TestProducer_Announce_ErrorOnEmptyEventName(t *testing.T) {
	tr := &captureTransport{}
	hc := &http.Client{Transport: tr}

	prod, err := maestro.NewProducer("https://maestro.example.com", "svc",
		maestro.WithProducerHTTPClient(hc))
	require.NoError(t, err)

	_, err = prod.Announce(context.Background(), maestro.Event{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Name must not be empty")
	assert.Nil(t, tr.captured, "no HTTP call should be made when Name is empty")
}

func TestNewProducer_RejectsEmptyBaseURL(t *testing.T) {
	_, err := maestro.NewProducer("", "svc")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "baseURL")
}

func TestNewProducer_RejectsEmptyProducedBy(t *testing.T) {
	_, err := maestro.NewProducer("https://x", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "producedBy")
}

func TestProducer_RequestEditor_RunsBeforeSend(t *testing.T) {
	tr := &captureTransport{}
	hc := &http.Client{Transport: tr}

	prod, err := maestro.NewProducer("https://maestro.example.com", "svc",
		maestro.WithProducerHTTPClient(hc),
		maestro.WithProducerRequestEditor(func(_ context.Context, req *http.Request) error {
			req.Header.Set("Authorization", "Bearer test-token")
			return nil
		}),
	)
	require.NoError(t, err)

	_, err = prod.Announce(context.Background(), maestro.Event{Name: "e"})
	require.NoError(t, err)
	require.NotNil(t, tr.captured)
	assert.Equal(t, "Bearer test-token", tr.captured.Header.Get("Authorization"),
		"request editor must apply before the HTTP client sees the request")
}
