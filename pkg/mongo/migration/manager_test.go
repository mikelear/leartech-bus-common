package migration_test

import (
	"context"
	"testing"

	"github.com/hashicorp/go-version"
	"github.com/mikelear/leartech-bus-common/pkg/mongo"
	"github.com/mikelear/leartech-bus-common/pkg/mongo/migration"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	gomongo "go.mongodb.org/mongo-driver/v2/mongo"
)

const testDatabaseName = "testDB"

// fakeMigration implements MigrationInterface.
type fakeMigration struct{}

func (fm fakeMigration) Up(_ mongo.MongoDatabase) error {
	return nil
}

func (fm fakeMigration) Down(_ mongo.MongoDatabase) error {
	return nil
}

func (fm fakeMigration) Name() string {
	return "fake"
}

func (fm fakeMigration) Version() *version.Version {
	return version.Must(version.NewSemver("0.0.0"))
}

// fakeMigrationRepository implements MigrationRepository.
type fakeMigrationRepository struct {
	migs []migration.MigrationInterface
}

func (fmr *fakeMigrationRepository) LoadMigrations() []migration.MigrationInterface {
	return fmr.migs
}

// fakeNodeFactory implements NodeFactory.
type fakeNodeFactory struct {
	UseTail      bool
	ThreeApplied bool
}

// CreateMigrationNodes creates a chain of 2 nodes.
// If any migration models exist, use the first as node1's model; otherwise, create a new one.
func (fnf fakeNodeFactory) ParseAndSortMigrations(migrations []migration.MigrationInterface, allMigrationModels []migration.MigrationsModel) ([]migration.MigrationNode, []migration.MigrationNode, error) {
	var base migration.MigrationsModel
	if len(allMigrationModels) > 0 {
		base = allMigrationModels[0]
	} else {
		base = migration.MigrationsModel{
			BaseModel: mongo.BaseModel{ID: "1"},
			Name:      "base",
			IsCurrent: true,
			Version:   "0.0.0",
		}
	}
	// Node1 keyed as "1"
	node1 := migration.MigrationNode{
		Model:     base,
		Migration: migrations[0],
	}
	// Node2 keyed as "2"
	node2Model := migration.MigrationsModel{
		BaseModel: mongo.BaseModel{ID: "2"},
		Name:      "migration1",
		IsCurrent: false,
		Version:   "0.0.1",
	}
	node2 := migration.MigrationNode{
		Model:     node2Model,
		Migration: migrations[0],
	}
	// Node3 keyed as "3"
	node3Model := migration.MigrationsModel{
		BaseModel: mongo.BaseModel{ID: "3"},
		Name:      "migration2",
		IsCurrent: false,
		Version:   "0.1.0",
	}
	node3 := migration.MigrationNode{
		Model:     node3Model,
		Migration: migrations[0],
	}

	appliedMigrations := []migration.MigrationNode{node1}
	remainingMigrations := []migration.MigrationNode{node2, node3}

	if fnf.ThreeApplied {
		appliedMigrations = append(appliedMigrations, node2, node3)
		remainingMigrations = []migration.MigrationNode{}
		return appliedMigrations, remainingMigrations, nil
	}

	if fnf.UseTail {
		appliedMigrations = append(appliedMigrations, node2)
		remainingMigrations = []migration.MigrationNode{node3}
		return appliedMigrations, remainingMigrations, nil
	}
	return appliedMigrations, remainingMigrations, nil
}

func TestRunMigrationsForward(t *testing.T) {
	client, collection := setupMongoMocks(t)

	// Mock the LoadMigrationModels call - return initial migration model
	initial := migration.MigrationsModel{
		BaseModel: mongo.BaseModel{ID: "1"},
		Name:      "base",
		IsCurrent: true,
		Version:   "0.0.0",
	}

	dbDataInterface := []interface{}{
		initial,
	}

	// Create a mock cursor that returns the initial model
	cursor, err := gomongo.NewCursorFromDocuments(dbDataInterface, nil, nil)
	require.NoError(t, err)

	// Mock collection operations
	collection.On("Find", mock.Anything, mock.Anything).Return(cursor, nil)
	collection.On("CountDocuments", mock.Anything, mock.Anything).Return(int64(0), nil)
	collection.On("InsertOne", mock.Anything, mock.Anything).Return(nil, nil)
	collection.On("UpdateOne", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	collection.On("DeleteOne", mock.Anything, mock.Anything).Return(nil, nil)

	// Use a fake migration repository with one fake migration.
	repo := &fakeMigrationRepository{
		migs: []migration.MigrationInterface{fakeMigration{}},
	}
	// For forward migration, return head node (node1) so that forward moves to node2.
	nodeFactory := &fakeNodeFactory{UseTail: false}

	manager := migration.NewMigrationManager(repo, nodeFactory, client, testDatabaseName)

	// Execute forward migration with 1 step.
	if err := manager.RunMigrations(context.Background(), 1); err != nil {
		t.Fatalf("RunMigrations (forward) failed: %v", err)
	}

	// Verify that InsertOne was called for the new migration (node2)
	collection.AssertCalled(t, "InsertOne", mock.Anything, mock.MatchedBy(func(model migration.MigrationsModel) bool {
		return model.ID == "2" && model.Name == "migration1"
	}))

	// Verify that UpdateOne was called to update the previous node to not current
	collection.AssertCalled(t, "UpdateOne", mock.Anything, map[string]interface{}{"_id": "1"}, map[string]interface{}{"$set": map[string]interface{}{"isCurrent": false, "status": migration.CompletedMigrationStatus}})

	// Verify that UpdateOne was called to update the current node to current
	collection.AssertCalled(t, "UpdateOne", mock.Anything, map[string]interface{}{"_id": "2"}, map[string]interface{}{"$set": map[string]interface{}{"isCurrent": true, "status": migration.CompletedMigrationStatus}})
}

func TestRunMigrationsForwardApplyTwo(t *testing.T) {
	client, collection := setupMongoMocks(t)

	// Mock the LoadMigrationModels call - return initial migration model
	initial := migration.MigrationsModel{
		BaseModel: mongo.BaseModel{ID: "1"},
		Name:      "base",
		IsCurrent: true,
		Version:   "0.0.0",
	}

	dbDataInterface := []interface{}{
		initial,
	}

	// Create a mock cursor that returns the initial model
	cursor, err := gomongo.NewCursorFromDocuments(dbDataInterface, nil, nil)
	require.NoError(t, err)

	// Mock collection operations
	collection.On("Find", mock.Anything, mock.Anything).Return(cursor, nil)
	collection.On("CountDocuments", mock.Anything, mock.Anything).Return(int64(0), nil)
	collection.On("InsertOne", mock.Anything, mock.Anything).Return(nil, nil)
	collection.On("UpdateOne", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	collection.On("DeleteOne", mock.Anything, mock.Anything).Return(nil, nil)

	// Use a fake migration repository with one fake migration.
	repo := &fakeMigrationRepository{
		migs: []migration.MigrationInterface{fakeMigration{}},
	}
	// For forward migration, return head node (node1) so that forward moves to node2.
	nodeFactory := &fakeNodeFactory{UseTail: false}

	manager := migration.NewMigrationManager(repo, nodeFactory, client, testDatabaseName)

	// Execute forward migration with 2 steps.
	if err := manager.RunMigrations(context.Background(), 2); err != nil {
		t.Fatalf("RunMigrations (forward) failed: %v", err)
	}

	// Verify that InsertOne was called twice for the new migrations (node2 and node3)
	collection.AssertNumberOfCalls(t, "InsertOne", 3) // 1 for SetMigrationToRunning + 2 for migrations

	// Verify that UpdateOne was called multiple times for updating current status
	// Each migration step involves 2 UpdateOne calls (setting previous to false, current to true)
	// So 2 steps = 4 UpdateOne calls
	collection.AssertNumberOfCalls(t, "UpdateOne", 4)

	// Verify DeleteOne was called once to remove the running migration
	collection.AssertCalled(t, "DeleteOne", mock.Anything, map[string]interface{}{
		"status":  migration.RunningMigrationStatus,
		"name":    migration.TemporaryMigrationName,
		"_id":     migration.TemporaryMigrationName,
		"deleted": false,
	})
}

func TestRunMigrationsBackward(t *testing.T) {
	client, collection := setupMongoMocks(t)

	// Mock LoadMigrationModels to return both node1 and node2 (simulating applied migrations)
	initial := migration.MigrationsModel{
		BaseModel: mongo.BaseModel{ID: "1"},
		Name:      "base",
		IsCurrent: false,
		Version:   "0.0.0",
	}
	node2 := migration.MigrationsModel{
		BaseModel: mongo.BaseModel{ID: "2"},
		Name:      "migration1",
		IsCurrent: true,
		Version:   "0.0.1",
	}

	dbDataInterface := []interface{}{initial, node2}

	// Create a mock cursor that returns both models
	cursor, err := gomongo.NewCursorFromDocuments(dbDataInterface, nil, nil)
	require.NoError(t, err)

	// Mock collection operations
	collection.On("Find", mock.Anything, map[string]interface{}{}).Return(cursor, nil)
	collection.On("CountDocuments", mock.Anything, map[string]interface{}{"_id": migration.TemporaryMigrationName, "deleted": false, "name": migration.TemporaryMigrationName, "status": migration.RunningMigrationStatus}).Return(int64(0), nil)
	collection.On("InsertOne", mock.Anything, mock.Anything).Return(nil, nil)
	collection.On("UpdateOne", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	collection.On("DeleteOne", mock.Anything, mock.Anything).Return(nil, nil)

	// Use a fake migration repository.
	repo := &fakeMigrationRepository{
		migs: []migration.MigrationInterface{fakeMigration{}},
	}
	// For backward migration, have the NodeFactory return tail node (node2).
	nodeFactory := &fakeNodeFactory{UseTail: true}
	manager := migration.NewMigrationManager(repo, nodeFactory, client, testDatabaseName)

	// Execute backward migration with 1 step.
	if err := manager.RunMigrations(context.Background(), -1); err != nil {
		t.Fatalf("RunMigrations (backward) failed: %v", err)
	}

	// Verify that DeleteOne was called to delete node2
	collection.AssertCalled(t, "DeleteOne", mock.Anything, map[string]interface{}{"_id": "2"})

	// Verify that UpdateOne was called to set node1 as current
	collection.AssertCalled(t, "UpdateOne", mock.Anything, map[string]interface{}{"_id": "1"}, map[string]interface{}{"$set": map[string]interface{}{"isCurrent": true, "status": migration.CompletedMigrationStatus}})

	// Verify DeleteOne was called to remove the running migration
	collection.AssertCalled(t, "DeleteOne", mock.Anything, map[string]interface{}{
		"status":  migration.RunningMigrationStatus,
		"name":    migration.TemporaryMigrationName,
		"_id":     migration.TemporaryMigrationName,
		"deleted": false,
	})
}

func TestRunMigrationsBackwardApply2(t *testing.T) {
	client, collection := setupMongoMocks(t)

	// Mock LoadMigrationModels to return all three nodes (simulating all migrations applied)
	initial := migration.MigrationsModel{
		BaseModel: mongo.BaseModel{ID: "1"},
		Name:      "base",
		IsCurrent: false,
		Version:   "0.0.0",
	}
	node2 := migration.MigrationsModel{
		BaseModel: mongo.BaseModel{ID: "2"},
		Name:      "migration1",
		IsCurrent: false,
		Version:   "0.0.1",
	}
	node3 := migration.MigrationsModel{
		BaseModel: mongo.BaseModel{ID: "3"},
		Name:      "migration2",
		IsCurrent: true,
		Version:   "0.1.0",
	}

	dbDataInterface := []interface{}{initial, node2, node3}

	// Create a mock cursor that returns all models
	cursor, err := gomongo.NewCursorFromDocuments(dbDataInterface, nil, nil)
	require.NoError(t, err)

	// Mock collection operations
	collection.On("Find", mock.Anything, map[string]interface{}{}).Return(cursor, nil)
	collection.On("CountDocuments", mock.Anything, map[string]interface{}{
		"_id":     migration.TemporaryMigrationName,
		"name":    migration.TemporaryMigrationName,
		"status":  migration.RunningMigrationStatus,
		"deleted": false,
	}).Return(int64(0), nil)
	collection.On("InsertOne", mock.Anything, mock.Anything).Return(nil, nil)
	collection.On("UpdateOne", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	collection.On("DeleteOne", mock.Anything, mock.Anything).Return(nil, nil)

	// Use a fake migration repository.
	repo := &fakeMigrationRepository{
		migs: []migration.MigrationInterface{fakeMigration{}},
	}
	// For backward migration, have the NodeFactory return all three applied.
	nodeFactory := &fakeNodeFactory{ThreeApplied: true}
	manager := migration.NewMigrationManager(repo, nodeFactory, client, testDatabaseName)

	// Execute backward migration with 2 steps.
	if err := manager.RunMigrations(context.Background(), -2); err != nil {
		t.Fatalf("RunMigrations (backward) failed: %v", err)
	}

	// Verify that DeleteOne was called twice (for node3 and node2)
	collection.AssertCalled(t, "DeleteOne", mock.Anything, map[string]interface{}{"_id": "3"})
	collection.AssertCalled(t, "DeleteOne", mock.Anything, map[string]interface{}{"_id": "2"})

	// Verify that UpdateOne was called to set node1 as current after the final step
	collection.AssertCalled(t, "UpdateOne", mock.Anything, map[string]interface{}{"_id": "1"}, map[string]interface{}{"$set": map[string]interface{}{"isCurrent": true, "status": migration.CompletedMigrationStatus}})

	// Verify DeleteOne was called to remove the running migration
	collection.AssertCalled(t, "DeleteOne", mock.Anything, map[string]interface{}{
		"status":  migration.RunningMigrationStatus,
		"name":    migration.TemporaryMigrationName,
		"_id":     migration.TemporaryMigrationName,
		"deleted": false,
	})
}

func TestRunMigrationsNoSteps(t *testing.T) {
	client, collection := setupMongoMocks(t)

	// Mock LoadMigrationModels to return initial migration model
	initial := migration.MigrationsModel{
		BaseModel: mongo.BaseModel{ID: "1"},
		Name:      "base",
		IsCurrent: true,
		Version:   "0.0.0",
	}

	dbDataInterface := []interface{}{initial}

	// Create a mock cursor that returns the initial model
	cursor, err := gomongo.NewCursorFromDocuments(dbDataInterface, nil, nil)
	require.NoError(t, err)

	// Mock collection operations
	collection.On("Find", mock.Anything, mock.Anything).Return(cursor, nil)
	collection.On("CountDocuments", mock.Anything, mock.Anything).Return(int64(0), nil)
	collection.On("InsertOne", mock.Anything, mock.Anything).Return(nil, nil)
	collection.On("DeleteOne", mock.Anything, map[string]interface{}{
		"_id":     migration.TemporaryMigrationName,
		"deleted": false,
		"name":    migration.TemporaryMigrationName,
		"status":  migration.RunningMigrationStatus,
	}).Return(nil, nil)

	// Use fake migration repository and node factory.
	repo := &fakeMigrationRepository{
		migs: []migration.MigrationInterface{fakeMigration{}},
	}
	nodeFactory := &fakeNodeFactory{UseTail: false}
	manager := migration.NewMigrationManager(repo, nodeFactory, client, testDatabaseName)

	// Execute with 0 steps, expecting no change.
	if err := manager.RunMigrations(context.Background(), 0); err != nil {
		t.Fatalf("RunMigrations with 0 steps failed: %v", err)
	}

	// Verify that only SetMigrationToRunning and RemoveMigrationFromRunning were called
	// No migration steps should be performed
	collection.AssertCalled(t, "InsertOne", mock.Anything, mock.MatchedBy(func(model migration.MigrationsModel) bool {
		return model.ID == migration.TemporaryMigrationName // SetMigrationToRunning call
	}))
	collection.AssertCalled(t, "DeleteOne", mock.Anything, map[string]interface{}{
		"_id":     migration.TemporaryMigrationName,
		"deleted": false,
		"name":    migration.TemporaryMigrationName,
		"status":  migration.RunningMigrationStatus,
	})

	// Should only have 1 InsertOne call (for SetMigrationToRunning)
	collection.AssertNumberOfCalls(t, "InsertOne", 1)
}

func TestMultipleMigrationsWontRun(t *testing.T) {
	client, collection := setupMongoMocks(t)

	// CountDocuments should return 1 to indicate that current migration is running
	// This will cause the migration to fail early without calling Find
	collection.On("CountDocuments", mock.Anything, mock.Anything).Return(int64(1), nil)

	// Use fake migration repository and node factory.
	repo := &fakeMigrationRepository{
		migs: []migration.MigrationInterface{fakeMigration{}},
	}
	nodeFactory := &fakeNodeFactory{UseTail: false}
	manager := migration.NewMigrationManager(repo, nodeFactory, client, testDatabaseName)

	// Attempt to run migrations while another migration is running.
	err := manager.RunMigrations(context.Background(), 1)
	if err == nil {
		t.Fatalf("expected error when migration is already running, got nil")
	}
	if err.Error() != "migration is already running" {
		t.Fatalf("expected 'migration is already running' error, got: %v", err)
	}

	// Verify that CountDocuments was called to check if migration is running
	collection.AssertCalled(t, "CountDocuments", mock.Anything, map[string]interface{}{
		"_id":     migration.TemporaryMigrationName,
		"deleted": false,
		"name":    migration.TemporaryMigrationName,
		"status":  migration.RunningMigrationStatus,
	})
}

func TestEnsureIsRunningModelDeletedCompletelyAfterMigration(t *testing.T) {
	client, collection := setupMongoMocks(t)

	// Mock LoadMigrationModels to return initial migration model
	initial := migration.MigrationsModel{
		BaseModel: mongo.BaseModel{ID: "1"},
		Name:      "base",
		IsCurrent: true,
		Version:   "0.0.0",
	}

	dbDataInterface := []interface{}{initial}

	// Create a mock cursor that returns the initial model
	cursor, err := gomongo.NewCursorFromDocuments(dbDataInterface, nil, nil)
	require.NoError(t, err)

	// Mock collection operations
	collection.On("Find", mock.Anything, mock.Anything).Return(cursor, nil)
	collection.On("CountDocuments", mock.Anything, mock.Anything).Return(int64(0), nil)
	collection.On("InsertOne", mock.Anything, mock.Anything).Return(nil, nil)
	collection.On("DeleteOne", mock.Anything, mock.Anything).Return(nil, nil)

	// Use fake migration repository and node factory so that
	// RunMigrations executes without applying any real migration steps.
	repo := &fakeMigrationRepository{
		migs: []migration.MigrationInterface{fakeMigration{}},
	}
	nodeFactory := &fakeNodeFactory{UseTail: false}
	manager := migration.NewMigrationManager(repo, nodeFactory, client, testDatabaseName)

	// Run migration with 0 steps.
	// This will cause the migration to be set to running and then removed.
	if err := manager.RunMigrations(context.Background(), 0); err != nil {
		t.Fatalf("RunMigrations with 0 steps failed: %v", err)
	}

	// Verify that the running migration model (with ID equal to service testDatabaseName)
	// was created (SetMigrationToRunning) and then deleted (RemoveMigrationFromRunning)
	collection.AssertCalled(t, "InsertOne", mock.Anything, mock.MatchedBy(func(model migration.MigrationsModel) bool {
		return model.ID == migration.TemporaryMigrationName // SetMigrationToRunning call
	}))
	collection.AssertCalled(t, "DeleteOne", mock.Anything, map[string]interface{}{
		"_id":     migration.TemporaryMigrationName,
		"deleted": false,
		"name":    migration.TemporaryMigrationName,
		"status":  migration.RunningMigrationStatus,
	})
}

func setupMongoMocks(t *testing.T) (*mongo.MockMongoClient, *mongo.MockMongoCollection) {
	client := mongo.NewMockMongoClient(t)
	db := mongo.NewMockMongoDatabase(t)
	collection := mongo.NewMockMongoCollection(t)

	client.On("Database", testDatabaseName).Return(db)
	db.On("Collection", "Migrations").Return(collection)

	return client, collection
}
