package mongo

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type MongoIndex struct {
	Name string
	Keys bson.D
}

// MongoIndexView defines the interface for MongoDB index operations
type MongoIndexView interface {
	CreateOne(ctx context.Context, model mongo.IndexModel, opts ...options.Lister[options.CreateIndexesOptions]) (string, error)
	List(ctx context.Context, opts ...options.Lister[options.ListIndexesOptions]) (*mongo.Cursor, error)
}

// IndexUseCase defines the interface for index operations in MongoDB
type IndexUseCase interface {
	CreateIndexIfNotExist(ctx context.Context, iv MongoIndexView, existingIndexNames []string, index MongoIndex) error
	GetExistingIndexes(ctx context.Context, iv MongoIndexView) ([]string, error)
}

type indexUseCase struct{}

func NewIndexUseCase() IndexUseCase {
	return &indexUseCase{}
}

func (uc indexUseCase) CreateIndexIfNotExist(ctx context.Context, iv MongoIndexView, existingIndexNames []string, index MongoIndex) error {
	if slices.Contains(existingIndexNames, index.Name) {
		return nil
	}

	indexModel := mongo.IndexModel{
		Keys: index.Keys,
	}

	_, err := iv.CreateOne(ctx, indexModel)
	if err != nil {
		return err
	}

	log.Logger.Info().Str("name", index.Name).Msg("Created mongo index")
	return nil
}

func (uc indexUseCase) GetExistingIndexes(ctx context.Context, iv MongoIndexView) ([]string, error) {
	indexes, err := iv.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list existing indexes: %w", err)
	}

	var indexNames []string
	for indexes.Next(ctx) {
		var index bson.M
		err := indexes.Decode(&index)
		if err != nil {
			return nil, fmt.Errorf("failed to decode existing index: %w", err)
		}

		name, ok := index["name"].(string)
		if !ok {
			return nil, errors.New("index name could not be cast to a string")
		}
		indexNames = append(indexNames, name)
	}

	return indexNames, nil
}
