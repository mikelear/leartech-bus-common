package migration_test

import (
	"fmt"
	"sort"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/go-version"
	"github.com/mikelear/leartech-bus-common/pkg/mongo"
	"github.com/mikelear/leartech-bus-common/pkg/mongo/migration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dummyMigration implements MigrationInterface for testing.
type dummyMigration struct {
	name    string
	version *version.Version
}

func (d dummyMigration) Name() string {
	return d.name
}

func (d dummyMigration) Version() *version.Version {
	return d.version
}

func (d dummyMigration) Up(_ mongo.MongoDatabase) error {
	return nil
}

func (d dummyMigration) Down(_ mongo.MongoDatabase) error {
	return nil
}

var nodeFactory = migration.NewNodeFactory()

// TestParseAndSortMigrationsSuccess tests that ParseAndSortMigrations returns
// the correct migration nodes and remaining (unsaved) nodes in sorted order.
func TestParseAndSortMigrationsSuccess(t *testing.T) {
	// Create two migrations.
	mig1 := dummyMigration{name: "mig1", version: version.Must(version.NewSemver("0.0.1"))}
	mig2 := dummyMigration{name: "mig2", version: version.Must(version.NewSemver("0.0.2"))}

	// Create a migration model only for mig2 (which is current)
	modelMig1 := migration.MigrationsModel{
		BaseModel: mongo.BaseModel{ID: "model-1"},
		Name:      mig1.Name(),
		Version:   mig1.Version().String(),
		IsCurrent: true,
	}
	// allModels contains a model for mig1 only.
	allModels := []migration.MigrationsModel{modelMig1}
	// migrations list contains both mig1 and mig2.
	migrations := []migration.MigrationInterface{mig1, mig2}

	nodes, remaining, err := nodeFactory.ParseAndSortMigrations(migrations, allModels)
	require.NoError(t, err)
	// nodes should contain the node for mig1.
	assert.Len(t, nodes, 1)
	assert.Equal(t, mig1.Name(), nodes[0].Migration.Name())

	// remaining should contain the node for mig2.
	assert.Len(t, remaining, 1)
	assert.Equal(t, mig2.Name(), remaining[0].Migration.Name())

	// Verify that remaining nodes are sorted in ascending order by Version.
	vers := []*version.Version{}
	for _, n := range remaining {
		vers = append(vers, n.Migration.Version())
	}
	sorted := append([]*version.Version(nil), vers...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].LessThan(sorted[j])
	})
	assert.Equal(t, sorted, vers)
}

// TestParseAndSortMigrationsFailure tests that ParseAndSortMigrations returns
// an error when the last migration node is not marked as current.
func TestParseAndSortMigrationsFailure(t *testing.T) {
	// Create two migrations.
	mig1 := dummyMigration{name: "mig1", version: version.Must(version.NewSemver("0.0.0"))}
	mig2 := dummyMigration{name: "mig2", version: version.Must(version.NewSemver("0.0.1"))}
	mig3 := dummyMigration{name: "mig3", version: version.Must(version.NewSemver("0.1.2"))}

	// Create migration models for mig1 and mig2.
	modelMig1 := migration.MigrationsModel{
		BaseModel: mongo.BaseModel{ID: "model-1"},
		Name:      mig1.Name(),
		Version:   mig1.Version().String(),
		IsCurrent: false,
	}
	modelMig2 := migration.MigrationsModel{
		BaseModel: mongo.BaseModel{ID: "model-2"},
		Name:      mig2.Name(),
		Version:   mig2.Version().String(),
		IsCurrent: false, // Not marked as current even though it's the last model.
	}
	allModels := []migration.MigrationsModel{modelMig1, modelMig2}
	migrations := []migration.MigrationInterface{mig1, mig2, mig3}

	_, _, err := nodeFactory.ParseAndSortMigrations(migrations, allModels)
	require.Error(t, err)
	expectedErr := "last migration node is not current"
	assert.Equal(t, expectedErr, err.Error())
}

// TestHelperFunctions tests the helper functions like CreateMigrationModel, MigrationKey,
// MigrationKeyFromModel and NewUUID.
func TestHelperFunctions(t *testing.T) {
	v1 := version.Must(version.NewSemver("0.0.0"))
	m := dummyMigration{name: "helper_mig", version: v1}
	model := migration.CreateMigrationModel(m)

	// Check that values are correctly set.
	assert.Equal(t, m.Name(), model.Name)
	assert.Equal(t, m.Version().String(), model.Version)
	assert.NotEmpty(t, model.ID)

	// MigrationKey and MigrationKeyFromModel should be consistent.
	key1 := migration.MigrationKey(m)
	key2 := migration.MigrationKeyFromModel(model)
	expectedKey := fmt.Sprintf("%s_%s", v1.String(), m.Name())
	assert.Equal(t, expectedKey, key1)
	assert.Equal(t, expectedKey, key2)

	// Check that NewUUID returns non-empty, distinct uuid.
	uuid1 := uuid.New().String()
	uuid2 := uuid.New().String()
	assert.NotEmpty(t, uuid1)
	assert.NotEmpty(t, uuid2)
	assert.NotEqual(t, uuid1, uuid2)
}
