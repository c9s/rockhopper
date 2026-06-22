package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/rockhopper/v2"
)

func TestDefaultMigrationsDir(t *testing.T) {
	t.Run("no directory configured falls back to migrations", func(t *testing.T) {
		assert.Equal(t, "migrations", defaultMigrationsDir(&rockhopper.Config{}))
	})

	t.Run("legacy migrationsDir is honored", func(t *testing.T) {
		// LoadConfig migrates a non-empty MigrationsDir into MigrationsDirs, so a
		// legacy goose-style config arrives here as a one-element list.
		assert.Equal(t, "migrations/sqlite3", defaultMigrationsDir(&rockhopper.Config{
			MigrationsDirs: []string{"migrations/sqlite3"},
		}))
	})

	t.Run("first of multiple migrationsDirs is used", func(t *testing.T) {
		assert.Equal(t, "migrations/mysql/app1", defaultMigrationsDir(&rockhopper.Config{
			MigrationsDirs: []string{"migrations/mysql/app1", "migrations/mysql/app2"},
		}))
	})
}
