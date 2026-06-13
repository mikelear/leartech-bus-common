package mongo

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type Collection struct {
	collection *mongo.Collection
}

var _ MongoCollection = &Collection{} // ensure Collection implements MongoCollection

func (m *Collection) CreateIndexesIfNotExist(ctx context.Context, idx []MongoIndex) error {
	indexUc := NewIndexUseCase()
	indexView := m.Indexes()

	existingIndexes, err := indexUc.GetExistingIndexes(ctx, indexView)
	if err != nil {
		return fmt.Errorf("failed to get existing indexes: %w", err)
	}

	for _, index := range idx {
		if err := indexUc.CreateIndexIfNotExist(ctx, indexView, existingIndexes, index); err != nil {
			return fmt.Errorf("failed to create index %s: %w", index.Name, err)
		}
	}

	return nil
}

func (m *Collection) ReplaceOne(ctx context.Context, filter interface{}, replacement interface{}, opts ...options.Lister[options.ReplaceOptions]) (*mongo.UpdateResult, error) {
	return m.collection.ReplaceOne(ctx, filter, replacement, opts...)
}

func (m *Collection) Aggregate(ctx context.Context, pipeline interface{}, opts ...options.Lister[options.AggregateOptions]) (MongoCursor, error) {
	return m.collection.Aggregate(ctx, pipeline, opts...)
}

func (m *Collection) FindOne(ctx context.Context, filter interface{}, opts ...options.Lister[options.FindOneOptions]) MongoSingleResult {
	return m.collection.FindOne(ctx, filter, opts...)
}

func (m *Collection) Indexes() MongoIndexView {
	return m.collection.Indexes()
}

func (m *Collection) CountDocuments(ctx context.Context, filter interface{}, opts ...options.Lister[options.CountOptions]) (int64, error) {
	return m.collection.CountDocuments(ctx, filter, opts...)
}

func (m *Collection) DeleteOne(ctx context.Context, filter interface{}, opts ...options.Lister[options.DeleteOneOptions]) (*mongo.DeleteResult, error) {
	return m.collection.DeleteOne(ctx, filter, opts...)
}

func (m *Collection) Find(ctx context.Context, filter interface{}, opts ...options.Lister[options.FindOptions]) (MongoCursor, error) {
	return m.collection.Find(ctx, filter, opts...)
}

func (m *Collection) InsertOne(ctx context.Context, document interface{}, opts ...options.Lister[options.InsertOneOptions]) (*mongo.InsertOneResult, error) {
	return m.collection.InsertOne(ctx, document, opts...)
}

func (m *Collection) InsertMany(ctx context.Context, document []interface{}, opts ...options.Lister[options.InsertManyOptions]) (*mongo.InsertManyResult, error) {
	return m.collection.InsertMany(ctx, document, opts...)
}

func (m *Collection) UpdateOne(ctx context.Context, filter interface{}, update interface{}, opts ...options.Lister[options.UpdateOneOptions]) (*mongo.UpdateResult, error) {
	return m.collection.UpdateOne(ctx, filter, update, opts...)
}

func (m *Collection) UpdateMany(ctx context.Context, filter interface{}, update interface{}, opts ...options.Lister[options.UpdateManyOptions]) (*mongo.UpdateResult, error) {
	return m.collection.UpdateMany(ctx, filter, update, opts...)
}

func (m *Collection) Clone(opts ...options.Lister[options.CollectionOptions]) *mongo.Collection {
	return m.collection.Clone(opts...)
}

func (m *Collection) Name() string {
	return m.collection.Name()
}

func (m *Collection) Database() *mongo.Database {
	return m.collection.Database()
}

func (m *Collection) BulkWrite(ctx context.Context, models []mongo.WriteModel, opts ...options.Lister[options.BulkWriteOptions]) (*mongo.BulkWriteResult, error) {
	return m.collection.BulkWrite(ctx, models, opts...)
}

func (m *Collection) DeleteMany(ctx context.Context, filter any, opts ...options.Lister[options.DeleteManyOptions]) (*mongo.DeleteResult, error) {
	return m.collection.DeleteMany(ctx, filter, opts...)
}

func (m *Collection) UpdateByID(ctx context.Context, id any, update any, opts ...options.Lister[options.UpdateOneOptions]) (*mongo.UpdateResult, error) {
	return m.collection.UpdateByID(ctx, id, update, opts...)
}

func (m *Collection) EstimatedDocumentCount(ctx context.Context, opts ...options.Lister[options.EstimatedDocumentCountOptions]) (int64, error) {
	return m.collection.EstimatedDocumentCount(ctx, opts...)
}

func (m *Collection) Distinct(ctx context.Context, fieldName string, filter any, opts ...options.Lister[options.DistinctOptions]) *mongo.DistinctResult {
	return m.collection.Distinct(ctx, fieldName, filter, opts...)
}

func (m *Collection) FindOneAndDelete(ctx context.Context, filter any, opts ...options.Lister[options.FindOneAndDeleteOptions]) *mongo.SingleResult {
	return m.collection.FindOneAndDelete(ctx, filter, opts...)
}

func (m *Collection) FindOneAndReplace(ctx context.Context, filter any, replacement any, opts ...options.Lister[options.FindOneAndReplaceOptions]) *mongo.SingleResult {
	return m.collection.FindOneAndReplace(ctx, filter, replacement, opts...)
}

func (m *Collection) FindOneAndUpdate(ctx context.Context, filter any, update any, opts ...options.Lister[options.FindOneAndUpdateOptions]) *mongo.SingleResult {
	return m.collection.FindOneAndUpdate(ctx, filter, update, opts...)
}

func (m *Collection) Watch(ctx context.Context, pipeline any, opts ...options.Lister[options.ChangeStreamOptions]) (*mongo.ChangeStream, error) {
	return m.collection.Watch(ctx, pipeline, opts...)
}

func (m *Collection) SearchIndexes() mongo.SearchIndexView {
	return m.collection.SearchIndexes()
}

func (m *Collection) Drop(ctx context.Context, opts ...options.Lister[options.DropCollectionOptions]) error {
	return m.collection.Drop(ctx, opts...)
}
