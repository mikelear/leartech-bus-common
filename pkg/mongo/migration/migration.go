package migration

import (
	"github.com/hashicorp/go-version"
	"github.com/mikelear/leartech-bus-common/pkg/mongo"
)

// MigrationInterface represents a database migration
// It can be applied to migrate a database to a new version
type MigrationInterface interface {
	// Up applies the current migration to the database
	Up(db mongo.MongoDatabase) error
	// Down rolls back the current migration from the database
	Down(db mongo.MongoDatabase) error
	// Name returns the name of the migration
	Name() string
	// Version returns the version of the migration
	Version() *version.Version
}

// MigrationRepositoryInterface represents a repository for migrations
type MigrationRepositoryInterface interface {
	// LoadMigrations loads all the migrations registered in the manager
	LoadMigrations() []MigrationInterface
}

// NodeFactoryInterface represents a factory for creating migration nodes
type NodeFactoryInterface interface {
	// ParseAndSortMigrations creates an ordered list of migration nodes, for both applied and remaining migrations
	ParseAndSortMigrations(migrations []MigrationInterface, allMigrationModels []MigrationsModel) ([]MigrationNode, []MigrationNode, error)
}

// MigrationManagerInterface represents a manager for running migrations
type MigrationManagerInterface interface {
	// RunMigrations runs all the migrations registered in the manager
	RunMigrations(client *mongo.MongoClient, steps int) error
}
