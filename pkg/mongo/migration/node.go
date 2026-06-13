package migration

import (
	"errors"
	"fmt"
	"sort"

	"github.com/google/uuid"
	"github.com/hashicorp/go-version"
	"github.com/mikelear/leartech-bus-common/pkg/mongo"
)

const (
	BaseMigrationName    = "base"
	BaseMigrationVersion = "0.0.0"
)

var (
	BaseMigration = dummyMigration{name: BaseMigrationName, version: version.Must(version.NewVersion(BaseMigrationVersion))}
)

// NodeFactory is a factory for creating migration nodes
type NodeFactory struct{}

func NewNodeFactory() NodeFactory {
	return NodeFactory{}
}

// ParseAndSortMigrations creates an ordered list of migration nodes
func (nf NodeFactory) ParseAndSortMigrations(migrations []MigrationInterface, allMigrationModels []MigrationsModel) ([]MigrationNode, []MigrationNode, error) {
	var migrationNodes []MigrationNode
	var remainingNodes []MigrationNode

	migrationsMap := make(map[string]MigrationInterface)
	for _, m := range migrations {
		migrationsMap[MigrationKey(m)] = m
	}

	// Create a map of migration models by key
	for _, mm := range allMigrationModels {
		// skip the base migration model
		if mm.Name == BaseMigrationName {
			node := MigrationNode{Model: mm, Migration: &BaseMigration}
			migrationNodes = append(migrationNodes, node)
			continue
		}
		migration, ok := migrationsMap[MigrationKeyFromModel(mm)]
		if !ok {
			return nil, nil, fmt.Errorf("migration not found for migration registered in DB %s", mm.Name)
		}

		node := NewMigrationNode(mm, migration)
		migrationNodes = append(migrationNodes, node)
		delete(migrationsMap, MigrationKeyFromModel(mm))
	}

	// sort the migration nodes by version in ascending order
	SortNodes(migrationNodes)

	if len(migrationNodes) > 0 && !migrationNodes[len(migrationNodes)-1].Model.IsCurrent {
		return nil, nil, errors.New("last migration node is not current")
	}

	if len(migrationNodes) == 0 {
		// if the current migration node is not set, set it to the first migration node
		initialNode := MigrationNode{
			Model: MigrationsModel{
				BaseModel: mongo.BaseModel{ID: uuid.New().String()},
				Name:      BaseMigrationName,
				IsCurrent: true,
				Version:   BaseMigrationVersion,
				Status:    CompletedMigrationStatus,
			},
			Migration: &BaseMigration,
		}
		migrationNodes = append(migrationNodes, initialNode)
	}

	for _, m := range migrationsMap {
		node := NewMigrationNode(CreateMigrationModel(m), m)
		remainingNodes = append(remainingNodes, node)
	}

	// sort the remaining migrations by added date in ascending order
	SortNodes(remainingNodes)

	// validate that the next migration is newer than the current migration
	if len(remainingNodes) > 0 {
		currentNode := migrationNodes[len(migrationNodes)-1]
		nextNode := remainingNodes[0]
		if currentNode.Migration.Version().GreaterThan(nextNode.Migration.Version()) {
			return nil, nil, errors.New("next migration node is older than the current migration node")
		}
	}

	return migrationNodes, remainingNodes, nil
}

// SortNodes sorts the migration nodes by version in ascending order
func SortNodes(nodes []MigrationNode) {
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Migration.Version().LessThan(nodes[j].Migration.Version())
	})
}

// CreateMigrationModel creates a new migration model from a migration
func CreateMigrationModel(m MigrationInterface) MigrationsModel {
	model := MigrationsModel{
		BaseModel: mongo.BaseModel{
			ID:      uuid.New().String(),
			Deleted: false,
		},
		Name:    m.Name(),
		Status:  RunningMigrationStatus,
		Version: m.Version().String(),
	}
	model.UpdateTimestamps()
	return model
}

func MigrationKey(m MigrationInterface) string {
	// in the format isodate_name
	return fmt.Sprintf("%s_%s", m.Version().String(), m.Name())
}

func MigrationKeyFromModel(m MigrationsModel) string {
	// in the format isodate_name
	return fmt.Sprintf("%s_%s", m.Version, m.Name)
}
