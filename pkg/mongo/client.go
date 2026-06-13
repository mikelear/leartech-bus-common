package mongo

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

type client struct {
	client *mongo.Client
}

func NewMongoClient(connectionString string) (MongoClient, error) {
	mongoClient, err := mongo.Connect(options.Client().ApplyURI(connectionString))
	if err != nil {
		return nil, err
	}

	return &client{
		client: mongoClient,
	}, nil
}

func (m *client) Ping(ctx context.Context, rp *readpref.ReadPref) error {
	return m.client.Ping(ctx, rp)
}

func (m *client) Database(name string, opts ...options.Lister[options.DatabaseOptions]) MongoDatabase {
	db := m.client.Database(name, opts...)
	return &Database{db: db}
}

func (m *client) Disconnect(ctx context.Context) error {
	return m.client.Disconnect(ctx)
}
