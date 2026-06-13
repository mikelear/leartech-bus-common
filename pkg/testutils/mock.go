package testutils

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// MockAnythingOfTypeContext returns a mock matcher that matches any context.Context.
// This is because testify/mock does not have built-in support for matching interface types directly so different
// context implementations i.e. context.Background() would not match directly.
func MockAnythingOfTypeContext() interface{} {
	return mock.MatchedBy(func(ctx interface{}) bool {
		_, ok := ctx.(context.Context)
		return ok
	})
}
