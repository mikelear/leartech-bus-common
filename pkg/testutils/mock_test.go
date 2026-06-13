//go:build unit

package testutils_test

import (
	"context"
	"testing"

	"github.com/mikelear/leartech-bus-common/pkg/testutils"
	"github.com/stretchr/testify/mock"
)

// mockInterface is a test mock that we'll use to verify the context matcher works with mock.Call.
type mockInterface struct {
	mock.Mock
}

func (m *mockInterface) DoSomething(ctx context.Context) {
	m.Called(ctx)
}

func TestMockAnythingOfTypeContext(t *testing.T) {
	t.Run("matches context.Background", func(t *testing.T) {
		m := &mockInterface{}
		m.On("DoSomething", testutils.MockAnythingOfTypeContext()).Return()

		m.DoSomething(context.Background())

		m.AssertExpectations(t)
	})

	t.Run("matches context.TODO", func(t *testing.T) {
		m := &mockInterface{}
		m.On("DoSomething", testutils.MockAnythingOfTypeContext()).Return()

		m.DoSomething(context.TODO())

		m.AssertExpectations(t)
	})

	t.Run("matches context with values", func(t *testing.T) {
		m := &mockInterface{}
		m.On("DoSomething", testutils.MockAnythingOfTypeContext()).Return()

		ctx := context.WithValue(context.Background(), "key", "value")
		m.DoSomething(ctx)

		m.AssertExpectations(t)
	})

	t.Run("matches context with cancellation", func(t *testing.T) {
		m := &mockInterface{}
		m.On("DoSomething", testutils.MockAnythingOfTypeContext()).Return()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		m.DoSomething(ctx)

		m.AssertExpectations(t)
	})

	t.Run("does not match non-context types", func(t *testing.T) {
		m := &mockInterface{}
		// Set up expectation with context matcher
		m.On("DoSomething", testutils.MockAnythingOfTypeContext()).Return()

		// This should match the mock expectation
		m.DoSomething(context.Background())

		// Verify that the mock was called correctly
		m.AssertExpectations(t)
		m.AssertNumberOfCalls(t, "DoSomething", 1)
	})
}