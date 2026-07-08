//go:build unit

package events_test

import (
	"testing"

	"github.com/mikelear/leartech-bus-common/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInMemoryTransport_PublishFanOut(t *testing.T) {
	tr := events.NewInMemoryTransport()
	t.Cleanup(func() { _ = tr.Close() })

	var got1, got2 []string
	require.NoError(t, tr.Subscribe("topic-a", func(d []byte) { got1 = append(got1, string(d)) }))
	require.NoError(t, tr.Subscribe("topic-a", func(d []byte) { got2 = append(got2, string(d)) }))

	require.NoError(t, tr.Publish("topic-a", []byte("hello")))
	require.NoError(t, tr.Publish("topic-a", []byte("world")))

	assert.Equal(t, []string{"hello", "world"}, got1)
	assert.Equal(t, []string{"hello", "world"}, got2)
}

func TestInMemoryTransport_TopicIsolation(t *testing.T) {
	tr := events.NewInMemoryTransport()
	t.Cleanup(func() { _ = tr.Close() })

	var a, b [][]byte
	require.NoError(t, tr.Subscribe("a", func(d []byte) { a = append(a, d) }))
	require.NoError(t, tr.Subscribe("b", func(d []byte) { b = append(b, d) }))

	require.NoError(t, tr.Publish("a", []byte("A1")))
	require.NoError(t, tr.Publish("b", []byte("B1")))

	assert.Equal(t, [][]byte{[]byte("A1")}, a)
	assert.Equal(t, [][]byte{[]byte("B1")}, b)
}

func TestInMemoryTransport_PublishWithNoSubscribers(t *testing.T) {
	tr := events.NewInMemoryTransport()
	t.Cleanup(func() { _ = tr.Close() })
	assert.NoError(t, tr.Publish("nobody-home", []byte("x")))
}

func TestInMemoryTransport_ClosedRejects(t *testing.T) {
	tr := events.NewInMemoryTransport()
	require.NoError(t, tr.Close())

	err := tr.Publish("topic", []byte("x"))
	assert.Error(t, err)

	err = tr.Subscribe("topic", func([]byte) {})
	assert.Error(t, err)
}

func TestInMemoryTransport_CloseIdempotent(t *testing.T) {
	tr := events.NewInMemoryTransport()
	require.NoError(t, tr.Close())
	assert.NoError(t, tr.Close())
}

// TestInMemoryTransport_SubscriberCanRepublish exercises the deadlock-safety
// note in Publish: if a handler publishes back into the same transport, it
// must not block on the mutex.
func TestInMemoryTransport_SubscriberCanRepublish(t *testing.T) {
	tr := events.NewInMemoryTransport()
	t.Cleanup(func() { _ = tr.Close() })

	var received []string
	require.NoError(t, tr.Subscribe("chain", func(d []byte) {
		received = append(received, string(d))
		if string(d) == "first" {
			_ = tr.Publish("chain", []byte("second"))
		}
	}))

	require.NoError(t, tr.Publish("chain", []byte("first")))
	assert.Equal(t, []string{"first", "second"}, received)
}
