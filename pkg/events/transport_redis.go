package events

import (
	"context"
	"sync"

	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// RedisPubSubClient is the minimal go-redis surface the [RedisTransport] uses.
// The concrete *redis.Client satisfies this interface; a subset is exposed
// so the transport can be tested with narrower fakes if desired.
type RedisPubSubClient interface {
	Publish(ctx context.Context, channel string, message any) *redis.IntCmd
	Subscribe(ctx context.Context, channels ...string) *redis.PubSub
	Close() error
}

// RedisTransport implements [Transport] over Redis pub/sub. Publish maps to
// PUBLISH on `topic`; Subscribe starts a goroutine that reads from a
// per-topic PubSub connection and hands raw payload bytes to the callback.
//
// The transport owns the client lifecycle when the caller uses NewBus with a
// RedisTransport it constructed — closing the bus closes the transport,
// which closes any live PubSubs and (importantly) the underlying client.
// If the client is shared with other subsystems, wrap it before passing in
// or ignore the "closes underlying client" behaviour by managing lifecycle
// externally.
type RedisTransport struct {
	client RedisPubSubClient

	mu      sync.Mutex
	closed  bool
	ctx     context.Context //nolint:containedctx // long-lived subscribe lifecycle, cancelled on Close
	cancel  context.CancelFunc
	subs    []*redis.PubSub
	workers sync.WaitGroup
}

// NewRedisTransport constructs a RedisTransport around the given client.
// The client must be non-nil; connectivity is not verified here (the
// caller is expected to have already PINGed it via pkg/redis.NewRedisClient
// or an equivalent).
func NewRedisTransport(client RedisPubSubClient) (*RedisTransport, error) {
	if client == nil {
		return nil, errors.New("redis transport: nil client")
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &RedisTransport{
		client: client,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

// Publish sends data as a single Redis PUBLISH on the given channel.
// Returns an error if the transport has been closed or Redis rejects the
// publish. The number-of-receivers value returned by Redis is ignored —
// Redis pub/sub is fire-and-forget by design.
func (t *RedisTransport) Publish(topic string, data []byte) error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return errors.New("redis transport: closed")
	}
	ctx := t.ctx
	t.mu.Unlock()

	if err := t.client.Publish(ctx, topic, data).Err(); err != nil {
		return errors.Wrapf(err, "redis transport: publish to %q", topic)
	}
	return nil
}

// Subscribe registers cb on `topic` and spawns a goroutine that pumps
// received messages into cb. The goroutine exits when Close is called
// (which closes the underlying PubSub and its Channel).
func (t *RedisTransport) Subscribe(topic string, cb func([]byte)) error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return errors.New("redis transport: closed")
	}
	sub := t.client.Subscribe(t.ctx, topic)
	t.subs = append(t.subs, sub)
	t.workers.Add(1)
	ctx := t.ctx
	t.mu.Unlock()

	go t.consume(ctx, sub, topic, cb)
	return nil
}

// Close cancels the transport context, closes every live PubSub (which
// unblocks each consume goroutine), waits for the goroutines to exit, and
// then closes the underlying client. Idempotent.
func (t *RedisTransport) Close() error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil
	}
	t.closed = true
	subs := t.subs
	t.subs = nil
	t.cancel()
	t.mu.Unlock()

	for _, s := range subs {
		if err := s.Close(); err != nil {
			log.Warn().Err(err).Msg("redis transport: closing pubsub")
		}
	}
	t.workers.Wait()

	if err := t.client.Close(); err != nil {
		return errors.Wrap(err, "redis transport: close client")
	}
	return nil
}

// consume pumps messages from the pubsub channel into cb until the channel
// closes (which happens when the pubsub is closed by Close, or when the
// context is cancelled).
func (t *RedisTransport) consume(ctx context.Context, sub *redis.PubSub, topic string, cb func([]byte)) {
	defer t.workers.Done()
	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			cb([]byte(msg.Payload))
			_ = topic // reserved for future per-topic instrumentation
		}
	}
}
