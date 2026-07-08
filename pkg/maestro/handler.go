package maestro

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// ConsumeEventRequestDto is the wire body Maestro POSTs to a subscribing
// service's /consume_event endpoint.
//
// This mirrors the ConsumeEventRequest schema in swagger.yaml. It is
// hand-written (not generated) because we serve — not call — the endpoint,
// and gin's JSON binding wants an exported struct with `json` tags.
type ConsumeEventRequestDto struct {
	// ID is the unique event id Maestro assigned at announce time.
	ID string `json:"id"`

	// Name is the dotted event name — the dispatch key.
	Name string `json:"name"`

	// ProducedTime is the RFC3339 timestamp of the original announcement.
	// Maestro emits an RFC3339 string; we decode with time.Time so
	// downstream handlers get a native value.
	ProducedTime time.Time `json:"producedTime"`

	// Annotations are the string k/v pairs from the announcement. Same
	// vocabulary as pkg/maestro.Event.Annotations on the producer side.
	Annotations map[string]string `json:"annotations,omitempty"`

	// ActionedBy is the userId (or system-actor id) from the announcement,
	// when present.
	ActionedBy string `json:"actionedBy,omitempty"`
}

// ConsumeHandler is the per-event callback shape. Handlers should be
// idempotent — Maestro retries on non-2xx and may deliver the same event
// more than once (transient network partitions).
//
// Returning nil signals success (HTTP 200 to Maestro; event is settled).
// Returning a non-nil error signals failure (HTTP 500 to Maestro; the
// broker retries later).
type ConsumeHandler func(ctx context.Context, evt ConsumeEventRequestDto) error

// Registry is a name→handler dispatch map used by ConsumeHandlerFunc.
//
// A single service typically constructs one Registry, calls Register for
// each event it subscribes to, and mounts ConsumeHandlerFunc on POST
// /consume_event. The registry is safe for concurrent use across
// goroutines — Register acquires a lock, Dispatch takes a read-lock.
type Registry struct {
	mu       sync.RWMutex
	handlers map[string]ConsumeHandler
}

// NewRegistry constructs an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		handlers: map[string]ConsumeHandler{},
	}
}

// Register wires a handler for the given event name. Re-registering the
// same name overwrites the previous handler (typical use-case is startup
// wiring, not runtime rebinding, so the overwrite semantic is intentional
// — it means test setup can call Register in any order without cleanup).
//
// Register panics if name is empty; that's a programming error, not a
// runtime one.
func (r *Registry) Register(name string, fn ConsumeHandler) {
	if name == "" {
		panic("maestro: Registry.Register called with empty event name")
	}
	if fn == nil {
		panic("maestro: Registry.Register called with nil handler")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[name] = fn
}

// Handler returns the handler registered for name, or (nil, false) if
// none. Exposed for tests; production dispatch goes through Dispatch.
func (r *Registry) Handler(name string) (ConsumeHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	fn, ok := r.handlers[name]
	return fn, ok
}

// Dispatch looks up evt.Name in the registry and invokes the handler.
// Returns ErrUnknownEvent when no handler is registered — callers map
// this to HTTP 404 so Maestro can distinguish "we don't consume this"
// from "we consume it but hit a transient error".
func (r *Registry) Dispatch(ctx context.Context, evt ConsumeEventRequestDto) error {
	fn, ok := r.Handler(evt.Name)
	if !ok {
		return errors.Wrapf(ErrUnknownEvent, "event %q", evt.Name)
	}
	if err := fn(ctx, evt); err != nil {
		return errors.Wrapf(err, "handler for event %q", evt.Name)
	}
	return nil
}

// ErrUnknownEvent is returned by Registry.Dispatch when no handler is
// registered for the incoming event name. Distinct so callers can 404
// on it (letting Maestro drop the event) rather than 500 (which would
// trigger a retry loop for events we'll never consume).
var ErrUnknownEvent = errors.New("maestro: unknown event")

// ConsumeEventResponse is the response body Maestro reads back from
// /consume_event. Hand-written (not generated) because we serve, not
// call, this endpoint — pointer fields let us omit unset keys.
type ConsumeEventResponse struct {
	// IsConsumed is true when the event was successfully handled.
	// Maestro treats IsConsumed=true (regardless of HTTP status) as
	// "settled, do not retry".
	IsConsumed *bool `json:"isConsumed,omitempty"`

	// IsErrored is true when handler execution failed. Distinct from
	// !IsConsumed so we can signal "no handler registered" (neither
	// consumed nor errored) vs. "handler ran and blew up" (errored).
	IsErrored *bool `json:"isErrored,omitempty"`

	// ErrorReason is a human-readable description when IsErrored is
	// true or when the event was dropped for a non-error reason
	// (unknown event, malformed body).
	ErrorReason *string `json:"errorReason,omitempty"`
}

// ConsumeHandlerFunc returns the gin.HandlerFunc that decodes the
// ConsumeEventRequestDto, dispatches to the registered handler, and maps
// the outcome to HTTP status:
//
//   - 200 on nil error (event settled)
//   - 400 on JSON decode failure (Maestro sent malformed body — no retry)
//   - 404 when no handler is registered (unknown event — Maestro drops)
//   - 500 on any other handler error (Maestro retries per its policy)
//
// The response body is a ConsumeEventResponse-shaped payload so the wire
// shape matches Maestro's own /consume_event example.
func ConsumeHandlerFunc(reg *Registry) gin.HandlerFunc {
	if reg == nil {
		panic("maestro: ConsumeHandlerFunc called with nil registry")
	}
	return func(c *gin.Context) {
		var body ConsumeEventRequestDto
		if err := c.ShouldBindJSON(&body); err != nil {
			log.Ctx(c.Request.Context()).Warn().Err(err).Msg("maestro: consume decode failed")
			c.JSON(http.StatusBadRequest, ConsumeEventResponse{
				IsConsumed: boolPtr(false),
				IsErrored:  boolPtr(true),
				ErrorReason: strPtr(
					"invalid JSON body: " + err.Error(),
				),
			})
			return
		}

		err := reg.Dispatch(c.Request.Context(), body)
		switch {
		case err == nil:
			c.JSON(http.StatusOK, ConsumeEventResponse{
				IsConsumed: boolPtr(true),
				IsErrored:  boolPtr(false),
			})
		case errors.Is(err, ErrUnknownEvent):
			log.Ctx(c.Request.Context()).Info().
				Str("event", body.Name).
				Msg("maestro: no handler registered")
			c.JSON(http.StatusNotFound, ConsumeEventResponse{
				IsConsumed:  boolPtr(false),
				IsErrored:   boolPtr(false),
				ErrorReason: strPtr("no handler registered for event " + body.Name),
			})
		default:
			log.Ctx(c.Request.Context()).Error().Err(err).
				Str("event", body.Name).
				Msg("maestro: handler failed")
			c.JSON(http.StatusInternalServerError, ConsumeEventResponse{
				IsConsumed:  boolPtr(false),
				IsErrored:   boolPtr(true),
				ErrorReason: strPtr(err.Error()),
			})
		}
	}
}

func boolPtr(b bool) *bool { return &b }
func strPtr(s string) *string {
	return &s
}
