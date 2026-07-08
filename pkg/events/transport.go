package events

// Transport is the pluggable substrate the bus uses to move envelopes
// between processes. Implementations should be safe for concurrent use;
// the bus makes no locking guarantees of its own.
//
// A transport is not aware of the envelope schema — it handles raw bytes.
// The bus serialises before Publish and deserialises inside the Subscribe
// callback.
type Transport interface {
	// Publish delivers `data` to all subscribers of `topic`. It must
	// return an error only when the transport itself cannot accept the
	// message (e.g. connection lost). Delivery-side failures should be
	// handled inside the transport (logged, retried, or surfaced via
	// out-of-band monitoring).
	Publish(topic string, data []byte) error

	// Subscribe registers a callback for a topic. The callback receives
	// each incoming envelope's raw bytes. Whether callbacks run
	// synchronously with Publish (as in-memory does) or asynchronously
	// on a goroutine (as Redis does) is transport-specific.
	Subscribe(topic string, cb func([]byte)) error

	// Close releases transport-held resources. After Close, Publish and
	// Subscribe must return an error.
	Close() error
}
