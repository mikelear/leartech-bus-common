//go:build unit

package maestro_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mikelear/leartech-bus-common/pkg/maestro"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	// Silence gin's default noisy request-log during tests.
	gin.SetMode(gin.TestMode)
}

// newRouter mounts the consume handler on a fresh gin engine so each test
// starts from a clean state.
func newRouter(reg *maestro.Registry) *gin.Engine {
	r := gin.New()
	r.POST("/consume_event", maestro.ConsumeHandlerFunc(reg))
	return r
}

func postJSON(t *testing.T, r http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestConsumeHandler_RegisteredHandler_Invoked(t *testing.T) {
	reg := maestro.NewRegistry()

	var (
		mu       sync.Mutex
		received maestro.ConsumeEventRequestDto
		called   bool
	)
	reg.Register("initiative.fired", func(_ context.Context, evt maestro.ConsumeEventRequestDto) error {
		mu.Lock()
		defer mu.Unlock()
		received = evt
		called = true
		return nil
	})

	r := newRouter(reg)
	body := `{
	  "id": "evt-1",
	  "name": "initiative.fired",
	  "producedTime": "2026-05-01T12:34:56Z",
	  "annotations": {"initiative_id": "init-42"},
	  "actionedBy": "user-7"
	}`
	rec := postJSON(t, r, "/consume_event", body)

	require.Equal(t, http.StatusOK, rec.Code, "successful dispatch should return 200; body: %s", rec.Body.String())

	var resp maestro.ConsumeEventResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.NotNil(t, resp.IsConsumed)
	assert.True(t, *resp.IsConsumed)
	require.NotNil(t, resp.IsErrored)
	assert.False(t, *resp.IsErrored)

	mu.Lock()
	defer mu.Unlock()
	require.True(t, called, "registered handler must be invoked")
	assert.Equal(t, "evt-1", received.ID)
	assert.Equal(t, "initiative.fired", received.Name)
	assert.Equal(t, "user-7", received.ActionedBy)
	assert.Equal(t, "init-42", received.Annotations["initiative_id"])

	// producedTime must decode to a real time.Time (not a string).
	expected, err := time.Parse(time.RFC3339, "2026-05-01T12:34:56Z")
	require.NoError(t, err)
	assert.Equal(t, expected.UTC(), received.ProducedTime.UTC())
}

func TestConsumeHandler_UnknownEvent_404(t *testing.T) {
	reg := maestro.NewRegistry()
	// Nothing registered — the handler must 404 rather than 500 so Maestro
	// drops the event instead of retrying.
	r := newRouter(reg)

	rec := postJSON(t, r, "/consume_event",
		`{"id":"e","name":"unknown.event","producedTime":"2026-05-01T00:00:00Z"}`)

	require.Equal(t, http.StatusNotFound, rec.Code)
	var resp maestro.ConsumeEventResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.NotNil(t, resp.IsConsumed)
	assert.False(t, *resp.IsConsumed)
	require.NotNil(t, resp.IsErrored)
	assert.False(t, *resp.IsErrored,
		"unknown-event responses must not set IsErrored — Maestro treats errored=true as a retry trigger")
	require.NotNil(t, resp.ErrorReason)
	assert.Contains(t, *resp.ErrorReason, "unknown.event")
}

func TestConsumeHandler_HandlerError_500(t *testing.T) {
	reg := maestro.NewRegistry()
	reg.Register("initiative.fired", func(_ context.Context, _ maestro.ConsumeEventRequestDto) error {
		return errors.New("downstream storage unavailable")
	})
	r := newRouter(reg)

	rec := postJSON(t, r, "/consume_event",
		`{"id":"e","name":"initiative.fired","producedTime":"2026-05-01T00:00:00Z"}`)

	require.Equal(t, http.StatusInternalServerError, rec.Code,
		"handler errors must surface as 500 so Maestro retries; got body: %s", rec.Body.String())
	var resp maestro.ConsumeEventResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.NotNil(t, resp.IsErrored)
	assert.True(t, *resp.IsErrored)
	require.NotNil(t, resp.ErrorReason)
	assert.Contains(t, *resp.ErrorReason, "downstream storage unavailable")
}

func TestConsumeHandler_MalformedJSON_400(t *testing.T) {
	reg := maestro.NewRegistry()
	reg.Register("initiative.fired", func(_ context.Context, _ maestro.ConsumeEventRequestDto) error {
		t.Fatal("handler must not be invoked on malformed body")
		return nil
	})
	r := newRouter(reg)

	rec := postJSON(t, r, "/consume_event", `{"name": "initiative.fired", "producedTime": not-a-date}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code,
		"malformed JSON must 400 (no retry); got body: %s", rec.Body.String())
}

func TestRegistry_Register_OverwritesExisting(t *testing.T) {
	reg := maestro.NewRegistry()
	reg.Register("x", func(_ context.Context, _ maestro.ConsumeEventRequestDto) error {
		return errors.New("first")
	})
	reg.Register("x", func(_ context.Context, _ maestro.ConsumeEventRequestDto) error {
		return errors.New("second")
	})

	fn, ok := reg.Handler("x")
	require.True(t, ok)
	err := fn(context.Background(), maestro.ConsumeEventRequestDto{Name: "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "second",
		"latest Register call must win — startup wiring is typically last-write-wins")
}

func TestRegistry_Dispatch_UnknownEvent_ReturnsSentinel(t *testing.T) {
	reg := maestro.NewRegistry()
	err := reg.Dispatch(context.Background(), maestro.ConsumeEventRequestDto{Name: "nope"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, maestro.ErrUnknownEvent),
		"Dispatch must return ErrUnknownEvent so callers can distinguish 404 from 500")
}

func TestRegistry_Register_PanicsOnEmptyName(t *testing.T) {
	reg := maestro.NewRegistry()
	assert.Panics(t, func() {
		reg.Register("", func(_ context.Context, _ maestro.ConsumeEventRequestDto) error { return nil })
	}, "empty event name is a programming error")
}

func TestRegistry_Register_PanicsOnNilHandler(t *testing.T) {
	reg := maestro.NewRegistry()
	assert.Panics(t, func() {
		reg.Register("x", nil)
	}, "nil handler is a programming error")
}

func TestConsumeHandlerFunc_PanicsOnNilRegistry(t *testing.T) {
	assert.Panics(t, func() {
		_ = maestro.ConsumeHandlerFunc(nil)
	})
}
