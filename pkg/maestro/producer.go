package maestro

import (
	"context"
	"net/http"

	"github.com/pkg/errors"
)

// Producer wraps the generated Maestro client with the Tier-2 (durable)
// event-bus vocabulary consumers of pkg/events already know.
//
// # Auth
//
// The caller supplies the *http.Client. Real deployments plug in a client
// whose transport attaches the s2s bearer (typically minted from the
// bus-common LEARTECH_AUTH_* environment variables). This package does NOT
// mint or embed credentials — that separation is deliberate so tests can
// pass in a no-op httptest client, and so credential rotation lives in one
// place (the auth minter) rather than being duplicated per producer.
//
// # Usage
//
//	prod, err := maestro.NewProducer("https://maestro.internal", "orchestrator",
//	    maestro.WithProducerHTTPClient(authedClient))
//	if err != nil { return err }
//
//	_, err = prod.Announce(ctx, maestro.Event{
//	    Name:        "initiative.fired",
//	    Annotations: map[string]string{"initiative_id": "init-42"},
//	    ActionedBy:  "user-7",
//	})
//
// # Relationship to pkg/events
//
// pkg/events is the Tier-1 (Redis pub/sub, at-most-once) bus.
// pkg/maestro is the Tier-2 (durable, retried) bus. Both surfaces share
// the "domain event with annotations" mental model; the underlying wire
// protocols differ (JSON envelope on Redis vs OpenAPI HTTP on Maestro).
// Services choose the tier per event based on delivery guarantees, not
// per service.
type Producer struct {
	client     *ClientWithResponses
	producedBy string
}

// ProducerOption configures Producer at construction time.
type ProducerOption func(*producerConfig)

type producerConfig struct {
	httpClient   HttpRequestDoer
	editors      []RequestEditorFn
	clientOption []ClientOption
}

// WithProducerHTTPClient plugs a caller-supplied *http.Client (or any
// HttpRequestDoer). Callers wire the bus-common s2s auth minter here.
// If not supplied, the producer builds a client with http.DefaultTransport
// and NO auth — suitable only for tests against a mock server.
//
// (The generator emits a `WithHTTPClient` ClientOption for the raw
// generated client; this is the ProducerOption-shaped equivalent so
// Producer construction stays uniform.)
func WithProducerHTTPClient(hc HttpRequestDoer) ProducerOption {
	return func(cfg *producerConfig) {
		cfg.httpClient = hc
	}
}

// WithProducerRequestEditor registers a callback that mutates outgoing
// HTTP requests just before they are sent. Useful for late-bound auth
// headers (e.g. per-request token minting) when a static *http.Client is
// not enough.
func WithProducerRequestEditor(fn RequestEditorFn) ProducerOption {
	return func(cfg *producerConfig) {
		cfg.editors = append(cfg.editors, fn)
	}
}

// Event is the input to Producer.Announce. It mirrors the generated
// AnnounceEventRequest but takes value-typed fields (not pointers) for
// convenient call-site syntax. Empty ActionedBy and nil Annotations are
// omitted from the wire body — see AnnounceEventRequest tags.
type Event struct {
	// Name is the dotted event name (e.g. "initiative.fired").
	Name string

	// Annotations are string key/value pairs that scope the event to a
	// domain (case_id, initiative_id, tenant_id, ...). Maestro indexes
	// events by annotation so consumers can query events_by_annotation.
	Annotations map[string]string

	// ActionedBy is the userId (or system-actor id) that caused the
	// event, when applicable. Empty means "no user actioned this event".
	ActionedBy string
}

// NewProducer constructs a Producer for the given Maestro base URL,
// stamping producedBy into every announcement.
//
// producedBy is the service's own name (e.g. "orchestrator",
// "webcoder-service"). It must be non-empty — Maestro rejects
// announcements without a producer.
func NewProducer(baseURL, producedBy string, opts ...ProducerOption) (*Producer, error) {
	if baseURL == "" {
		return nil, errors.New("maestro: baseURL must not be empty")
	}
	if producedBy == "" {
		return nil, errors.New("maestro: producedBy must not be empty")
	}

	cfg := &producerConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	clientOpts := cfg.clientOption
	if cfg.httpClient != nil {
		clientOpts = append(clientOpts, WithHTTPClient(cfg.httpClient))
	}
	for _, ed := range cfg.editors {
		clientOpts = append(clientOpts, WithRequestEditorFn(ed))
	}

	c, err := NewClientWithResponses(baseURL, clientOpts...)
	if err != nil {
		return nil, errors.Wrap(err, "maestro: build client")
	}

	return &Producer{
		client:     c,
		producedBy: producedBy,
	}, nil
}

// Announce publishes evt to Maestro. Returns the parsed 200 response body
// (or nil + error on non-200).
//
// The wire body is AnnounceEventRequest with:
//   - name        = evt.Name
//   - producedBy  = the producer's producedBy field
//   - annotations = evt.Annotations (omitted if nil/empty)
//   - actionedBy  = evt.ActionedBy (omitted if empty)
//
// Success is defined as HTTP 200 AND response.Success == true. Any other
// combination surfaces as an error so callers can retry or fan out.
func (p *Producer) Announce(ctx context.Context, evt Event) (*AnnounceEventResponse, error) {
	if evt.Name == "" {
		return nil, errors.New("maestro: event Name must not be empty")
	}

	body := AnnounceEventRequest{
		Name:        evt.Name,
		ProducedBy:  p.producedBy,
		Annotations: nilOrRefMap(evt.Annotations),
		ActionedBy:  nilOrRefString(evt.ActionedBy),
	}

	resp, err := p.client.AnnounceEventWithResponse(ctx, body)
	if err != nil {
		return nil, errors.Wrap(err, "maestro: announce event")
	}
	if resp.HTTPResponse == nil {
		return nil, errors.New("maestro: nil HTTP response")
	}
	if resp.HTTPResponse.StatusCode != http.StatusOK {
		return nil, errors.Errorf("maestro: unexpected status %d announcing %q",
			resp.HTTPResponse.StatusCode, evt.Name)
	}
	if resp.JSON200 == nil {
		return nil, errors.Errorf("maestro: 200 response with no parsed body announcing %q",
			evt.Name)
	}
	if resp.JSON200.Success != nil && !*resp.JSON200.Success {
		desc := ""
		if resp.JSON200.ErrorDescription != nil {
			desc = *resp.JSON200.ErrorDescription
		}
		return resp.JSON200, errors.Errorf("maestro: broker rejected %q: %s", evt.Name, desc)
	}

	return resp.JSON200, nil
}

// nilOrRefMap returns nil when m is empty (so the generated struct emits
// no key), or a pointer to the map otherwise. Matches the generated field
// type `*map[string]string`.
func nilOrRefMap(m map[string]string) *map[string]string {
	if len(m) == 0 {
		return nil
	}
	return &m
}

// nilOrRefString returns nil when s is empty (so the generated struct
// emits no key), or a pointer to s otherwise.
func nilOrRefString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
