package queue

import "context"

// Queue is a generic interface for a distributed queue system.
//
// Implementations:
//   - Queue[T]: Distributed cache using Queue with JSON serialization
type Queue[T any] interface {
	// Push adds a value to the end of the queue.
	Push(ctx context.Context, queue string, value T) error

	// Pop retrieves and removes the oldest value from the queue.
	Pop(ctx context.Context, queue string) (T, error)
}
