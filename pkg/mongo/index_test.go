package mongo_test

import (
	"errors"
	"testing"

	"github.com/mikelear/leartech-bus-common/pkg/mongo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	gomongo "go.mongodb.org/mongo-driver/v2/mongo"
)

func TestCreateIndexIfNotExist(t *testing.T) {
	testCases := []struct {
		name          string
		existingIndex bool
	}{
		{
			name:          "Creates index if it does not exist",
			existingIndex: false,
		},
		{
			name:          "Index already exists so an index is not created and no error is returned",
			existingIndex: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockIndexView := mongo.NewMockMongoIndexView(t)
			indexName := "indexName"
			uc := mongo.NewIndexUseCase()

			existingIndexes := make([]string, 0)
			if tc.existingIndex {
				existingIndexes = append(existingIndexes, indexName)
			}

			index := mongo.MongoIndex{
				Name: "indexName",
				Keys: bson.D{{Key: "caseId", Value: 1}},
			}

			if !tc.existingIndex {
				mockIndexView.On("CreateOne", mock.Anything, mock.Anything).Return(indexName, nil).Once()
			}

			err := uc.CreateIndexIfNotExist(t.Context(), mockIndexView, existingIndexes, index)
			require.NoError(t, err)
			mockIndexView.AssertExpectations(t)
		})
	}
}

func TestCreateIndexCreateErrors(t *testing.T) {
	mockIndexView := mongo.NewMockMongoIndexView(t)
	expectedError := errors.New("some expected error")
	uc := mongo.NewIndexUseCase()

	existingIndexes := make([]string, 0)

	index := mongo.MongoIndex{
		Name: "indexName",
		Keys: bson.D{{Key: "caseId", Value: 1}},
	}

	mockIndexView.On("CreateOne", mock.Anything, mock.Anything).Return("", expectedError).Once()

	err := uc.CreateIndexIfNotExist(t.Context(), mockIndexView, existingIndexes, index)
	require.Error(t, err)
	require.ErrorIs(t, err, expectedError, "expected error was different")
	mockIndexView.AssertExpectations(t)
}

func TestGetExistingIndexesHappyPath(t *testing.T) {
	mockIndexView := mongo.NewMockMongoIndexView(t)

	databaseContents := []interface{}{
		bson.M{
			"name": "index1",
		},
	}

	indexCursor, err := gomongo.NewCursorFromDocuments(databaseContents, nil, nil)
	require.NoError(t, err)

	mockIndexView.On("List", mock.Anything).Return(indexCursor, nil)

	uc := mongo.NewIndexUseCase()
	indexes, err := uc.GetExistingIndexes(t.Context(), mockIndexView)

	require.NoError(t, err)
	assert.Len(t, indexes, 1)
	assert.Equal(t, "index1", indexes[0])
	mockIndexView.AssertExpectations(t)
}

func TestGetExistingIndexesListingReturnsError(t *testing.T) {
	mockIndexView := mongo.NewMockMongoIndexView(t)
	expectedErr := errors.New("failed to list indexes")
	mockIndexView.On("List", mock.Anything).Return(nil, expectedErr).Once()

	uc := mongo.NewIndexUseCase()
	_, err := uc.GetExistingIndexes(t.Context(), mockIndexView)

	require.Error(t, err)
	require.ErrorIs(t, err, expectedErr, "expected error was different")
	mockIndexView.AssertExpectations(t)
}
