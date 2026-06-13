package mongo_test

import (
	"testing"
	"time"

	"github.com/mikelear/leartech-bus-common/pkg/mongo"
	"github.com/stretchr/testify/assert"
)

func TestBaseModel_UpdateTimestamps(t *testing.T) {
	testCases := []struct {
		name     string
		initial  mongo.BaseModel
		expected func(mongo.BaseModel) bool
	}{
		{
			name: "NewModelSetsCreatedAtAndUpdatedAt",
			initial: mongo.BaseModel{
				ID: "test-id",
			},
			expected: func(bm mongo.BaseModel) bool {
				return !bm.CreatedAt.IsZero() && !bm.UpdatedAt.IsZero() &&
					bm.CreatedAt.Equal(bm.UpdatedAt)
			},
		},
		{
			name: "ExistingModelOnlyUpdatesUpdatedAt",
			initial: mongo.BaseModel{
				ID:        "test-id",
				CreatedAt: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
				UpdatedAt: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			expected: func(bm mongo.BaseModel) bool {
				originalCreated := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
				return bm.CreatedAt.Equal(originalCreated) && bm.UpdatedAt.After(originalCreated)
			},
		},
		{
			name: "ZeroCreatedAtGetsSet",
			initial: mongo.BaseModel{
				ID:        "test-id",
				CreatedAt: time.Time{},
				UpdatedAt: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			expected: func(bm mongo.BaseModel) bool {
				return !bm.CreatedAt.IsZero() && bm.UpdatedAt.After(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Record time before update for comparison
			beforeUpdate := time.Now()

			// Execute UpdateTimestamps
			tc.initial.UpdateTimestamps()

			// Record time after update for comparison
			afterUpdate := time.Now()

			// Verify the expected behavior
			assert.True(t, tc.expected(tc.initial), "Timestamp update behavior didn't match expected")

			// Verify UpdatedAt is within reasonable bounds
			assert.True(t, tc.initial.UpdatedAt.After(beforeUpdate) || tc.initial.UpdatedAt.Equal(beforeUpdate),
				"UpdatedAt should be after or equal to time before update")
			assert.True(t, tc.initial.UpdatedAt.Before(afterUpdate) || tc.initial.UpdatedAt.Equal(afterUpdate),
				"UpdatedAt should be before or equal to time after update")
		})
	}
}

func TestBaseModel_Fields(t *testing.T) {
	bm := mongo.BaseModel{
		ID:        "test-id",
		CreatedAt: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC),
		Deleted:   false,
	}

	assert.Equal(t, "test-id", bm.ID)
	assert.Equal(t, time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC), bm.CreatedAt)
	assert.Equal(t, time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC), bm.UpdatedAt)
	assert.False(t, bm.Deleted)
}

func TestBaseModel_EmbeddedUsage(t *testing.T) {
	// Test that BaseModel can be embedded in other structs
	type TestEntity struct {
		mongo.BaseModel `bson:",inline"`
		Name            string `bson:"name"`
		Value           int    `bson:"value"`
	}

	entity := TestEntity{
		BaseModel: mongo.BaseModel{ID: "embedded-test"},
		Name:      "Test Entity",
		Value:     42,
	}

	// Update timestamps
	entity.UpdateTimestamps()

	// Verify the embedded BaseModel functionality works
	assert.Equal(t, "embedded-test", entity.ID)
	assert.Equal(t, "Test Entity", entity.Name)
	assert.Equal(t, 42, entity.Value)
	assert.False(t, entity.Deleted)
	assert.False(t, entity.CreatedAt.IsZero())
	assert.False(t, entity.UpdatedAt.IsZero())
}

func TestBaseModel_UpdateTimestamps_MultipleCalls(t *testing.T) {
	bm := mongo.BaseModel{ID: "test-multiple"}

	// First call
	bm.UpdateTimestamps()
	firstCreatedAt := bm.CreatedAt
	firstUpdatedAt := bm.UpdatedAt

	// Small delay to ensure timestamps are different
	time.Sleep(1 * time.Millisecond)

	// Second call
	bm.UpdateTimestamps()
	secondCreatedAt := bm.CreatedAt
	secondUpdatedAt := bm.UpdatedAt

	// CreatedAt should remain the same, UpdatedAt should be newer
	assert.Equal(t, firstCreatedAt, secondCreatedAt, "CreatedAt should not change on subsequent calls")
	assert.True(t, secondUpdatedAt.After(firstUpdatedAt), "UpdatedAt should be updated on subsequent calls")
}
