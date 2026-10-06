// Package rockhopper provides database migration tools for Go applications.
package rockhopper

import (
	"context"

	log "github.com/sirupsen/logrus"
)

// Align moves the database to versionID by applying or rolling back migrations.
func Align(ctx context.Context, db *DB, versionID int64, migrations MigrationSlice) error {
	_, lastAppliedMigration, err := db.FindLastAppliedMigration(ctx, migrations)
	if err != nil {
		return err
	}

	if lastAppliedMigration == nil {
		return Up(ctx, db, migrations.Head(), versionID)
	}

	if versionID < lastAppliedMigration.Version {
		return Down(ctx, db, lastAppliedMigration, versionID)
	}

	if versionID > lastAppliedMigration.Version {
		return Up(ctx, db, lastAppliedMigration.Next, versionID)
	}

	log.Infof("the migration version is already aligned to %d", versionID)
	return nil
}
