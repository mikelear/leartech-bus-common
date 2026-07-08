//go:build unit

package events_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mikelear/leartech-bus-common/pkg/events"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRedisPubSubClient covers only the Publish + Close surface of
// [events.RedisPubSubClient]. Subscribe returns nil because the exercised
// unit tests don't invoke Subscribe -- that path requires a live broker
// and is covered by the integration test.
type fakeRedisPubSubClient struct {
	publishErr    error
	publishCalled int
	publishedTo   []string
	publishedData []string
	closeErr      error
	closeCalled   int
}

func (f *fakeRedisPubSubClient) Publish(_ context.Context, channel string, message any) *redis.IntCmd {
	f.publishCalled++
	f.publishedTo = append(f.publishedTo, channel)
	if b, ok := message.([]byte); ok {
		f.publishedData = append(f.publishedData, string(b))
	}
	if f.publishErr != nil {
		return redis.NewIntResult(0, f.publishErr)
	}
	return redis.NewIntResult(1, nil)
}

func (f *fakeRedisPubSubClient) Subscribe(_ context.Context, _ ...string) *redis.PubSub {
	// Not exercised in unit tests.
	return nil
}

func (f *fakeRedisPubSubClient) Close() error {
	f.closeCalled++
	return f.closeErr
}

func TestNewRedisTransport_NilClientReturnsError(t *testing.T) {
	tr, err := events.NewRedisTransport(nil)
	require.Error(t, err)
	assert.Nil(t, tr)
}

func TestRedisTransport_Publish_ForwardsToClient(t *testing.T) {
	fake := &fakeRedisPubSubClient{}
	tr, err := events.NewRedisTransport(fake)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tr.Close() })

	require.NoError(t, tr.Publish("topic-a", []byte("hello")))

	assert.Equal(t, 1, fake.publishCalled)
	assert.Equal(t, []string{"topic-a"}, fake.publishedTo)
	assert.Equal(t, []string{"hello"}, fake.publishedData)
}

func TestRedisTransport_Publish_WrapsError(t *testing.T) {
	fake := &fakeRedisPubSubClient{publishErr: errors.New("connection refused")}
	tr, err := events.NewRedisTransport(fake)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tr.Close() })

	err = tr.Publish("topic-a", []byte("hello"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection refused")
	assert.Contains(t, err.Error(), "topic-a")
}

func TestRedisTransport_Publish_ClosedRejects(t *testing.T) {
	fake := &fakeRedisPubSubClient{}
	tr, err := events.NewRedisTransport(fake)
	require.NoError(t, err)
	require.NoError(t, tr.Close())

	err = tr.Publish("topic", []byte("x"))
	require.Error(t, err)
}

func TestRedisTransport_Subscribe_ClosedRejects(t *testing.T) {
	fake := &fakeRedisPubSubClient{}
	tr, err := events.NewRedisTransport(fake)
	require.NoError(t, err)
	require.NoError(t, tr.Close())

	err = tr.Subscribe("topic", func([]byte) {})
	require.Error(t, err)
}

func TestRedisTransport_CloseIdempotent(t *testing.T) {
	fake := &fakeRedisPubSubClient{}
	tr, err := events.NewRedisTransport(fake)
	require.NoError(t, err)

	require.NoError(t, tr.Close())
	require.NoError(t, tr.Close())
	// Underlying client Close should have been called exactly once -- the
	// second Close is a no-op.
	assert.Equal(t, 1, fake.closeCalled)
}

func TestRedisTransport_Close_WrapsClientError(t *testing.T) {
	fake := &fakeRedisPubSubClient{closeErr: errors.New("client gone")}
	tr, err := events.NewRedisTransport(fake)
	require.NoError(t, err)

	err = tr.Close()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client gone")
}
