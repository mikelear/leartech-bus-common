package migration

import (
	"github.com/hashicorp/go-version"
	"github.com/mikelear/leartech-bus-common/pkg/mongo"
)

// dummyMigration is a dummy migration that does nothing
// this is used for the base migration
type dummyMigration struct {
	name    string
	version *version.Version
}

// Up applies the current migration to the database
func (m *dummyMigration) Up(_ mongo.MongoDatabase) error {
	return nil
}

// Down rolls back the current migration from the database
func (m *dummyMigration) Down(_ mongo.MongoDatabase) error {
	return nil
}

// Name returns the name of the migration
func (m *dummyMigration) Name() string {
	return m.name
}

// Version returns the version of the migration
func (m *dummyMigration) Version() *version.Version {
	return m.version
}
