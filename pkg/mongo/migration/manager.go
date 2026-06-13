package migration

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/rs/zerolog/log"
	"github.com/mikelear/leartech-bus-common/pkg/mongo"
)

// TemporaryMigrationName is a migration inserted to tell other services that a migration is running
const TemporaryMigrationName = "InProgressMigration"

type MigrationManager struct {
	MigrationRepo       MigrationRepositoryInterface
	NodeFactory         NodeFactoryInterface
	mongoDB             mongo.MongoDatabase
	migrationCollection mongo.MongoCollection
}

func NewMigrationManager(mr MigrationRepositoryInterface, nf NodeFactoryInterface, client mongo.MongoClient, dbName string) *MigrationManager {
	db := client.Database(dbName)
	collection := db.Collection("Migrations")
	return &MigrationManager{
		MigrationRepo:       mr,
		NodeFactory:         nf,
		migrationCollection: collection,
		mongoDB:             db,
	}
}

// RunMigrations runs all the migrations registered in the manager
func (mm *MigrationManager) RunMigrations(ctx context.Context, steps int) error {
	// avoid running multiple migrations at the same time across multiple services
	isRunning, err := mm.IsMigrationRunning(ctx)
	if err != nil {
		return fmt.Errorf("failed to check if migration is running: %w", err)
	}
	if isRunning {
		return errors.New("migration is already running")
	}

	allMigrationModels, err := mm.LoadMigrationModels(ctx)
	if err != nil {
		return err
	}

	// Load the migrations
	migrations := mm.MigrationRepo.LoadMigrations()
	appliedMigrations, remainingMigrations, err := mm.NodeFactory.ParseAndSortMigrations(migrations, allMigrationModels)
	if err != nil {
		return fmt.Errorf("failed to parse and sort migrations: %w", err)
	}

	err = mm.SetMigrationToRunning(ctx)
	if err != nil {
		return fmt.Errorf("failed to set migration to running: %w", err)
	}

	// Apply the migrations
	if steps > 0 {
		err = mm.ApplyMigrationForward(ctx, appliedMigrations, remainingMigrations, steps)
		if err != nil {
			return fmt.Errorf("failed to apply migration forward: %w", err)
		}
	} else if steps < 0 {
		err = mm.ApplyMigrationBackward(ctx, appliedMigrations, -steps)
		if err != nil {
			return fmt.Errorf("failed to apply migration backward: %w", err)
		}
	}

	err = mm.RemoveMigrationFromRunning(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to remove migration from running")
	}

	return err
}

// ApplyMigrationForward applies the migration forward by n steps
func (mm *MigrationManager) ApplyMigrationForward(ctx context.Context, appliedMigrations []MigrationNode, remainingMigrations []MigrationNode, steps int) error {
	currentNode := appliedMigrations[len(appliedMigrations)-1]

	// check we only do the number of steps that are available
	steps = min(steps, len(remainingMigrations))
	for _, nextNode := range remainingMigrations[:steps] {
		prevNode := currentNode
		currentNode = nextNode

		// Insert the new migration model
		_, err := mm.migrationCollection.InsertOne(ctx, currentNode.Model)
		if err != nil {
			return fmt.Errorf("failed to save new migration model. aborting migration: %w", err)
		}

		// Apply the migration
		err = currentNode.Migration.Up(mm.mongoDB)
		if err != nil {
			return fmt.Errorf("failed to apply migration forward: %w", err)
		}

		// Update previous node to not be current
		prevNode.Model.IsCurrent = false
		filter := map[string]interface{}{"_id": prevNode.Model.ID}
		update := map[string]interface{}{"$set": map[string]interface{}{
			"isCurrent": false,
			"status":    CompletedMigrationStatus,
		}}
		_, err = mm.migrationCollection.UpdateOne(ctx, filter, update)
		if err != nil {
			return fmt.Errorf("failed to set IsCurrent to false: %w", err)
		}

		// Update current node to be current
		currentNode.Model.IsCurrent = true
		filter = map[string]interface{}{"_id": currentNode.Model.ID}
		update = map[string]interface{}{"$set": map[string]interface{}{
			"isCurrent": true,
			"status":    CompletedMigrationStatus,
		}}
		_, err = mm.migrationCollection.UpdateOne(ctx, filter, update)
		if err != nil {
			return fmt.Errorf("failed to set IsCurrent to true: %w", err)
		}
	}

	return nil
}

// ApplyMigrationBackward applies the migration backward by n steps
func (mm *MigrationManager) ApplyMigrationBackward(ctx context.Context, appliedMigrations []MigrationNode, steps int) error {
	slices.Reverse(appliedMigrations)
	currentNode := appliedMigrations[0]

	// check we only do the number of steps that are available
	// we don't want to go below the initial state
	steps = min(steps, len(appliedMigrations)-1)
	for i, nextNode := range appliedMigrations[1 : steps+1] {
		// Apply the migration backward
		err := currentNode.Migration.Down(mm.mongoDB)
		if err != nil {
			return fmt.Errorf("failed to apply migration backward: %w", err)
		}

		// Delete the regressed migration model
		filter := map[string]interface{}{"_id": currentNode.Model.ID}
		_, err = mm.migrationCollection.DeleteOne(ctx, filter)
		if err != nil {
			return fmt.Errorf("failed to delete migration model: %w", err)
		}

		// Set the next migration as current
		currentNode = nextNode
		currentNode.Model.IsCurrent = true
		filter = map[string]interface{}{"_id": currentNode.Model.ID}
		update := map[string]interface{}{"$set": map[string]interface{}{
			"isCurrent": true,
			"status":    CompletedMigrationStatus,
		}}
		_, err = mm.migrationCollection.UpdateOne(ctx, filter, update)
		if err != nil {
			return fmt.Errorf("failed to apply migration backward for step %d: %w", i+1, err)
		}
	}

	return nil
}

// LoadMigrationModels loads all the migration models from MongoDB
func (mm *MigrationManager) LoadMigrationModels(ctx context.Context) ([]MigrationsModel, error) {
	var result []MigrationsModel

	filter := map[string]interface{}{}

	cursor, err := mm.migrationCollection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}

	defer func(cursor mongo.MongoCursor, ctx context.Context) {
		err := cursor.Close(ctx)
		if err != nil {
			log.Error().Err(err).Msg("failed to close cursor")
			return
		}
	}(cursor, ctx)

	err = cursor.All(ctx, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// IsMigrationRunning checks if the migration is running
func (mm *MigrationManager) IsMigrationRunning(ctx context.Context) (bool, error) {
	filter := map[string]interface{}{
		"status":  RunningMigrationStatus,
		"name":    TemporaryMigrationName,
		"_id":     TemporaryMigrationName,
		"deleted": false,
	}

	runningMigrationCount, err := mm.migrationCollection.CountDocuments(ctx, filter)
	if err != nil {
		return false, err
	}

	return runningMigrationCount > 0, nil
}

// SetMigrationToRunning sets the migration to running
func (mm *MigrationManager) SetMigrationToRunning(ctx context.Context) error {
	model := MigrationsModel{
		BaseModel: mongo.BaseModel{
			ID:      TemporaryMigrationName,
			Deleted: false,
		},
		Name:   TemporaryMigrationName,
		Status: RunningMigrationStatus,
	}
	model.UpdateTimestamps()

	_, err := mm.migrationCollection.InsertOne(ctx, model)
	if err != nil {
		return err
	}

	return nil
}

// RemoveMigrationFromRunning removes the migration from running
func (mm *MigrationManager) RemoveMigrationFromRunning(ctx context.Context) error {
	filter := map[string]interface{}{
		"status":  RunningMigrationStatus,
		"name":    TemporaryMigrationName,
		"_id":     TemporaryMigrationName,
		"deleted": false,
	}

	_, err := mm.migrationCollection.DeleteOne(ctx, filter)
	if err != nil {
		return err
	}

	return nil
}
