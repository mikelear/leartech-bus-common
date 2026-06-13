package migration

import "github.com/mikelear/leartech-bus-common/pkg/mongo"

type MigrationStatus string

const (
	CompletedMigrationStatus MigrationStatus = "Completed"
	RunningMigrationStatus   MigrationStatus = "Running"
)

// MigrationsModel represents the database model for a migration
type MigrationsModel struct {
	mongo.BaseModel `bson:",inline"`
	Name            string          `bson:"name"`      // The name of the migration
	Status          MigrationStatus `bson:"status"`    // The status of the migration
	IsCurrent       bool            `bson:"isCurrent"` // Is the configuration current
	Version         string          `bson:"version"`   // The version of the migration
}

func (MigrationsModel) CollectionName() string {
	return "Migrations"
}

type MigrationNode struct {
	Model     MigrationsModel
	Migration MigrationInterface
}

func NewMigrationNode(model MigrationsModel, migration MigrationInterface) MigrationNode {
	return MigrationNode{
		Model:     model,
		Migration: migration,
	}
}
