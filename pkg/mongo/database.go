package mongo

import (
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type Database struct {
	db *mongo.Database
}

func (m *Database) Collection(name string, opts ...options.Lister[options.CollectionOptions]) MongoCollection {
	collection := m.db.Collection(name, opts...)
	return &Collection{collection: collection}
}
