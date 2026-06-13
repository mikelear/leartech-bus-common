package random_test

import (
	"github.com/mikelear/leartech-bus-common/pkg/random"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestGenerator_NewString(t *testing.T) {
	generator := random.NewGenerator()
	length := 16

	strA, err := generator.NewString(length)
	require.NoError(t, err)
	assert.Len(t, strA, length)

	strB, err := generator.NewString(length)
	require.NoError(t, err)
	assert.Len(t, strB, length)

	assert.NotEqual(t, strA, strB)
}

func TestGenerator_NewUUID(t *testing.T) {
	generator := random.NewGenerator()

	uuidA := generator.NewUUID()
	uuidB := generator.NewUUID()

	assert.NotEqual(t, uuidA, uuidB)
}
