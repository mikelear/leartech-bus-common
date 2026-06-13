package mongo

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

// MongoClient defines the interface for a MongoDB client from the mongo package
type MongoClient interface {
	Disconnect(ctx context.Context) error
	Ping(ctx context.Context, rp *readpref.ReadPref) error
	Database(name string, opts ...options.Lister[options.DatabaseOptions]) MongoDatabase
}

// MongoDatabase defines the interface for a MongoDB database from the mongo package
type MongoDatabase interface {
	Collection(name string, opts ...options.Lister[options.CollectionOptions]) MongoCollection
}

// MongoCollection defines the interface for a MongoDB collection from the mongo package
type MongoCollection interface {
	CreateIndexesIfNotExist(ctx context.Context, indexes []MongoIndex) error

	Clone(opts ...options.Lister[options.CollectionOptions]) *mongo.Collection
	Name() string
	Database() *mongo.Database
	BulkWrite(ctx context.Context, models []mongo.WriteModel, opts ...options.Lister[options.BulkWriteOptions]) (*mongo.BulkWriteResult, error)
	InsertOne(ctx context.Context, document interface{}, opts ...options.Lister[options.InsertOneOptions]) (*mongo.InsertOneResult, error)
	InsertMany(ctx context.Context, documents []interface{}, opts ...options.Lister[options.InsertManyOptions]) (*mongo.InsertManyResult, error)
	DeleteOne(ctx context.Context, filter interface{}, opts ...options.Lister[options.DeleteOneOptions]) (*mongo.DeleteResult, error)
	DeleteMany(ctx context.Context, filter any, opts ...options.Lister[options.DeleteManyOptions]) (*mongo.DeleteResult, error)
	UpdateByID(ctx context.Context, id any, update any, opts ...options.Lister[options.UpdateOneOptions]) (*mongo.UpdateResult, error)
	UpdateOne(ctx context.Context, filter interface{}, update interface{}, opts ...options.Lister[options.UpdateOneOptions]) (*mongo.UpdateResult, error)
	UpdateMany(ctx context.Context, filter interface{}, update interface{}, opts ...options.Lister[options.UpdateManyOptions]) (*mongo.UpdateResult, error)
	ReplaceOne(ctx context.Context, filter interface{}, replacement interface{}, opts ...options.Lister[options.ReplaceOptions]) (*mongo.UpdateResult, error)
	Aggregate(ctx context.Context, pipeline interface{}, opts ...options.Lister[options.AggregateOptions]) (MongoCursor, error)
	CountDocuments(ctx context.Context, filter interface{}, opts ...options.Lister[options.CountOptions]) (int64, error)
	EstimatedDocumentCount(ctx context.Context, opts ...options.Lister[options.EstimatedDocumentCountOptions]) (int64, error)
	Distinct(ctx context.Context, fieldName string, filter any, opts ...options.Lister[options.DistinctOptions]) *mongo.DistinctResult
	Find(ctx context.Context, filter interface{}, opts ...options.Lister[options.FindOptions]) (MongoCursor, error)
	FindOne(ctx context.Context, filter interface{}, opts ...options.Lister[options.FindOneOptions]) MongoSingleResult
	FindOneAndDelete(ctx context.Context, filter any, opts ...options.Lister[options.FindOneAndDeleteOptions]) *mongo.SingleResult
	FindOneAndReplace(ctx context.Context, filter any, replacement any, opts ...options.Lister[options.FindOneAndReplaceOptions]) *mongo.SingleResult
	FindOneAndUpdate(ctx context.Context, filter any, update any, opts ...options.Lister[options.FindOneAndUpdateOptions]) *mongo.SingleResult
	Watch(ctx context.Context, pipeline any, opts ...options.Lister[options.ChangeStreamOptions]) (*mongo.ChangeStream, error)
	Indexes() MongoIndexView
	SearchIndexes() mongo.SearchIndexView
	Drop(ctx context.Context, opts ...options.Lister[options.DropCollectionOptions]) error
}

// MongoSingleResult defines the interface for a single result from a MongoDB query, allowing for mocking in tests
type MongoSingleResult interface {
	Decode(v interface{}) error
}

// MongoCursor defines the interface for a MongoDB cursor, allowing for mocking in tests
type MongoCursor interface {
	ID() int64
	Next(ctx context.Context) bool
	TryNext(ctx context.Context) bool
	Decode(v interface{}) error
	Err() error
	Close(ctx context.Context) error
	All(ctx context.Context, results interface{}) error
}
